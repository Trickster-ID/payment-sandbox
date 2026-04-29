package repositories

import (
	"database/sql"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceRepository_MerchantIDByUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewInvoiceRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance::double precision, created_at, updated_at FROM merchants")).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "created_at", "updated_at"}).
				AddRow("merchant-1", "user-1", 0.0, now, now))

		id, err := repo.MerchantIDByUserID("user-1")
		require.NoError(t, err)
		assert.Equal(t, "merchant-1", id)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).WillReturnError(sql.ErrNoRows)
		_, err := repo.MerchantIDByUserID("unknown")
		assert.Error(t, err)
	})
}

func TestInvoiceRepository_CreateInvoice(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewInvoiceRepository(db)
	now := time.Now()
	dueDate := now.Add(24 * time.Hour)

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO invoices")).
			WithArgs("merchant-1", sqlmock.AnyArg(), "Alice", "alice@example.com", 1000.0, "desc", dueDate, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "payment_link_token", "created_at", "updated_at"}).
				AddRow("inv-1", "merchant-1", "INV-123", "Alice", "alice@example.com", 1000.0, "desc", dueDate, "PENDING", "token", now, now))

		inv, err := repo.CreateInvoice("merchant-1", "Alice", "alice@example.com", 1000.0, "desc", dueDate)
		require.NoError(t, err)
		assert.Equal(t, "inv-1", inv.ID)
	})
}

func TestInvoiceRepository_ListInvoices(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewInvoiceRepository(db)
	now := time.Now()

	t.Run("success with status", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL AND status=$2")).
			WithArgs("merchant-1", "PAID").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount::double precision, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL AND status=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4")).
			WithArgs("merchant-1", "PAID", 10, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", "merchant-1", "INV-1", "Alice", "alice@example.com", 1000.0, "", now, "PAID", "t", now, now))

		items, total := repo.ListInvoices("merchant-1", "PAID", invoiceEntity.ListOptions{Page: 1, Limit: 10})
		assert.Equal(t, 1, total)
		assert.Len(t, items, 1)
	})
}

func TestInvoiceRepository_MerchantInvoiceByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewInvoiceRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email, amount::double precision, COALESCE(description, ''), due_date, status::text, payment_link_token, created_at, updated_at FROM invoices")).
			WithArgs("inv-1", "merchant-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "invoice_number", "customer_name", "customer_email", "amount", "description", "due_date", "status", "token", "created_at", "updated_at"}).
				AddRow("inv-1", "merchant-1", "INV-1", "Alice", "alice@example.com", 1000.0, "", now, "PENDING", "t", now, now))

		inv, err := repo.MerchantInvoiceByID("inv-1", "merchant-1")
		require.NoError(t, err)
		assert.Equal(t, "inv-1", inv.ID)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).WillReturnError(sql.ErrNoRows)

		_, err := repo.MerchantInvoiceByID("inv-1", "merchant-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invoice not found")
	})
}
