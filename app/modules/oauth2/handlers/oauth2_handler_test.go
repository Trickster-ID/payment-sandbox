package handlers

// Branch analysis for OAuth2Handler:
// RegisterClient: bind error → 400; service error → 500; success → 201
// ListClients: service error → 500; success empty → 200 {data:null}; success with clients → 200
// DeleteClient: service error → 500; success → 200
// Authorize: bind error → 400; GetClient error → 400; user not in ctx → early return (200 empty);
//   IssueAuthCode error → 500; success without state → 302; success with state → 302
// ApproveAuthorize: bind error → 400; IssueAuthCode error → 500; success → 200
// Token: bind error → 400; each of 4 grant type paths with error and success branches
// Introspect: bind error → 400; invalid token → 200 {active:false}; valid (ExpiresAt set) → 200 with exp;
//   valid (ExpiresAt nil) → 200 without exp
// Revoke: bind error → 400; ValidateClient error → 401; RevokeRefreshToken error → 500; success → 200
// UserInfo: user not in ctx → 401; GetUserByID error → 500; success → 200

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/middleware"
	entity "payment-sandbox/app/modules/oauth2/models/entity"
	"payment-sandbox/app/modules/oauth2/services"
	serviceMocks "payment-sandbox/app/modules/oauth2/services/mocks"
	userEntity "payment-sandbox/app/modules/users/models/entity"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

var handlerCfg = config.Config{OAuth2AccessTokenDuration: time.Hour}

// serveOAuth2Router builds a minimal router; injects userID into context when non-empty.
func serveOAuth2Router(h *OAuth2Handler, userID string) *gin.Engine {
	r := gin.New()
	if userID != "" {
		r.Use(func(c *gin.Context) {
			c.Set(middleware.ContextUserID, userID)
			c.Next()
		})
	}
	r.POST("/clients", h.RegisterClient)
	r.GET("/clients", h.ListClients)
	r.DELETE("/clients/:id", h.DeleteClient)
	r.GET("/oauth2/authorize", h.Authorize)
	r.POST("/oauth2/approve", h.ApproveAuthorize)
	r.POST("/oauth2/token", h.Token)
	r.POST("/oauth2/introspect", h.Introspect)
	r.POST("/oauth2/revoke", h.Revoke)
	r.GET("/oauth2/userinfo", h.UserInfo)
	return r
}

func formBody(values url.Values) *strings.Reader {
	return strings.NewReader(values.Encode())
}

// ─── RegisterClient ──────────────────────────────────────────────────────────

func TestOAuth2Handler_RegisterClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct {
		userID      string
		body        string
		contentType string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode   int
		body         string
		bodyContains string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. invalid JSON body -> 400 validation_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", body: `{}`, contentType: "application/json"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest, bodyContains: `"code":"validation_error"`},
		},
		{
			name:   "2. service returns error -> 500 registration_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", body: `{"name":"App","redirect_uris":["https://example.com/cb"],"scopes":["read"]}`, contentType: "application/json"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					RegisterClient("u-1", "App", []string{"https://example.com/cb"}, []string{"read"}).
					Return(services.ClientWithSecret{}, errors.New("db error")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"registration_error","message":"db error"}}`},
		},
		{
			name:   "3. success -> 201 with client and secret",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", body: `{"name":"App","redirect_uris":["https://example.com/cb"],"scopes":["read"]}`, contentType: "application/json"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					RegisterClient("u-1", "App", []string{"https://example.com/cb"}, []string{"read"}).
					Return(services.ClientWithSecret{
						Client: entity.OAuthClient{
							ID: "c-uuid", Name: "App",
							RedirectURIs: []string{"https://example.com/cb"}, Scopes: []string{"read"},
							IsFirstParty: false, IsConfidential: true,
						},
						ClientSecret: "my-secret",
					}, nil).Once()
			}},
			wants: wants{
				statusCode: http.StatusCreated,
				body: `{"data":{"client":{"id":"c-uuid","name":"App","redirect_uris":["https://example.com/cb"],` +
					`"scopes":["read"],"is_first_party":false,"is_confidential":true,"created_at":"0001-01-01T00:00:00Z"},` +
					`"client_secret":"my-secret"}}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodPost, "/clients", strings.NewReader(tt.args.body))
			req.Header.Set("Content-Type", tt.args.contentType)
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, tt.args.userID).ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}
			if tt.wants.bodyContains != "" {
				assert.Contains(t, rec.Body.String(), tt.wants.bodyContains, "response body contains")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListClients ─────────────────────────────────────────────────────────────

func TestOAuth2Handler_ListClients(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct{ userID string }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. service returns error -> 500 list_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ListClients("u-1").Return(nil, errors.New("db error")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"list_error","message":"db error"}}`},
		},
		{
			name:   "2. service returns nil list -> 200 with null data",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ListClients("u-1").Return(nil, nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":null}`},
		},
		{
			name:   "3. service returns clients -> 200 with client array",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ListClients("u-1").
					Return([]entity.OAuthClient{{
						ID: "c-1", Name: "My App",
						RedirectURIs: []string{"https://example.com/cb"}, Scopes: []string{"read"},
						IsFirstParty: false, IsConfidential: true,
					}}, nil).Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body: `{"data":[{"id":"c-1","name":"My App","redirect_uris":["https://example.com/cb"],` +
					`"scopes":["read"],"is_first_party":false,"is_confidential":true,"created_at":"0001-01-01T00:00:00Z"}]}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodGet, "/clients", nil)
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, tt.args.userID).ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── DeleteClient ────────────────────────────────────────────────────────────

func TestOAuth2Handler_DeleteClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct {
		userID   string
		clientID string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. service returns error -> 500 delete_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					DeleteClient("c-1", "u-1").Return(errors.New("db error")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"delete_error","message":"db error"}}`},
		},
		{
			name:   "2. success -> 200 status deleted",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", clientID: "c-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					DeleteClient("c-1", "u-1").Return(nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"status":"deleted"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodDelete, "/clients/"+tt.args.clientID, nil)
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, tt.args.userID).ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── Authorize ───────────────────────────────────────────────────────────────

func TestOAuth2Handler_Authorize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct {
		userID string
		query  string
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode   int
		body         string
		bodyContains string
		location     string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. missing required query param -> 400 invalid_request",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", query: "client_id=c-1&redirect_uri=https://example.com/cb"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest, bodyContains: `"code":"invalid_request"`},
		},
		{
			name:   "2. GetClient returns error -> 400 invalid_client",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", query: "response_type=code&client_id=unknown&redirect_uri=https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetClient("unknown").Return(entity.OAuthClient{}, errors.New("not found")).Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, body: `{"error":{"code":"invalid_client","message":"client not found"}}`},
		},
		{
			name:   "3. user not in context -> 401 unauthorized from MustUserID",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "", query: "response_type=code&client_id=c-1&redirect_uri=https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetClient("c-1").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
			}},
			wants: wants{statusCode: http.StatusUnauthorized, body: `{"error":{"code":"auth_unauthorized","message":"unauthorized"}}`},
		},
		{
			name:   "4. IssueAuthCode error -> 500 auth_code_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", query: "response_type=code&client_id=c-1&redirect_uri=https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetClient("c-1").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAuthCode("c-1", "u-1", "https://example.com/cb", "").Return("", errors.New("rand error")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"auth_code_error","message":"failed to issue auth code"}}`},
		},
		{
			name:   "5. success without state -> 302 redirect with code only",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", query: "response_type=code&client_id=c-1&redirect_uri=https://example.com/cb"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetClient("c-1").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAuthCode("c-1", "u-1", "https://example.com/cb", "").Return("test-code-abc", nil).Once()
			}},
			wants: wants{statusCode: http.StatusFound, location: "https://example.com/cb?code=test-code-abc"},
		},
		{
			name:   "6. success with state -> 302 redirect with code and state",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", query: "response_type=code&client_id=c-1&redirect_uri=https://example.com/cb&scope=read&state=xyz123"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetClient("c-1").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAuthCode("c-1", "u-1", "https://example.com/cb", "read").Return("test-code-abc", nil).Once()
			}},
			wants: wants{statusCode: http.StatusFound, location: "https://example.com/cb?code=test-code-abc&state=xyz123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodGet, "/oauth2/authorize?"+tt.args.query, nil)
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, tt.args.userID).ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}
			if tt.wants.bodyContains != "" {
				assert.Contains(t, rec.Body.String(), tt.wants.bodyContains, "response body contains")
			}
			if tt.wants.location != "" {
				assert.Equal(t, tt.wants.location, rec.Header().Get("Location"), "Location header")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ApproveAuthorize ────────────────────────────────────────────────────────

func TestOAuth2Handler_ApproveAuthorize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct {
		userID string
		form   url.Values
	}
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode   int
		body         string
		bodyContains string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. invalid form body -> 400 invalid_request",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", form: url.Values{}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest, bodyContains: `"code":"invalid_request"`},
		},
		{
			name:   "2. IssueAuthCode error -> 500 auth_code_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", form: url.Values{"response_type": {"code"}, "client_id": {"c-1"}, "redirect_uri": {"https://example.com/cb"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAuthCode("c-1", "u-1", "https://example.com/cb", "").Return("", errors.New("rand error")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"auth_code_error","message":"failed to issue auth code"}}`},
		},
		{
			name:   "3. success -> 200 with redirect_uri containing code",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1", form: url.Values{"response_type": {"code"}, "client_id": {"c-1"}, "redirect_uri": {"https://example.com/cb"}, "scope": {"read"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAuthCode("c-1", "u-1", "https://example.com/cb", "read").Return("test-code-abc", nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"redirect_uri":"https://example.com/cb?code=test-code-abc"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodPost, "/oauth2/approve", formBody(tt.args.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, tt.args.userID).ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}
			if tt.wants.bodyContains != "" {
				assert.Contains(t, rec.Body.String(), tt.wants.bodyContains, "response body contains")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── Token ───────────────────────────────────────────────────────────────────

func TestOAuth2Handler_Token(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct{ form url.Values }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode   int
		body         string
		bodyContains string
	}

	const tokenSuccessBody = `{"data":{"access_token":"access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"refresh-token","scope":"read"}}`
	const ccSuccessBody = `{"data":{"access_token":"access-token","token_type":"Bearer","expires_in":3600,"scope":"read"}}`

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		// Bind error
		{
			name:   "1. missing grant_type -> 400 invalid_request",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"code": {"x"}}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest, bodyContains: `"code":"invalid_request"`},
		},
		// authorization_code grant
		{
			name:   "2. authorization_code, ValidateClient error -> 401 invalid_client",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"authorization_code"}, "code": {"c"}, "redirect_uri": {"https://example.com/cb"}, "client_id": {"c-1"}, "client_secret": {"bad"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "bad").Return(entity.OAuthClient{}, errors.New("invalid")).Once()
			}},
			wants: wants{statusCode: http.StatusUnauthorized, body: `{"error":{"code":"invalid_client","message":"client authentication failed"}}`},
		},
		{
			name:   "3. authorization_code, ExchangeAuthCode error -> 400 invalid_grant",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"authorization_code"}, "code": {"bad-code"}, "redirect_uri": {"https://example.com/cb"}, "client_id": {"c-1"}, "client_secret": {"secret"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ExchangeAuthCode("bad-code", "c-1", "https://example.com/cb").Return(entity.AuthorizationCode{}, errors.New("expired")).Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, body: `{"error":{"code":"invalid_grant","message":"expired"}}`},
		},
		{
			name:   "4. authorization_code, GetUserByID error -> 500 user_lookup_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"authorization_code"}, "code": {"code-abc"}, "redirect_uri": {"https://example.com/cb"}, "client_id": {"c-1"}, "client_secret": {"secret"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ExchangeAuthCode("code-abc", "c-1", "https://example.com/cb").Return(entity.AuthorizationCode{UserID: "u-1", Scope: "read"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetUserByID("u-1").Return(userEntity.User{}, errors.New("not found")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"user_lookup_error","message":"failed to fetch user"}}`},
		},
		{
			name:   "5. authorization_code success -> 200 with tokens",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"authorization_code"}, "code": {"code-abc"}, "redirect_uri": {"https://example.com/cb"}, "client_id": {"c-1"}, "client_secret": {"secret"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ExchangeAuthCode("code-abc", "c-1", "https://example.com/cb").Return(entity.AuthorizationCode{UserID: "u-1", Scope: "read"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetUserByID("u-1").Return(userEntity.User{ID: "u-1", Role: userEntity.RoleMerchant}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAccessToken("c-1", "u-1", "read", userEntity.RoleMerchant).Return("access-token", nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueRefreshToken("c-1", "u-1", "read").Return("refresh-token", nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: tokenSuccessBody},
		},
		// client_credentials grant
		{
			name:   "6. client_credentials, ValidateClient error -> 401",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"client_credentials"}, "client_id": {"c-1"}, "client_secret": {"bad"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "bad").Return(entity.OAuthClient{}, errors.New("invalid")).Once()
			}},
			wants: wants{statusCode: http.StatusUnauthorized},
		},
		{
			name:   "7. client_credentials success with provided scope -> 200 with access token only",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"client_credentials"}, "client_id": {"c-1"}, "client_secret": {"secret"}, "scope": {"read"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1", Scopes: []string{"read", "write"}}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAccessToken("c-1", "", "read", userEntity.Role("")).Return("access-token", nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: ccSuccessBody},
		},
		// refresh_token grant
		{
			name:   "8. refresh_token, ExchangeRefreshToken error -> 400 invalid_grant",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"refresh_token"}, "client_id": {"c-1"}, "client_secret": {"secret"}, "refresh_token": {"bad-rt"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ExchangeRefreshToken("bad-rt", "c-1").Return(entity.RefreshToken{}, errors.New("expired")).Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, body: `{"error":{"code":"invalid_grant","message":"expired"}}`},
		},
		{
			name:   "9. refresh_token success -> 200 with new tokens",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"refresh_token"}, "client_id": {"c-1"}, "client_secret": {"secret"}, "refresh_token": {"rt-abc"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ExchangeRefreshToken("rt-abc", "c-1").Return(entity.RefreshToken{UserID: "u-1", Scope: "read"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetUserByID("u-1").Return(userEntity.User{ID: "u-1", Role: userEntity.RoleMerchant}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAccessToken("c-1", "u-1", "read", userEntity.RoleMerchant).Return("access-token", nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueRefreshToken("c-1", "u-1", "read").Return("refresh-token", nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: tokenSuccessBody},
		},
		// password grant
		{
			name:   "10. password, ValidateUserCredentials error -> 401 invalid_grant",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"password"}, "client_id": {"c-1"}, "client_secret": {"secret"}, "username": {"alice"}, "password": {"wrong"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateUserCredentials("alice", "wrong").Return(userEntity.User{}, errors.New("invalid credentials")).Once()
			}},
			wants: wants{statusCode: http.StatusUnauthorized, body: `{"error":{"code":"invalid_grant","message":"invalid user credentials"}}`},
		},
		{
			name:   "11. password success with empty scope -> 200 using client scopes",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"grant_type": {"password"}, "client_id": {"c-1"}, "client_secret": {"secret"}, "username": {"alice"}, "password": {"pass123"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1", Scopes: []string{"read"}}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateUserCredentials("alice", "pass123").Return(userEntity.User{ID: "u-1", Role: userEntity.RoleMerchant}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueAccessToken("c-1", "u-1", "read", userEntity.RoleMerchant).Return("access-token", nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					IssueRefreshToken("c-1", "u-1", "read").Return("refresh-token", nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: tokenSuccessBody},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodPost, "/oauth2/token", formBody(tt.args.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, "").ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}
			if tt.wants.bodyContains != "" {
				assert.Contains(t, rec.Body.String(), tt.wants.bodyContains, "response body contains")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── Introspect ──────────────────────────────────────────────────────────────

func TestOAuth2Handler_Introspect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	expiry := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC) // Unix: 1798761600

	type fields struct{ service services.IOAuth2Service }
	type args struct{ form url.Values }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. missing token field -> 400 invalid_request",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest},
		},
		{
			name:   "2. invalid/expired token -> 200 active:false",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"token": {"bad-token"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateToken("bad-token").Return(nil, errors.New("invalid")).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"active":false}}`},
		},
		{
			name:   "3. valid token with ExpiresAt -> 200 active:true with exp field",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"token": {"valid-token"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateToken("valid-token").Return(&services.OAuth2Claims{
						UserID:   "u-1",
						ClientID: "c-1",
						Scope:    "read",
						RegisteredClaims: jwt.RegisteredClaims{
							ExpiresAt: jwt.NewNumericDate(expiry),
						},
					}, nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"active":true,"scope":"read","client_id":"c-1","user_id":"u-1","exp":1798761600}}`},
		},
		{
			name:   "4. valid token without ExpiresAt -> 200 active:true without exp field",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{"token": {"valid-token-no-exp"}}},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateToken("valid-token-no-exp").Return(&services.OAuth2Claims{
						UserID:   "u-2",
						ClientID: "c-2",
						Scope:    "write",
					}, nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"active":true,"scope":"write","client_id":"c-2","user_id":"u-2"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", formBody(tt.args.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, "").ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── Revoke ──────────────────────────────────────────────────────────────────

func TestOAuth2Handler_Revoke(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct{ form url.Values }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode int
		body       string
	}

	validForm := url.Values{"token": {"rt-abc"}, "client_id": {"c-1"}, "client_secret": {"secret"}}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. missing required fields -> 400 invalid_request",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: url.Values{}},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest},
		},
		{
			name:   "2. ValidateClient error -> 401 invalid_client",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: validForm},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{}, errors.New("invalid")).Once()
			}},
			wants: wants{statusCode: http.StatusUnauthorized, body: `{"error":{"code":"invalid_client","message":"client authentication failed"}}`},
		},
		{
			name:   "3. RevokeRefreshToken error -> 500 revoke_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: validForm},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					RevokeRefreshToken("rt-abc", "c-1").Return(errors.New("db error")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"revoke_error","message":"db error"}}`},
		},
		{
			name:   "4. success -> 200 status revoked",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{form: validForm},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					ValidateClient("c-1", "secret").Return(entity.OAuthClient{ID: "c-1"}, nil).Once()
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					RevokeRefreshToken("rt-abc", "c-1").Return(nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"status":"revoked"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodPost, "/oauth2/revoke", formBody(tt.args.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, "").ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── UserInfo ────────────────────────────────────────────────────────────────

func TestOAuth2Handler_UserInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct{ service services.IOAuth2Service }
	type args struct{ userID string }
	type mocks struct{ setup func(f fields, a args) }
	type wants struct {
		statusCode   int
		body         string
		bodyContains string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			// MustUserID writes auth_unauthorized; then UserInfo writes auth_required — two JSON objects.
			// Use bodyContains to assert the handler's own error is present.
			name:   "1. user not in context -> 401 auth_required",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: ""},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusUnauthorized, bodyContains: `"auth_required"`},
		},
		{
			name:   "2. GetUserByID error -> 500 user_lookup_error",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetUserByID("u-1").Return(userEntity.User{}, errors.New("not found")).Once()
			}},
			wants: wants{statusCode: http.StatusInternalServerError, body: `{"error":{"code":"user_lookup_error","message":"failed to fetch user"}}`},
		},
		{
			name:   "3. success -> 200 with user data",
			fields: fields{service: serviceMocks.NewMockIOAuth2Service(t)},
			args:   args{userID: "u-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIOAuth2Service).EXPECT().
					GetUserByID("u-1").Return(userEntity.User{
						ID:    "u-1",
						Name:  "Alice",
						Email: "alice@example.com",
						Role:  userEntity.RoleMerchant,
					}, nil).Once()
			}},
			wants: wants{statusCode: http.StatusOK, body: `{"data":{"id":"u-1","name":"Alice","email":"alice@example.com","role":"MERCHANT"}}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			handler := NewOAuth2Handler(tt.fields.service, handlerCfg)
			req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
			rec := httptest.NewRecorder()
			serveOAuth2Router(handler, tt.args.userID).ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")
			}
			if tt.wants.bodyContains != "" {
				assert.Contains(t, rec.Body.String(), tt.wants.bodyContains, "response body contains")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIOAuth2Service); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
