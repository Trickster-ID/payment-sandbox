package api

import (
	merchantHandlers "payment-sandbox/app/modules/merchants/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *merchantHandlers.MerchantsHandler) {
	admin.GET("/merchants", handler.ListMerchants)
}
