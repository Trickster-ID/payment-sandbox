package middleware

import (
	"strings"
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/modules/users/models/entity"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserID   = "user_id"
	ContextRole     = "role"
	ContextClientID = "client_id"
	ContextScope    = "scope"
)

type JWTService struct {
	Secret   string
	Duration time.Duration
}

func NewJWTService(cfg config.Config) JWTService {
	return JWTService{
		Secret:   cfg.JWTSecret,
		Duration: cfg.JWTDuration,
	}
}

type Claims struct {
	UserID   string      `json:"user_id,omitempty"`
	Role     entity.Role `json:"role,omitempty"`
	ClientID string      `json:"client_id,omitempty"`
	Scope    string      `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

func (j JWTService) GenerateToken(userID string, role entity.Role) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.Duration)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.Secret))
}

func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if tokenString == "" || tokenString == header {
			response.Fail(c, appErrors.Unauthorized("auth_missing_bearer_token", "missing bearer token", nil))
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			response.Fail(c, appErrors.Unauthorized("auth_invalid_token", "invalid token", nil))
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			response.Fail(c, appErrors.Unauthorized("auth_invalid_claims", "invalid claims", nil))
			c.Abort()
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, string(claims.Role))
		c.Set(ContextClientID, claims.ClientID)
		c.Set(ContextScope, claims.Scope)
		c.Next()
	}
}

func RequireRoles(allowed ...entity.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleAny, found := c.Get(ContextRole)
		if !found {
			response.Fail(c, appErrors.Forbidden("auth_role_not_found", "role not found", nil))
			c.Abort()
			return
		}
		role, ok := roleAny.(string)
		if !ok {
			response.Fail(c, appErrors.Forbidden("auth_invalid_role", "invalid role", nil))
			c.Abort()
			return
		}
		for _, allow := range allowed {
			if role == string(allow) {
				c.Next()
				return
			}
		}
		response.Fail(c, appErrors.Forbidden("auth_forbidden", "forbidden", nil))
		c.Abort()
	}
}

func RequireScopes(required ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeAny, found := c.Get(ContextScope)
		if !found {
			response.Fail(c, appErrors.Forbidden("auth_scope_not_found", "scope not found", nil))
			c.Abort()
			return
		}
		scopeStr, ok := scopeAny.(string)
		if !ok {
			response.Fail(c, appErrors.Forbidden("auth_invalid_scope", "invalid scope", nil))
			c.Abort()
			return
		}

		scopes := strings.Fields(scopeStr)
		for _, req := range required {
			foundReq := false
			for _, s := range scopes {
				if s == req {
					foundReq = true
					break
				}
			}
			if !foundReq {
				response.Fail(c, appErrors.Forbidden("auth_insufficient_scope", "insufficient scope: "+req, nil))
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func MustUserID(c *gin.Context) (string, bool) {
	value, found := c.Get(ContextUserID)
	if !found {
		response.Fail(c, appErrors.Unauthorized("auth_unauthorized", "unauthorized", nil))
		return "", false
	}
	userID, ok := value.(string)
	if !ok || userID == "" {
		response.Fail(c, appErrors.Unauthorized("auth_unauthorized", "unauthorized", nil))
		return "", false
	}
	return userID, true
}
