package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"payment-sandbox/app/config"
	"payment-sandbox/app/middleware"
	userEntity "payment-sandbox/app/modules/users/models/entity"
	"payment-sandbox/app/modules/oauth2/models/entity"
	"payment-sandbox/app/modules/oauth2/services"
	serviceMocks "payment-sandbox/app/modules/oauth2/services/mocks"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOAuth2Handler_ClientManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RegisterClient binding error", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.POST("/clients", h.RegisterClient)

		body := `{"name":"","redirect_uris":[]}`
		req := httptest.NewRequest(http.MethodPost, "/clients", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("DeleteClient service error", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		s.EXPECT().DeleteClient("c1", "user-1").Return(fmt.Errorf("db error"))

		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.DELETE("/clients/:id", func(c *gin.Context) {
			c.Set(middleware.ContextUserID, "user-1")
			h.DeleteClient(c)
		})

		req := httptest.NewRequest(http.MethodDelete, "/clients/c1", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestOAuth2Handler_Authorize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
		setupMocks  func(s *serviceMocks.MockIOAuth2Service)
		wantStatus  int
		wantLocation string
	}{
		{
			name:        "success redirect",
			queryParams: "response_type=code&client_id=c1&redirect_uri=http://localhost:3000/cb&state=xyz",
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().GetClient("c1").Return(entity.OAuthClient{ID: "c1"}, nil)
				s.EXPECT().IssueAuthCode("c1", "user-1", "http://localhost:3000/cb", mock.Anything).Return("code123", nil)
			},
			wantStatus: http.StatusFound,
			wantLocation: "http://localhost:3000/cb?code=code123&state=xyz",
		},
		{
			name:        "invalid client",
			queryParams: "response_type=code&client_id=invalid&redirect_uri=http://localhost:3000/cb",
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().GetClient("invalid").Return(entity.OAuthClient{}, fmt.Errorf("not found"))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "binding error",
			queryParams: "client_id=",
			setupMocks:  func(s *serviceMocks.MockIOAuth2Service) {},
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := serviceMocks.NewMockIOAuth2Service(t)
			tc.setupMocks(s)

			h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
			r := gin.New()
			r.GET("/oauth2/authorize", func(c *gin.Context) {
				c.Set(middleware.ContextUserID, "user-1")
				h.Authorize(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/oauth2/authorize?"+tc.queryParams, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantLocation != "" {
				assert.Equal(t, tc.wantLocation, rec.Header().Get("Location"))
			}
		})
	}
}

func TestOAuth2Handler_ApproveAuthorize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		s.EXPECT().IssueAuthCode("c1", "user-1", "http://cb", "read").Return("code123", nil)

		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.POST("/oauth2/authorize", func(c *gin.Context) {
			c.Set(middleware.ContextUserID, "user-1")
			h.ApproveAuthorize(c)
		})

		body := url.Values{
			"response_type": {"code"},
			"client_id":     {"c1"},
			"redirect_uri":  {"http://cb"},
			"scope":         {"read"},
		}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize", bytes.NewBufferString(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var payload map[string]any
		json.Unmarshal(rec.Body.Bytes(), &payload)
		data := payload["data"].(map[string]any)
		assert.Contains(t, data["redirect_uri"], "code=code123")
	})
}

func TestOAuth2Handler_Token(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       url.Values
		setupMocks func(s *serviceMocks.MockIOAuth2Service)
		wantStatus int
		wantToken  bool
	}{
		{
			name: "auth code grant success",
			body: url.Values{
				"grant_type":    {"authorization_code"},
				"code":          {"code123"},
				"client_id":     {"c1"},
				"client_secret": {"secret"},
				"redirect_uri":  {"http://localhost:3000/cb"},
			},
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().ValidateClient("c1", "secret").Return(entity.OAuthClient{ID: "c1"}, nil)
				s.EXPECT().ExchangeAuthCode("code123", "c1", "http://localhost:3000/cb").Return(entity.AuthorizationCode{UserID: "u1", Scope: "read"}, nil)
				s.EXPECT().GetUserByID("u1").Return(userEntity.User{ID: "u1", Role: userEntity.RoleMerchant}, nil)
				s.EXPECT().IssueAccessToken("c1", "u1", "read", userEntity.RoleMerchant).Return("access-token", nil)
				s.EXPECT().IssueRefreshToken("c1", "u1", "read").Return("refresh-token", nil)
			},
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name: "client credentials success",
			body: url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {"c1"},
				"client_secret": {"secret"},
				"scope":         {"read"},
			},
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().ValidateClient("c1", "secret").Return(entity.OAuthClient{ID: "c1", Scopes: []string{"read"}}, nil)
				s.EXPECT().IssueAccessToken("c1", "", "read", userEntity.Role("")).Return("access-token", nil)
			},
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name: "refresh token success",
			body: url.Values{
				"grant_type":    {"refresh_token"},
				"client_id":     {"c1"},
				"client_secret": {"secret"},
				"refresh_token": {"rt123"},
			},
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().ValidateClient("c1", "secret").Return(entity.OAuthClient{ID: "c1"}, nil)
				s.EXPECT().ExchangeRefreshToken("rt123", "c1").Return(entity.RefreshToken{UserID: "u1", Scope: "read"}, nil)
				s.EXPECT().GetUserByID("u1").Return(userEntity.User{ID: "u1", Role: userEntity.RoleMerchant}, nil)
				s.EXPECT().IssueAccessToken("c1", "u1", "read", userEntity.RoleMerchant).Return("access-token", nil)
				s.EXPECT().IssueRefreshToken("c1", "u1", "read").Return("refresh-token", nil)
			},
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name: "password grant success",
			body: url.Values{
				"grant_type":    {"password"},
				"username":      {"user@example.com"},
				"password":      {"pass123"},
				"client_id":     {"c1"},
				"client_secret": {"secret"},
			},
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().ValidateClient("c1", "secret").Return(entity.OAuthClient{ID: "c1", Scopes: []string{"read"}}, nil)
				s.EXPECT().ValidateUserCredentials("user@example.com", "pass123").Return(userEntity.User{ID: "u1", Role: userEntity.RoleMerchant}, nil)
				s.EXPECT().IssueAccessToken("c1", "u1", "read", userEntity.RoleMerchant).Return("access-token", nil)
				s.EXPECT().IssueRefreshToken("c1", "u1", "read").Return("refresh-token", nil)
			},
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name: "unsupported grant type",
			body: url.Values{
				"grant_type": {"implicit"},
			},
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid client",
			body: url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {"c1"},
				"client_secret": {"wrong"},
			},
			setupMocks: func(s *serviceMocks.MockIOAuth2Service) {
				s.EXPECT().ValidateClient("c1", "wrong").Return(entity.OAuthClient{}, fmt.Errorf("invalid"))
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := serviceMocks.NewMockIOAuth2Service(t)
			tc.setupMocks(s)

			h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
			r := gin.New()
			r.POST("/oauth2/token", h.Token)

			req := httptest.NewRequest(http.MethodPost, "/oauth2/token", nil)
			req.PostForm = tc.body
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantToken {
				var payload map[string]any
				err := json.Unmarshal(rec.Body.Bytes(), &payload)
				require.NoError(t, err)
				data := payload["data"].(map[string]any)
				assert.NotEmpty(t, data["access_token"])
			}
		})
	}
}

func TestOAuth2Handler_Introspect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("active token", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		s.EXPECT().ValidateToken("valid-token").Return(&services.OAuth2Claims{
			UserID:   "u1",
			ClientID: "c1",
			Scope:    "read",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}, nil)

		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.POST("/oauth2/introspect", h.Introspect)

		body := url.Values{"token": {"valid-token"}}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", nil)
		req.PostForm = body
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var payload map[string]any
		json.Unmarshal(rec.Body.Bytes(), &payload)
		data := payload["data"].(map[string]any)
		assert.True(t, data["active"].(bool))
	})

	t.Run("inactive token", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		s.EXPECT().ValidateToken("invalid-token").Return(nil, fmt.Errorf("invalid"))

		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.POST("/oauth2/introspect", h.Introspect)

		body := url.Values{"token": {"invalid-token"}}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", nil)
		req.PostForm = body
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var payload map[string]any
		json.Unmarshal(rec.Body.Bytes(), &payload)
		data := payload["data"].(map[string]any)
		assert.False(t, data["active"].(bool))
	})
}

func TestOAuth2Handler_Revoke(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		s.EXPECT().ValidateClient("c1", "secret").Return(entity.OAuthClient{ID: "c1"}, nil)
		s.EXPECT().RevokeRefreshToken("rt123", "c1").Return(nil)

		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.POST("/oauth2/revoke", h.Revoke)

		body := url.Values{
			"token":         {"rt123"},
			"client_id":     {"c1"},
			"client_secret": {"secret"},
		}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/revoke", nil)
		req.PostForm = body
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestOAuth2Handler_UserInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		s.EXPECT().GetUserByID("user-1").Return(userEntity.User{
			ID:    "user-1",
			Name:  "Test User",
			Email: "test@example.com",
			Role:  userEntity.RoleAdmin,
		}, nil)
		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.GET("/oauth2/userinfo", func(c *gin.Context) {
			c.Set(middleware.ContextUserID, "user-1")
			h.UserInfo(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var payload map[string]any
		json.Unmarshal(rec.Body.Bytes(), &payload)
		data := payload["data"].(map[string]any)
		assert.Equal(t, "user-1", data["id"])
		assert.Equal(t, "Test User", data["name"])
		assert.Equal(t, "test@example.com", data["email"])
		assert.Equal(t, "ADMIN", data["role"])
	})

	t.Run("unauthorized", func(t *testing.T) {
		s := serviceMocks.NewMockIOAuth2Service(t)
		h := NewOAuth2Handler(s, config.Config{OAuth2AccessTokenDuration: time.Hour})
		r := gin.New()
		r.GET("/oauth2/userinfo", h.UserInfo)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
