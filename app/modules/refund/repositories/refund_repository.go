package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerRepo "payment-sandbox/app/modules/ledger/repositories"
	ledgerSvc "payment-sandbox/app/modules/ledger/services"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"

	"github.com/google/uuid"
)

type IRefundRepository interface {
	MerchantIDByUserID(userID string) (string, error)
	RequestRefund(merchantID, invoiceID, reason string) (refundEntity.Refund, error)
	ListMerchantRefunds(merchantID, status string) []refundEntity.Refund
	ListRefunds(status string) []refundEntity.Refund
	ReviewRefund(refundID string, approved bool) (refundEntity.Refund, error)
	ProcessRefund(refundID string, nextStatus refundEntity.RefundStatus) (refundEntity.Refund, walletEntity.Merchant, error)
}

type RefundRepository struct {
	db         *sql.DB
	ledgerRepo ledgerRepo.IRepository
}

func NewRefundRepository(db *sql.DB, ledger ledgerRepo.IRepository) *RefundRepository {
	return &RefundRepository{db: db, ledgerRepo: ledger}
}

func (r *RefundRepository) MerchantIDByUserID(userID string) (string, error) {
	merchant, found := r.getMerchantByUserID(userID)
	if !found {
		return "", errors.New("merchant not found")
	}
	return merchant.ID, nil
}

func (r *RefundRepository) RequestRefund(merchantID, invoiceID, reason string) (refundEntity.Refund, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return refundEntity.Refund{}, err
	}
	defer tx.Rollback()

	var paymentIntentID string
	var paymentStatus paymentEntity.PaymentStatus
	var ownerMerchantID string
	var amount int64
	if err := tx.QueryRow(`
		SELECT pi.id::text, pi.status::text, inv.merchant_id::text, inv.amount
		FROM payment_intents pi
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE inv.id = $1 AND pi.deleted_at IS NULL AND inv.deleted_at IS NULL
		FOR UPDATE OF pi
	`, invoiceID).Scan(&paymentIntentID, &paymentStatus, &ownerMerchantID, &amount); err != nil {
		return refundEntity.Refund{}, errors.New("invoice not found or payment not created")
	}
	if paymentStatus != paymentEntity.PaymentSuccess {
		return refundEntity.Refund{}, errors.New("refund can be requested for successful payment only")
	}
	if ownerMerchantID != merchantID {
		return refundEntity.Refund{}, errors.New("invoice does not belong to merchant")
	}

	var refund refundEntity.Refund
	if err := tx.QueryRow(`
		INSERT INTO refunds (payment_intent_id, reason, amount)
		VALUES ($1, $2, $3)
		RETURNING id::text, payment_intent_id::text, amount, status::text, created_at, updated_at
	`, paymentIntentID, strings.TrimSpace(reason), amount).
		Scan(&refund.ID, &refund.PaymentIntentID, &refund.Amount, &refund.Status, &refund.CreatedAt, &refund.UpdatedAt); err != nil {
		return refundEntity.Refund{}, err
	}
	refund.MerchantID = merchantID
	refund.Reason = strings.TrimSpace(reason)

	if err := tx.Commit(); err != nil {
		return refundEntity.Refund{}, err
	}
	normalizeRefundTimes(&refund)
	return refund, nil
}

func (r *RefundRepository) ListMerchantRefunds(merchantID, status string) []refundEntity.Refund {
	base := `
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		JOIN merchants m ON m.id = inv.merchant_id AND m.deleted_at IS NULL
		JOIN users u ON m.user_id = u.id AND u.deleted_at IS NULL
		WHERE r.deleted_at IS NULL AND inv.merchant_id = $1
	`
	args := []any{merchantID}
	if status != "" {
		base += " AND r.status=$2"
		args = append(args, status)
	}
	base += " ORDER BY r.created_at DESC"

	rows, err := r.db.Query(base, args...)
	if err != nil {
		return []refundEntity.Refund{}
	}
	defer rows.Close()

	items := make([]refundEntity.Refund, 0)
	for rows.Next() {
		var item refundEntity.Refund
		var invoiceNumber string
		var merchantName string
		if err := rows.Scan(&item.ID, &item.PaymentIntentID, &item.MerchantID, &item.Reason, &item.Status, &item.Amount, &invoiceNumber, &item.CreatedAt, &item.UpdatedAt, &merchantName); err == nil {
			item.InvoiceNumber = &invoiceNumber
			item.MerchantName = &merchantName
			normalizeRefundTimes(&item)
			items = append(items, item)
		}
	}
	return items
}

func (r *RefundRepository) ListRefunds(status string) []refundEntity.Refund {
	base := `
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		JOIN merchants m ON m.id = inv.merchant_id AND m.deleted_at IS NULL
		JOIN users u ON m.user_id = u.id AND u.deleted_at IS NULL
		WHERE r.deleted_at IS NULL
	`
	args := []any{}
	if status != "" {
		base += " AND r.status=$1"
		args = append(args, strings.ToUpper(status))
	}
	base += " ORDER BY r.created_at DESC"

	rows, err := r.db.Query(base, args...)
	if err != nil {
		return []refundEntity.Refund{}
	}
	defer rows.Close()

	items := make([]refundEntity.Refund, 0)
	for rows.Next() {
		var item refundEntity.Refund
		var invoiceNumber string
		var merchantName string
		if err := rows.Scan(&item.ID, &item.PaymentIntentID, &item.MerchantID, &item.Reason, &item.Status, &item.Amount, &invoiceNumber, &item.CreatedAt, &item.UpdatedAt, &merchantName); err == nil {
			item.InvoiceNumber = &invoiceNumber
			item.MerchantName = &merchantName
			normalizeRefundTimes(&item)
			items = append(items, item)
		}
	}
	return items
}

func (r *RefundRepository) ReviewRefund(refundID string, approved bool) (refundEntity.Refund, error) {
	nextStatus := refundEntity.RefundRejected
	if approved {
		nextStatus = refundEntity.RefundApproved
	}

	res, err := r.db.Exec(`
		UPDATE refunds
		SET status=$1
		WHERE id=$2 AND status='REQUESTED' AND deleted_at IS NULL
	`, string(nextStatus), refundID)
	if err != nil {
		return refundEntity.Refund{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return refundEntity.Refund{}, errors.New("refund already reviewed or not found")
	}

	refund, ok := r.getRefundByID(refundID)
	if !ok {
		return refundEntity.Refund{}, errors.New("refund not found")
	}
	return refund, nil
}

func (r *RefundRepository) ProcessRefund(refundID string, nextStatus refundEntity.RefundStatus) (refundEntity.Refund, walletEntity.Merchant, error) {
	ctx := context.Background()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, err
	}
	defer tx.Rollback()

	var refund refundEntity.Refund
	var invoiceNumber string
	if err := tx.QueryRowContext(ctx, `
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			r.amount, inv.invoice_number, r.created_at, r.updated_at
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE r.id = $1 AND r.deleted_at IS NULL
		FOR UPDATE OF r
	`, refundID).Scan(&refund.ID, &refund.PaymentIntentID, &refund.MerchantID, &refund.Reason, &refund.Status, &refund.Amount, &invoiceNumber, &refund.CreatedAt, &refund.UpdatedAt); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("refund not found")
	}
	refund.InvoiceNumber = &invoiceNumber
	if refund.Status != refundEntity.RefundApproved {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("refund must be approved before processing")
	}

	var merchant walletEntity.Merchant
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, user_id::text, balance, created_at, updated_at
		FROM merchants
		WHERE id=$1 AND deleted_at IS NULL
		FOR UPDATE
	`, refund.MerchantID).Scan(&merchant.ID, &merchant.UserID, &merchant.Balance, &merchant.CreatedAt, &merchant.UpdatedAt); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("merchant not found")
	}
	if nextStatus == refundEntity.RefundSuccess && merchant.Balance < refund.Amount {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("insufficient merchant balance")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE refunds SET status=$1 WHERE id=$2`, string(nextStatus), refundID); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, err
	}

	if nextStatus == refundEntity.RefundSuccess {
		merchantUUID, err := uuid.Parse(refund.MerchantID)
		if err != nil {
			return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("invalid merchant id")
		}
		walletAcct, err := r.ledgerRepo.GetAccountByMerchantID(ctx, merchantUUID)
		if err != nil {
			return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("merchant ledger account not found")
		}
		posting := ledgerEntity.Posting{
			Reference:   "refund_" + refundID,
			Description: "Refund settlement",
			Entries: []ledgerEntity.Entry{
				{AccountID: walletAcct.ID, Direction: ledgerEntity.Credit, Amount: refund.Amount, Currency: "IDR"},
				{AccountID: ledgerEntity.RefundsExpenseAccountID, Direction: ledgerEntity.Debit, Amount: refund.Amount, Currency: "IDR"},
			},
		}
		if err := ledgerSvc.ValidatePosting(posting); err != nil {
			return refundEntity.Refund{}, walletEntity.Merchant{}, err
		}
		if _, err := r.ledgerRepo.Post(ctx, tx, posting); err != nil {
			return refundEntity.Refund{}, walletEntity.Merchant{}, err
		}
		// Sync merchants.balance cache
		if _, err := tx.ExecContext(ctx,
			`UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2 AND deleted_at IS NULL`,
			walletAcct.ID, refund.MerchantID); err != nil {
			return refundEntity.Refund{}, walletEntity.Merchant{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, err
	}
	refund, _ = r.getRefundByID(refundID)
	merchant, _ = r.getMerchantByUserID(merchant.UserID)
	return refund, merchant, nil
}

func (r *RefundRepository) getMerchantByUserID(userID string) (walletEntity.Merchant, bool) {
	var merchant walletEntity.Merchant
	err := r.db.QueryRow(`
		SELECT id::text, user_id::text, balance, created_at, updated_at
		FROM merchants
		WHERE user_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`, userID).Scan(&merchant.ID, &merchant.UserID, &merchant.Balance, &merchant.CreatedAt, &merchant.UpdatedAt)
	if err != nil {
		return walletEntity.Merchant{}, false
	}
	normalizeMerchantTimes(&merchant)
	return merchant, true
}

func (r *RefundRepository) getRefundByID(id string) (refundEntity.Refund, bool) {
	var refund refundEntity.Refund
	var invoiceNumber string
	var merchantName string
	err := r.db.QueryRow(`
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		JOIN merchants m ON m.id = inv.merchant_id AND m.deleted_at IS NULL
		JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL
		WHERE r.id = $1 AND r.deleted_at IS NULL
	`, id).Scan(&refund.ID, &refund.PaymentIntentID, &refund.MerchantID, &refund.Reason, &refund.Status, &refund.Amount, &invoiceNumber, &refund.CreatedAt, &refund.UpdatedAt, &merchantName)
	if err != nil {
		return refundEntity.Refund{}, false
	}
	refund.InvoiceNumber = &invoiceNumber
	refund.MerchantName = &merchantName
	normalizeRefundTimes(&refund)
	return refund, true
}

func normalizeRefundTimes(refund *refundEntity.Refund) {
	refund.CreatedAt = refund.CreatedAt.UTC()
	refund.UpdatedAt = refund.UpdatedAt.UTC()
}

func normalizeMerchantTimes(merchant *walletEntity.Merchant) {
	merchant.CreatedAt = merchant.CreatedAt.UTC()
	merchant.UpdatedAt = merchant.UpdatedAt.UTC()
}
