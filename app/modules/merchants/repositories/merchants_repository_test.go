package repositories

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerchantsRepository_ListMerchants(t *testing.T) {
	ctx := context.Background()
	cols := []string{"id", "name", "email"}

	t.Run("no search — returns all paginated", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
		mock.ExpectQuery("SELECT m.id").
			WillReturnRows(sqlmock.NewRows(cols).
				AddRow("m1", "Alice", "alice@example.com").
				AddRow("m2", "Bob", "bob@example.com"))

		repo := NewMerchantsRepository(db)
		items, total, err := repo.ListMerchants(ctx, "", 1, 20)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, items, 2)
		assert.Equal(t, "Alice", items[0].Name)
	})

	t.Run("with search — passes prefix arg", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WithArgs("ali%").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT m.id").
			WithArgs("ali%", 20, 0).
			WillReturnRows(sqlmock.NewRows(cols).
				AddRow("m1", "Alice", "alice@example.com"))

		repo := NewMerchantsRepository(db)
		items, total, err := repo.ListMerchants(ctx, "ali", 1, 20)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, items, 1)
		assert.Equal(t, "m1", items[0].ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("pagination offset — page 2 limit 5", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))
		mock.ExpectQuery("SELECT m.id").
			WithArgs(5, 5). // limit=5, offset=(2-1)*5=5
			WillReturnRows(sqlmock.NewRows(cols))

		repo := NewMerchantsRepository(db)
		_, total, err := repo.ListMerchants(ctx, "", 2, 5)
		require.NoError(t, err)
		assert.Equal(t, 12, total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("count error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WillReturnError(sql.ErrConnDone)

		repo := NewMerchantsRepository(db)
		_, _, err = repo.ListMerchants(ctx, "", 1, 20)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "count merchants")
	})

	t.Run("query error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT m.id").
			WillReturnError(sql.ErrConnDone)

		repo := NewMerchantsRepository(db)
		_, _, err = repo.ListMerchants(ctx, "", 1, 20)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query merchants")
	})
}
