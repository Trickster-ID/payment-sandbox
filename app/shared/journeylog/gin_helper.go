package journeylog

import (
	"log"

	"payment-sandbox/app/middleware"

	"github.com/gin-gonic/gin"
)

func ActorFromContext(c *gin.Context) (string, string) {
	actorID := ""
	actorRole := "PUBLIC"

	if value, ok := c.Get(middleware.ContextUserID); ok {
		if userID, ok := value.(string); ok {
			actorID = userID
		}
	}
	if value, ok := c.Get(middleware.ContextRole); ok {
		if role, ok := value.(string); ok && role != "" {
			actorRole = role
		}
	}

	return actorID, actorRole
}

func RequestIDFromContext(c *gin.Context) string {
	if value, ok := c.Get(middleware.ContextRequestID); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func LogBestEffort(c *gin.Context, logger IJourneyLogger, event Event) {
	if err := logger.Log(c.Request.Context(), event); err != nil {
		log.Printf("journey log write failed: %v", err)
	}
}
