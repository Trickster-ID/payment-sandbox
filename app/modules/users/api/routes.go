package api

import (
	userHandlers "payment-sandbox/app/modules/users/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterPublicRoutes(v1 *gin.RouterGroup, handler *userHandlers.UserHandler) {
	v1.POST("/users/register", handler.RegisterMerchant)
}
