package api

import (
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	"payment-sandbox/app/shared/idempotency"

	"github.com/gin-gonic/gin"
)

func RegisterMerchantRoutes(merchant *gin.RouterGroup, handler *walletHandlers.WalletHandler, idem *idempotency.Middleware) {
	merchant.GET("/wallet", handler.Wallet)
	merchant.GET("/topups", handler.ListMerchantTopups)
	merchant.POST("/topups", idem.Handle(), handler.CreateTopup)
}

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *walletHandlers.WalletHandler) {
	admin.GET("/topups", handler.ListTopups)
	admin.PATCH("/topups/:id/status", handler.UpdateTopupStatus)
}
