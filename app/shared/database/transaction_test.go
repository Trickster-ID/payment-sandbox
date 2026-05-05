package database

// Branch map for BeginMoneyTx (Section 3.1 of the plan):
//
// BeginMoneyTx(ctx, db):
// ├── db.BeginTx fails                -> return nil, error
// ├── tx.ExecContext fails (SET LOCAL timeout)  -> return nil, error, tx.Rollback called
// └── all succeed                     -> return *sql.Tx, nil

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBeginMoneyTx(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		txIsNil bool
		errMsg  string
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. db.BeginTx fails -> error returned",
			args: args{ctx: context.Background()},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin().WillReturnError(sqlmock.ErrCancelled)
				},
			},
			wants: wants{txIsNil: true, errMsg: "canceling"},
		},
		{
			name: "2. tx.ExecContext fails (SET LOCAL statement_timeout) -> error returned, tx rolled back",
			args: args{ctx: context.Background()},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectExec(regexp.QuoteMeta("SET LOCAL statement_timeout = '5s'")).
						WillReturnError(sqlmock.ErrCancelled)
					m.ExpectRollback()
				},
			},
			wants: wants{txIsNil: true, errMsg: "canceling"},
		},
		{
			name: "3. all succeed -> transaction returned",
			args: args{ctx: context.Background()},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectExec(regexp.QuoteMeta("SET LOCAL statement_timeout = '5s'")).
						WillReturnResult(sqlmock.NewResult(0, 0))
				},
			},
			wants: wants{txIsNil: false, errMsg: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.mocks.setup(sqlMock)

			tx, err := BeginMoneyTx(tt.args.ctx, db)

			if tt.wants.txIsNil {
				assert.Nil(t, tx, "tx should be nil on error")
				assert.Error(t, err, "expected error")
				if tt.wants.errMsg != "" {
					assert.Contains(t, err.Error(), tt.wants.errMsg, "error message contains expected text")
				}
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.NotNil(t, tx, "tx should not be nil on success")
				if tx != nil {
					_ = tx.Rollback()
				}
			}

			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}
