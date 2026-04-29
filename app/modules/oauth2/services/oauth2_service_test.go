package services

import (
	"errors"
	"payment-sandbox/app/config"
	userEntity "payment-sandbox/app/modules/users/models/entity"
	"payment-sandbox/app/modules/oauth2/models/entity"
	repoMocks "payment-sandbox/app/modules/oauth2/repositories/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var testCfg = config.Config{
	JWTSecret:                  "test-secret",
	JWTDuration:                time.Hour,
	OAuth2AccessTokenDuration:  15 * time.Minute,
	OAuth2RefreshTokenDuration: 30 * 24 * time.Hour,
	OAuth2AuthCodeDuration:     10 * time.Minute,
}

func TestOAuth2Service_RegisterClient(t *testing.T) {
	tests := []struct {
		name         string
		ownerID      string
		clientName   string
		redirectURIs []string
		scopes       []string
		mockRepo     func(r *repoMocks.MockIOAuth2Repository)
		wantErr      bool
		errMatch     string
	}{
		{
			name:         "success registration",
			ownerID:      "owner-1",
			clientName:   "My Client",
			redirectURIs: []string{"http://localhost:3000/callback"},
			scopes:       []string{"read", "write"},
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("CreateClient", mock.MatchedBy(func(c entity.OAuthClient) bool {
					return c.Name == "My Client" && *c.OwnerID == "owner-1" && len(c.SecretHash) > 0
				})).Return(entity.OAuthClient{ID: "client-1", Name: "My Client", OwnerID: stringPtr("owner-1")}, nil)
			},
			wantErr: false,
		},
		{
			name:         "empty client name",
			ownerID:      "owner-1",
			clientName:   "",
			redirectURIs: []string{"http://localhost:3000/callback"},
			scopes:       []string{"read"},
			mockRepo:     func(r *repoMocks.MockIOAuth2Repository) {},
			wantErr:      true,
			errMatch:     "invalid client name",
		},
		{
			name:         "invalid redirect URI",
			ownerID:      "owner-1",
			clientName:   "My Client",
			redirectURIs: []string{"invalid-url"},
			scopes:       []string{"read"},
			mockRepo:     func(r *repoMocks.MockIOAuth2Repository) {},
			wantErr:      true,
			errMatch:     "invalid redirect URI",
		},
		{
			name:         "invalid scope",
			ownerID:      "owner-1",
			clientName:   "My Client",
			redirectURIs: []string{"http://localhost:3000/callback"},
			scopes:       []string{"invalid-scope"},
			mockRepo:     func(r *repoMocks.MockIOAuth2Repository) {},
			wantErr:      true,
			errMatch:     "invalid scope",
		},
		{
			name:         "repository error",
			ownerID:      "owner-1",
			clientName:   "My Client",
			redirectURIs: []string{"http://localhost:3000/callback"},
			scopes:       []string{"read"},
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("CreateClient", mock.Anything).Return(entity.OAuthClient{}, errors.New("db error"))
			},
			wantErr:  true,
			errMatch: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)

			res, err := s.RegisterClient(tt.ownerID, tt.clientName, tt.redirectURIs, tt.scopes)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.clientName, res.Client.Name)
				assert.NotEmpty(t, res.ClientSecret)
				assert.Equal(t, 64, len(res.ClientSecret)) // 32 bytes * 2 hex chars
			}
		})
	}
}

func TestOAuth2Service_ListClients(t *testing.T) {
	tests := []struct {
		name     string
		ownerID  string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
	}{
		{
			name:    "success",
			ownerID: "owner-1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("ListClientsByOwner", "owner-1").Return([]entity.OAuthClient{{ID: "c1"}}, nil)
			},
			wantErr: false,
		},
		{
			name:    "repo error",
			ownerID: "owner-1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("ListClientsByOwner", "owner-1").Return(nil, errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			res, err := s.ListClients(tt.ownerID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, res)
			}
		})
	}
}

func TestOAuth2Service_GetClient(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
	}{
		{
			name: "success",
			id:   "c1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindClientByID", "c1").Return(entity.OAuthClient{ID: "c1"}, true)
			},
			wantErr: false,
		},
		{
			name: "not found",
			id:   "c1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindClientByID", "c1").Return(entity.OAuthClient{}, false)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			_, err := s.GetClient(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2Service_DeleteClient(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		ownerID  string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
	}{
		{
			name:    "success",
			id:      "c1",
			ownerID: "owner-1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("DeleteClient", "c1", "owner-1").Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "repo error",
			id:      "c1",
			ownerID: "owner-1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("DeleteClient", "c1", "owner-1").Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			err := s.DeleteClient(tt.id, tt.ownerID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2Service_ValidateClient(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		mockRepo     func(r *repoMocks.MockIOAuth2Repository)
		wantErr      bool
		errMatch     string
	}{
		{
			name:         "success",
			clientID:     "c1",
			clientSecret: "secret",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindClientByID", "c1").Return(entity.OAuthClient{ID: "c1", SecretHash: string(hash)}, true)
			},
			wantErr: false,
		},
		{
			name:         "not found",
			clientID:     "c1",
			clientSecret: "secret",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindClientByID", "c1").Return(entity.OAuthClient{}, false)
			},
			wantErr:  true,
			errMatch: "invalid client_id",
		},
		{
			name:         "wrong secret",
			clientID:     "c1",
			clientSecret: "wrong",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindClientByID", "c1").Return(entity.OAuthClient{ID: "c1", SecretHash: string(hash)}, true)
			},
			wantErr:  true,
			errMatch: "invalid client_secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			_, err := s.ValidateClient(tt.clientID, tt.clientSecret)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2Service_IssueAuthCode(t *testing.T) {
	tests := []struct {
		name     string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
	}{
		{
			name: "success",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("SaveAuthCode", mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "repo error",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("SaveAuthCode", mock.Anything).Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			code, err := s.IssueAuthCode("c1", "u1", "http://cb", "read")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, code, 48)
			}
		})
	}
}

func TestOAuth2Service_ExchangeAuthCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		clientID    string
		redirectURI string
		mockRepo    func(r *repoMocks.MockIOAuth2Repository)
		wantErr     bool
		errMatch    string
	}{
		{
			name:        "success",
			code:        "code1",
			clientID:    "c1",
			redirectURI: "http://cb",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindAuthCode", "code1").Return(entity.AuthorizationCode{Code: "code1", ClientID: "c1", RedirectURI: "http://cb"}, true)
				r.On("MarkAuthCodeUsed", "code1").Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "not found",
			code:        "code1",
			clientID:    "c1",
			redirectURI: "http://cb",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindAuthCode", "code1").Return(entity.AuthorizationCode{}, false)
			},
			wantErr:  true,
			errMatch: "invalid or expired authorization code",
		},
		{
			name:        "client mismatch",
			code:        "code1",
			clientID:    "wrong",
			redirectURI: "http://cb",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindAuthCode", "code1").Return(entity.AuthorizationCode{Code: "code1", ClientID: "c1", RedirectURI: "http://cb"}, true)
			},
			wantErr:  true,
			errMatch: "client mismatch",
		},
		{
			name:        "redirect_uri mismatch",
			code:        "code1",
			clientID:    "c1",
			redirectURI: "http://wrong",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindAuthCode", "code1").Return(entity.AuthorizationCode{Code: "code1", ClientID: "c1", RedirectURI: "http://cb"}, true)
			},
			wantErr:  true,
			errMatch: "redirect_uri mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			_, err := s.ExchangeAuthCode(tt.code, tt.clientID, tt.redirectURI)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2Service_ExchangeRefreshToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		clientID string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
		errMatch string
	}{
		{
			name:     "success",
			token:    "token1",
			clientID: "c1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindRefreshToken", "token1").Return(entity.RefreshToken{Token: "token1", ClientID: "c1"}, true)
				r.On("RevokeRefreshToken", "token1").Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "not found",
			token:    "token1",
			clientID: "c1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindRefreshToken", "token1").Return(entity.RefreshToken{}, false)
			},
			wantErr:  true,
			errMatch: "invalid or expired refresh token",
		},
		{
			name:     "client mismatch",
			token:    "token1",
			clientID: "wrong",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindRefreshToken", "token1").Return(entity.RefreshToken{Token: "token1", ClientID: "c1"}, true)
			},
			wantErr:  true,
			errMatch: "client mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			_, err := s.ExchangeRefreshToken(tt.token, tt.clientID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2Service_ValidateToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := repoMocks.NewMockIOAuth2Repository(t)
		s := NewOAuth2Service(r, testCfg)
		token, _ := s.IssueAccessToken("c1", "u1", "read", "merchant")
		claims, err := s.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "u1", claims.UserID)
		assert.Equal(t, "c1", claims.ClientID)
	})

	t.Run("invalid token", func(t *testing.T) {
		r := repoMocks.NewMockIOAuth2Repository(t)
		s := NewOAuth2Service(r, testCfg)
		_, err := s.ValidateToken("invalid.token.string")
		assert.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		r := repoMocks.NewMockIOAuth2Repository(t)
		shortLivedCfg := testCfg
		shortLivedCfg.OAuth2AccessTokenDuration = -1 * time.Hour
		s := NewOAuth2Service(r, shortLivedCfg)
		token, _ := s.IssueAccessToken("c1", "u1", "read", "merchant")
		_, err := s.ValidateToken(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})
}

func TestOAuth2Service_ValidateUserCredentials(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	
	tests := []struct {
		name     string
		email    string
		password string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
	}{
		{
			name:     "success",
			email:    "user@example.com",
			password: "password123",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindUserByEmail", "user@example.com").Return(userEntity.User{ID: "u1", PasswordHash: string(hash)}, true)
			},
			wantErr: false,
		},
		{
			name:     "user not found",
			email:    "user@example.com",
			password: "password123",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindUserByEmail", "user@example.com").Return(userEntity.User{}, false)
			},
			wantErr: true,
		},
		{
			name:     "wrong password",
			email:    "user@example.com",
			password: "wrong",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindUserByEmail", "user@example.com").Return(userEntity.User{ID: "u1", PasswordHash: string(hash)}, true)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			_, err := s.ValidateUserCredentials(tt.email, tt.password)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOAuth2Service_RevokeRefreshToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		clientID string
		mockRepo func(r *repoMocks.MockIOAuth2Repository)
		wantErr  bool
		errMatch string
	}{
		{
			name:     "success",
			token:    "token1",
			clientID: "c1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindRefreshToken", "token1").Return(entity.RefreshToken{Token: "token1", ClientID: "c1"}, true)
				r.On("RevokeRefreshToken", "token1").Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "not found - success",
			token:    "token1",
			clientID: "c1",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindRefreshToken", "token1").Return(entity.RefreshToken{}, false)
			},
			wantErr: false,
		},
		{
			name:     "client mismatch",
			token:    "token1",
			clientID: "wrong",
			mockRepo: func(r *repoMocks.MockIOAuth2Repository) {
				r.On("FindRefreshToken", "token1").Return(entity.RefreshToken{Token: "token1", ClientID: "c1"}, true)
			},
			wantErr:  true,
			errMatch: "client mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repoMocks.NewMockIOAuth2Repository(t)
			tt.mockRepo(r)
			s := NewOAuth2Service(r, testCfg)
			err := s.RevokeRefreshToken(tt.token, tt.clientID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
