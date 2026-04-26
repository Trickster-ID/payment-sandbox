package repositories

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"

	"github.com/google/uuid"
)

type InvoiceRepository interface {
	MerchantIDByUserID(userID string) (string, error)
	CreateInvoice(merchantID, customerName, customerEmail string, amount float64, description string, dueDate time.Time) (invoiceEntity.Invoice, error)
	ListInvoices(merchantID string, status string, options invoiceEntity.ListOptions) ([]invoiceEntity.Invoice, int)
	MerchantInvoiceByID(invoiceID, merchantID string) (invoiceEntity.Invoice, error)
}

type SQLInvoiceRepository struct {
	db *sql.DB
}

func NewInvoiceRepository(db *sql.DB) *SQLInvoiceRepository {
	return &SQLInvoiceRepository{db: db}
}

func (r *SQLInvoiceRepository) MerchantIDByUserID(userID string) (string, error) {
	merchant, found := r.getMerchantByUserID(userID)
	if !found {
		return "", errors.New("merchant not found")
	}
	return merchant.ID, nil
}

func (r *SQLInvoiceRepository) CreateInvoice(merchantID, customerName, customerEmail string, amount float64, description string, dueDate time.Time) (invoiceEntity.Invoice, error) {
	var invoice invoiceEntity.Invoice
	invoiceNumber := "INV-" + strings.ToUpper(uuid.NewString()[:8])
	token := uuid.NewString()

	err := r.db.QueryRow(`
		INSERT INTO invoices (
			merchant_id, invoice_number, customer_name, customer_email,
			amount, description, due_date, payment_link_token
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
	`, merchantID, invoiceNumber, strings.TrimSpace(customerName), strings.TrimSpace(strings.ToLower(customerEmail)), amount, description, dueDate, token).
		Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		return invoiceEntity.Invoice{}, err
	}
	return invoice, nil
}

func (r *SQLInvoiceRepository) ListInvoices(merchantID string, status string, options invoiceEntity.ListOptions) ([]invoiceEntity.Invoice, int) {
	r.expireDueInvoices()
	page, limit := sanitizePaging(options.Page, options.Limit)
	offset := (page - 1) * limit

	where := "merchant_id=$1 AND deleted_at IS NULL"
	args := []any{merchantID}
	if status != "" {
		where += " AND status=$2"
		args = append(args, strings.ToUpper(status))
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM invoices WHERE " + where
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return []invoiceEntity.Invoice{}, 0
	}

	listSQL := `
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(listSQL, args...)
	if err != nil {
		return []invoiceEntity.Invoice{}, total
	}
	defer rows.Close()

	items := make([]invoiceEntity.Invoice, 0)
	for rows.Next() {
		var item invoiceEntity.Invoice
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.InvoiceNumber, &item.CustomerName, &item.CustomerEmail, &item.Amount, &item.Description, &item.DueDate, &item.Status, &item.PaymentLinkToken, &item.CreatedAt, &item.UpdatedAt); err == nil {
			items = append(items, item)
		}
	}
	return items, total
}

func (r *SQLInvoiceRepository) MerchantInvoiceByID(invoiceID, merchantID string) (invoiceEntity.Invoice, error) {
	r.expireDueInvoices()
	var invoice invoiceEntity.Invoice
	err := r.db.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL
	`, invoiceID, merchantID).
		Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		return invoiceEntity.Invoice{}, errors.New("invoice not found")
	}
	return invoice, nil
}

func (r *SQLInvoiceRepository) expireDueInvoices() {
	_, _ = r.db.Exec(`
		UPDATE invoices
		SET status='EXPIRED'
		WHERE status='PENDING'
			AND due_date < NOW()
			AND deleted_at IS NULL
	`)
}

func (r *SQLInvoiceRepository) getMerchantByUserID(userID string) (walletEntity.Merchant, bool) {
	var merchant walletEntity.Merchant
	err := r.db.QueryRow(`
		SELECT id::text, user_id::text, balance::double precision, created_at, updated_at
		FROM merchants
		WHERE user_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`, userID).Scan(&merchant.ID, &merchant.UserID, &merchant.Balance, &merchant.CreatedAt, &merchant.UpdatedAt)
	if err != nil {
		return walletEntity.Merchant{}, false
	}
	return merchant, true
}

func sanitizePaging(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return page, limit
}
