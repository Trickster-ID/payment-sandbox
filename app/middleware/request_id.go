package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ContextRequestID = "request_id"

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Set(ContextRequestID, requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}
