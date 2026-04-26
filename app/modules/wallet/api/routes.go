package api

import (
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterMerchantRoutes(merchant *gin.RouterGroup, handler *walletHandlers.WalletHandler) {
	merchant.GET("/wallet", handler.Wallet)
	merchant.POST("/topups", handler.CreateTopup)
}

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *walletHandlers.WalletHandler) {
	admin.GET("/topups", handler.ListTopups)
	admin.PATCH("/topups/:id/status", handler.UpdateTopupStatus)
}
