package repositories

import (
	"payment-sandbox/app/modules/oauth2/models/entity"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stringArray []string

func (a *stringArray) Scan(src interface{}) error {
	switch v := src.(type) {
	case []string:
		*a = v
		return nil
	case string:
		// simple mock parsing
		*a = []string{v}
		return nil
	}
	return nil
}

func TestOAuth2Repository_SaveAuthCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewOAuth2Repository(db)
	exp := time.Now().Add(time.Minute)

	t.Run("success", func(t *testing.T) {
		code := entity.AuthorizationCode{
			Code:        "code-123",
			ClientID:    "client-1",
			UserID:      "user-1",
			RedirectURI: "http://localhost",
			Scope:       "read",
			ExpiresAt:   exp,
		}

		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_authorization_codes")).
			WithArgs(code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope, code.ExpiresAt).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.SaveAuthCode(code)
		assert.NoError(t, err)
	})
}

func TestOAuth2Repository_FindRefreshToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewOAuth2Repository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, token, client_id::text, user_id::text, scope, expires_at, revoked, created_at FROM oauth2_refresh_tokens")).
			WithArgs("token-123").
			WillReturnRows(sqlmock.NewRows([]string{"id", "token", "client_id", "user_id", "scope", "expires_at", "revoked", "created_at"}).
				AddRow("t-1", "token-123", "client-1", "user-1", "read", now.Add(time.Hour), false, now))

		token, found := repo.FindRefreshToken("token-123")
		assert.True(t, found)
		assert.Equal(t, "token-123", token.Token)
	})
}

func TestOAuth2Repository_FindUserByEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewOAuth2Repository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at FROM users")).
			WithArgs("alice@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "password_hash", "role", "created_at"}).
				AddRow("user-1", "Alice", "alice@example.com", "hash", "MERCHANT", now))

		user, found := repo.FindUserByEmail("alice@example.com")
		assert.True(t, found)
		assert.Equal(t, "user-1", user.ID)
	})
}
