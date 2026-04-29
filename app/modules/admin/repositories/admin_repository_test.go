package repositories

import (
	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminRepository_DashboardStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewAdminRepository(db)

	t.Run("success without filter", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.status='PAID'")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.status='EXPIRED'")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(inv.amount::double precision), 0)")).
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(5000.0))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(inv.amount::double precision), 0)")).
			WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(100.0))

		stats := repo.DashboardStats(adminEntity.StatsFilter{})
		assert.Equal(t, 10, stats.TotalInvoiceCreated)
		assert.Equal(t, 5000.0, stats.TotalPaymentNominal)
		assert.Equal(t, 100.0, stats.TotalRefundNominal)
		assert.Equal(t, 5, stats.TotalByStatus["PAID"])
	})

	t.Run("success with merchant filter", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.merchant_id::text = $1")).
			WithArgs("m-1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		
		// ... repeat for other 5 queries, or just let them run with defaults
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.status='PAID' AND i.merchant_id::text = $1")).WithArgs("m-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.status='EXPIRED' AND i.merchant_id::text = $1")).WithArgs("m-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).WithArgs("m-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE")).WithArgs("m-1").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(100.0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE")).WithArgs("m-1").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(0))

		stats := repo.DashboardStats(adminEntity.StatsFilter{MerchantID: "m-1"})
		assert.Equal(t, 1, stats.TotalInvoiceCreated)
	})
}
