package repositories

import (
	"context"
	"database/sql"
	"errors"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerRepo "payment-sandbox/app/modules/ledger/repositories"
	ledgerSvc "payment-sandbox/app/modules/ledger/services"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"

	"github.com/google/uuid"
)

type IWalletRepository interface {
	GetMerchantWallet(userID string) (walletEntity.Merchant, error)
	MerchantIDByUserID(userID string) (string, error)
	CreateTopup(merchantID string, amount int64) (walletEntity.Topup, error)
	ListTopups() []walletEntity.Topup
	UpdateTopupStatus(topupID string, nextStatus paymentEntity.PaymentStatus) (walletEntity.Topup, error)
}

type WalletRepository struct {
	db         *sql.DB
	ledgerRepo ledgerRepo.IRepository
}

func NewWalletRepository(db *sql.DB, ledger ledgerRepo.IRepository) *WalletRepository {
	return &WalletRepository{db: db, ledgerRepo: ledger}
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

func (r *WalletRepository) CreateTopup(merchantID string, amount int64) (walletEntity.Topup, error) {
	var topup walletEntity.Topup
	err := r.db.QueryRow(`
		INSERT INTO topups (merchant_id, amount)
		VALUES ($1, $2)
		RETURNING id::text, merchant_id::text, amount, status::text, created_at, updated_at
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
		SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at
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
	ctx := context.Background()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return walletEntity.Topup{}, err
	}
	defer tx.Rollback()

	var merchantID string
	var amount int64
	var currentStatus paymentEntity.PaymentStatus
	if err := tx.QueryRowContext(ctx, `
		SELECT merchant_id::text, amount, status::text
		FROM topups
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, topupID).Scan(&merchantID, &amount, &currentStatus); err != nil {
		return walletEntity.Topup{}, errors.New("topup not found")
	}
	if currentStatus != paymentEntity.PaymentPending {
		return walletEntity.Topup{}, errors.New("topup already finalized")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE topups SET status=$1 WHERE id=$2 AND deleted_at IS NULL`, string(nextStatus), topupID); err != nil {
		return walletEntity.Topup{}, err
	}

	if nextStatus == paymentEntity.PaymentSuccess {
		merchantUUID, err := uuid.Parse(merchantID)
		if err != nil {
			return walletEntity.Topup{}, errors.New("invalid merchant id")
		}
		walletAcct, err := r.ledgerRepo.GetAccountByMerchantID(ctx, merchantUUID)
		if err != nil {
			return walletEntity.Topup{}, errors.New("merchant ledger account not found")
		}
		posting := ledgerEntity.Posting{
			Reference:   "topup_" + topupID,
			Description: "Topup completion",
			Entries: []ledgerEntity.Entry{
				{AccountID: walletAcct.ID, Direction: ledgerEntity.Debit, Amount: amount, Currency: "IDR"},
				{AccountID: ledgerEntity.TopupClearingAccountID, Direction: ledgerEntity.Credit, Amount: amount, Currency: "IDR"},
			},
		}
		if err := ledgerSvc.ValidatePosting(posting); err != nil {
			return walletEntity.Topup{}, err
		}
		if _, err := r.ledgerRepo.Post(ctx, tx, posting); err != nil {
			return walletEntity.Topup{}, err
		}
		// Sync merchants.balance cache from the authoritative accounts.balance
		if _, err := tx.ExecContext(ctx,
			`UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2 AND deleted_at IS NULL`,
			walletAcct.ID, merchantID); err != nil {
			return walletEntity.Topup{}, err
		}
	}

	var topup walletEntity.Topup
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at
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

func normalizeMerchantTimes(merchant *walletEntity.Merchant) {
	merchant.CreatedAt = merchant.CreatedAt.UTC()
	merchant.UpdatedAt = merchant.UpdatedAt.UTC()
}

func normalizeTopupTimes(topup *walletEntity.Topup) {
	topup.CreatedAt = topup.CreatedAt.UTC()
	topup.UpdatedAt = topup.UpdatedAt.UTC()
}
