package repositories

import (
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentRepository_GetInvoiceByToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPaymentRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount::double precision, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("token-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", "m-1", "INV-1", "Alice", "alice@example.com", 100.0, "", now, "PENDING", "token-1", now, now))

		inv, found := repo.GetInvoiceByToken("token-1")
		assert.True(t, found)
		assert.Equal(t, "inv-1", inv.ID)
	})
}

func TestPaymentRepository_CreatePaymentIntent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPaymentRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount::double precision, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("token-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", "m-1", "INV-1", "Alice", "alice@example.com", 100.0, "", now, "PENDING", "token-1", now, now))
		
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO payment_intents")).
			WithArgs("inv-1", "WALLET").
			WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
				AddRow("intent-1", "inv-1", "WALLET", "PENDING", now, now))
		
		mock.ExpectCommit()

		intent, _, err := repo.CreatePaymentIntent("token-1", paymentEntity.MethodWallet)
		require.NoError(t, err)
		assert.Equal(t, "intent-1", intent.ID)
	})
}

func TestPaymentRepository_UpdatePaymentStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPaymentRepository(db)
	now := time.Now()

	t.Run("success SUCCESS", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
			WithArgs("intent-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
				AddRow("intent-1", "inv-1", "WALLET", "PENDING", now, now))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount::double precision, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", "m-1", "INV-1", "Alice", "alice@example.com", 100.0, "", now, "PENDING", "token-1", now, now))
		
		mock.ExpectExec(regexp.QuoteMeta("UPDATE payment_intents SET status=$1")).
			WithArgs("SUCCESS", "intent-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices SET status='PAID'")).
			WithArgs("inv-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectCommit()

		// Repos calls helper methods after commit
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at FROM payment_intents")).
			WithArgs("intent-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "invoice_id", "method", "status", "created_at", "updated_at"}).
				AddRow("intent-1", "inv-1", "WALLET", "SUCCESS", now, now))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount::double precision, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", "m-1", "INV-1", "Alice", "alice@example.com", 100.0, "", now, "PAID", "token-1", now, now))

		intent, inv, err := repo.UpdatePaymentStatus("intent-1", paymentEntity.PaymentSuccess)
		require.NoError(t, err)
		assert.Equal(t, paymentEntity.PaymentSuccess, intent.Status)
		assert.Equal(t, invoiceEntity.InvoicePaid, inv.Status)
	})
}
