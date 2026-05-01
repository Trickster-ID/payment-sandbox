package api

import (
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	"payment-sandbox/app/shared/idempotency"

	"github.com/gin-gonic/gin"
)

func RegisterMerchantRoutes(merchant *gin.RouterGroup, handler *invoiceHandlers.InvoiceHandler, idem *idempotency.Middleware) {
	merchant.POST("/invoices", idem.Handle(), handler.CreateInvoice)
	merchant.GET("/invoices", handler.ListInvoices)
	merchant.GET("/invoices/:id", handler.GetInvoice)
}
