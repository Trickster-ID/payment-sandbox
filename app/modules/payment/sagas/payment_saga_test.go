// Branch maps:
//
// ValidatePaymentStep.Execute:
// ├── DB query fails                  -> "payment intent not found"          [Case 1]
// ├── status != PENDING               -> "payment intent already finalized"  [Case 2]
// └── success                         -> nil + state populated               [Case 3]
//
// ValidatePaymentStep.Compensate:
// └── always nil (read-only step)                                            [Case 1]
//
// PostLedgerStep.Execute:
// ├── invalid merchant_id             -> "invalid merchant id"               [Case 1]
// ├── GetAccountByMerchantID fails    -> "merchant ledger account not found" [Case 2]
// ├── amount=0 -> ValidatePosting err -> "entry amount must be positive"     [Case 3]
// ├── LedgerRepo.Post fails           -> error                               [Case 4]
// ├── UPDATE merchants fails          -> error                               [Case 5]
// └── success                         -> nil + state populated               [Case 6]
//
// PostLedgerStep.Compensate:
// ├── ledger_ref empty                -> nil (no-op)                         [Case 1]
// └── success                         -> nil                                 [Case 2]
//
// MarkPaymentSuccessStep.Execute:
// ├── BeginTx fails                   -> error                               [Case 1]
// ├── UPDATE payment_intents fails    -> error                               [Case 2]
// ├── UPDATE invoices fails           -> error                               [Case 3]
// └── success                         -> nil                                 [Case 4]
//
// MarkPaymentSuccessStep.Compensate:
// └── always nil                                                             [Case 1]
package sagas_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerRepo "payment-sandbox/app/modules/ledger/repositories"
	"payment-sandbox/app/modules/payment/sagas"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testSagaMerchantID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	testSagaWalletID   = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
)

// fakeLedger is a hand-rolled stub for ledgerRepo.IRepository.
// It avoids testify's mock framework (and thus avoids the data race between
// database/sql's internal awaitDone goroutine and testify's reflect-based
// AssertExpectations that reads *sql.Tx fields non-atomically).
type fakeLedger struct {
	getAccountResult ledgerEntity.Account
	getAccountErr    error
	postErr          error
	reverseErr       error
	getAccountCalled bool
	postCalled       bool
	reverseCalled    bool
}

func (f *fakeLedger) GetAccountByMerchantID(_ context.Context, _ uuid.UUID) (ledgerEntity.Account, error) {
	f.getAccountCalled = true
	return f.getAccountResult, f.getAccountErr
}
func (f *fakeLedger) Post(_ context.Context, _ *sql.Tx, _ ledgerEntity.Posting) (uuid.UUID, error) {
	f.postCalled = true
	return uuid.Nil, f.postErr
}
func (f *fakeLedger) Reverse(_ context.Context, _ *sql.Tx, _, _ string, _ uuid.UUID) (uuid.UUID, error) {
	f.reverseCalled = true
	return uuid.Nil, f.reverseErr
}
func (f *fakeLedger) ListEntriesByAccount(_ context.Context, _ uuid.UUID, _ ledgerEntity.EntryFilter, _, _ int) ([]ledgerEntity.EntryWithTxn, int, error) {
	return nil, 0, nil
}

var _ ledgerRepo.IRepository = (*fakeLedger)(nil) // compile-time interface check

// ── ValidatePaymentStep ────────────────────────────────────────────────────

func TestValidatePaymentStep_Execute(t *testing.T) {
	type args struct {
		state map[string]any
	}
	type mocks struct {
		setupDB func(dbMock sqlmock.Sqlmock)
	}
	type wants struct {
		errMsg    string
		invoiceID string
		amount    int64
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. DB query fails -> payment intent not found error",
			args: args{state: map[string]any{"payment_id": "pi-1"}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.invoice_id::text")).
						WithArgs("pi-1").
						WillReturnError(errors.New("db error"))
				},
			},
			wants: wants{errMsg: "payment intent not found"},
		},
		{
			name: "2. payment status is SUCCESS -> payment intent already finalized error",
			args: args{state: map[string]any{"payment_id": "pi-1"}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.invoice_id::text")).
						WithArgs("pi-1").
						WillReturnRows(sqlmock.NewRows([]string{"invoice_id", "merchant_id", "amount", "status"}).
							AddRow("inv-1", testSagaMerchantID.String(), int64(500), "SUCCESS"))
				},
			},
			wants: wants{errMsg: "payment intent already finalized"},
		},
		{
			name: "3. valid PENDING payment -> success and state populated",
			args: args{state: map[string]any{"payment_id": "pi-1"}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.invoice_id::text")).
						WithArgs("pi-1").
						WillReturnRows(sqlmock.NewRows([]string{"invoice_id", "merchant_id", "amount", "status"}).
							AddRow("inv-1", testSagaMerchantID.String(), int64(500), "PENDING"))
				},
			},
			wants: wants{invoiceID: "inv-1", amount: 500},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock is not goroutine-safe

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.mocks.setupDB(dbMock)

			state := copyState(tt.args.state)
			step := &sagas.ValidatePaymentStep{DB: db}
			err = step.Execute(context.Background(), state)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.invoiceID, state["invoice_id"], "state invoice_id")
				assert.Equal(t, tt.wants.amount, state["amount"], "state amount")
				assert.Equal(t, testSagaMerchantID.String(), state["merchant_id"], "state merchant_id")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "sqlmock expectations")
		})
	}
}

func TestValidatePaymentStep_Compensate(t *testing.T) {
	// not parallel: consistent with other saga tests
	step := &sagas.ValidatePaymentStep{DB: nil}
	err := step.Compensate(context.Background(), map[string]any{})
	assert.NoError(t, err, "Compensate should always return nil (read-only step)")
}

// ── PostLedgerStep ─────────────────────────────────────────────────────────

func TestPostLedgerStep_Execute(t *testing.T) {
	type args struct {
		state map[string]any
	}
	type mocks struct {
		setupDB    func(dbMock sqlmock.Sqlmock)
		ledger     *fakeLedger
	}
	type wants struct {
		errMsg       string
		ledgerRef    string
		postCalled   bool
	}

	validState := map[string]any{
		"payment_id":  "pi-1",
		"merchant_id": testSagaMerchantID.String(),
		"amount":      int64(1000),
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. invalid merchant_id UUID -> invalid merchant id error",
			args: args{state: map[string]any{
				"payment_id":  "pi-1",
				"merchant_id": "not-a-uuid",
				"amount":      int64(100),
			}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {},
				ledger:  nil,
			},
			wants: wants{errMsg: "invalid merchant id"},
		},
		{
			name: "2. GetAccountByMerchantID fails -> merchant ledger account not found error",
			args: args{state: copyState(validState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectRollback()
				},
				ledger: &fakeLedger{
					getAccountErr: errors.New("account not found"),
				},
			},
			wants: wants{errMsg: "merchant ledger account not found"},
		},
		{
			name: "3. amount zero -> ValidatePosting returns entry amount must be positive",
			args: args{state: map[string]any{
				"payment_id":  "pi-1",
				"merchant_id": testSagaMerchantID.String(),
				"amount":      int64(0),
			}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectRollback()
				},
				ledger: &fakeLedger{
					getAccountResult: ledgerEntity.Account{ID: testSagaWalletID},
				},
			},
			wants: wants{errMsg: "entry amount must be positive"},
		},
		{
			name: "4. LedgerRepo.Post fails -> error propagated",
			args: args{state: copyState(validState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectRollback()
				},
				ledger: &fakeLedger{
					getAccountResult: ledgerEntity.Account{ID: testSagaWalletID},
					postErr:          errors.New("ledger write error"),
				},
			},
			wants: wants{errMsg: "ledger write error", postCalled: true},
		},
		{
			name: "5. UPDATE merchants fails -> error propagated",
			args: args{state: copyState(validState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance")).
						WillReturnError(errors.New("db exec error"))
					dbMock.ExpectRollback()
				},
				ledger: &fakeLedger{
					getAccountResult: ledgerEntity.Account{ID: testSagaWalletID},
				},
			},
			wants: wants{errMsg: "db exec error", postCalled: true},
		},
		{
			name: "6. all steps succeed -> nil and state populated",
			args: args{state: copyState(validState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					dbMock.ExpectCommit()
				},
				ledger: &fakeLedger{
					getAccountResult: ledgerEntity.Account{ID: testSagaWalletID},
				},
			},
			wants: wants{ledgerRef: "payment_pi-1", postCalled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock is not goroutine-safe

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.mocks.setupDB(dbMock)

			state := copyState(tt.args.state)
			step := &sagas.PostLedgerStep{DB: db, LedgerRepo: tt.mocks.ledger}
			err = step.Execute(context.Background(), state)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.ledgerRef, state["ledger_ref"], "state ledger_ref")
				assert.Equal(t, testSagaWalletID.String(), state["wallet_acct_id"], "state wallet_acct_id")
			}

			if tt.mocks.ledger != nil {
				assert.Equal(t, tt.wants.postCalled, tt.mocks.ledger.postCalled, "Post was called")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "sqlmock expectations")
		})
	}
}

func TestPostLedgerStep_Compensate(t *testing.T) {
	type args struct {
		state map[string]any
	}
	type mocks struct {
		setupDB func(dbMock sqlmock.Sqlmock)
		ledger  *fakeLedger
	}
	type wants struct {
		errMsg        string
		reverseCalled bool
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. ledger_ref empty -> no-op returns nil",
			args: args{state: map[string]any{"payment_id": "pi-1", "ledger_ref": ""}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {},
				ledger:  nil,
			},
			wants: wants{},
		},
		{
			name: "2. valid ledger_ref -> reverse posted, payment marked FAILED",
			args: args{state: map[string]any{
				"payment_id":     "pi-1",
				"merchant_id":    testSagaMerchantID.String(),
				"ledger_ref":     "payment_pi-1",
				"wallet_acct_id": testSagaWalletID.String(),
			}},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status='FAILED'")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					dbMock.ExpectCommit()
				},
				ledger: &fakeLedger{},
			},
			wants: wants{reverseCalled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock is not goroutine-safe

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.mocks.setupDB(dbMock)

			state := copyState(tt.args.state)
			step := &sagas.PostLedgerStep{DB: db, LedgerRepo: tt.mocks.ledger}
			err = step.Compensate(context.Background(), state)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
			}

			if tt.mocks.ledger != nil {
				assert.Equal(t, tt.wants.reverseCalled, tt.mocks.ledger.reverseCalled, "Reverse was called")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "sqlmock expectations")
		})
	}
}

// ── MarkPaymentSuccessStep ─────────────────────────────────────────────────

func TestMarkPaymentSuccessStep_Execute(t *testing.T) {
	type args struct {
		state map[string]any
	}
	type mocks struct {
		setupDB func(dbMock sqlmock.Sqlmock)
	}
	type wants struct {
		errMsg string
	}

	baseState := map[string]any{
		"payment_id": "pi-1",
		"invoice_id": "inv-1",
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. BeginTx fails -> error propagated",
			args: args{state: copyState(baseState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin().WillReturnError(errors.New("connection lost"))
				},
			},
			wants: wants{errMsg: "connection lost"},
		},
		{
			name: "2. UPDATE payment_intents fails -> error propagated",
			args: args{state: copyState(baseState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status='SUCCESS'")).
						WillReturnError(errors.New("exec error"))
					dbMock.ExpectRollback()
				},
			},
			wants: wants{errMsg: "exec error"},
		},
		{
			name: "3. UPDATE invoices fails -> error propagated",
			args: args{state: copyState(baseState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status='SUCCESS'")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices SET status='PAID'")).
						WillReturnError(errors.New("invoice update error"))
					dbMock.ExpectRollback()
				},
			},
			wants: wants{errMsg: "invoice update error"},
		},
		{
			name: "4. all updates succeed -> nil",
			args: args{state: copyState(baseState)},
			mocks: mocks{
				setupDB: func(dbMock sqlmock.Sqlmock) {
					dbMock.ExpectBegin()
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status='SUCCESS'")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices SET status='PAID'")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					dbMock.ExpectCommit()
				},
			},
			wants: wants{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock is not goroutine-safe

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.mocks.setupDB(dbMock)

			state := copyState(tt.args.state)
			step := &sagas.MarkPaymentSuccessStep{DB: db}
			err = step.Execute(context.Background(), state)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "sqlmock expectations")
		})
	}
}

func TestMarkPaymentSuccessStep_Compensate(t *testing.T) {
	// not parallel: consistent with other saga tests
	step := &sagas.MarkPaymentSuccessStep{DB: nil}
	err := step.Compensate(context.Background(), map[string]any{"payment_id": "pi-1"})
	assert.NoError(t, err, "Compensate should always return nil")
}

// ── helpers ────────────────────────────────────────────────────────────────

func copyState(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
