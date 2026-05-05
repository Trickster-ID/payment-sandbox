package repositories

// Branch analysis for MerchantsRepository.ListMerchants:
// ├── search == "" → WHERE without ILIKE clause, no prefix arg
// ├── search != "" → WHERE with ILIKE $1, args=[search+"%"]
// ├── r.db.QueryRowContext(countQ).Scan → error → "count merchants: %w"
// ├── r.db.QueryContext(dataQ) → error → "query merchants: %w"
// └── rows.Scan(&m.ID, &m.Name, &m.Email) → error → "scan merchant: %w"
//     └── all rows scanned successfully → items assembled, empty slice if no rows

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"payment-sandbox/app/modules/merchants/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerchantsRepository_ListMerchants(t *testing.T) {
	// not parallel: sqlmock expectations are sequential per db instance;
	// each subtest creates its own db to avoid cross-contamination.

	ctx := context.Background()
	cols := []string{"id", "name", "email"}

	type args struct {
		search string
		page   int
		limit  int
	}
	type wants struct {
		items      []entity.MerchantSummary
		total      int
		errContains string
	}

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. no search no rows -> empty items total=0",
			args: args{search: "", page: 1, limit: 20},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				m.ExpectQuery("SELECT m.id").
					WithArgs(20, 0).
					WillReturnRows(sqlmock.NewRows(cols))
			},
			wants: wants{items: []entity.MerchantSummary{}, total: 0},
		},
		{
			name: "2. no search two rows -> items returned with correct total",
			args: args{search: "", page: 1, limit: 20},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				m.ExpectQuery("SELECT m.id").
					WithArgs(20, 0).
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("m1", "Alice", "alice@example.com").
						AddRow("m2", "Bob", "bob@example.com"))
			},
			wants: wants{
				items: []entity.MerchantSummary{
					{ID: "m1", Name: "Alice", Email: "alice@example.com"},
					{ID: "m2", Name: "Bob", Email: "bob@example.com"},
				},
				total: 2,
			},
		},
		{
			name: "3. search=ali -> ILIKE clause added with prefix arg ali%",
			args: args{search: "ali", page: 1, limit: 20},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WithArgs("ali%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				m.ExpectQuery("SELECT m.id").
					WithArgs("ali%", 20, 0).
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("m1", "Alice", "alice@example.com"))
			},
			wants: wants{
				items: []entity.MerchantSummary{{ID: "m1", Name: "Alice", Email: "alice@example.com"}},
				total: 1,
			},
		},
		{
			name: "4. page=2 limit=5 -> offset=5 passed to query",
			args: args{search: "", page: 2, limit: 5},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))
				m.ExpectQuery("SELECT m.id").
					WithArgs(5, 5). // limit=5, offset=(2-1)*5=5
					WillReturnRows(sqlmock.NewRows(cols))
			},
			wants: wants{items: []entity.MerchantSummary{}, total: 12},
		},
		{
			name: "5. count query error -> error wraps count merchants",
			args: args{search: "", page: 1, limit: 20},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errContains: "count merchants:"},
		},
		{
			name: "6. data query error -> error wraps query merchants",
			args: args{search: "", page: 1, limit: 20},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				m.ExpectQuery("SELECT m.id").
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errContains: "query merchants:"},
		},
		{
			name: "7. scan error (column mismatch) -> error wraps scan merchant",
			args: args{search: "", page: 1, limit: 20},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				// Return only 2 columns; Scan expects 3 → scan error
				m.ExpectQuery("SELECT m.id").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
						AddRow("m1", "Alice"))
			},
			wants: wants{errContains: "scan merchant:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			repo := NewMerchantsRepository(db)
			items, total, err := repo.ListMerchants(ctx, tt.args.search, tt.args.page, tt.args.limit)

			if tt.wants.errContains != "" {
				require.Error(t, err, "expected an error")
				assert.ErrorContains(t, err, tt.wants.errContains, "error message")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.total, total, "total")
				assert.Equal(t, tt.wants.items, items, "items")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}
