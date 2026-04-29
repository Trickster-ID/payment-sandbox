package middleware

import (
	"net/http"
	"net/http/httptest"
	"payment-sandbox/app/modules/users/models/entity"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTService(t *testing.T) {
	svc := JWTService{
		Secret:   "test-secret",
		Duration: time.Hour,
	}

	t.Run("generate and parse token", func(t *testing.T) {
		token, err := svc.GenerateToken("user-1", entity.RoleMerchant)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Test parsing using AuthMiddleware logic (or just jwt package directly)
		// We can test AuthMiddleware instead
	})
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	svc := JWTService{Secret: secret, Duration: time.Hour}

	t.Run("valid token", func(t *testing.T) {
		token, _ := svc.GenerateToken("user-1", entity.RoleMerchant)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)

		AuthMiddleware(secret)(c)

		assert.False(t, c.IsAborted())
		userID, _ := c.Get(ContextUserID)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("missing token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)

		AuthMiddleware(secret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer invalid-token")

		AuthMiddleware(secret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestRequireRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("allowed role", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ContextRole, string(entity.RoleAdmin))

		RequireRoles(entity.RoleAdmin)(c)

		assert.False(t, c.IsAborted())
	})

	t.Run("forbidden role", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ContextRole, string(entity.RoleMerchant))

		RequireRoles(entity.RoleAdmin)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
