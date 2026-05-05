package middleware

// Branch analysis
//
// NewJWTService(cfg):
// └── single path: assigns cfg.JWTSecret and cfg.JWTDuration to JWTService
//
// JWTService.GenerateToken(userID, role):
// └── single path: creates HS256-signed token, returns (string, error)
//
// AuthMiddleware(secret):
// ├── header == "" (no Authorization header)               → 401 + abort
// ├── tokenString == header (no "Bearer " prefix)          → 401 + abort
// ├── tokenString == "" (only whitespace after "Bearer ")  → 401 + abort
// ├── jwt.ParseWithClaims error (malformed / wrong secret) → 401 + abort
// ├── token expired                                        → 401 + abort
// └── valid token                                          → sets 4 ctx keys, Next
//
// RequireRoles(allowed ...entity.Role):
// ├── c.Get(ContextRole) not found          → 403 + abort
// ├── roleAny not string                    → 403 + abort
// ├── role not in allowed list              → 403 + abort
// ├── role matches single allowed           → Next
// └── role matches one of multiple allowed → Next
//
// RequireScopes(required ...string):
// ├── c.Get(ContextScope) not found              → 403 + abort
// ├── scopeAny not string                        → 403 + abort
// ├── single required scope missing             → 403 + abort
// ├── one of multiple required scopes missing   → 403 + abort
// ├── all required scopes present (single)      → Next
// ├── all required scopes present (multiple)    → Next
// └── no required scopes (zero args)            → Next
//
// MustUserID(c):
// ├── ContextUserID not in ctx          → writes 401, returns "", false
// ├── value not string OR userID == ""  → writes 401, returns "", false
// └── valid non-empty userID            → returns userID, true

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/modules/users/models/entity"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// makeSignedToken is a test helper that builds a JWT with arbitrary expiry.
func makeSignedToken(t *testing.T, secret, userID string, role entity.Role, expiresAt time.Time) string {
	t.Helper()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("makeSignedToken: %v", err)
	}
	return signed
}

// ginCtx returns a fresh recorder and gin context for each subtest.
func ginCtx(method, path string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return w, c
}

// ─── NewJWTService ────────────────────────────────────────────────────────────

func TestNewJWTService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		cfg config.Config
	}
	type wants struct {
		secret   string
		duration time.Duration
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. config with secret and duration -> JWTService fields assigned correctly",
			args: args{cfg: config.Config{
				JWTSecret:   "my-secret",
				JWTDuration: 2 * time.Hour,
			}},
			wants: wants{secret: "my-secret", duration: 2 * time.Hour},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewJWTService(tt.args.cfg)

			assert.Equal(t, tt.wants.secret, got.Secret, "Secret")
			assert.Equal(t, tt.wants.duration, got.Duration, "Duration")
		})
	}
}

// ─── JWTService.GenerateToken ─────────────────────────────────────────────────

func TestJWTService_GenerateToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const secret = "test-secret"
	svc := JWTService{Secret: secret, Duration: time.Hour}

	type args struct {
		userID string
		role   entity.Role
	}
	type wants struct {
		wantErr    bool
		wantUserID string
		wantRole   entity.Role
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. valid userID and role -> non-empty signed token, claims round-trip correctly",
			args:  args{userID: "user-1", role: entity.RoleMerchant},
			wants: wants{wantErr: false, wantUserID: "user-1", wantRole: entity.RoleMerchant},
		},
		{
			name:  "2. empty userID -> token still generated without error",
			args:  args{userID: "", role: entity.RoleAdmin},
			wants: wants{wantErr: false, wantUserID: "", wantRole: entity.RoleAdmin},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tokenStr, err := svc.GenerateToken(tt.args.userID, tt.args.role)

			if tt.wants.wantErr {
				assert.Error(t, err, "error")
				return
			}

			assert.NoError(t, err, "unexpected error")
			assert.NotEmpty(t, tokenStr, "token string")

			// validate the claims round-trip
			parsed, parseErr := jwt.ParseWithClaims(tokenStr, &Claims{}, func(tok *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})
			assert.NoError(t, parseErr, "token should parse without error")
			assert.True(t, parsed.Valid, "parsed token should be valid")

			claims, ok := parsed.Claims.(*Claims)
			assert.True(t, ok, "claims type assertion")
			assert.Equal(t, tt.wants.wantUserID, claims.UserID, "claims.UserID")
			assert.Equal(t, tt.wants.wantRole, claims.Role, "claims.Role")
			assert.True(t, claims.ExpiresAt.After(time.Now()), "token must not be expired")
		})
	}
}

// ─── AuthMiddleware ───────────────────────────────────────────────────────────

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const secret = "test-secret"

	type args struct {
		authHeader string
	}
	type wants struct {
		statusCode  int
		aborted     bool
		body        string
		ctxUserID   string
		ctxRole     string
		ctxClientID string
		ctxScope    string
	}

	validToken := makeSignedToken(t, secret, "user-42", entity.RoleMerchant, time.Now().Add(time.Hour))
	expiredToken := makeSignedToken(t, secret, "user-42", entity.RoleMerchant, time.Now().Add(-time.Hour))
	wrongSecretToken := makeSignedToken(t, "other-secret", "user-42", entity.RoleMerchant, time.Now().Add(time.Hour))

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. no Authorization header -> 401 missing bearer token",
			args:  args{authHeader: ""},
			wants: wants{statusCode: http.StatusUnauthorized, aborted: true, body: `{"error":{"code":"auth_missing_bearer_token","message":"missing bearer token"}}`},
		},
		{
			name:  "2. Authorization without \"Bearer \" prefix -> 401 missing bearer token",
			args:  args{authHeader: "sometoken"},
			wants: wants{statusCode: http.StatusUnauthorized, aborted: true, body: `{"error":{"code":"auth_missing_bearer_token","message":"missing bearer token"}}`},
		},
		{
			name:  "3. \"Bearer \" with whitespace-only token -> 401 missing bearer token",
			args:  args{authHeader: "Bearer   "},
			wants: wants{statusCode: http.StatusUnauthorized, aborted: true, body: `{"error":{"code":"auth_missing_bearer_token","message":"missing bearer token"}}`},
		},
		{
			name:  "4. malformed JWT string -> 401 invalid token",
			args:  args{authHeader: "Bearer not.a.jwt"},
			wants: wants{statusCode: http.StatusUnauthorized, aborted: true, body: `{"error":{"code":"auth_invalid_token","message":"invalid token"}}`},
		},
		{
			name:  "5. token signed with wrong secret -> 401 invalid token",
			args:  args{authHeader: "Bearer " + wrongSecretToken},
			wants: wants{statusCode: http.StatusUnauthorized, aborted: true, body: `{"error":{"code":"auth_invalid_token","message":"invalid token"}}`},
		},
		{
			name:  "6. expired token -> 401 invalid token",
			args:  args{authHeader: "Bearer " + expiredToken},
			wants: wants{statusCode: http.StatusUnauthorized, aborted: true, body: `{"error":{"code":"auth_invalid_token","message":"invalid token"}}`},
		},
		{
			name: "7. valid token -> not aborted, all four context keys set",
			args: args{authHeader: "Bearer " + validToken},
			wants: wants{
				statusCode:  http.StatusOK,
				aborted:     false,
				body:        "",
				ctxUserID:   "user-42",
				ctxRole:     string(entity.RoleMerchant),
				ctxClientID: "",
				ctxScope:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, c := ginCtx("GET", "/")
			if tt.args.authHeader != "" {
				c.Request.Header.Set("Authorization", tt.args.authHeader)
			}

			AuthMiddleware(secret)(c)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			assert.Equal(t, tt.wants.aborted, c.IsAborted(), "aborted")

			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, w.Body.String(), "response body")
			}

			if !tt.wants.aborted {
				userID, _ := c.Get(ContextUserID)
				role, _ := c.Get(ContextRole)
				clientID, _ := c.Get(ContextClientID)
				scope, _ := c.Get(ContextScope)
				assert.Equal(t, tt.wants.ctxUserID, userID, "ctx user_id")
				assert.Equal(t, tt.wants.ctxRole, role, "ctx role")
				assert.Equal(t, tt.wants.ctxClientID, clientID, "ctx client_id")
				assert.Equal(t, tt.wants.ctxScope, scope, "ctx scope")
			}
		})
	}
}

// ─── RequireRoles ─────────────────────────────────────────────────────────────

func TestRequireRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		ctxRole interface{} // value stored in context (nil means "do not set")
		allowed []entity.Role
	}
	type wants struct {
		statusCode int
		aborted    bool
		body       string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. ContextRole not in context -> 403 role not found",
			args:  args{ctxRole: nil, allowed: []entity.Role{entity.RoleAdmin}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_role_not_found","message":"role not found"}}`},
		},
		{
			name:  "2. ContextRole is non-string type -> 403 invalid role",
			args:  args{ctxRole: 99, allowed: []entity.Role{entity.RoleAdmin}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_invalid_role","message":"invalid role"}}`},
		},
		{
			name:  "3. role not in allowed list -> 403 forbidden",
			args:  args{ctxRole: string(entity.RoleMerchant), allowed: []entity.Role{entity.RoleAdmin}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_forbidden","message":"forbidden"}}`},
		},
		{
			name:  "4. role matches single allowed -> not aborted",
			args:  args{ctxRole: string(entity.RoleAdmin), allowed: []entity.Role{entity.RoleAdmin}},
			wants: wants{statusCode: http.StatusOK, aborted: false},
		},
		{
			name:  "5. role matches one of multiple allowed -> not aborted",
			args:  args{ctxRole: string(entity.RoleMerchant), allowed: []entity.Role{entity.RoleAdmin, entity.RoleMerchant}},
			wants: wants{statusCode: http.StatusOK, aborted: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, c := ginCtx("GET", "/")
			if tt.args.ctxRole != nil {
				c.Set(ContextRole, tt.args.ctxRole)
			}

			RequireRoles(tt.args.allowed...)(c)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			assert.Equal(t, tt.wants.aborted, c.IsAborted(), "aborted")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, w.Body.String(), "response body")
			}
		})
	}
}

// ─── RequireScopes ────────────────────────────────────────────────────────────

func TestRequireScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		ctxScope interface{} // value stored in context (nil means "do not set")
		required []string
	}
	type wants struct {
		statusCode int
		aborted    bool
		body       string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. ContextScope not in context -> 403 scope not found",
			args:  args{ctxScope: nil, required: []string{"read"}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_scope_not_found","message":"scope not found"}}`},
		},
		{
			name:  "2. ContextScope is non-string type -> 403 invalid scope",
			args:  args{ctxScope: 42, required: []string{"read"}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_invalid_scope","message":"invalid scope"}}`},
		},
		{
			name:  "3. single required scope missing from context -> 403 insufficient scope",
			args:  args{ctxScope: "write", required: []string{"read"}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_insufficient_scope","message":"insufficient scope: read"}}`},
		},
		{
			name:  "4. one of two required scopes missing -> 403 insufficient scope for missing one",
			args:  args{ctxScope: "read", required: []string{"read", "write"}},
			wants: wants{statusCode: http.StatusForbidden, aborted: true, body: `{"error":{"code":"auth_insufficient_scope","message":"insufficient scope: write"}}`},
		},
		{
			name:  "5. all required scopes present (single) -> not aborted",
			args:  args{ctxScope: "read write", required: []string{"read"}},
			wants: wants{statusCode: http.StatusOK, aborted: false},
		},
		{
			name:  "6. all required scopes present (multiple) -> not aborted",
			args:  args{ctxScope: "read write admin", required: []string{"read", "write"}},
			wants: wants{statusCode: http.StatusOK, aborted: false},
		},
		{
			name:  "7. no required scopes (zero args) -> always passes through",
			args:  args{ctxScope: "", required: []string{}},
			wants: wants{statusCode: http.StatusOK, aborted: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, c := ginCtx("GET", "/")
			if tt.args.ctxScope != nil {
				c.Set(ContextScope, tt.args.ctxScope)
			}

			RequireScopes(tt.args.required...)(c)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			assert.Equal(t, tt.wants.aborted, c.IsAborted(), "aborted")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, w.Body.String(), "response body")
			}
		})
	}
}

// ─── MustUserID ───────────────────────────────────────────────────────────────

func TestMustUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		ctxUserID interface{} // nil means "do not set"
	}
	type wants struct {
		returnedID string
		returnedOK bool
		statusCode int
		body       string
	}

	const unauthorizedBody = `{"error":{"code":"auth_unauthorized","message":"unauthorized"}}`

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. ContextUserID not in context -> returns \"\", false, 401",
			args:  args{ctxUserID: nil},
			wants: wants{returnedID: "", returnedOK: false, statusCode: http.StatusUnauthorized, body: unauthorizedBody},
		},
		{
			name:  "2. ContextUserID is non-string type -> returns \"\", false, 401",
			args:  args{ctxUserID: 123},
			wants: wants{returnedID: "", returnedOK: false, statusCode: http.StatusUnauthorized, body: unauthorizedBody},
		},
		{
			name:  "3. ContextUserID is empty string -> returns \"\", false, 401",
			args:  args{ctxUserID: ""},
			wants: wants{returnedID: "", returnedOK: false, statusCode: http.StatusUnauthorized, body: unauthorizedBody},
		},
		{
			name:  "4. valid non-empty userID -> returns userID, true, no response written",
			args:  args{ctxUserID: "user-99"},
			wants: wants{returnedID: "user-99", returnedOK: true, statusCode: http.StatusOK, body: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, c := ginCtx("GET", "/")
			if tt.args.ctxUserID != nil {
				c.Set(ContextUserID, tt.args.ctxUserID)
			}

			gotID, gotOK := MustUserID(c)

			assert.Equal(t, tt.wants.returnedID, gotID, "returned userID")
			assert.Equal(t, tt.wants.returnedOK, gotOK, "returned ok")
			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			if tt.wants.body != "" {
				assert.JSONEq(t, tt.wants.body, w.Body.String(), "response body")
			} else {
				assert.Empty(t, w.Body.String(), "body should be empty on success")
			}
		})
	}
}
