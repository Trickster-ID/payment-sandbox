package repositories

import (
	"database/sql"
	"fmt"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
)

type IAdminRepository interface {
	DashboardStats(filter adminEntity.StatsFilter) adminEntity.DashboardStats
}

type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) DashboardStats(filter adminEntity.StatsFilter) adminEntity.DashboardStats {
	r.expireDueInvoices()
	stats := adminEntity.DashboardStats{
		TotalByStatus: map[string]int{
			string(invoiceEntity.InvoicePaid):    0,
			string(paymentEntity.PaymentFailed):  0,
			string(invoiceEntity.InvoiceExpired): 0,
		},
	}

	invoiceWhere, invoiceArgs := buildFilter("i.merchant_id::text", "i.created_at", filter)
	paymentWhere, paymentArgs := buildFilter("inv.merchant_id::text", "pi.created_at", filter)
	refundWhere, refundArgs := buildFilter("inv.merchant_id::text", "r.created_at", filter)

	_ = r.db.QueryRow(`SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL`+invoiceWhere, invoiceArgs...).Scan(&stats.TotalInvoiceCreated)
	var paidCount int
	var expiredCount int
	var failedCount int
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.status='PAID'`+invoiceWhere, invoiceArgs...).Scan(&paidCount)
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL AND i.status='EXPIRED'`+invoiceWhere, invoiceArgs...).Scan(&expiredCount)
	_ = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM payment_intents pi
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE pi.deleted_at IS NULL AND pi.status='FAILED'`+paymentWhere, paymentArgs...).Scan(&failedCount)
	_ = r.db.QueryRow(`
		SELECT COALESCE(SUM(inv.amount), 0)
		FROM payment_intents pi
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE pi.deleted_at IS NULL AND pi.status='SUCCESS'`+paymentWhere, paymentArgs...).Scan(&stats.TotalPaymentNominal)
	_ = r.db.QueryRow(`
		SELECT COALESCE(SUM(inv.amount), 0)
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE r.deleted_at IS NULL AND r.status='SUCCESS'`+refundWhere, refundArgs...).Scan(&stats.TotalRefundNominal)
	stats.TotalByStatus[string(invoiceEntity.InvoicePaid)] = paidCount
	stats.TotalByStatus[string(invoiceEntity.InvoiceExpired)] = expiredCount
	stats.TotalByStatus[string(paymentEntity.PaymentFailed)] = failedCount
	return stats
}

func (r *AdminRepository) expireDueInvoices() {
	_, _ = r.db.Exec(`
		UPDATE invoices
		SET status='EXPIRED'
		WHERE status='PENDING'
			AND due_date < NOW()
			AND deleted_at IS NULL
	`)
}

func buildFilter(merchantColumn, dateColumn string, filter adminEntity.StatsFilter) (string, []any) {
	clause := ""
	args := make([]any, 0, 3)

	if filter.MerchantID != "" {
		clause += fmt.Sprintf(" AND %s = $%d", merchantColumn, len(args)+1)
		args = append(args, filter.MerchantID)
	}
	if filter.StartDate != nil {
		clause += fmt.Sprintf(" AND %s >= $%d", dateColumn, len(args)+1)
		args = append(args, *filter.StartDate)
	}
	if filter.EndDate != nil {
		clause += fmt.Sprintf(" AND %s <= $%d", dateColumn, len(args)+1)
		args = append(args, *filter.EndDate)
	}
	return clause, args
}
