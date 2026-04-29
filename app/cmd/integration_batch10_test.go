package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/middleware"
	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	adminRepo "payment-sandbox/app/modules/admin/repositories"
	adminSvc "payment-sandbox/app/modules/admin/services"
	authHandlers "payment-sandbox/app/modules/auth/handlers"
	authRepo "payment-sandbox/app/modules/auth/repositories"
	authSvc "payment-sandbox/app/modules/auth/services"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	invoiceRepo "payment-sandbox/app/modules/invoice/repositories"
	invoiceSvc "payment-sandbox/app/modules/invoice/services"
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"
	paymentRepo "payment-sandbox/app/modules/payment/repositories"
	paymentSvc "payment-sandbox/app/modules/payment/services"
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	refundRepo "payment-sandbox/app/modules/refund/repositories"
	refundSvc "payment-sandbox/app/modules/refund/services"
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	walletRepo "payment-sandbox/app/modules/wallet/repositories"
	walletSvc "payment-sandbox/app/modules/wallet/services"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	oauth2Repo "payment-sandbox/app/modules/oauth2/repositories"
	oauth2Svc "payment-sandbox/app/modules/oauth2/services"
	"payment-sandbox/app/shared/database"
	"payment-sandbox/app/shared/journeylog"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type integrationSuite struct {
	router *gin.Engine
	db     *sql.DB
}

type apiEnvelope struct {
	Data  any `json:"data"`
	Meta  any `json:"meta"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details"`
	} `json:"error"`
}

func TestIntegration_AuthAndAccessGuards(t *testing.T) {
	suite := setupIntegrationSuite(t)

	email := integrationEmail(t.Name())
	password := "merchant1234"

	status, registerResp := doJSONRequest(t, suite.router, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name":     "Batch10 Merchant",
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, registerResp.Error)
	registerData := mustMap(t, registerResp.Data)
	assert.Equal(t, email, registerData["email"])
	assert.Equal(t, "MERCHANT", registerData["role"])

	status, duplicateResp := doJSONRequest(t, suite.router, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name":     "Batch10 Merchant",
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusBadRequest, status)
	require.NotNil(t, duplicateResp.Error)
	assert.Equal(t, "validation_error", duplicateResp.Error.Code)

	merchantToken := loginAndGetToken(t, suite.router, email, password)

	status, unauthorizedResp := doJSONRequest(t, suite.router, http.MethodGet, "/api/v1/merchant/wallet", "", nil)
	require.Equal(t, http.StatusUnauthorized, status)
	require.NotNil(t, unauthorizedResp.Error)
	assert.Equal(t, "auth_missing_bearer_token", unauthorizedResp.Error.Code)

	status, forbiddenResp := doJSONRequest(t, suite.router, http.MethodGet, "/api/v1/admin/topups", merchantToken, nil)
	require.Equal(t, http.StatusForbidden, status)
	require.NotNil(t, forbiddenResp.Error)
	assert.Equal(t, "auth_forbidden", forbiddenResp.Error.Code)

	status, invalidTokenResp := doJSONRequest(t, suite.router, http.MethodGet, "/api/v1/merchant/wallet", "not-a-jwt-token", nil)
	require.Equal(t, http.StatusUnauthorized, status)
	require.NotNil(t, invalidTokenResp.Error)
	assert.Equal(t, "auth_invalid_token", invalidTokenResp.Error.Code)

	adminToken := loginAndGetToken(t, suite.router, "admin@sandbox.local", "admin1234")
	status, adminOnMerchantResp := doJSONRequest(t, suite.router, http.MethodGet, "/api/v1/merchant/wallet", adminToken, nil)
	require.Equal(t, http.StatusForbidden, status)
	require.NotNil(t, adminOnMerchantResp.Error)
	assert.Equal(t, "auth_forbidden", adminOnMerchantResp.Error.Code)
}

func TestIntegration_AdminPaymentAndRefundFlows(t *testing.T) {
	suite := setupIntegrationSuite(t)

	email := integrationEmail(t.Name())
	password := "merchant1234"
	registerMerchant(t, suite.router, email, password)

	merchantToken := loginAndGetToken(t, suite.router, email, password)
	adminToken := loginAndGetToken(t, suite.router, "admin@sandbox.local", "admin1234")

	topupID := createTopup(t, suite.router, merchantToken, 1000)
	updateTopupStatus(t, suite.router, adminToken, topupID, "SUCCESS", http.StatusOK)

	dueDate := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	invoiceID, invoiceToken := createInvoice(t, suite.router, merchantToken, 200, dueDate)
	require.NotEmpty(t, invoiceID)

	paymentIntentID := createPaymentIntent(t, suite.router, invoiceToken, "WALLET")
	updatePaymentIntentStatus(t, suite.router, adminToken, paymentIntentID, "SUCCESS", http.StatusOK, "")
	updatePaymentIntentStatus(t, suite.router, adminToken, paymentIntentID, "FAILED", http.StatusBadRequest, "payment_intent_update_failed")

	assertPaymentAndInvoiceStatus(t, suite.db, paymentIntentID, "SUCCESS", "PAID")

	refundID := requestRefund(t, suite.router, merchantToken, paymentIntentID)
	reviewRefund(t, suite.router, adminToken, refundID, "APPROVE", http.StatusOK, "")
	processRefund(t, suite.router, adminToken, refundID, "SUCCESS", http.StatusOK, "")
	processRefund(t, suite.router, adminToken, refundID, "FAILED", http.StatusBadRequest, "refund_process_failed")

	assertRefundAndBalance(t, suite.db, refundID, "SUCCESS", 800)
}

func TestIntegration_InvalidPayloadAndTransitionNegatives(t *testing.T) {
	suite := setupIntegrationSuite(t)

	email := integrationEmail(t.Name())
	password := "merchant1234"
	registerMerchant(t, suite.router, email, password)

	merchantToken := loginAndGetToken(t, suite.router, email, password)
	adminToken := loginAndGetToken(t, suite.router, "admin@sandbox.local", "admin1234")

	topupID := createTopup(t, suite.router, merchantToken, 1000)
	updateTopupStatus(t, suite.router, adminToken, topupID, "SUCCESS", http.StatusOK)

	dueDate := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	_, invoiceToken := createInvoice(t, suite.router, merchantToken, 200, dueDate)
	paymentIntentID := createPaymentIntent(t, suite.router, invoiceToken, "WALLET")
	updatePaymentIntentStatus(t, suite.router, adminToken, paymentIntentID, "SUCCESS", http.StatusOK, "")

	refundID := requestRefund(t, suite.router, merchantToken, paymentIntentID)

	tests := []struct {
		name        string
		method      string
		path        string
		token       string
		body        any
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "register invalid email",
			method:      http.MethodPost,
			path:        "/api/v1/auth/register",
			body:        map[string]any{"name": "Bad Email", "email": "invalid-email", "password": "merchant1234"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "login invalid credentials",
			method:      http.MethodPost,
			path:        "/api/v1/auth/login",
			body:        map[string]any{"email": email, "password": "wrong-password"},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "auth_invalid_credentials",
		},
		{
			name:        "invoice invalid due date format",
			method:      http.MethodPost,
			path:        "/api/v1/merchant/invoices",
			token:       merchantToken,
			body:        map[string]any{"customer_name": "Invalid Due Date", "customer_email": "invalid.due@example.com", "amount": 10, "description": "bad date", "due_date": "2026-04-30"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "invoice_create_failed",
		},
		{
			name:        "create payment intent invalid method",
			method:      http.MethodPost,
			path:        "/api/v1/pay/" + invoiceToken + "/intents",
			body:        map[string]any{"method": "CRYPTO"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "payment_intent_create_failed",
		},
		{
			name:        "update payment intent missing status field",
			method:      http.MethodPatch,
			path:        "/api/v1/admin/payment-intents/" + paymentIntentID + "/status",
			token:       adminToken,
			body:        map[string]any{},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "review refund invalid decision",
			method:      http.MethodPatch,
			path:        "/api/v1/admin/refunds/" + refundID + "/review",
			token:       adminToken,
			body:        map[string]any{"decision": "MAYBE"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "refund_review_failed",
		},
		{
			name:        "process refund before approval",
			method:      http.MethodPatch,
			path:        "/api/v1/admin/refunds/" + refundID + "/process",
			token:       adminToken,
			body:        map[string]any{"status": "SUCCESS"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "refund_process_failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, resp := doJSONRequest(t, suite.router, tc.method, tc.path, tc.token, tc.body)
			require.Equal(t, tc.wantStatus, status)
			require.NotNil(t, resp.Error)
			assert.Equal(t, tc.wantErrCode, resp.Error.Code)
		})
	}
}

func setupIntegrationSuite(t *testing.T) *integrationSuite {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	cfg := config.Load()
	cfg.MongoJourneyEnable = false
	cfg.JWTSecret = "batch10-integration-secret"

	db, err := database.New(cfg)
	if err != nil {
		t.Skipf("skipping integration tests; postgres unavailable: %v", err)
	}
	if err := ensureRequiredSchema(db); err != nil {
		t.Skipf("skipping integration tests; schema is not ready: %v", err)
	}

	require.NoError(t, cleanupIntegrationData(db))
	t.Cleanup(func() {
		if err := cleanupIntegrationData(db); err != nil {
			t.Logf("integration cleanup failed: %v", err)
		}
		_ = db.Close()
	})

	authRepository := authRepo.NewAuthRepository(db)
	require.NoError(t, authRepository.EnsureAdminSeed())

	jwtService := middleware.NewJWTService(cfg)
	authService := authSvc.NewAuthService(authRepository, jwtService)
	adminService := adminSvc.NewAdminService(adminRepo.NewAdminRepository(db))
	walletService := walletSvc.NewWalletService(walletRepo.NewWalletRepository(db))
	invoiceService := invoiceSvc.NewInvoiceService(invoiceRepo.NewInvoiceRepository(db))
	paymentService := paymentSvc.NewPaymentService(paymentRepo.NewPaymentRepository(db))
	refundService := refundSvc.NewRefundService(refundRepo.NewRefundRepository(db))
	oauth2Service := oauth2Svc.NewOAuth2Service(oauth2Repo.NewOAuth2Repository(db), cfg)

	journeyLogger := journeylog.NewNoopJourneyLogger()
	router := newRouter(
		cfg,
		authHandlers.NewAuthHandler(authService),
		adminHandlers.NewAdminHandler(adminService),
		walletHandlers.NewWalletHandler(walletService, journeyLogger),
		invoiceHandlers.NewInvoiceHandler(invoiceService, journeyLogger),
		paymentHandlers.NewPaymentHandler(paymentService, journeyLogger),
		refundHandlers.NewRefundHandler(refundService, journeyLogger),
		oauth2Handlers.NewOAuth2Handler(oauth2Service),
	)

	return &integrationSuite{
		router: router,
		db:     db,
	}
}

func ensureRequiredSchema(db *sql.DB) error {
	required := []string{"users", "merchants", "invoices", "payment_intents", "refunds", "topups"}
	for _, table := range required {
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public'
					AND table_name = $1
			)
		`, table).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("required table %s not found", table)
		}
	}
	return nil
}

func cleanupIntegrationData(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM refunds;
		DELETE FROM payment_intents;
		DELETE FROM invoices;
		DELETE FROM topups;
		DELETE FROM merchants
		WHERE user_id IN (
			SELECT id FROM users WHERE email LIKE 'it_%@example.com'
		);
		DELETE FROM users WHERE email LIKE 'it_%@example.com';
	`)
	return err
}

func registerMerchant(t *testing.T, router *gin.Engine, email, password string) {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name":     "Batch10 Merchant",
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
}

func loginAndGetToken(t *testing.T, router *gin.Engine, email, password string) string {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)

	data := mustMap(t, resp.Data)
	token, ok := data["access_token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, token)
	return token
}

func createTopup(t *testing.T, router *gin.Engine, merchantToken string, amount float64) string {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/topups", merchantToken, map[string]any{
		"amount": amount,
	})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)

	data := mustMap(t, resp.Data)
	topupID, ok := data["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, topupID)
	return topupID
}

func updateTopupStatus(t *testing.T, router *gin.Engine, adminToken, topupID, statusValue string, expectedStatus int) {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/topups/"+topupID+"/status", adminToken, map[string]any{
		"status": statusValue,
	})
	require.Equal(t, expectedStatus, status)
	if expectedStatus >= 400 {
		require.NotNil(t, resp.Error)
		return
	}
	require.Nil(t, resp.Error)
}

func createInvoice(t *testing.T, router *gin.Engine, merchantToken string, amount float64, dueDate string) (string, string) {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/invoices", merchantToken, map[string]any{
		"customer_name":  "Integration Customer",
		"customer_email": "integration.customer@example.com",
		"amount":         amount,
		"description":    "batch 10 integration invoice",
		"due_date":       dueDate,
	})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)

	data := mustMap(t, resp.Data)
	invoiceID, ok := data["id"].(string)
	require.True(t, ok)
	token, ok := data["payment_link_token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, invoiceID)
	require.NotEmpty(t, token)
	return invoiceID, token
}

func createPaymentIntent(t *testing.T, router *gin.Engine, invoiceToken, method string) string {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPost, "/api/v1/pay/"+invoiceToken+"/intents", "", map[string]any{
		"method": method,
	})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)

	data := mustMap(t, resp.Data)
	intent := mustMap(t, data["payment_intent"])
	paymentIntentID, ok := intent["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, paymentIntentID)
	return paymentIntentID
}

func updatePaymentIntentStatus(t *testing.T, router *gin.Engine, adminToken, paymentIntentID, nextStatus string, expectedHTTP int, expectedErrCode string) {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/payment-intents/"+paymentIntentID+"/status", adminToken, map[string]any{
		"status": nextStatus,
	})
	require.Equal(t, expectedHTTP, status)
	if expectedHTTP >= 400 {
		require.NotNil(t, resp.Error)
		if expectedErrCode != "" {
			assert.Equal(t, expectedErrCode, resp.Error.Code)
		}
		return
	}
	require.Nil(t, resp.Error)
}

func requestRefund(t *testing.T, router *gin.Engine, merchantToken, paymentIntentID string) string {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/refunds", merchantToken, map[string]any{
		"payment_intent_id": paymentIntentID,
		"reason":            "integration refund request",
	})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	refundID, ok := data["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, refundID)
	return refundID
}

func reviewRefund(t *testing.T, router *gin.Engine, adminToken, refundID, decision string, expectedHTTP int, expectedErrCode string) {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/refunds/"+refundID+"/review", adminToken, map[string]any{
		"decision": decision,
	})
	require.Equal(t, expectedHTTP, status)
	if expectedHTTP >= 400 {
		require.NotNil(t, resp.Error)
		if expectedErrCode != "" {
			assert.Equal(t, expectedErrCode, resp.Error.Code)
		}
		return
	}
	require.Nil(t, resp.Error)
}

func processRefund(t *testing.T, router *gin.Engine, adminToken, refundID, statusValue string, expectedHTTP int, expectedErrCode string) {
	t.Helper()
	status, resp := doJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/refunds/"+refundID+"/process", adminToken, map[string]any{
		"status": statusValue,
	})
	require.Equal(t, expectedHTTP, status)
	if expectedHTTP >= 400 {
		require.NotNil(t, resp.Error)
		if expectedErrCode != "" {
			assert.Equal(t, expectedErrCode, resp.Error.Code)
		}
		return
	}
	require.Nil(t, resp.Error)
}

func assertPaymentAndInvoiceStatus(t *testing.T, db *sql.DB, paymentIntentID, expectedPayment, expectedInvoice string) {
	t.Helper()
	var paymentStatus string
	var invoiceStatus string
	err := db.QueryRow(`
		SELECT pi.status::text, inv.status::text
		FROM payment_intents pi
		JOIN invoices inv ON inv.id = pi.invoice_id
		WHERE pi.id = $1 AND pi.deleted_at IS NULL AND inv.deleted_at IS NULL
	`, paymentIntentID).Scan(&paymentStatus, &invoiceStatus)
	require.NoError(t, err)
	assert.Equal(t, expectedPayment, paymentStatus)
	assert.Equal(t, expectedInvoice, invoiceStatus)
}

func assertRefundAndBalance(t *testing.T, db *sql.DB, refundID, expectedRefundStatus string, expectedBalance float64) {
	t.Helper()
	var refundStatus string
	var merchantBalance float64
	err := db.QueryRow(`
		SELECT r.status::text, m.balance::double precision
		FROM refunds r
		JOIN payment_intents pi ON pi.id = r.payment_intent_id AND pi.deleted_at IS NULL
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		JOIN merchants m ON m.id = inv.merchant_id AND m.deleted_at IS NULL
		WHERE r.id = $1 AND r.deleted_at IS NULL
	`, refundID).Scan(&refundStatus, &merchantBalance)
	require.NoError(t, err)
	assert.Equal(t, expectedRefundStatus, refundStatus)
	assert.Equal(t, expectedBalance, merchantBalance)
}

func doJSONRequest(t *testing.T, router *gin.Engine, method, path, token string, body any) (int, apiEnvelope) {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var envelope apiEnvelope
	if rec.Body.Len() > 0 {
		err = json.Unmarshal(rec.Body.Bytes(), &envelope)
		require.NoError(t, err)
	}

	return rec.Code, envelope
}

func mustMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	require.True(t, ok)
	return m
}

func integrationEmail(testName string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(testName, "/", "_"))
	return fmt.Sprintf("it_%s_%d@example.com", sanitized, time.Now().UnixNano())
}
