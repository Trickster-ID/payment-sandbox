package main

import (
	"testing"
	"time"

	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	usersHandlers "payment-sandbox/app/modules/users/handlers"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	"payment-sandbox/app/shared/journeylog"

	"payment-sandbox/app/config"

	"github.com/gin-gonic/gin"
)

func TestNewRouter_RegistersExpectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.Config{
		AppPort:     "8080",
		JWTSecret:   "test-secret",
		JWTDuration: time.Hour,
		ShutdownTTL: time.Second,
	}

	usersHandler := &usersHandlers.UserHandler{}
	adminHandler := &adminHandlers.AdminHandler{}
	journeyLogger := journeylog.NewNoopJourneyLogger()
	walletHandler := walletHandlers.NewWalletHandler(nil, journeyLogger)
	invoiceHandler := invoiceHandlers.NewInvoiceHandler(nil, journeyLogger)
	paymentHandler := paymentHandlers.NewPaymentHandler(nil, journeyLogger)
	refundHandler := refundHandlers.NewRefundHandler(nil, journeyLogger)
	oauth2Handler := oauth2Handlers.NewOAuth2Handler(nil)

	router := newRouter(cfg, usersHandler, adminHandler, walletHandler, invoiceHandler, paymentHandler, refundHandler, oauth2Handler)
	registered := routeMap(router.Routes())

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "swagger docs", method: "GET", path: "/swagger/*any"},
		{name: "health check", method: "GET", path: "/api/v1/ping"},
		{name: "users register", method: "POST", path: "/api/v1/users/register"},
		{name: "public invoice", method: "GET", path: "/api/v1/pay/:token"},
		{name: "public payment intent", method: "POST", path: "/api/v1/pay/:token/intents"},
		{name: "merchant wallet", method: "GET", path: "/api/v1/merchant/wallet"},
		{name: "merchant topup create", method: "POST", path: "/api/v1/merchant/topups"},
		{name: "merchant invoice create", method: "POST", path: "/api/v1/merchant/invoices"},
		{name: "merchant invoice list", method: "GET", path: "/api/v1/merchant/invoices"},
		{name: "merchant invoice detail", method: "GET", path: "/api/v1/merchant/invoices/:id"},
		{name: "merchant refund request", method: "POST", path: "/api/v1/merchant/refunds"},
		{name: "admin topup list", method: "GET", path: "/api/v1/admin/topups"},
		{name: "admin topup status update", method: "PATCH", path: "/api/v1/admin/topups/:id/status"},
		{name: "admin payment intent list", method: "GET", path: "/api/v1/admin/payment-intents"},
		{name: "admin payment intent status update", method: "PATCH", path: "/api/v1/admin/payment-intents/:id/status"},
		{name: "admin refund list", method: "GET", path: "/api/v1/admin/refunds"},
		{name: "admin refund review", method: "PATCH", path: "/api/v1/admin/refunds/:id/review"},
		{name: "admin refund process", method: "PATCH", path: "/api/v1/admin/refunds/:id/process"},
		{name: "admin stats", method: "GET", path: "/api/v1/admin/stats"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.method + " " + tc.path
			if !registered[key] {
				t.Fatalf("route not registered: %s", key)
			}
		})
	}
}

func routeMap(routes gin.RoutesInfo) map[string]bool {
	out := make(map[string]bool, len(routes))
	for _, route := range routes {
		out[route.Method+" "+route.Path] = true
	}
	return out
}
