package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"payment-sandbox/app/modules/ledger/models/entity"
)

//go:generate mockery --name IRepository --output mocks --outpkg mocks --case underscore

type IRepository interface {
	Post(ctx context.Context, tx *sql.Tx, p entity.Posting) (uuid.UUID, error)
	Reverse(ctx context.Context, tx *sql.Tx, originalRef, reason string, actor uuid.UUID) (uuid.UUID, error)
	GetAccountByMerchantID(ctx context.Context, merchantID uuid.UUID) (entity.Account, error)
	ListEntriesByAccount(ctx context.Context, accountID uuid.UUID, filter entity.EntryFilter, page, limit int) ([]entity.EntryWithTxn, int, error)
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Post(ctx context.Context, tx *sql.Tx, p entity.Posting) (uuid.UUID, error) {
	metaBytes, _ := json.Marshal(p.Metadata)

	var txID uuid.UUID
	err := tx.QueryRowContext(ctx, `
		INSERT INTO ledger_transactions (reference, description, metadata, created_by)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING id
	`, p.Reference, p.Description, string(metaBytes), p.CreatedBy).Scan(&txID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert ledger_transactions: %w", err)
	}

	for _, e := range p.Entries {
		var balance int64
		var accType string
		err := tx.QueryRowContext(ctx, `
			SELECT balance, type::text FROM accounts WHERE id=$1 AND is_active=true FOR UPDATE
		`, e.AccountID).Scan(&balance, &accType)
		if err != nil {
			return uuid.Nil, fmt.Errorf("lock account %s: %w", e.AccountID, err)
		}

		// Sign convention:
		//   asset/expense:           debit increases, credit decreases
		//   liability/revenue/equity: credit increases, debit decreases
		var delta int64
		assetLike := accType == "asset" || accType == "expense"
		if (e.Direction == entity.Debit && assetLike) || (e.Direction == entity.Credit && !assetLike) {
			delta = e.Amount
		} else {
			delta = -e.Amount
		}
		newBalance := balance + delta

		if assetLike && newBalance < 0 {
			return uuid.Nil, fmt.Errorf("insufficient balance on account %s", e.AccountID)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE accounts SET balance=$1, version=version+1, updated_at=now() WHERE id=$2
		`, newBalance, e.AccountID); err != nil {
			return uuid.Nil, fmt.Errorf("update account balance: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ledger_entries (transaction_id, account_id, direction, amount, currency, balance_after)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, txID, e.AccountID, string(e.Direction), e.Amount, e.Currency, newBalance); err != nil {
			return uuid.Nil, fmt.Errorf("insert ledger entry: %w", err)
		}
	}

	return txID, nil
}

func (r *Repository) Reverse(ctx context.Context, tx *sql.Tx, originalRef, reason string, actor uuid.UUID) (uuid.UUID, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT le.account_id, le.direction, le.amount, le.currency
		FROM ledger_entries le
		JOIN ledger_transactions lt ON lt.id = le.transaction_id
		WHERE lt.reference = $1
	`, originalRef)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()

	var entries []entity.Entry
	for rows.Next() {
		var e entity.Entry
		var dir string
		if err := rows.Scan(&e.AccountID, &dir, &e.Amount, &e.Currency); err != nil {
			return uuid.Nil, err
		}
		if entity.Direction(dir) == entity.Debit {
			e.Direction = entity.Credit
		} else {
			e.Direction = entity.Debit
		}
		entries = append(entries, e)
	}
	return r.Post(ctx, tx, entity.Posting{
		Reference:   "reversal_" + originalRef,
		Description: "Reversal: " + reason,
		Entries:     entries,
		Metadata:    map[string]any{"reverses": originalRef},
		CreatedBy:   actor,
	})
}

func (r *Repository) ListEntriesByAccount(ctx context.Context, accountID uuid.UUID, filter entity.EntryFilter, page, limit int) ([]entity.EntryWithTxn, int, error) {
	args := []any{accountID}
	conds := []string{"e.account_id = $1"}

	if filter.From != nil {
		args = append(args, *filter.From)
		conds = append(conds, fmt.Sprintf("e.created_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		conds = append(conds, fmt.Sprintf("e.created_at <= $%d", len(args)))
	}
	if filter.Direction != nil {
		args = append(args, *filter.Direction)
		conds = append(conds, fmt.Sprintf("e.direction = $%d", len(args)))
	}
	if filter.ReferencePrefix != nil {
		args = append(args, *filter.ReferencePrefix+"%")
		conds = append(conds, fmt.Sprintf("t.reference LIKE $%d", len(args)))
	}

	where := strings.Join(conds, " AND ")

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM ledger_entries e JOIN ledger_transactions t ON t.id = e.transaction_id WHERE %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ledger entries: %w", err)
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	dataQuery := fmt.Sprintf(`
		SELECT e.id, e.transaction_id, e.account_id, e.direction, e.amount, e.currency, e.balance_after, e.created_at,
		       t.reference, t.description, t.metadata
		FROM ledger_entries e
		JOIN ledger_transactions t ON t.id = e.transaction_id
		WHERE %s
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query ledger entries: %w", err)
	}
	defer rows.Close()

	items := make([]entity.EntryWithTxn, 0)
	for rows.Next() {
		var item entity.EntryWithTxn
		var dir string
		var metaBytes []byte
		if err := rows.Scan(
			&item.ID, &item.TxnID, &item.AccountID, &dir, &item.Amount, &item.Currency, &item.BalanceAfter, &item.CreatedAt,
			&item.Reference, &item.Description, &metaBytes,
		); err != nil {
			return nil, 0, fmt.Errorf("scan ledger entry: %w", err)
		}
		item.Direction = entity.Direction(dir)
		item.CreatedAt = item.CreatedAt.UTC()
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &item.Metadata)
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (r *Repository) GetAccountByMerchantID(ctx context.Context, merchantID uuid.UUID) (entity.Account, error) {
	var a entity.Account
	var mid uuid.UUID
	err := r.db.QueryRowContext(ctx, `
		SELECT id, merchant_id, name, type::text, currency, balance, version, is_active, created_at, updated_at
		FROM accounts WHERE merchant_id=$1 AND is_active=true
	`, merchantID).Scan(&a.ID, &mid, &a.Name, &a.Type, &a.Currency, &a.Balance, &a.Version, &a.IsActive, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return entity.Account{}, err
	}
	a.MerchantID = &mid
	return a, nil
}
