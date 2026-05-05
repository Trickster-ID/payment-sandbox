package repositories

// Branch map (Section 3.1 of the plan):
//
// CreateUser:
// ├── r.db.Begin() fails                                   -> return User{}, error
// ├── INSERT users scan fails (non-pgError)                -> isUniqueViolation=false, return User{}, error
// ├── INSERT users scan fails (pgError, code != 23505)     -> isUniqueViolation=false, return User{}, pgErr
// ├── INSERT users scan fails (pgError, code == 23505)     -> return User{}, "email already registered"
// ├── role==MERCHANT, INSERT merchants fails               -> return User{}, error
// ├── tx.Commit() fails                                    -> return User{}, error
// ├── role==MERCHANT, all succeed                          -> return User, nil (with merchants row)
// └── role==ADMIN, all succeed (no merchants insert)       -> return User, nil
//
// FindUserByEmail:
// ├── db query fails (not found)  -> return User{}, false
// └── db query succeeds           -> return User, true
//
// isUniqueViolation (exercised via CreateUser branches):
// ├── err is not a *pgconn.PgError     -> false  (covered by scan non-pgError case)
// ├── pgError.Code == "23505"          -> true   (covered by unique-violation case)
// └── pgError.Code != "23505"          -> false  (covered by wrong-code pgError case)

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"payment-sandbox/app/modules/users/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_CreateUser(t *testing.T) {
	now := time.Now()

	type args struct {
		name         string
		email        string
		passwordHash string
		role         entity.Role
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		userID string
		err    string
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. db.Begin fails -> error returned",
			args: args{name: "Alice", email: "alice@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin().WillReturnError(sql.ErrConnDone)
				},
			},
			wants: wants{err: "sql: connection is already closed"},
		},
		{
			name: "2. INSERT users scan fails (non-pgError) -> error returned",
			args: args{name: "Alice", email: "alice@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Alice", "alice@example.com", "hash", "MERCHANT").
						WillReturnError(sql.ErrConnDone)
					m.ExpectRollback()
				},
			},
			wants: wants{err: "sql: connection is already closed"},
		},
		{
			name: "3. INSERT users fails with non-23505 pgError -> error returned",
			args: args{name: "Alice", email: "alice@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Alice", "alice@example.com", "hash", "MERCHANT").
						WillReturnError(&pgconn.PgError{Code: "23503"}) // foreign key violation, not unique
					m.ExpectRollback()
				},
			},
			wants: wants{err: ":  (SQLSTATE 23503)"},
		},
		{
			name: "4. INSERT users fails with unique violation (23505) -> email already registered",
			args: args{name: "Bob", email: "bob@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Bob", "bob@example.com", "hash", "MERCHANT").
						WillReturnError(&pgconn.PgError{Code: "23505"})
					m.ExpectRollback()
				},
			},
			wants: wants{err: "email already registered"},
		},
		{
			name: "5. role MERCHANT, INSERT merchants fails -> error returned",
			args: args{name: "Alice", email: "alice@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Alice", "alice@example.com", "hash", "MERCHANT").
						WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "role", "created_at"}).
							AddRow("user-1", "Alice", "alice@example.com", "MERCHANT", now))
					m.ExpectExec(regexp.QuoteMeta("INSERT INTO merchants")).
						WithArgs("user-1").
						WillReturnError(sql.ErrConnDone)
					m.ExpectRollback()
				},
			},
			wants: wants{err: "sql: connection is already closed"},
		},
		{
			name: "6. tx.Commit fails -> error returned",
			args: args{name: "Alice", email: "alice@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Alice", "alice@example.com", "hash", "MERCHANT").
						WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "role", "created_at"}).
							AddRow("user-1", "Alice", "alice@example.com", "MERCHANT", now))
					m.ExpectExec(regexp.QuoteMeta("INSERT INTO merchants")).
						WithArgs("user-1").
						WillReturnResult(sqlmock.NewResult(0, 1))
					// commit failure marks tx as done; defer tx.Rollback() returns ErrTxDone
					// without calling the driver, so no ExpectRollback needed here
					m.ExpectCommit().WillReturnError(sql.ErrConnDone)
				},
			},
			wants: wants{err: "sql: connection is already closed"},
		},
		{
			name: "7. role MERCHANT, all steps succeed -> user with ID returned",
			args: args{name: "Alice", email: "alice@example.com", passwordHash: "hash", role: entity.RoleMerchant},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Alice", "alice@example.com", "hash", "MERCHANT").
						WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "role", "created_at"}).
							AddRow("user-1", "Alice", "alice@example.com", "MERCHANT", now))
					m.ExpectExec(regexp.QuoteMeta("INSERT INTO merchants")).
						WithArgs("user-1").
						WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectCommit()
				},
			},
			wants: wants{userID: "user-1"},
		},
		{
			name: "8. role ADMIN (no merchants insert), all steps succeed -> user returned",
			args: args{name: "Admin", email: "admin@example.com", passwordHash: "hash", role: entity.RoleAdmin},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO users")).
						WithArgs("Admin", "admin@example.com", "hash", "ADMIN").
						WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "role", "created_at"}).
							AddRow("user-2", "Admin", "admin@example.com", "ADMIN", now))
					m.ExpectCommit()
				},
			},
			wants: wants{userID: "user-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			repo := NewUserRepository(db)
			tt.mocks.setup(sqlMock)

			user, err := repo.CreateUser(tt.args.name, tt.args.email, tt.args.passwordHash, tt.args.role)

			if tt.wants.err != "" {
				assert.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, user.ID, "user ID should be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.userID, user.ID, "user ID")
			}
			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}

func TestUserRepository_FindUserByEmail(t *testing.T) {
	now := time.Now()
	passwordHash := "bcrypt-hash"

	type args struct {
		email string
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		found  bool
		userID string
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. user exists -> user returned with found=true",
			args: args{email: "alice@example.com"},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
						WithArgs("alice@example.com").
						WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "password_hash", "role", "created_at"}).
							AddRow("user-1", "Alice", "alice@example.com", passwordHash, "MERCHANT", now))
				},
			},
			wants: wants{found: true, userID: "user-1"},
		},
		{
			name: "2. user does not exist -> empty user with found=false",
			args: args{email: "notfound@example.com"},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
						WithArgs("notfound@example.com").
						WillReturnError(sql.ErrNoRows)
				},
			},
			wants: wants{found: false, userID: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			repo := NewUserRepository(db)
			tt.mocks.setup(sqlMock)

			user, found := repo.FindUserByEmail(tt.args.email)

			assert.Equal(t, tt.wants.found, found, "found flag")
			assert.Equal(t, tt.wants.userID, user.ID, "user ID")
			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}
