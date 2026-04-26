package api

import (
	authHandlers "payment-sandbox/app/modules/auth/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterPublicRoutes(v1 *gin.RouterGroup, handler *authHandlers.AuthHandler) {
	v1.POST("/auth/register", handler.RegisterMerchant)
	v1.POST("/auth/login", handler.Login)
}
