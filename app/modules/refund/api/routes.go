package api

import (
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	"payment-sandbox/app/shared/idempotency"

	"github.com/gin-gonic/gin"
)

func RegisterMerchantRoutes(merchant *gin.RouterGroup, handler *refundHandlers.RefundHandler, idem *idempotency.Middleware) {
	merchant.POST("/refunds", idem.Handle(), handler.RequestRefund)
	merchant.GET("/refunds", handler.MerchantListRefunds)
}

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *refundHandlers.RefundHandler) {
	admin.GET("/refunds", handler.ListRefunds)
	admin.PATCH("/refunds/:id/review", handler.ReviewRefund)
	admin.PATCH("/refunds/:id/process", handler.ProcessRefund)
}
