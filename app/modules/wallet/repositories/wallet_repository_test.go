package repositories

// Branch analysis for WalletRepository:
//
// getMerchantByUserID (shared internal helper):
// ├── db.QueryRow → Scan succeeds → return merchant, true
// └── db.QueryRow → Scan fails   → return zero, false
//
// GetMerchantWallet:
// ├── getMerchantByUserID found=false → "merchant wallet not found"
// └── found=true → return merchant, nil
//
// MerchantIDByUserID:
// ├── getMerchantByUserID found=false → "merchant not found"
// └── found=true → return merchant.ID, nil
//
// CreateTopup:
// ├── db.QueryRow.Scan → error → return empty Topup, err
// └── Scan succeeds → normalizeTopupTimes, return topup, nil
//
// ListTopups:
// ├── db.Query → error → return []Topup{} (empty, no error returned)
// └── rows.Next() loop:
//     ├── rows.Scan → error → row silently skipped (no append)
//     └── rows.Scan → success → append, normalizeTopupTimes
//
// ListMerchantTopups:
// ├── db.QueryRow (count).Scan → error → return []Topup{}, 0
// ├── db.Query (data) → error → return []Topup{}, total
// └── rows.Next() loop:
//     ├── rows.Scan → error → row silently skipped
//     └── rows.Scan → success → append
//
// UpdateTopupStatus:
// ├── db.BeginTx → error → return empty Topup, err
// ├── tx.QueryRowContext (FOR UPDATE).Scan → error → "topup not found"
// ├── currentStatus != PaymentPending → "topup already finalized"
// ├── tx.ExecContext (UPDATE topups SET status) → error → return err
// ├── nextStatus == PaymentSuccess:
// │   ├── uuid.Parse(merchantID) → error → "invalid merchant id"
// │   ├── ledgerRepo.GetAccountByMerchantID → error → "merchant ledger account not found"
// │   ├── ledgerRepo.Post → error → return err
// │   └── tx.ExecContext (UPDATE merchants SET balance) → error → return err
// ├── tx.QueryRowContext (final SELECT).Scan → error → return err
// └── tx.Commit → error → return err
//     └── all succeed → normalizeTopupTimes, return topup, nil
//
// nextStatus == PaymentFailed path skips the entire SUCCESS ledger block.
//
// ListTransactions:
// ├── uuid.Parse(merchantID) → error → "invalid merchant id"
// ├── ledgerRepo.GetAccountByMerchantID → error → "merchant ledger account not found"
// ├── ledgerRepo.ListEntriesByAccount → error → propagate
// └── all succeed → return entries, total, nil

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerMocks "payment-sandbox/app/modules/ledger/repositories/mocks"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testMerchantUUID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testAccountUUID  = uuid.MustParse("00000000-0000-0000-0000-000000000010")
)

// merchantCols are the columns returned by getMerchantByUserID's SELECT.
var merchantCols = []string{"id", "user_id", "balance", "created_at", "updated_at"}

// topupCols are the columns returned by topup SELECT statements.
var topupCols = []string{"id", "merchant_id", "amount", "status", "created_at", "updated_at"}

// ─── GetMerchantWallet ────────────────────────────────────────────────────────

func TestWalletRepository_GetMerchantWallet(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	now := time.Now().UTC()

	type args struct {
		userID string
	}
	type wants struct {
		merchantID string
		errMsg     string
	}

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. merchant found -> success merchant returned",
			args: args{userID: "user-1"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text, user_id::text").
					WithArgs("user-1").
					WillReturnRows(sqlmock.NewRows(merchantCols).
						AddRow("merchant-1", "user-1", int64(1000), now, now))
			},
			wants: wants{merchantID: "merchant-1"},
		},
		{
			name: "2. no merchant row -> merchant wallet not found error",
			args: args{userID: "user-unknown"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text, user_id::text").
					WithArgs("user-unknown").
					WillReturnError(sql.ErrNoRows)
			},
			wants: wants{errMsg: "merchant wallet not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			repo := NewWalletRepository(db, nil)
			merchant, err := repo.GetMerchantWallet(tt.args.userID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.merchantID, merchant.ID, "merchant ID")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── MerchantIDByUserID ───────────────────────────────────────────────────────

func TestWalletRepository_MerchantIDByUserID(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	now := time.Now().UTC()

	type args struct {
		userID string
	}
	type wants struct {
		merchantID string
		errMsg     string
	}

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. merchant found -> merchant ID returned",
			args: args{userID: "user-1"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text, user_id::text").
					WithArgs("user-1").
					WillReturnRows(sqlmock.NewRows(merchantCols).
						AddRow("merchant-1", "user-1", int64(0), now, now))
			},
			wants: wants{merchantID: "merchant-1"},
		},
		{
			name: "2. no merchant row -> merchant not found error",
			args: args{userID: "user-missing"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text, user_id::text").
					WithArgs("user-missing").
					WillReturnError(sql.ErrNoRows)
			},
			wants: wants{errMsg: "merchant not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			repo := NewWalletRepository(db, nil)
			merchantID, err := repo.MerchantIDByUserID(tt.args.userID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, merchantID, "merchant ID must be empty on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.merchantID, merchantID, "merchant ID")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── CreateTopup ─────────────────────────────────────────────────────────────

func TestWalletRepository_CreateTopup(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	now := time.Now().UTC()

	type args struct {
		merchantID string
		amount     int64
	}
	type wants struct {
		topupID string
		errMsg  string
	}

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. INSERT succeeds -> topup returned with ID",
			args: args{merchantID: testMerchantUUID.String(), amount: 50000},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("INSERT INTO topups")).
					WithArgs(testMerchantUUID.String(), int64(50000)).
					WillReturnRows(sqlmock.NewRows(topupCols).
						AddRow("topup-1", testMerchantUUID.String(), int64(50000), "PENDING", now, now))
			},
			wants: wants{topupID: "topup-1"},
		},
		{
			name: "2. INSERT scan error -> error returned empty topup",
			args: args{merchantID: testMerchantUUID.String(), amount: 100},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("INSERT INTO topups")).
					WithArgs(testMerchantUUID.String(), int64(100)).
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errMsg: "sql: connection is already closed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			repo := NewWalletRepository(db, nil)
			topup, err := repo.CreateTopup(tt.args.merchantID, tt.args.amount)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, topup.ID, "topup ID must be empty on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.topupID, topup.ID, "topup ID")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── ListTopups ──────────────────────────────────────────────────────────────

func TestWalletRepository_ListTopups(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	now := time.Now().UTC()

	type wants struct {
		count int // expected length of returned slice (always non-nil)
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. db.Query error -> empty slice returned (no error surfaced)",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text").
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{count: 0},
		},
		{
			name: "2. no rows -> empty slice returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text").
					WillReturnRows(sqlmock.NewRows(topupCols))
			},
			wants: wants{count: 0},
		},
		{
			name: "3. scan error on a row -> row silently skipped empty result",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text").
					WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id"}).
						AddRow("t1", "m1")) // too few columns → Scan fails silently
			},
			wants: wants{count: 0},
		},
		{
			name: "4. two valid rows -> two items returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id::text").
					WillReturnRows(sqlmock.NewRows(topupCols).
						AddRow("t1", testMerchantUUID.String(), int64(100), "PENDING", now, now).
						AddRow("t2", testMerchantUUID.String(), int64(200), "SUCCESS", now, now))
			},
			wants: wants{count: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			repo := NewWalletRepository(db, nil)
			items := repo.ListTopups()

			assert.NotNil(t, items, "result must never be nil")
			assert.Len(t, items, tt.wants.count, "item count")

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── ListMerchantTopups ───────────────────────────────────────────────────────

func TestWalletRepository_ListMerchantTopups(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	now := time.Now().UTC()
	merchantID := testMerchantUUID.String()

	type args struct {
		merchantID string
		page       int
		limit      int
	}
	type wants struct {
		count int
		total int
	}

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. count query error -> empty slice zero total returned",
			args: args{merchantID: merchantID, page: 1, limit: 10},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{count: 0, total: 0},
		},
		{
			name: "2. data query error -> empty slice with count total returned",
			args: args{merchantID: merchantID, page: 1, limit: 10},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WithArgs(merchantID).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				m.ExpectQuery("SELECT id::text").
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{count: 0, total: 5},
		},
		{
			name: "3. success with two items -> items and total returned",
			args: args{merchantID: merchantID, page: 1, limit: 10},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WithArgs(merchantID).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				m.ExpectQuery("SELECT id::text").
					WithArgs(merchantID, 10, 0).
					WillReturnRows(sqlmock.NewRows(topupCols).
						AddRow("t1", merchantID, int64(100), "PENDING", now, now).
						AddRow("t2", merchantID, int64(200), "SUCCESS", now, now))
			},
			wants: wants{count: 2, total: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			repo := NewWalletRepository(db, nil)
			items, total := repo.ListMerchantTopups(tt.args.merchantID, tt.args.page, tt.args.limit)

			assert.NotNil(t, items, "items must never be nil")
			assert.Len(t, items, tt.wants.count, "item count")
			assert.Equal(t, tt.wants.total, total, "total")

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── UpdateTopupStatus ────────────────────────────────────────────────────────

func TestWalletRepository_UpdateTopupStatus(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	now := time.Now().UTC()
	merchantIDStr := testMerchantUUID.String()

	type args struct {
		topupID    string
		nextStatus paymentEntity.PaymentStatus
	}
	type wants struct {
		topupStatus paymentEntity.PaymentStatus
		errMsg      string
	}

	tests := []struct {
		name        string
		args        args
		setupMock   func(m sqlmock.Sqlmock)
		setupLedger func(l *ledgerMocks.MockIRepository)
		wants       wants
	}{
		{
			name: "1. BeginTx error -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin().WillReturnError(errors.New("connection refused"))
			},
			wants: wants{errMsg: "connection refused"},
		},
		{
			name: "2. FOR UPDATE scan error -> topup not found error",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnError(sql.ErrNoRows)
				m.ExpectRollback()
			},
			wants: wants{errMsg: "topup not found"},
		},
		{
			name: "3. topup already finalized (status != PENDING) -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "SUCCESS"))
				m.ExpectRollback()
			},
			wants: wants{errMsg: "topup already finalized"},
		},
		{
			name: "4. UPDATE topups SET status exec error -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WillReturnError(errors.New("exec failed"))
				m.ExpectRollback()
			},
			wants: wants{errMsg: "exec failed"},
		},
		{
			name: "5. SUCCESS path: invalid merchant UUID -> invalid merchant id error",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow("not-a-uuid", int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("SUCCESS", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectRollback()
			},
			wants: wants{errMsg: "invalid merchant id"},
		},
		{
			name: "6. SUCCESS path: ledger GetAccountByMerchantID error -> merchant ledger account not found",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("SUCCESS", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectRollback()
			},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{}, errors.New("not found")).
					Once()
			},
			wants: wants{errMsg: "merchant ledger account not found"},
		},
		{
			name: "7. SUCCESS path: ledger Post error -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("SUCCESS", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectRollback()
			},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{ID: testAccountUUID}, nil).
					Once()
				l.EXPECT().
					Post(mock.Anything, mock.Anything, mock.Anything).
					Return(uuid.Nil, errors.New("ledger post failed")).
					Once()
			},
			wants: wants{errMsg: "ledger post failed"},
		},
		{
			name: "8. SUCCESS path: UPDATE merchants SET balance exec error -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("SUCCESS", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("UPDATE merchants SET balance").
					WillReturnError(errors.New("balance update failed"))
				m.ExpectRollback()
			},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{ID: testAccountUUID}, nil).
					Once()
				l.EXPECT().
					Post(mock.Anything, mock.Anything, mock.Anything).
					Return(uuid.New(), nil).
					Once()
			},
			wants: wants{errMsg: "balance update failed"},
		},
		{
			name: "9. final SELECT scan error -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentFailed},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("FAILED", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				// FAILED: no ledger block
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at FROM topups")).
					WithArgs("topup-1").
					WillReturnError(errors.New("final scan failed"))
				m.ExpectRollback()
			},
			wants: wants{errMsg: "final scan failed"},
		},
		{
			name: "10. Commit error -> error returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentFailed},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("FAILED", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at FROM topups")).
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows(topupCols).
						AddRow("topup-1", merchantIDStr, int64(100), "FAILED", now, now))
				m.ExpectCommit().WillReturnError(errors.New("commit failed"))
				// after a failed Commit(), sqlmock does not call the deferred Rollback handler
			},
			wants: wants{errMsg: "commit failed"},
		},
		{
			name: "11. FAILED status full happy path -> topup with FAILED status returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentFailed},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("FAILED", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				// FAILED: no ledger block
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at FROM topups")).
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows(topupCols).
						AddRow("topup-1", merchantIDStr, int64(100), "FAILED", now, now))
				m.ExpectCommit()
			},
			wants: wants{topupStatus: paymentEntity.PaymentFailed},
		},
		{
			name: "12. SUCCESS status full happy path -> topup with SUCCESS status returned",
			args: args{topupID: "topup-1", nextStatus: paymentEntity.PaymentSuccess},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT merchant_id::text").
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
						AddRow(merchantIDStr, int64(100), "PENDING"))
				m.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
					WithArgs("SUCCESS", "topup-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2")).
					WithArgs(testAccountUUID, merchantIDStr).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at FROM topups")).
					WithArgs("topup-1").
					WillReturnRows(sqlmock.NewRows(topupCols).
						AddRow("topup-1", merchantIDStr, int64(100), "SUCCESS", now, now))
				m.ExpectCommit()
			},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{ID: testAccountUUID}, nil).
					Once()
				l.EXPECT().
					Post(mock.Anything, mock.Anything, mock.Anything).
					Return(uuid.New(), nil).
					Once()
			},
			wants: wants{topupStatus: paymentEntity.PaymentSuccess},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			var ledger *ledgerMocks.MockIRepository
			if tt.setupLedger != nil {
				ledger = ledgerMocks.NewMockIRepository(t)
				tt.setupLedger(ledger)
			}

			repo := NewWalletRepository(db, ledger)
			topup, err := repo.UpdateTopupStatus(tt.args.topupID, tt.args.nextStatus)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, topup.ID, "topup ID must be empty on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.topupStatus, topup.Status, "topup status")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
			if ledger != nil {
				ledger.AssertExpectations(t)
			}
		})
	}
}

// ─── ListTransactions ─────────────────────────────────────────────────────────

func TestWalletRepository_ListTransactions(t *testing.T) {
	// not parallel: sqlmock expectations are sequential; each subtest creates its own db.

	entry := ledgerEntity.EntryWithTxn{ID: 1, Reference: "topup:t1"}

	type args struct {
		merchantID string
		filter     ledgerEntity.EntryFilter
		page       int
		limit      int
	}
	type wants struct {
		entries []ledgerEntity.EntryWithTxn
		total   int
		errMsg  string
	}

	tests := []struct {
		name        string
		args        args
		setupLedger func(l *ledgerMocks.MockIRepository)
		wants       wants
	}{
		{
			name:  "1. invalid merchant UUID -> invalid merchant id error",
			args:  args{merchantID: "not-a-uuid", filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			wants: wants{errMsg: "invalid merchant id"},
		},
		{
			name: "2. GetAccountByMerchantID error -> merchant ledger account not found",
			args: args{merchantID: testMerchantUUID.String(), filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{}, errors.New("not found")).
					Once()
			},
			wants: wants{errMsg: "merchant ledger account not found"},
		},
		{
			name: "3. ListEntriesByAccount error -> error propagated",
			args: args{merchantID: testMerchantUUID.String(), filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{ID: testAccountUUID}, nil).
					Once()
				l.EXPECT().
					ListEntriesByAccount(mock.Anything, testAccountUUID, ledgerEntity.EntryFilter{}, 1, 10).
					Return(nil, 0, errors.New("db error")).
					Once()
			},
			wants: wants{errMsg: "db error"},
		},
		{
			name: "4. all succeed -> entries and total returned",
			args: args{merchantID: testMerchantUUID.String(), filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			setupLedger: func(l *ledgerMocks.MockIRepository) {
				l.EXPECT().
					GetAccountByMerchantID(mock.Anything, testMerchantUUID).
					Return(ledgerEntity.Account{ID: testAccountUUID}, nil).
					Once()
				l.EXPECT().
					ListEntriesByAccount(mock.Anything, testAccountUUID, ledgerEntity.EntryFilter{}, 1, 10).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			},
			wants: wants{entries: []ledgerEntity.EntryWithTxn{entry}, total: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, _, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			var ledger *ledgerMocks.MockIRepository
			if tt.setupLedger != nil {
				ledger = ledgerMocks.NewMockIRepository(t)
				tt.setupLedger(ledger)
			}

			repo := NewWalletRepository(db, ledger)
			entries, total, err := repo.ListTransactions(tt.args.merchantID, tt.args.filter, tt.args.page, tt.args.limit)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Nil(t, entries, "entries must be nil on error")
				assert.Zero(t, total, "total must be zero on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.entries, entries, "entries")
				assert.Equal(t, tt.wants.total, total, "total")
			}

			if ledger != nil {
				ledger.AssertExpectations(t)
			}
		})
	}
}

// ensure context import is used
var _ = context.Background
