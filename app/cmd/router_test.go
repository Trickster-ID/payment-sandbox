package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	ledgerHandlers "payment-sandbox/app/modules/ledger/handlers"
	merchantHandlers "payment-sandbox/app/modules/merchants/handlers"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	usersHandlers "payment-sandbox/app/modules/users/handlers"
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	"payment-sandbox/app/shared/audit"
	"payment-sandbox/app/shared/idempotency"

	"payment-sandbox/app/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	auditLogger := audit.NewNoopLogger()
	idemMW := &idempotency.Middleware{
		Store: &idempotency.Store{TTL: time.Hour},
		Cache: &idempotency.Cache{TTL: time.Hour},
	}
	walletHandler := walletHandlers.NewWalletHandler(nil, auditLogger)
	invoiceHandler := invoiceHandlers.NewInvoiceHandler(nil, auditLogger)
	paymentHandler := paymentHandlers.NewPaymentHandler(nil, auditLogger)
	refundHandler := refundHandlers.NewRefundHandler(nil, auditLogger)
	oauth2Handler := oauth2Handlers.NewOAuth2Handler(nil, cfg)
	ledgerHandler := ledgerHandlers.NewLedgerHandler(nil)
	merchantHandler := merchantHandlers.NewMerchantsHandler(nil)

	router := newRouter(cfg, idemMW, usersHandler, adminHandler, merchantHandler, walletHandler, invoiceHandler, paymentHandler, refundHandler, oauth2Handler, ledgerHandler)
	registered := routeMap(router.Routes())

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "swagger docs", method: "GET", path: "/swagger/*any"},
		{name: "health check", method: "GET", path: "/api/v1/ping"},
		{name: "users register", method: "POST", path: "/api/v1/users/register"},
		{name: "oauth2 token", method: "POST", path: "/api/v1/oauth2/token"},
		{name: "oauth2 introspect", method: "POST", path: "/api/v1/oauth2/introspect"},
		{name: "oauth2 revoke", method: "POST", path: "/api/v1/oauth2/revoke"},
		{name: "oauth2 authorize get", method: "GET", path: "/api/v1/oauth2/authorize"},
		{name: "oauth2 authorize post", method: "POST", path: "/api/v1/oauth2/authorize"},
		{name: "oauth2 userinfo", method: "GET", path: "/api/v1/oauth2/userinfo"},
		{name: "public invoice", method: "GET", path: "/api/v1/pay/:token"},
		{name: "public payment intent", method: "POST", path: "/api/v1/pay/:token/intents"},
		{name: "merchant wallet", method: "GET", path: "/api/v1/merchant/wallet"},
		{name: "merchant wallet transactions", method: "GET", path: "/api/v1/merchant/wallet/transactions"},
		{name: "merchant topup create", method: "POST", path: "/api/v1/merchant/topups"},
		{name: "merchant topup list", method: "GET", path: "/api/v1/merchant/topups"},
		{name: "merchant invoice create", method: "POST", path: "/api/v1/merchant/invoices"},
		{name: "merchant invoice list", method: "GET", path: "/api/v1/merchant/invoices"},
		{name: "merchant invoice detail", method: "GET", path: "/api/v1/merchant/invoices/:id"},
		{name: "merchant refund request", method: "POST", path: "/api/v1/merchant/refunds"},
		{name: "merchant refund list", method: "GET", path: "/api/v1/merchant/refunds"},
		{name: "merchant oauth client create", method: "POST", path: "/api/v1/merchant/clients"},
		{name: "merchant oauth client list", method: "GET", path: "/api/v1/merchant/clients"},
		{name: "merchant oauth client delete", method: "DELETE", path: "/api/v1/merchant/clients/:id"},
		{name: "admin topup list", method: "GET", path: "/api/v1/admin/topups"},
		{name: "admin topup status update", method: "PATCH", path: "/api/v1/admin/topups/:id/status"},
		{name: "admin wallet transactions", method: "GET", path: "/api/v1/admin/wallet/transactions"},
		{name: "admin payment intent list", method: "GET", path: "/api/v1/admin/payment-intents"},
		{name: "admin payment intent status update", method: "PATCH", path: "/api/v1/admin/payment-intents/:id/status"},
		{name: "admin refund list", method: "GET", path: "/api/v1/admin/refunds"},
		{name: "admin refund review", method: "PATCH", path: "/api/v1/admin/refunds/:id/review"},
		{name: "admin refund process", method: "PATCH", path: "/api/v1/admin/refunds/:id/process"},
		{name: "admin stats", method: "GET", path: "/api/v1/admin/stats"},
		{name: "admin ledger account", method: "GET", path: "/api/v1/admin/ledger/accounts/:merchant_id"},
		{name: "admin merchants list", method: "GET", path: "/api/v1/admin/merchants"},
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

func TestNewRouter_TrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.Config{AppPort: "8080", JWTSecret: "test-secret", JWTDuration: time.Hour, ShutdownTTL: time.Second}
	router := newRouter(
		cfg,
		&idempotency.Middleware{Store: &idempotency.Store{TTL: time.Hour}, Cache: &idempotency.Cache{TTL: time.Hour}},
		&usersHandlers.UserHandler{},
		&adminHandlers.AdminHandler{},
		merchantHandlers.NewMerchantsHandler(nil),
		walletHandlers.NewWalletHandler(nil, audit.NewNoopLogger()),
		invoiceHandlers.NewInvoiceHandler(nil, audit.NewNoopLogger()),
		paymentHandlers.NewPaymentHandler(nil, audit.NewNoopLogger()),
		refundHandlers.NewRefundHandler(nil, audit.NewNoopLogger()),
		oauth2Handlers.NewOAuth2Handler(nil, cfg),
		ledgerHandlers.NewLedgerHandler(nil),
	)
	router.GET("/client-ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "untrusted proxy ignores forwarded IP", remoteAddr: "203.0.113.10:1234", want: "203.0.113.10"},
		{name: "Docker bridge proxy accepts forwarded IP", remoteAddr: "172.17.0.1:1234", want: "198.51.100.5"},
		{name: "loopback proxy accepts forwarded IP", remoteAddr: "127.0.0.1:1234", want: "198.51.100.5"},
		{name: "IPv6 loopback proxy accepts forwarded IP", remoteAddr: "[::1]:1234", want: "198.51.100.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("X-Forwarded-For", "198.51.100.5")
			res := httptest.NewRecorder()

			router.ServeHTTP(res, req)

			require.Equal(t, http.StatusOK, res.Code)
			assert.Equal(t, tc.want, res.Body.String())
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
