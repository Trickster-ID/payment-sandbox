package repositories

// Branch analysis for OAuth2Repository methods:
//
// FindClientByID:   row found → (client, true) | scan error → (zero, false)
// ListClientsByOwner: success rows → slice | Query error → (nil, err) | Scan error → (nil, err)
// DeleteClient:     exec success | exec error
// SaveAuthCode:     exec success | exec error
// FindAuthCode:     row found → (code, true) | scan error → (zero, false)
// MarkAuthCodeUsed: exec success | exec error
// SaveRefreshToken: exec success | exec error
// FindRefreshToken: row found → (token, true) | scan error → (zero, false)
// RevokeRefreshToken:    exec success | exec error
// RevokeAllRefreshTokens: exec success | exec error
// FindConsent:      row found → (consent, true) | scan error → (zero, false)
// SaveConsent:      exec success | exec error
// FindUserByID:     row found → (user, true) | scan error → (zero, false)
// FindUserByEmail:  row found → (user, true) | scan error → (zero, false)

import (
	"errors"
	"regexp"
	"testing"
	"time"

	entity "payment-sandbox/app/modules/oauth2/models/entity"
	userEntity "payment-sandbox/app/modules/users/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newRepo(t *testing.T) (*OAuth2Repository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	t.Cleanup(func() { db.Close() })
	return NewOAuth2Repository(db), mock
}

var fixedTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// ─── FindClientByID ───────────────────────────────────────────────────────────

func TestOAuth2Repository_FindClientByID(t *testing.T) {
	type wants struct {
		found  bool
		client entity.OAuthClient
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. row found -> returns client and true",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text")).
					WithArgs("c-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "owner_id", "client_secret", "name",
						"redirect_uris", "scopes",
						"is_first_party", "is_confidential",
						"created_at", "updated_at",
					}).AddRow(
						"c-1", "o-1", "secret-hash", "My App",
						"{https://example.com/cb}", "{read,write}",
						false, true,
						fixedTime, fixedTime,
					))
			},
			wants: wants{
				found: true,
				client: entity.OAuthClient{
					ID: "c-1", Name: "My App", SecretHash: "secret-hash",
					IsConfidential: true,
				},
			},
		},
		{
			name: "2. no row -> returns zero value and false",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text")).
					WithArgs("c-999").
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			},
			wants: wants{found: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			id := "c-1"
			if !tt.wants.found {
				id = "c-999"
			}
			got, found := repo.FindClientByID(id)

			assert.Equal(t, tt.wants.found, found, "found")
			if tt.wants.found {
				assert.Equal(t, tt.wants.client.ID, got.ID, "ID")
				assert.Equal(t, tt.wants.client.Name, got.Name, "Name")
				assert.Equal(t, tt.wants.client.SecretHash, got.SecretHash, "SecretHash")
				assert.Equal(t, tt.wants.client.IsConfidential, got.IsConfidential, "IsConfidential")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── ListClientsByOwner ───────────────────────────────────────────────────────

func TestOAuth2Repository_ListClientsByOwner(t *testing.T) {
	type wants struct {
		count int
		err   bool
	}

	tests := []struct {
		name      string
		ownerID   string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name:    "1. rows returned -> slice of clients, no error",
			ownerID: "o-1",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text")).
					WithArgs("o-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "owner_id", "name",
						"redirect_uris", "scopes",
						"is_first_party", "is_confidential",
						"created_at", "updated_at",
					}).
						AddRow("c-1", "o-1", "App1", "{https://a.com/cb}", "{read}", false, true, fixedTime, fixedTime).
						AddRow("c-2", "o-1", "App2", "{https://b.com/cb}", "{write}", false, false, fixedTime, fixedTime))
			},
			wants: wants{count: 2},
		},
		{
			name:    "2. query error -> nil slice, error returned",
			ownerID: "o-2",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text")).
					WithArgs("o-2").
					WillReturnError(errors.New("db error"))
			},
			wants: wants{count: 0, err: true},
		},
		{
			name:    "3. no rows -> nil slice, no error",
			ownerID: "o-3",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text")).
					WithArgs("o-3").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "owner_id", "name",
						"redirect_uris", "scopes",
						"is_first_party", "is_confidential",
						"created_at", "updated_at",
					}))
			},
			wants: wants{count: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			clients, err := repo.ListClientsByOwner(tt.ownerID)

			if tt.wants.err {
				assert.Error(t, err)
				assert.Nil(t, clients)
			} else {
				assert.NoError(t, err)
				assert.Len(t, clients, tt.wants.count)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── DeleteClient ─────────────────────────────────────────────────────────────

func TestOAuth2Repository_DeleteClient(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_clients")).
					WithArgs("c-1", "o-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_clients")).
					WithArgs("c-1", "o-1").
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.DeleteClient("c-1", "o-1")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── SaveAuthCode ─────────────────────────────────────────────────────────────

func TestOAuth2Repository_SaveAuthCode(t *testing.T) {
	code := entity.AuthorizationCode{
		Code: "auth-code-1", ClientID: "c-1", UserID: "u-1",
		RedirectURI: "https://example.com/cb", Scope: "read",
		ExpiresAt: fixedTime,
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_authorization_codes")).
					WithArgs(code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope, code.ExpiresAt).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_authorization_codes")).
					WithArgs(code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope, code.ExpiresAt).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.SaveAuthCode(code)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── FindAuthCode ─────────────────────────────────────────────────────────────

func TestOAuth2Repository_FindAuthCode(t *testing.T) {
	type wants struct {
		found bool
		code  entity.AuthorizationCode
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. row found -> returns auth code and true",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, code, client_id::text")).
					WithArgs("auth-code-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "code", "client_id", "user_id",
						"redirect_uri", "scope", "expires_at", "used", "created_at",
					}).AddRow("ac-1", "auth-code-1", "c-1", "u-1",
						"https://example.com/cb", "read", fixedTime, false, fixedTime))
			},
			wants: wants{
				found: true,
				code: entity.AuthorizationCode{
					ID: "ac-1", Code: "auth-code-1", ClientID: "c-1", UserID: "u-1",
					RedirectURI: "https://example.com/cb", Scope: "read",
				},
			},
		},
		{
			name: "2. no row -> returns zero value and false",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, code, client_id::text")).
					WithArgs("bad-code").
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			},
			wants: wants{found: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			input := "auth-code-1"
			if !tt.wants.found {
				input = "bad-code"
			}
			got, found := repo.FindAuthCode(input)

			assert.Equal(t, tt.wants.found, found, "found")
			if tt.wants.found {
				assert.Equal(t, tt.wants.code.ID, got.ID)
				assert.Equal(t, tt.wants.code.Code, got.Code)
				assert.Equal(t, tt.wants.code.ClientID, got.ClientID)
				assert.Equal(t, tt.wants.code.UserID, got.UserID)
				assert.Equal(t, tt.wants.code.Scope, got.Scope)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── MarkAuthCodeUsed ─────────────────────────────────────────────────────────

func TestOAuth2Repository_MarkAuthCodeUsed(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_authorization_codes")).
					WithArgs("auth-code-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_authorization_codes")).
					WithArgs("auth-code-1").
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.MarkAuthCodeUsed("auth-code-1")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── SaveRefreshToken ─────────────────────────────────────────────────────────

func TestOAuth2Repository_SaveRefreshToken(t *testing.T) {
	tok := entity.RefreshToken{
		Token: "rt-1", ClientID: "c-1", UserID: "u-1",
		Scope: "read", ExpiresAt: fixedTime,
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_refresh_tokens")).
					WithArgs(tok.Token, tok.ClientID, tok.UserID, tok.Scope, tok.ExpiresAt).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_refresh_tokens")).
					WithArgs(tok.Token, tok.ClientID, tok.UserID, tok.Scope, tok.ExpiresAt).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.SaveRefreshToken(tok)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── FindRefreshToken ─────────────────────────────────────────────────────────

func TestOAuth2Repository_FindRefreshToken(t *testing.T) {
	type wants struct {
		found bool
		token entity.RefreshToken
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. row found -> returns refresh token and true",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, token, client_id::text")).
					WithArgs("rt-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "token", "client_id", "user_id",
						"scope", "expires_at", "revoked", "created_at",
					}).AddRow("t-1", "rt-1", "c-1", "u-1", "read", fixedTime, false, fixedTime))
			},
			wants: wants{
				found: true,
				token: entity.RefreshToken{ID: "t-1", Token: "rt-1", ClientID: "c-1", UserID: "u-1", Scope: "read"},
			},
		},
		{
			name: "2. no row -> returns zero value and false",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, token, client_id::text")).
					WithArgs("bad-token").
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			},
			wants: wants{found: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			input := "rt-1"
			if !tt.wants.found {
				input = "bad-token"
			}
			got, found := repo.FindRefreshToken(input)

			assert.Equal(t, tt.wants.found, found, "found")
			if tt.wants.found {
				assert.Equal(t, tt.wants.token.ID, got.ID)
				assert.Equal(t, tt.wants.token.Token, got.Token)
				assert.Equal(t, tt.wants.token.ClientID, got.ClientID)
				assert.Equal(t, tt.wants.token.Scope, got.Scope)
				assert.False(t, got.Revoked)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── RevokeRefreshToken ───────────────────────────────────────────────────────

func TestOAuth2Repository_RevokeRefreshToken(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_refresh_tokens")).
					WithArgs("rt-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_refresh_tokens")).
					WithArgs("rt-1").
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.RevokeRefreshToken("rt-1")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── RevokeAllRefreshTokens ───────────────────────────────────────────────────

func TestOAuth2Repository_RevokeAllRefreshTokens(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_refresh_tokens")).
					WithArgs("c-1", "u-1").
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE oauth2_refresh_tokens")).
					WithArgs("c-1", "u-1").
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.RevokeAllRefreshTokens("c-1", "u-1")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── FindConsent ──────────────────────────────────────────────────────────────

func TestOAuth2Repository_FindConsent(t *testing.T) {
	type wants struct {
		found   bool
		consent entity.Consent
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. row found -> returns consent and true",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, client_id::text")).
					WithArgs("u-1", "c-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "user_id", "client_id", "scope", "created_at",
					}).AddRow("con-1", "u-1", "c-1", "read", fixedTime))
			},
			wants: wants{
				found: true,
				consent: entity.Consent{ID: "con-1", UserID: "u-1", ClientID: "c-1", Scope: "read"},
			},
		},
		{
			name: "2. no row -> returns zero value and false",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, client_id::text")).
					WithArgs("u-99", "c-99").
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			},
			wants: wants{found: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			userID, clientID := "u-1", "c-1"
			if !tt.wants.found {
				userID, clientID = "u-99", "c-99"
			}
			got, found := repo.FindConsent(userID, clientID)

			assert.Equal(t, tt.wants.found, found, "found")
			if tt.wants.found {
				assert.Equal(t, tt.wants.consent.ID, got.ID)
				assert.Equal(t, tt.wants.consent.Scope, got.Scope)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── SaveConsent ──────────────────────────────────────────────────────────────

func TestOAuth2Repository_SaveConsent(t *testing.T) {
	consent := entity.Consent{UserID: "u-1", ClientID: "c-1", Scope: "read"}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. exec succeeds -> nil error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_consents")).
					WithArgs(consent.UserID, consent.ClientID, consent.Scope).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "2. exec error -> error returned",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO oauth2_consents")).
					WithArgs(consent.UserID, consent.ClientID, consent.Scope).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			err := repo.SaveConsent(consent)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── FindUserByID ─────────────────────────────────────────────────────────────

func TestOAuth2Repository_FindUserByID(t *testing.T) {
	type wants struct {
		found bool
		user  userEntity.User
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. row found -> returns user and true",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
					WithArgs("u-1").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "email", "password_hash", "role", "created_at",
					}).AddRow("u-1", "Alice", "alice@example.com", "hash", "MERCHANT", fixedTime))
			},
			wants: wants{
				found: true,
				user:  userEntity.User{ID: "u-1", Name: "Alice", Email: "alice@example.com", Role: "MERCHANT"},
			},
		},
		{
			name: "2. no row -> returns zero value and false",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
					WithArgs("u-999").
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			},
			wants: wants{found: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			id := "u-1"
			if !tt.wants.found {
				id = "u-999"
			}
			got, found := repo.FindUserByID(id)

			assert.Equal(t, tt.wants.found, found, "found")
			if tt.wants.found {
				assert.Equal(t, tt.wants.user.ID, got.ID)
				assert.Equal(t, tt.wants.user.Name, got.Name)
				assert.Equal(t, tt.wants.user.Email, got.Email)
				assert.Equal(t, tt.wants.user.Role, got.Role)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── FindUserByEmail ──────────────────────────────────────────────────────────

func TestOAuth2Repository_FindUserByEmail(t *testing.T) {
	type wants struct {
		found bool
		user  userEntity.User
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. row found -> returns user and true",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
					WithArgs("alice@example.com").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "email", "password_hash", "role", "created_at",
					}).AddRow("u-1", "Alice", "alice@example.com", "hash", "MERCHANT", fixedTime))
			},
			wants: wants{
				found: true,
				user:  userEntity.User{ID: "u-1", Name: "Alice", Email: "alice@example.com", Role: "MERCHANT"},
			},
		},
		{
			name: "2. no row -> returns zero value and false",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, name, email, password_hash, role::text, created_at")).
					WithArgs("nobody@example.com").
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			},
			wants: wants{found: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, mock := newRepo(t)
			tt.setupMock(mock)

			email := "alice@example.com"
			if !tt.wants.found {
				email = "nobody@example.com"
			}
			got, found := repo.FindUserByEmail(email)

			assert.Equal(t, tt.wants.found, found, "found")
			if tt.wants.found {
				assert.Equal(t, tt.wants.user.ID, got.ID)
				assert.Equal(t, tt.wants.user.Email, got.Email)
				assert.Equal(t, tt.wants.user.Role, got.Role)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
