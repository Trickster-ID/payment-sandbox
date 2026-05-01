package audit

import (
	"log"
	"strings"

	"payment-sandbox/app/middleware"

	"github.com/gin-gonic/gin"
)

func ActorFromContext(c *gin.Context) (string, string) {
	actorID := ""
	actorType := "public"

	if value, ok := c.Get(middleware.ContextUserID); ok {
		if userID, ok := value.(string); ok {
			actorID = userID
		}
	}
	if value, ok := c.Get(middleware.ContextRole); ok {
		if role, ok := value.(string); ok && role != "" {
			actorType = strings.ToLower(role)
		}
	}

	return actorID, actorType
}

func RequestIDFromContext(c *gin.Context) string {
	if value, ok := c.Get(middleware.ContextRequestID); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func LogBestEffort(c *gin.Context, logger IAuditLogger, event Event) {
	if err := logger.Log(c.Request.Context(), event); err != nil {
		log.Printf("audit log write failed: %v", err)
	}
}
