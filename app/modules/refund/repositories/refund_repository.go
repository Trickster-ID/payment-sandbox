package repositories

import (
	"database/sql"
	"errors"
	"strings"

	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
)

type IRefundRepository interface {
	MerchantIDByUserID(userID string) (string, error)
	RequestRefund(merchantID, paymentIntentID, reason string) (refundEntity.Refund, error)
	ListRefunds(status string) []refundEntity.Refund
	ReviewRefund(refundID string, approved bool) (refundEntity.Refund, error)
	ProcessRefund(refundID string, nextStatus refundEntity.RefundStatus) (refundEntity.Refund, walletEntity.Merchant, error)
}

type RefundRepository struct {
	db *sql.DB
}

func NewRefundRepository(db *sql.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

func (r *RefundRepository) MerchantIDByUserID(userID string) (string, error) {
	merchant, found := r.getMerchantByUserID(userID)
	if !found {
		return "", errors.New("merchant not found")
	}
	return merchant.ID, nil
}

func (r *RefundRepository) RequestRefund(merchantID, paymentIntentID, reason string) (refundEntity.Refund, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return refundEntity.Refund{}, err
	}
	defer tx.Rollback()

	var paymentStatus paymentEntity.PaymentStatus
	var ownerMerchantID string
	var amount float64
	if err := tx.QueryRow(`
		SELECT pi.status::text, inv.merchant_id::text, inv.amount::double precision
		FROM payment_intents pi
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE pi.id = $1 AND pi.deleted_at IS NULL
		FOR UPDATE OF pi
	`, paymentIntentID).Scan(&paymentStatus, &ownerMerchantID, &amount); err != nil {
		return refundEntity.Refund{}, errors.New("payment intent not found")
	}
	if paymentStatus != paymentEntity.PaymentSuccess {
		return refundEntity.Refund{}, errors.New("refund can be requested for successful payment only")
	}
	if ownerMerchantID != merchantID {
		return refundEntity.Refund{}, errors.New("payment intent does not belong to merchant")
	}

	var refund refundEntity.Refund
	if err := tx.QueryRow(`
		INSERT INTO refunds (payment_intent_id, reason)
		VALUES ($1, $2)
		RETURNING id::text, payment_intent_id::text, status::text, created_at, updated_at
	`, paymentIntentID, strings.TrimSpace(reason)).
		Scan(&refund.ID, &refund.PaymentIntentID, &refund.Status, &refund.CreatedAt, &refund.UpdatedAt); err != nil {
		return refundEntity.Refund{}, err
	}
	refund.MerchantID = merchantID
	refund.Amount = amount
	refund.Reason = strings.TrimSpace(reason)

	if err := tx.Commit(); err != nil {
		return refundEntity.Refund{}, err
	}
	return refund, nil
}

func (r *RefundRepository) ListRefunds(status string) []refundEntity.Refund {
	base := `
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			inv.amount::double precision, r.created_at, r.updated_at
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
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
		if err := rows.Scan(&item.ID, &item.PaymentIntentID, &item.MerchantID, &item.Reason, &item.Status, &item.Amount, &item.CreatedAt, &item.UpdatedAt); err == nil {
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
	tx, err := r.db.Begin()
	if err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, err
	}
	defer tx.Rollback()

	var refund refundEntity.Refund
	if err := tx.QueryRow(`
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			inv.amount::double precision, r.created_at, r.updated_at
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE r.id = $1 AND r.deleted_at IS NULL
		FOR UPDATE OF r
	`, refundID).Scan(&refund.ID, &refund.PaymentIntentID, &refund.MerchantID, &refund.Reason, &refund.Status, &refund.Amount, &refund.CreatedAt, &refund.UpdatedAt); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("refund not found")
	}
	if refund.Status != refundEntity.RefundApproved {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("refund must be approved before processing")
	}

	var merchant walletEntity.Merchant
	if err := tx.QueryRow(`
		SELECT id::text, user_id::text, balance::double precision, created_at, updated_at
		FROM merchants
		WHERE id=$1 AND deleted_at IS NULL
		FOR UPDATE
	`, refund.MerchantID).Scan(&merchant.ID, &merchant.UserID, &merchant.Balance, &merchant.CreatedAt, &merchant.UpdatedAt); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("merchant not found")
	}
	if nextStatus == refundEntity.RefundSuccess && merchant.Balance < refund.Amount {
		return refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("insufficient merchant balance")
	}

	if _, err := tx.Exec(`UPDATE refunds SET status=$1 WHERE id=$2`, string(nextStatus), refundID); err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, err
	}
	if nextStatus == refundEntity.RefundSuccess {
		if _, err := tx.Exec(`UPDATE merchants SET balance = balance - $1 WHERE id=$2`, refund.Amount, merchant.ID); err != nil {
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

func (r *RefundRepository) getRefundByID(id string) (refundEntity.Refund, bool) {
	var refund refundEntity.Refund
	err := r.db.QueryRow(`
		SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text,
			inv.amount::double precision, r.created_at, r.updated_at
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE r.id = $1 AND r.deleted_at IS NULL
	`, id).Scan(&refund.ID, &refund.PaymentIntentID, &refund.MerchantID, &refund.Reason, &refund.Status, &refund.Amount, &refund.CreatedAt, &refund.UpdatedAt)
	if err != nil {
		return refundEntity.Refund{}, false
	}
	return refund, true
}
