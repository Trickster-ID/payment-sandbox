package repositories

import (
	"regexp"
	"testing"
	"time"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerMocks "payment-sandbox/app/modules/ledger/repositories/mocks"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testPaymentMerchantUUID = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	testPaymentAccountUUID  = uuid.MustParse("00000000-0000-0000-0000-000000000030")
)

func TestPaymentRepository_GetInvoiceByToken(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPaymentRepository(db, nil)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("token-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Alice", "alice@example.com", int64(100), "", now, "PENDING", "token-1", now, now))

		inv, found := repo.GetInvoiceByToken("token-1")
		assert.True(t, found)
		assert.Equal(t, "inv-1", inv.ID)
	})
}

func TestPaymentRepository_CreatePaymentIntent(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPaymentRepository(db, nil)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("token-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", testPaymentMerchantUUID.String(), "INV-1", "Alice", "alice@example.com", int64(100), "", now, "PENDING", "token-1", now, now))

		sqlMock.ExpectQuery(regexp.QuoteMeta("INSERT INTO payment_intents")).
			WithArgs("inv-1", "WALLET").
			WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
				AddRow("intent-1", "inv-1", "WALLET", "PENDING", now, now))

		sqlMock.ExpectCommit()

		intent, _, err := repo.CreatePaymentIntent("token-1", paymentEntity.MethodWallet)
		require.NoError(t, err)
		assert.Equal(t, "intent-1", intent.ID)
	})
}

func TestPaymentRepository_UpdatePaymentStatus(t *testing.T) {
	merchantIDStr := testPaymentMerchantUUID.String()
	now := time.Now()

	t.Run("success SUCCESS with ledger", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		ledgerMock := ledgerMocks.NewMockIRepository(t)
		repo := NewPaymentRepository(db, ledgerMock)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
			WithArgs("intent-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
				AddRow("intent-1", "inv-1", "WALLET", "PENDING", now, now))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", merchantIDStr, "INV-1", "Alice", "alice@example.com", int64(100), "", now, "PENDING", "token-1", now, now))

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status=$1")).
			WithArgs("SUCCESS", "intent-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE invoices SET status='PAID'")).
			WithArgs("inv-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		ledgerMock.EXPECT().
			GetAccountByMerchantID(mock.Anything, testPaymentMerchantUUID).
			Return(ledgerEntity.Account{ID: testPaymentAccountUUID}, nil)

		ledgerMock.EXPECT().
			Post(mock.Anything, mock.Anything, mock.Anything).
			Return(uuid.New(), nil)

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2")).
			WithArgs(testPaymentAccountUUID, merchantIDStr).
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectCommit()

		// Final lookups after commit
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
			WithArgs("intent-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
				AddRow("intent-1", "inv-1", "WALLET", "SUCCESS", now, now))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", merchantIDStr, "INV-1", "Alice", "alice@example.com", int64(100), "", now, "PAID", "token-1", now, now))

		intent, inv, err := repo.UpdatePaymentStatus("intent-1", paymentEntity.PaymentSuccess)
		require.NoError(t, err)
		assert.Equal(t, paymentEntity.PaymentSuccess, intent.Status)
		assert.Equal(t, invoiceEntity.InvoicePaid, inv.Status)
		assert.NoError(t, sqlMock.ExpectationsWereMet())
	})
}
