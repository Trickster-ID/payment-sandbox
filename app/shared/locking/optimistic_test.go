package locking

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockDB is a test mock for the db interface parameter
type mockDB struct {
	mock.Mock
}

func (m *mockDB) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	ret := m.Called(ctx, q, args)
	if ret.Get(0) == nil {
		return nil, ret.Error(1)
	}
	return ret.Get(0).(sql.Result), ret.Error(1)
}

// mockResult implements sql.Result
type mockResult struct {
	affectedRows int64
}

func (m mockResult) LastInsertId() (int64, error) { return 0, nil }
func (m mockResult) RowsAffected() (int64, error) { return m.affectedRows, nil }

func TestCheckedExec(t *testing.T) {
	type fields struct {
		db *mockDB
	}
	type args struct {
		ctx   context.Context
		query string
		args  []any
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		errNil bool
		errIs  error
		errMsg string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. ExecContext returns error -> propagates",
			fields: fields{db: new(mockDB)},
			args: args{
				ctx:   context.Background(),
				query: "UPDATE t SET v=2 WHERE id=1 AND v=1",
				args:  []any{},
			},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.db.On("ExecContext", a.ctx, a.query, a.args).
						Return(nil, errors.New("connection timeout")).
						Once()
				},
			},
			wants: wants{errNil: false, errMsg: "connection timeout"},
		},
		{
			name:   "2. zero rows affected -> ErrVersionConflict",
			fields: fields{db: new(mockDB)},
			args: args{
				ctx:   context.Background(),
				query: "UPDATE t SET v=2 WHERE id=1 AND v=99",
				args:  []any{},
			},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.db.On("ExecContext", a.ctx, a.query, a.args).
						Return(mockResult{affectedRows: 0}, nil).
						Once()
				},
			},
			wants: wants{errNil: false, errIs: ErrVersionConflict},
		},
		{
			name:   "3. one row affected -> success nil",
			fields: fields{db: new(mockDB)},
			args: args{
				ctx:   context.Background(),
				query: "UPDATE t SET v=2 WHERE id=1 AND v=1",
				args:  []any{},
			},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.db.On("ExecContext", a.ctx, a.query, a.args).
						Return(mockResult{affectedRows: 1}, nil).
						Once()
				},
			},
			wants: wants{errNil: true},
		},
		{
			name:   "4. multiple rows affected -> success nil",
			fields: fields{db: new(mockDB)},
			args: args{
				ctx:   context.Background(),
				query: "UPDATE t SET status='done' WHERE status='pending'",
				args:  []any{},
			},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.db.On("ExecContext", a.ctx, a.query, a.args).
						Return(mockResult{affectedRows: 10}, nil).
						Once()
				},
			},
			wants: wants{errNil: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			err := CheckedExec(tt.args.ctx, tt.fields.db, tt.args.query, tt.args.args...)

			if tt.wants.errNil {
				assert.NoError(t, err, "expected no error")
			} else {
				assert.Error(t, err, "expected error")
				if tt.wants.errIs != nil {
					assert.ErrorIs(t, err, tt.wants.errIs, "error type")
				}
				if tt.wants.errMsg != "" {
					assert.ErrorContains(t, err, tt.wants.errMsg, "error message")
				}
			}

			tt.fields.db.AssertExpectations(t)
		})
	}
}
