package middleware

import "github.com/gin-gonic/gin"

// CORSMiddleware allows any origin present in allowedOrigins. Falls back to
// the first configured origin when the request Origin header is absent or
// not in the list, preserving prior single-origin behavior.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	defaultOrigin := ""
	if len(allowedOrigins) > 0 {
		defaultOrigin = allowedOrigins[0]
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowOrigin := defaultOrigin
		if allowed[origin] {
			allowOrigin = origin
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Channel, X-Request-Id, Idempotency-Key")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
