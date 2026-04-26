package api

import (
	refundHandlers "payment-sandbox/app/modules/refund/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterMerchantRoutes(merchant *gin.RouterGroup, handler *refundHandlers.RefundHandler) {
	merchant.POST("/refunds", handler.RequestRefund)
}

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *refundHandlers.RefundHandler) {
	admin.GET("/refunds", handler.ListRefunds)
	admin.PATCH("/refunds/:id/review", handler.ReviewRefund)
	admin.PATCH("/refunds/:id/process", handler.ProcessRefund)
}
