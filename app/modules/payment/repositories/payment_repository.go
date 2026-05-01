package repositories

import (
	"context"
	"database/sql"
	"errors"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerRepo "payment-sandbox/app/modules/ledger/repositories"
	ledgerSvc "payment-sandbox/app/modules/ledger/services"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"

	"github.com/google/uuid"
)

type IPaymentRepository interface {
	GetInvoiceByToken(token string) (invoiceEntity.Invoice, bool)
	GetInvoiceByID(id string) (invoiceEntity.Invoice, bool)
	CreatePaymentIntent(invoiceToken string, method paymentEntity.PaymentMethod) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
	ListPaymentIntents(status string) []paymentEntity.PaymentIntent
	UpdatePaymentStatus(paymentID string, nextStatus paymentEntity.PaymentStatus) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
}

type PaymentRepository struct {
	db         *sql.DB
	ledgerRepo ledgerRepo.IRepository
}

func NewPaymentRepository(db *sql.DB, ledger ledgerRepo.IRepository) *PaymentRepository {
	return &PaymentRepository{db: db, ledgerRepo: ledger}
}

func (r *PaymentRepository) GetInvoiceByToken(token string) (invoiceEntity.Invoice, bool) {
	r.expireDueInvoices()
	var invoice invoiceEntity.Invoice
	err := r.db.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE payment_link_token=$1 AND deleted_at IS NULL
	`, token).Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		return invoiceEntity.Invoice{}, false
	}
	return invoice, true
}

func (r *PaymentRepository) CreatePaymentIntent(invoiceToken string, method paymentEntity.PaymentMethod) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error) {
	r.expireDueInvoices()
	tx, err := r.db.Begin()
	if err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	defer tx.Rollback()

	var invoice invoiceEntity.Invoice
	if err := tx.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount, COALESCE(description, ''), due_date, status::text,
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
	normalizePaymentIntentTimes(&intent)
	normalizeInvoiceTimes(&invoice)
	return intent, invoice, nil
}

func (r *PaymentRepository) ListPaymentIntents(status string) []paymentEntity.PaymentIntent {
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
			normalizePaymentIntentTimes(&item)
			items = append(items, item)
		}
	}
	return items
}

func (r *PaymentRepository) UpdatePaymentStatus(paymentID string, nextStatus paymentEntity.PaymentStatus) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error) {
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
			amount, COALESCE(description, ''), due_date, status::text,
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

		if r.ledgerRepo != nil {
			merchantUUID, err := uuid.Parse(invoice.MerchantID)
			if err != nil {
				return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invalid merchant id")
			}
			walletAcct, err := r.ledgerRepo.GetAccountByMerchantID(context.Background(), merchantUUID)
			if err != nil {
				return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("merchant ledger account not found")
			}
			posting := ledgerEntity.Posting{
				Reference:   "payment_" + paymentID,
				Description: "Payment settlement",
				Entries: []ledgerEntity.Entry{
					{AccountID: walletAcct.ID, Direction: ledgerEntity.Debit, Amount: invoice.Amount, Currency: "IDR"},
					{AccountID: ledgerEntity.PendingPaymentsAccountID, Direction: ledgerEntity.Credit, Amount: invoice.Amount, Currency: "IDR"},
				},
			}
			if err := ledgerSvc.ValidatePosting(posting); err != nil {
				return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
			}
			if _, err := r.ledgerRepo.Post(context.Background(), tx, posting); err != nil {
				return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
			}
			if _, err := tx.Exec(
				`UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2 AND deleted_at IS NULL`,
				walletAcct.ID, invoice.MerchantID); err != nil {
				return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	intent, _ = r.getPaymentIntentByID(paymentID)
	invoice, _ = r.GetInvoiceByID(invoice.ID)
	return intent, invoice, nil
}

func (r *PaymentRepository) expireDueInvoices() {
	_, _ = r.db.Exec(`
		UPDATE invoices
		SET status='EXPIRED'
		WHERE status='PENDING'
			AND due_date < NOW()
			AND deleted_at IS NULL
	`)
}

func (r *PaymentRepository) GetInvoiceByID(id string) (invoiceEntity.Invoice, bool) {
	var invoice invoiceEntity.Invoice
	err := r.db.QueryRow(`
		SELECT id::text, merchant_id::text, invoice_number, customer_name, customer_email,
			amount, COALESCE(description, ''), due_date, status::text,
			payment_link_token, created_at, updated_at
		FROM invoices
		WHERE id=$1 AND deleted_at IS NULL
	`, id).Scan(&invoice.ID, &invoice.MerchantID, &invoice.InvoiceNumber, &invoice.CustomerName, &invoice.CustomerEmail, &invoice.Amount, &invoice.Description, &invoice.DueDate, &invoice.Status, &invoice.PaymentLinkToken, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		return invoiceEntity.Invoice{}, false
	}
	normalizeInvoiceTimes(&invoice)
	return invoice, true
}

func (r *PaymentRepository) getPaymentIntentByID(id string) (paymentEntity.PaymentIntent, bool) {
	var intent paymentEntity.PaymentIntent
	err := r.db.QueryRow(`
		SELECT id::text, invoice_id::text, method::text, status::text, created_at, updated_at
		FROM payment_intents
		WHERE id=$1 AND deleted_at IS NULL
	`, id).Scan(&intent.ID, &intent.InvoiceID, &intent.Method, &intent.Status, &intent.CreatedAt, &intent.UpdatedAt)
	if err != nil {
		return paymentEntity.PaymentIntent{}, false
	}
	normalizePaymentIntentTimes(&intent)
	return intent, true
}

func normalizeInvoiceTimes(invoice *invoiceEntity.Invoice) {
	invoice.DueDate = invoice.DueDate.UTC()
	invoice.CreatedAt = invoice.CreatedAt.UTC()
	invoice.UpdatedAt = invoice.UpdatedAt.UTC()
}

func normalizePaymentIntentTimes(intent *paymentEntity.PaymentIntent) {
	intent.CreatedAt = intent.CreatedAt.UTC()
	intent.UpdatedAt = intent.UpdatedAt.UTC()
}
