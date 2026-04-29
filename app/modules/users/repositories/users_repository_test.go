package repositories

import (
	"database/sql"
	"payment-sandbox/app/modules/users/models/entity"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_CreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewUserRepository(db)
	now := time.Now()

	t.Run("success merchant", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
			WithArgs("Alice", "alice@example.com", "hash", "MERCHANT").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "role", "created_at"}).
				AddRow("user-1", "Alice", "alice@example.com", "MERCHANT", now))
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO merchants")).
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		user, err := repo.CreateUser("Alice", "alice@example.com", "hash", entity.RoleMerchant)
		require.NoError(t, err)
		assert.Equal(t, "user-1", user.ID)
		assert.Equal(t, "Alice", user.Name)
	})

	t.Run("success admin", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
			WithArgs("Admin", "admin@example.com", "hash", "ADMIN").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "role", "created_at"}).
				AddRow("user-2", "Admin", "admin@example.com", "ADMIN", now))
		mock.ExpectCommit()

		user, err := repo.CreateUser("Admin", "admin@example.com", "hash", entity.RoleAdmin)
		require.NoError(t, err)
		assert.Equal(t, "user-2", user.ID)
	})

	t.Run("email already registered", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505"}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
			WithArgs("Bob", "bob@example.com", "hash", "MERCHANT").
			WillReturnError(pgErr)
		mock.ExpectRollback()

		_, err := repo.CreateUser("Bob", "bob@example.com", "hash", entity.RoleMerchant)
		require.Error(t, err)
		assert.ErrorContains(t, err, "email already registered")
	})
}

func TestUserRepository_FindUserByEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewUserRepository(db)
	now := time.Now()
	passwordHash := "bcrypt-hash"

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
			WithArgs("alice@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "password_hash", "role", "created_at"}).
				AddRow("user-1", "Alice", "alice@example.com", passwordHash, "MERCHANT", now))

		user, found := repo.FindUserByEmail("alice@example.com")
		require.True(t, found)
		assert.Equal(t, "user-1", user.ID)
		assert.Equal(t, passwordHash, user.PasswordHash)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
			WithArgs("notfound@example.com").
			WillReturnError(sql.ErrNoRows)

		user, found := repo.FindUserByEmail("notfound@example.com")
		require.False(t, found)
		assert.Empty(t, user.ID)
	})

	t.Run("case insensitive", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
			WithArgs("ALICE@EXAMPLE.COM").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "password_hash", "role", "created_at"}).
				AddRow("user-1", "Alice", "alice@example.com", passwordHash, "MERCHANT", now))

		user, found := repo.FindUserByEmail("ALICE@EXAMPLE.COM")
		require.True(t, found)
		assert.Equal(t, "user-1", user.ID)
	})
}