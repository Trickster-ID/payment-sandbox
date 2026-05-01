package api

import (
	"payment-sandbox/app/modules/ledger/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(admin *gin.RouterGroup, h *handlers.LedgerHandler) {
	admin.GET("/ledger/accounts/:merchant_id", h.GetMerchantAccount)
}
