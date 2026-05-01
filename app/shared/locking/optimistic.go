package locking

import (
	"context"
	"database/sql"
	"errors"
)

var ErrVersionConflict = errors.New("version conflict: record was modified by another transaction")

// CheckedExec runs an UPDATE with a WHERE version=expected guard.
// Returns ErrVersionConflict if 0 rows affected.
func CheckedExec(ctx context.Context, db interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}, query string, args ...any) error {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrVersionConflict
	}
	return nil
}
