package api

import (
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterMerchantRoutes(merchant *gin.RouterGroup, handler *invoiceHandlers.InvoiceHandler) {
	merchant.POST("/invoices", handler.CreateInvoice)
	merchant.GET("/invoices", handler.ListInvoices)
	merchant.GET("/invoices/:id", handler.GetInvoice)
}
