package repositories

import (
	"database/sql"
	"errors"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
)

type PaymentRepository interface {
	GetInvoiceByToken(token string) (invoiceEntity.Invoice, bool)
	CreatePaymentIntent(invoiceToken string, method paymentEntity.PaymentMethod) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
	ListPaymentIntents(status string) []paymentEntity.PaymentIntent
	UpdatePaymentStatus(paymentID string, nextStatus paymentEntity.PaymentStatus) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
}

type SQLPaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *SQLPaymentRepository {
	return &SQLPaymentRepository{db: db}
}

func (r *SQLPaymentRepository) GetInvoiceByToken(token string) (invoiceEntity.Invoice, bool) {
	r.expireDueInvoices()
	var invoice invoiceEntity.Invoice
	err := r.db.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE payment_link_token=$1 AND deleted_at IS NULL
	`, token).Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		return invoiceEntity.Invoice{}, false
	}
	return invoice, true
}

func (r *SQLPaymentRepository) CreatePaymentIntent(invoiceToken string, method paymentEntity.PaymentMethod) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error) {
	r.expireDueInvoices()
	tx, err := r.db.Begin()
	if err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	defer tx.Rollback()

	var invoice invoiceEntity.Invoice
	if err := tx.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE payment_link_token=$1 AND deleted_at IS NULL
		FOR UPDATE
	`, invoiceToken).Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invoice token not found")
	}
	if invoice.Status != invoiceEntity.InvoicePending {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invoice not payable")
	}

	var intent paymentEntity.PaymentIntent
	if err := tx.QueryRow(`
		INSERT INTO payment_intents (invoice_id, method)
		VALUES ($1,$2)
		RETURNING id::text, invoice_id::text, method::text, status::text, created_at, updated_at
	`, invoice.ID, string(method)).Scan(&intent.ID, &intent.InvoiceID, &intent.Method, &intent.Status, &intent.CreatedAt, &intent.UpdatedAt); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}

	if err := tx.Commit(); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	return intent, invoice, nil
}

func (r *SQLPaymentRepository) ListPaymentIntents(status string) []paymentEntity.PaymentIntent {
	base := `
		SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at
		FROM payment_intents
		WHERE deleted_at IS NULL
	`
	args := []any{}
	if status != "" {
		base += " AND status=$1"
		args = append(args, status)
	}
	base += " ORDER BY created_at DESC"

	rows, err := r.db.Query(base, args...)
	if err != nil {
		return []paymentEntity.PaymentIntent{}
	}
	defer rows.Close()

	items := make([]paymentEntity.PaymentIntent, 0)
	for rows.Next() {
		var item paymentEntity.PaymentIntent
		if err := rows.Scan(&item.ID, &item.InvoiceID, &item.Method, &item.Status, &item.CreatedAt, &item.UpdatedAt); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *SQLPaymentRepository) UpdatePaymentStatus(paymentID string, nextStatus paymentEntity.PaymentStatus) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	defer tx.Rollback()

	var intent paymentEntity.PaymentIntent
	if err := tx.QueryRow(`
		SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at
		FROM payment_intents
		WHERE id=$1 AND deleted_at IS NULL
		FOR UPDATE
	`, paymentID).Scan(&intent.ID, &intent.InvoiceID, &intent.Method, &intent.Status, &intent.CreatedAt, &intent.UpdatedAt); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("payment intent not found")
	}
	if intent.Status != paymentEntity.PaymentPending {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("payment intent already finalized")
	}

	var invoice invoiceEntity.Invoice
	if err := tx.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE id=$1 AND deleted_at IS NULL
		FOR UPDATE
	`, intent.InvoiceID).Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invoice not found")
	}
	if invoice.Status != invoiceEntity.InvoicePending {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invoice is not pending")
	}

	if _, err := tx.Exec(`UPDATE payment_intents SET status=$1 WHERE id=$2 AND deleted_at IS NULL`, string(nextStatus), paymentID); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	intent.Status = nextStatus

	if nextStatus == paymentEntity.PaymentSuccess {
		if _, err := tx.Exec(`UPDATE invoices SET status='PAID' WHERE id=$1 AND deleted_at IS NULL`, invoice.ID); err != nil {
			return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
		}
		invoice.Status = invoiceEntity.InvoicePaid
	}

	if err := tx.Commit(); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	intent, _ = r.getPaymentIntentByID(paymentID)
	invoice, _ = r.getInvoiceByID(invoice.ID)
	return intent, invoice, nil
}

func (r *SQLPaymentRepository) expireDueInvoices() {
	_, _ = r.db.Exec(`
		UPDATE invoices
		SET status='EXPIRED'
		WHERE status='PENDING'
			AND due_date < NOW()
			AND deleted_at IS NULL
	`)
}

func (r *SQLPaymentRepository) getInvoiceByID(id string) (invoiceEntity.Invoice, bool) {
	var invoice invoiceEntity.Invoice
	err := r.db.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount::double precision, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE id=$1 AND deleted_at IS NULL
	`, id).Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		return invoiceEntity.Invoice{}, false
	}
	return invoice, true
}

func (r *SQLPaymentRepository) getPaymentIntentByID(id string) (paymentEntity.PaymentIntent, bool) {
	var intent paymentEntity.PaymentIntent
	err := r.db.QueryRow(`
		SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at
		FROM payment_intents
		WHERE id=$1 AND deleted_at IS NULL
	`, id).Scan(&intent.ID, &intent.InvoiceID, &intent.Method, &intent.Status, &intent.CreatedAt, &intent.UpdatedAt)
	if err != nil {
		return paymentEntity.PaymentIntent{}, false
	}
	return intent, true
}
