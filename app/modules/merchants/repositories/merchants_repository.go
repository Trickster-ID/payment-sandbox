package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"payment-sandbox/app/modules/merchants/models/entity"
)

//go:generate mockery --name IMerchantsRepository --output mocks --outpkg mocks --case underscore

type IMerchantsRepository interface {
	ListMerchants(ctx context.Context, search string, page, limit int) ([]entity.MerchantSummary, int, error)
}

type MerchantsRepository struct {
	db *sql.DB
}

func NewMerchantsRepository(db *sql.DB) *MerchantsRepository {
	return &MerchantsRepository{db: db}
}

func (r *MerchantsRepository) ListMerchants(ctx context.Context, search string, page, limit int) ([]entity.MerchantSummary, int, error) {
	args := []any{}
	where := "WHERE m.deleted_at IS NULL AND u.deleted_at IS NULL"

	if search != "" {
		args = append(args, search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (u.name ILIKE $%d OR u.email ILIKE $%d)", n, n)
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM merchants m JOIN users u ON u.id = m.user_id %s`, where)
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count merchants: %w", err)
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	dataQ := fmt.Sprintf(`
		SELECT m.id::text, u.name, u.email
		FROM merchants m
		JOIN users u ON u.id = m.user_id
		%s
		ORDER BY u.name ASC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, dataQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query merchants: %w", err)
	}
	defer rows.Close()

	items := make([]entity.MerchantSummary, 0)
	for rows.Next() {
		var m entity.MerchantSummary
		if err := rows.Scan(&m.ID, &m.Name, &m.Email); err != nil {
			return nil, 0, fmt.Errorf("scan merchant: %w", err)
		}
		items = append(items, m)
	}
	return items, total, nil
}
