package database

import (
	"context"
	"database/sql"
)

// BeginMoneyTx starts a transaction with a 5s statement timeout to prevent
// runaway locks from hanging the system.
func BeginMoneyTx(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL statement_timeout = '5s'"); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}
