package services

// Branch analysis for OAuth2Service:
// RegisterClient: name empty/too-long → "invalid client name"; no URIs → required;
//   bad URI → "invalid redirect URI: X"; bad scope → "invalid scope: X";
//   repo.CreateClient error → error; success → ClientWithSecret, secret non-empty
// ListClients / DeleteClient: repo pass-through (error or success)
// GetClient: !found → "client not found"; found → client
// ValidateClient: !found → "invalid client_id"; bcrypt mismatch → "invalid client_secret"; success
// IssueAuthCode: SaveAuthCode error → error; success → non-empty code
// ExchangeAuthCode: !found → expired; clientID mismatch; redirectURI mismatch; MarkUsed error; success
// IssueRefreshToken: SaveRefreshToken error → error; success → non-empty token
// ExchangeRefreshToken: !found → expired; clientID mismatch; Revoke error; success
// IssueAccessToken: single path → signed JWT with correct claims
// ValidateToken: malformed → error; expired → error; valid → claims returned
// ValidateUserCredentials: !found → invalid; bcrypt mismatch → invalid; success → user
// GetUserByID: !found → "user not found"; found → user
// RevokeRefreshToken: !found → nil (RFC7009); clientID mismatch; Revoke error; success
// isValidURL: valid URL → true; empty/no-scheme/no-host/fragment → false

import (
	"errors"
	"strings"
	"testing"
	"time"

	"payment-sandbox/app/config"
	entity "payment-sandbox/app/modules/oauth2/models/entity"
	repoMocks "payment-sandbox/app/modules/oauth2/repositories/mocks"
	userEntity "payment-sandbox/app/modules/users/models/entity"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var testCfg = config.Config{
	JWTSecret:                  "test-jwt-secret-for-oauth2!",
	JWTDuration:                time.Hour,
	OAuth2AccessTokenDuration:  time.Hour,
	OAuth2RefreshTokenDuration: 24 * time.Hour,
	OAuth2AuthCodeDuration:     10 * time.Minute,
}

// ─── RegisterClient ──────────────────────────────────────────────────────────

func TestOAuth2Service_RegisterClient(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		ownerID      string
		name         string
		redirectURIs []string
		scopes       []string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg         string
		secretNonEmpty bool
	}

	saved := entity.OAuthClient{ID: "c-1", Name: "My App", RedirectURIs: []string{"https://example.com/cb"}, Scopes: []string{"read"}}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. empty name after trim -> invalid client name",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: "   ", redirectURIs: []string{"https://example.com/cb"}, scopes: []string{"read"}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "invalid client name"},
		},
		{
			name:   "2. name longer than 255 chars -> invalid client name",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: strings.Repeat("x", 256), redirectURIs: []string{"https://example.com/cb"}, scopes: []string{"read"}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "invalid client name"},
		},
		{
			name:   "3. no redirect URIs -> at least one redirect URI is required",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: "My App", redirectURIs: []string{}, scopes: []string{"read"}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "at least one redirect URI is required"},
		},
		{
			name:   "4. invalid redirect URI -> invalid redirect URI: not-a-url",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: "My App", redirectURIs: []string{"not-a-url"}, scopes: []string{"read"}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "invalid redirect URI: not-a-url"},
		},
		{
			name:   "5. invalid scope -> invalid scope: superuser",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: "My App", redirectURIs: []string{"https://example.com/cb"}, scopes: []string{"superuser"}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "invalid scope: superuser"},
		},
		{
			name:   "6. repo.CreateClient error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: "My App", redirectURIs: []string{"https://example.com/cb"}, scopes: []string{"read"}},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().CreateClient(mock.Anything).Return(entity.OAuthClient{}, errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "7. valid inputs -> ClientWithSecret returned with non-empty secret",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1", name: "My App", redirectURIs: []string{"https://example.com/cb"}, scopes: []string{"read"}},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().CreateClient(mock.Anything).Return(saved, nil).Once()
			}},
			wants: wants{secretNonEmpty: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			result, err := svc.RegisterClient(tt.args.ownerID, tt.args.name, tt.args.redirectURIs, tt.args.scopes)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, result.ClientSecret, "secret empty on error")
			} else {
				assert.NoError(t, err, "error")
				if tt.wants.secretNonEmpty {
					assert.NotEmpty(t, result.ClientSecret, "client secret")
				}
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── ListClients ─────────────────────────────────────────────────────────────

func TestOAuth2Service_ListClients(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct{ ownerID string }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg string
		count  int
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo returns error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().ListClientsByOwner(a.ownerID).Return(nil, errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "2. repo returns clients -> clients returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{ownerID: "o-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().ListClientsByOwner(a.ownerID).Return([]entity.OAuthClient{{ID: "c-1"}}, nil).Once()
			}},
			wants: wants{count: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			result, err := svc.ListClients(tt.args.ownerID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Len(t, result, tt.wants.count, "client count")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── GetClient ───────────────────────────────────────────────────────────────

func TestOAuth2Service_GetClient(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct{ id string }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg   string
		clientID string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. client not found -> client not found error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{id: "unknown"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindClientByID(a.id).Return(entity.OAuthClient{}, false).Once()
			}},
			wants: wants{errMsg: "client not found"},
		},
		{
			name:   "2. client found -> client returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{id: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindClientByID(a.id).Return(entity.OAuthClient{ID: "c-1"}, true).Once()
			}},
			wants: wants{clientID: "c-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			result, err := svc.GetClient(tt.args.id)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Equal(t, tt.wants.clientID, result.ID, "client ID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── DeleteClient ────────────────────────────────────────────────────────────

func TestOAuth2Service_DeleteClient(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		clientID string
		ownerID  string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct{ errMsg string }

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo returns error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", ownerID: "o-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().DeleteClient(a.clientID, a.ownerID).Return(errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "2. repo succeeds -> nil error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", ownerID: "o-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().DeleteClient(a.clientID, a.ownerID).Return(nil).Once()
			}},
			wants: wants{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			err := svc.DeleteClient(tt.args.clientID, tt.args.ownerID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── ValidateClient ──────────────────────────────────────────────────────────

func TestOAuth2Service_ValidateClient(t *testing.T) {
	correctHash, _ := bcrypt.GenerateFromPassword([]byte("correct-secret"), bcrypt.MinCost)

	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		clientID     string
		clientSecret string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg   string
		clientID string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. client not found -> invalid client_id",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "unknown", clientSecret: "secret"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindClientByID(a.clientID).Return(entity.OAuthClient{}, false).Once()
			}},
			wants: wants{errMsg: "invalid client_id"},
		},
		{
			name:   "2. wrong client secret -> invalid client_secret",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", clientSecret: "wrong-secret"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindClientByID(a.clientID).Return(entity.OAuthClient{ID: "c-1", SecretHash: "not-a-bcrypt-hash"}, true).Once()
			}},
			wants: wants{errMsg: "invalid client_secret"},
		},
		{
			name:   "3. correct secret -> client returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", clientSecret: "correct-secret"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindClientByID(a.clientID).Return(entity.OAuthClient{ID: "c-1", SecretHash: string(correctHash)}, true).Once()
			}},
			wants: wants{clientID: "c-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			result, err := svc.ValidateClient(tt.args.clientID, tt.args.clientSecret)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Equal(t, tt.wants.clientID, result.ID, "client ID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── IssueAuthCode ───────────────────────────────────────────────────────────

func TestOAuth2Service_IssueAuthCode(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		clientID    string
		userID      string
		redirectURI string
		scope       string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg       string
		codeNonEmpty bool
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo.SaveAuthCode error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", userID: "u-1", redirectURI: "https://example.com/cb", scope: "read"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().SaveAuthCode(mock.Anything).Return(errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "2. success -> non-empty auth code returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", userID: "u-1", redirectURI: "https://example.com/cb", scope: "read"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().SaveAuthCode(mock.Anything).Return(nil).Once()
			}},
			wants: wants{codeNonEmpty: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			code, err := svc.IssueAuthCode(tt.args.clientID, tt.args.userID, tt.args.redirectURI, tt.args.scope)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, code, "code on error")
			} else {
				assert.NoError(t, err, "error")
				if tt.wants.codeNonEmpty {
					assert.NotEmpty(t, code, "auth code")
				}
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── ExchangeAuthCode ────────────────────────────────────────────────────────

func TestOAuth2Service_ExchangeAuthCode(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		code        string
		clientID    string
		redirectURI string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg string
		userID string
	}

	goodCode := entity.AuthorizationCode{
		Code:        "code-abc",
		ClientID:    "c-1",
		UserID:      "u-1",
		RedirectURI: "https://example.com/cb",
		Scope:       "read",
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. auth code not found -> invalid or expired authorization code",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{code: "bad-code", clientID: "c-1", redirectURI: "https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindAuthCode(a.code).Return(entity.AuthorizationCode{}, false).Once()
			}},
			wants: wants{errMsg: "invalid or expired authorization code"},
		},
		{
			name:   "2. client mismatch -> client mismatch error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{code: "code-abc", clientID: "other-client", redirectURI: "https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindAuthCode(a.code).Return(goodCode, true).Once()
			}},
			wants: wants{errMsg: "client mismatch"},
		},
		{
			name:   "3. redirect_uri mismatch -> redirect_uri mismatch error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{code: "code-abc", clientID: "c-1", redirectURI: "https://other.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindAuthCode(a.code).Return(goodCode, true).Once()
			}},
			wants: wants{errMsg: "redirect_uri mismatch"},
		},
		{
			name:   "4. MarkAuthCodeUsed error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{code: "code-abc", clientID: "c-1", redirectURI: "https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindAuthCode(a.code).Return(goodCode, true).Once()
				f.repo.EXPECT().MarkAuthCodeUsed(a.code).Return(errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "5. success -> authCode returned with correct userID",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{code: "code-abc", clientID: "c-1", redirectURI: "https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindAuthCode(a.code).Return(goodCode, true).Once()
				f.repo.EXPECT().MarkAuthCodeUsed(a.code).Return(nil).Once()
			}},
			wants: wants{userID: "u-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			result, err := svc.ExchangeAuthCode(tt.args.code, tt.args.clientID, tt.args.redirectURI)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Equal(t, tt.wants.userID, result.UserID, "UserID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── IssueRefreshToken ───────────────────────────────────────────────────────

func TestOAuth2Service_IssueRefreshToken(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		clientID string
		userID   string
		scope    string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg        string
		tokenNonEmpty bool
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo.SaveRefreshToken error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", userID: "u-1", scope: "read"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().SaveRefreshToken(mock.Anything).Return(errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "2. success -> non-empty refresh token returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{clientID: "c-1", userID: "u-1", scope: "read"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().SaveRefreshToken(mock.Anything).Return(nil).Once()
			}},
			wants: wants{tokenNonEmpty: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			token, err := svc.IssueRefreshToken(tt.args.clientID, tt.args.userID, tt.args.scope)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, token, "token on error")
			} else {
				assert.NoError(t, err, "error")
				if tt.wants.tokenNonEmpty {
					assert.NotEmpty(t, token, "refresh token")
				}
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── ExchangeRefreshToken ────────────────────────────────────────────────────

func TestOAuth2Service_ExchangeRefreshToken(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		token    string
		clientID string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg string
		userID string
	}

	goodToken := entity.RefreshToken{Token: "rt-abc", ClientID: "c-1", UserID: "u-1", Scope: "read"}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. refresh token not found -> invalid or expired refresh token",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "bad-token", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(entity.RefreshToken{}, false).Once()
			}},
			wants: wants{errMsg: "invalid or expired refresh token"},
		},
		{
			name:   "2. client mismatch -> client mismatch error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "rt-abc", clientID: "other-client"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(goodToken, true).Once()
			}},
			wants: wants{errMsg: "client mismatch"},
		},
		{
			name:   "3. RevokeRefreshToken error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "rt-abc", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(goodToken, true).Once()
				f.repo.EXPECT().RevokeRefreshToken(a.token).Return(errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "4. success -> refresh token returned with correct userID",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "rt-abc", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(goodToken, true).Once()
				f.repo.EXPECT().RevokeRefreshToken(a.token).Return(nil).Once()
			}},
			wants: wants{userID: "u-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			result, err := svc.ExchangeRefreshToken(tt.args.token, tt.args.clientID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Equal(t, tt.wants.userID, result.UserID, "UserID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── IssueAccessToken ────────────────────────────────────────────────────────

func TestOAuth2Service_IssueAccessToken(t *testing.T) {
	type args struct {
		clientID string
		userID   string
		scope    string
		role     userEntity.Role
	}
	type wants struct {
		clientID string
		userID   string
		scope    string
		role     userEntity.Role
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. valid inputs -> signed JWT with correct claims",
			args:  args{clientID: "c-1", userID: "u-1", scope: "read write", role: userEntity.RoleMerchant},
			wants: wants{clientID: "c-1", userID: "u-1", scope: "read write", role: userEntity.RoleMerchant},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewOAuth2Service(repoMocks.NewMockIOAuth2Repository(t), testCfg)
			token, err := svc.IssueAccessToken(tt.args.clientID, tt.args.userID, tt.args.scope, tt.args.role)

			assert.NoError(t, err, "error")
			assert.NotEmpty(t, token, "token")

			parsed, parseErr := jwt.ParseWithClaims(token, &OAuth2Claims{}, func(_ *jwt.Token) (interface{}, error) {
				return []byte(testCfg.JWTSecret), nil
			})
			require.NoError(t, parseErr, "token parseable")
			claims, ok := parsed.Claims.(*OAuth2Claims)
			require.True(t, ok, "claims cast")
			assert.Equal(t, tt.wants.clientID, claims.ClientID, "ClientID")
			assert.Equal(t, tt.wants.userID, claims.UserID, "UserID")
			assert.Equal(t, tt.wants.scope, claims.Scope, "Scope")
			assert.Equal(t, tt.wants.role, claims.Role, "Role")
		})
	}
}

// ─── ValidateToken ───────────────────────────────────────────────────────────

func TestOAuth2Service_ValidateToken(t *testing.T) {
	makeToken := func(expiresAt time.Time) string {
		claims := OAuth2Claims{
			UserID:   "u-1",
			ClientID: "c-1",
			Scope:    "read",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiresAt),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := tok.SignedString([]byte(testCfg.JWTSecret))
		return signed
	}

	validToken := makeToken(time.Now().Add(time.Hour))
	expiredToken := makeToken(time.Now().Add(-time.Hour))

	type args struct{ token string }
	type wants struct {
		errMsg   string
		clientID string
		userID   string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. malformed JWT -> invalid or expired token",
			args:  args{token: "not-a-jwt"},
			wants: wants{errMsg: "invalid or expired token"},
		},
		{
			name:  "2. expired token -> invalid or expired token",
			args:  args{token: expiredToken},
			wants: wants{errMsg: "invalid or expired token"},
		},
		{
			name:  "3. valid token -> claims returned",
			args:  args{token: validToken},
			wants: wants{clientID: "c-1", userID: "u-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewOAuth2Service(repoMocks.NewMockIOAuth2Repository(t), testCfg)
			claims, err := svc.ValidateToken(tt.args.token)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Nil(t, claims, "claims nil on error")
			} else {
				assert.NoError(t, err, "error")
				require.NotNil(t, claims, "claims")
				assert.Equal(t, tt.wants.clientID, claims.ClientID, "ClientID")
				assert.Equal(t, tt.wants.userID, claims.UserID, "UserID")
			}
		})
	}
}

// ─── ValidateUserCredentials ─────────────────────────────────────────────────

func TestOAuth2Service_ValidateUserCredentials(t *testing.T) {
	correctHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		email    string
		password string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg string
		userID string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. user not found -> invalid credentials",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{email: "nobody@example.com", password: "password123"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindUserByEmail(a.email).Return(userEntity.User{}, false).Once()
			}},
			wants: wants{errMsg: "invalid credentials"},
		},
		{
			name:   "2. wrong password -> invalid credentials",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{email: "alice@example.com", password: "wrong-password"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindUserByEmail(a.email).Return(userEntity.User{ID: "u-1", PasswordHash: "not-bcrypt"}, true).Once()
			}},
			wants: wants{errMsg: "invalid credentials"},
		},
		{
			name:   "3. correct credentials -> user returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{email: "alice@example.com", password: "password123"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindUserByEmail(a.email).Return(userEntity.User{ID: "u-1", PasswordHash: string(correctHash)}, true).Once()
			}},
			wants: wants{userID: "u-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			user, err := svc.ValidateUserCredentials(tt.args.email, tt.args.password)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Equal(t, tt.wants.userID, user.ID, "user ID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── GetUserByID ─────────────────────────────────────────────────────────────

func TestOAuth2Service_GetUserByID(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct{ userID string }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		errMsg string
		userID string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. user not found -> user not found error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{userID: "unknown"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindUserByID(a.userID).Return(userEntity.User{}, false).Once()
			}},
			wants: wants{errMsg: "user not found"},
		},
		{
			name:   "2. user found -> user returned",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{userID: "u-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindUserByID(a.userID).Return(userEntity.User{ID: "u-1", Name: "Alice"}, true).Once()
			}},
			wants: wants{userID: "u-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			user, err := svc.GetUserByID(tt.args.userID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
				assert.Equal(t, tt.wants.userID, user.ID, "user ID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── RevokeRefreshToken ──────────────────────────────────────────────────────

func TestOAuth2Service_RevokeRefreshToken(t *testing.T) {
	type fields struct{ repo *repoMocks.MockIOAuth2Repository }
	type args struct {
		token    string
		clientID string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct{ errMsg string }

	goodToken := entity.RefreshToken{Token: "rt-abc", ClientID: "c-1", UserID: "u-1"}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. token not found -> nil (RFC 7009 success)",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "unknown", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(entity.RefreshToken{}, false).Once()
			}},
			wants: wants{},
		},
		{
			name:   "2. client mismatch -> client mismatch error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "rt-abc", clientID: "other-client"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(goodToken, true).Once()
			}},
			wants: wants{errMsg: "client mismatch"},
		},
		{
			name:   "3. repo.RevokeRefreshToken error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "rt-abc", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(goodToken, true).Once()
				f.repo.EXPECT().RevokeRefreshToken(a.token).Return(errors.New("db error")).Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "4. success -> nil error",
			fields: fields{repo: repoMocks.NewMockIOAuth2Repository(t)},
			args:   args{token: "rt-abc", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.EXPECT().FindRefreshToken(a.token).Return(goodToken, true).Once()
				f.repo.EXPECT().RevokeRefreshToken(a.token).Return(nil).Once()
			}},
			wants: wants{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewOAuth2Service(tt.fields.repo, testCfg)
			err := svc.RevokeRefreshToken(tt.args.token, tt.args.clientID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "error")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}

// ─── isValidURL ──────────────────────────────────────────────────────────────

func TestIsValidURL(t *testing.T) {
	type args struct{ u string }
	type wants struct{ valid bool }

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. valid http URL -> true",
			args:  args{u: "http://example.com/callback"},
			wants: wants{valid: true},
		},
		{
			name:  "2. valid https URL with path -> true",
			args:  args{u: "https://app.example.com/oauth/cb"},
			wants: wants{valid: true},
		},
		{
			name:  "3. empty string -> false",
			args:  args{u: ""},
			wants: wants{valid: false},
		},
		{
			name:  "4. no scheme (bare domain) -> false",
			args:  args{u: "example.com/callback"},
			wants: wants{valid: false},
		},
		{
			name:  "5. URL with fragment -> false",
			args:  args{u: "https://example.com/cb#section"},
			wants: wants{valid: false},
		},
		{
			name:  "6. scheme only, no host -> false",
			args:  args{u: "https://"},
			wants: wants{valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isValidURL(tt.args.u)
			assert.Equal(t, tt.wants.valid, got, "isValidURL(%q)", tt.args.u)
		})
	}
}
