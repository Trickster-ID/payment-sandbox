package api

import (
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterPublicRoutes(v1 *gin.RouterGroup, handler *paymentHandlers.PaymentHandler) {
	v1.GET("/pay/:token", handler.PublicInvoice)
	v1.POST("/pay/:token/intents", handler.CreatePaymentIntent)
}

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *paymentHandlers.PaymentHandler) {
	admin.GET("/payment-intents", handler.ListPaymentIntents)
	admin.PATCH("/payment-intents/:id/status", handler.UpdatePaymentIntentStatus)
}
