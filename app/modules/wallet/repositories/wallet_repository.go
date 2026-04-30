package repositories

import (
	"database/sql"
	"errors"

	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
)

type IWalletRepository interface {
	GetMerchantWallet(userID string) (walletEntity.Merchant, error)
	MerchantIDByUserID(userID string) (string, error)
	CreateTopup(merchantID string, amount float64) (walletEntity.Topup, error)
	ListTopups() []walletEntity.Topup
	UpdateTopupStatus(topupID string, nextStatus paymentEntity.PaymentStatus) (walletEntity.Topup, error)
}

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) GetMerchantWallet(userID string) (walletEntity.Merchant, error) {
	merchant, found := r.getMerchantByUserID(userID)
	if !found {
		return walletEntity.Merchant{}, errors.New("merchant wallet not found")
	}
	return merchant, nil
}

func (r *WalletRepository) MerchantIDByUserID(userID string) (string, error) {
	merchant, found := r.getMerchantByUserID(userID)
	if !found {
		return "", errors.New("merchant not found")
	}
	return merchant.ID, nil
}

func (r *WalletRepository) CreateTopup(merchantID string, amount float64) (walletEntity.Topup, error) {
	var topup walletEntity.Topup
	err := r.db.QueryRow(`
		INSERT INTO topups (merchant_id, amount)
		VALUES ($1, $2)
		RETURNING id::text, merchant_id::text, amount::double precision, status::text, created_at, updated_at
	`, merchantID, amount).
		Scan(&topup.ID, &topup.MerchantID, &topup.Amount, &topup.Status, &topup.CreatedAt, &topup.UpdatedAt)
	if err != nil {
		return walletEntity.Topup{}, err
	}
	normalizeTopupTimes(&topup)
	return topup, nil
}

func (r *WalletRepository) ListTopups() []walletEntity.Topup {
	rows, err := r.db.Query(`
		SELECT id::text, merchant_id::text, amount::double precision, status::text, created_at, updated_at
		FROM topups
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return []walletEntity.Topup{}
	}
	defer rows.Close()

	items := make([]walletEntity.Topup, 0)
	for rows.Next() {
		var item walletEntity.Topup
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Amount, &item.Status, &item.CreatedAt, &item.UpdatedAt); err == nil {
			normalizeTopupTimes(&item)
			items = append(items, item)
		}
	}
	return items
}

func (r *WalletRepository) UpdateTopupStatus(topupID string, nextStatus paymentEntity.PaymentStatus) (walletEntity.Topup, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return walletEntity.Topup{}, err
	}
	defer tx.Rollback()

	var merchantID string
	var amount float64
	var currentStatus paymentEntity.PaymentStatus
	if err := tx.QueryRow(`
		SELECT merchant_id::text, amount::double precision, status::text
		FROM topups
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, topupID).Scan(&merchantID, &amount, &currentStatus); err != nil {
		return walletEntity.Topup{}, errors.New("topup not found")
	}
	if currentStatus != paymentEntity.PaymentPending {
		return walletEntity.Topup{}, errors.New("topup already finalized")
	}

	if _, err := tx.Exec(`UPDATE topups SET status=$1 WHERE id=$2 AND deleted_at IS NULL`, string(nextStatus), topupID); err != nil {
		return walletEntity.Topup{}, err
	}
	if nextStatus == paymentEntity.PaymentSuccess {
		if _, err := tx.Exec(`UPDATE merchants SET balance = balance + $1 WHERE id=$2 AND deleted_at IS NULL`, amount, merchantID); err != nil {
			return walletEntity.Topup{}, err
		}
	}

	var topup walletEntity.Topup
	if err := tx.QueryRow(`
		SELECT id::text, merchant_id::text, amount::double precision, status::text, created_at, updated_at
		FROM topups WHERE id=$1
	`, topupID).Scan(&topup.ID, &topup.MerchantID, &topup.Amount, &topup.Status, &topup.CreatedAt, &topup.UpdatedAt); err != nil {
		return walletEntity.Topup{}, err
	}

	if err := tx.Commit(); err != nil {
		return walletEntity.Topup{}, err
	}
	normalizeTopupTimes(&topup)
	return topup, nil
}

func (r *WalletRepository) getMerchantByUserID(userID string) (walletEntity.Merchant, bool) {
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
	normalizeMerchantTimes(&merchant)
	return merchant, true
}

func normalizeMerchantTimes(merchant *walletEntity.Merchant) {
	merchant.CreatedAt = merchant.CreatedAt.UTC()
	merchant.UpdatedAt = merchant.UpdatedAt.UTC()
}

func normalizeTopupTimes(topup *walletEntity.Topup) {
	topup.CreatedAt = topup.CreatedAt.UTC()
	topup.UpdatedAt = topup.UpdatedAt.UTC()
}
