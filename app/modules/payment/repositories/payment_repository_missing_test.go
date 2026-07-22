// Missing-branch tests for PaymentRepository.
// Each subtest creates a fresh db/sqlMock pair for isolation.
//
// Branch coverage targets:
//
//	GetInvoiceByToken    : not found (DB error)
//	CreatePaymentIntent  : invoice token not found, invoice not payable
//	ListPaymentIntents   : no filter / with filter / db error
//	GetInvoiceByID       : not found, success
//	UpdatePaymentStatus  : payment not found, already finalized,
//	                       invoice not found, invoice not pending,
//	                       FAILED success path (nil ledger)
package repositories

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── GetInvoiceByToken ──────────────────────────────────────────────────────

func TestPaymentRepository_GetInvoiceByToken_NotFound(t *testing.T) {
	// not parallel: sqlmock is not goroutine-safe
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
		WithArgs("bad-token").
		WillReturnError(sql.ErrNoRows)

	repo := NewPaymentRepository(db, nil)
	inv, found := repo.GetInvoiceByToken("bad-token")

	assert.False(t, found, "found should be false")
	assert.Empty(t, inv.ID, "invoice ID should be empty")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ── CreatePaymentIntent ───────────────────────────────────────────────────

func TestPaymentRepository_CreatePaymentIntent_Errors(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		token   string
		setup   func(dbMock sqlmock.Sqlmock)
		wantErr string
	}{
		{
			name:  "1. invoice token not found -> invoice token not found error",
			token: "unknown-token",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("unknown-token").
					WillReturnError(sql.ErrNoRows)
				dbMock.ExpectRollback()
			},
			wantErr: "invoice token not found",
		},
		{
			name:  "2. invoice status != PENDING -> invoice not payable error",
			token: "token-paid",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("token-paid").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
						"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
					}).AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Bob", "bob@example.com",
						int64(200), "", now, "PAID", "token-paid", now, now))
				dbMock.ExpectRollback()
			},
			wantErr: "invoice not payable",
		},
		{
			name:  "3. existing pending intent -> payment already pending error",
			token: "token-pending",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("token-pending").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
						"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
					}).AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Bob", "bob@example.com",
						int64(200), "", now, "PENDING", "token-pending", now, now))
				dbMock.ExpectQuery(`SELECT EXISTS\(\s*SELECT 1 FROM payment_intents`).
					WithArgs("inv-1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				dbMock.ExpectRollback()
			},
			wantErr: "payment already pending",
		},
		{
			name:  "4. insert payment intent error",
			token: "token-1",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).WithArgs("token-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
						AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Bob", "bob@example.com", int64(200), "", now, "PENDING", "token-1", now, now))
				dbMock.ExpectQuery(`SELECT EXISTS\(\s*SELECT 1 FROM payment_intents`).
					WithArgs("inv-1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				dbMock.ExpectQuery(regexp.QuoteMeta("INSERT INTO payment_intents")).WithArgs("inv-1", "WALLET").WillReturnError(sql.ErrConnDone)
				dbMock.ExpectRollback()
			},
			wantErr: sql.ErrConnDone.Error(),
		},
		{
			name:  "5. commit error",
			token: "token-1",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).WithArgs("token-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
						AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Bob", "bob@example.com", int64(200), "", now, "PENDING", "token-1", now, now))
				dbMock.ExpectQuery(`SELECT EXISTS\(\s*SELECT 1 FROM payment_intents`).
					WithArgs("inv-1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				dbMock.ExpectQuery(regexp.QuoteMeta("INSERT INTO payment_intents")).WithArgs("inv-1", "WALLET").
					WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).AddRow("pi-1", "inv-1", "WALLET", "PENDING", now, now))
				dbMock.ExpectCommit().WillReturnError(sql.ErrTxDone)
			},
			wantErr: sql.ErrTxDone.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.setup(dbMock)

			repo := NewPaymentRepository(db, nil)
			intent, inv, err := repo.CreatePaymentIntent(tt.token, paymentEntity.MethodWallet)

			require.Error(t, err, "expected an error")
			assert.EqualError(t, err, tt.wantErr, "error message")
			assert.Empty(t, intent.ID, "intent ID should be empty")
			assert.Empty(t, inv.ID, "invoice ID should be empty")
			assert.NoError(t, dbMock.ExpectationsWereMet())
		})
	}
}

// ── ListPaymentIntents ─────────────────────────────────────────────────────

func TestPaymentRepository_ListPaymentIntents(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		status  string
		setup   func(dbMock sqlmock.Sqlmock)
		wantLen int
	}{
		{
			name:   "1. no status filter -> returns all intents",
			status: "",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
					WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
						AddRow("pi-1", "inv-1", "WALLET", "PENDING", now, now).
						AddRow("pi-2", "inv-2", "VA_DUMMY", "SUCCESS", now, now))
			},
			wantLen: 2,
		},
		{
			name:   "2. status filter applied -> returns matching intents",
			status: "PENDING",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
					WithArgs("PENDING").
					WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
						AddRow("pi-1", "inv-1", "WALLET", "PENDING", now, now))
			},
			wantLen: 1,
		},
		{
			name:   "3. db query error -> returns empty slice",
			status: "",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text")).
					WillReturnError(sql.ErrConnDone)
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.setup(dbMock)

			repo := NewPaymentRepository(db, nil)
			result := repo.ListPaymentIntents(tt.status)

			assert.Len(t, result, tt.wantLen, "result length")
			assert.NoError(t, dbMock.ExpectationsWereMet())
		})
	}
}

// ── GetInvoiceByID ─────────────────────────────────────────────────────────

func TestPaymentRepository_GetInvoiceByID_Branches(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		invoiceID string
		setup     func(dbMock sqlmock.Sqlmock)
		wantFound bool
		wantID    string
	}{
		{
			name:      "1. invoice not in DB -> returns false",
			invoiceID: "inv-missing",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("inv-missing").
					WillReturnError(sql.ErrNoRows)
			},
			wantFound: false,
		},
		{
			name:      "2. invoice found -> returns true with data",
			invoiceID: "inv-1",
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("inv-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
						"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
					}).AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Alice", "alice@example.com",
						int64(100), "", now, "PENDING", "tok-1", now, now))
			},
			wantFound: true,
			wantID:    "inv-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.setup(dbMock)

			repo := NewPaymentRepository(db, nil)
			inv, found := repo.GetInvoiceByID(tt.invoiceID)

			assert.Equal(t, tt.wantFound, found, "found flag")
			assert.Equal(t, tt.wantID, inv.ID, "invoice ID")
			assert.NoError(t, dbMock.ExpectationsWereMet())
		})
	}
}

// ── UpdatePaymentStatus ────────────────────────────────────────────────────

func TestPaymentRepository_UpdatePaymentStatus_ErrorBranches(t *testing.T) {
	now := time.Now()
	merchantIDStr := testPaymentMerchantUUID.String()

	intentRows := func(status string) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
			AddRow("intent-1", "inv-1", "WALLET", status, now, now)
	}
	invoiceRows := func(status string) *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
			"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
		}).AddRow("inv-1", merchantIDStr, "INV-1", "Alice", "alice@example.com",
			int64(100), "", now, status, "tok-1", now, now)
	}

	tests := []struct {
		name       string
		paymentID  string
		nextStatus paymentEntity.PaymentStatus
		setup      func(dbMock sqlmock.Sqlmock)
		wantErr    string
	}{
		{
			name:       "1. payment intent not found -> payment intent not found error",
			paymentID:  "missing-id",
			nextStatus: paymentEntity.PaymentFailed,
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text")).
					WithArgs("missing-id").
					WillReturnError(sql.ErrNoRows)
				dbMock.ExpectRollback()
			},
			wantErr: "payment intent not found",
		},
		{
			name:       "2. payment already finalized (SUCCESS) -> payment intent already finalized error",
			paymentID:  "intent-1",
			nextStatus: paymentEntity.PaymentFailed,
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text")).
					WithArgs("intent-1").
					WillReturnRows(intentRows("SUCCESS"))
				dbMock.ExpectRollback()
			},
			wantErr: "payment intent already finalized",
		},
		{
			name:       "3. invoice lookup fails -> invoice not found error",
			paymentID:  "intent-1",
			nextStatus: paymentEntity.PaymentFailed,
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text")).
					WithArgs("intent-1").
					WillReturnRows(intentRows("PENDING"))
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("inv-1").
					WillReturnError(sql.ErrNoRows)
				dbMock.ExpectRollback()
			},
			wantErr: "invoice not found",
		},
		{
			name:       "4. invoice is not pending (PAID) -> invoice is not pending error",
			paymentID:  "intent-1",
			nextStatus: paymentEntity.PaymentFailed,
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text")).
					WithArgs("intent-1").
					WillReturnRows(intentRows("PENDING"))
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
					WithArgs("inv-1").
					WillReturnRows(invoiceRows("PAID"))
				dbMock.ExpectRollback()
			},
			wantErr: "invoice is not pending",
		},
		{
			name:       "5. payment status update error",
			paymentID:  "intent-1",
			nextStatus: paymentEntity.PaymentFailed,
			setup: func(dbMock sqlmock.Sqlmock) {
				dbMock.ExpectBegin()
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text")).WithArgs("intent-1").WillReturnRows(intentRows("PENDING"))
				dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).WithArgs("inv-1").WillReturnRows(invoiceRows("PENDING"))
				dbMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status=$1")).WithArgs("FAILED", "intent-1").WillReturnError(sql.ErrConnDone)
				dbMock.ExpectRollback()
			},
			wantErr: sql.ErrConnDone.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, dbMock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.setup(dbMock)

			repo := NewPaymentRepository(db, nil)
			intent, inv, err := repo.UpdatePaymentStatus(tt.paymentID, tt.nextStatus)

			require.Error(t, err, "expected error")
			assert.EqualError(t, err, tt.wantErr, "error message")
			assert.Empty(t, intent.ID, "intent ID should be empty on error")
			assert.Empty(t, inv.ID, "invoice ID should be empty on error")
			assert.NoError(t, dbMock.ExpectationsWereMet())
		})
	}
}

func TestPaymentRepository_UpdatePaymentStatus_FailedSuccess(t *testing.T) {
	// FAILED transition (nil ledger repo) — full success path not covered by existing tests.
	now := time.Now()
	merchantIDStr := testPaymentMerchantUUID.String()

	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dbMock.ExpectBegin()

	// Lock payment_intent
	dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
		WithArgs("intent-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
			AddRow("intent-1", "inv-1", "WALLET", "PENDING", now, now))

	// Lock invoice
	dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
			"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
		}).AddRow("inv-1", merchantIDStr, "INV-1", "Alice", "alice@example.com",
			int64(100), "", now, "PENDING", "tok-1", now, now))

	// Update intent status → FAILED
	dbMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status=$1")).
		WithArgs("FAILED", "intent-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// (no SUCCESS block for FAILED status)
	dbMock.ExpectCommit()

	// Final lookups after commit
	dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
		WithArgs("intent-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
			AddRow("intent-1", "inv-1", "WALLET", "FAILED", now, now))

	dbMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number")).
		WithArgs("inv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
			"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
		}).AddRow("inv-1", merchantIDStr, "INV-1", "Alice", "alice@example.com",
			int64(100), "", now, "PENDING", "tok-1", now, now))

	repo := NewPaymentRepository(db, nil)
	intent, inv, err := repo.UpdatePaymentStatus("intent-1", paymentEntity.PaymentFailed)

	require.NoError(t, err, "unexpected error")
	assert.Equal(t, paymentEntity.PaymentFailed, intent.Status, "intent status")
	assert.Equal(t, invoiceEntity.InvoicePending, inv.Status, "invoice status unchanged for FAILED")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}
