package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"payment-sandbox/app/config"
	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	ledgerHandlers "payment-sandbox/app/modules/ledger/handlers"
	merchantHandlers "payment-sandbox/app/modules/merchants/handlers"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	userHandlers "payment-sandbox/app/modules/users/handlers"
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	"payment-sandbox/app/shared/audit"
	"payment-sandbox/app/shared/idempotency"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	e2eClientID     = "00000000-0000-4000-8000-000000000001"
	e2eClientSecret = "payment-sandbox-secret"
)

type e2eTokenPair struct {
	AccessToken  string
	RefreshToken string
}

func TestE2EAPI_CompletePaymentSandboxLifecycle(t *testing.T) {
	suite := setupIntegrationSuite(t)

	status, pingResp := doE2EJSONRequest(t, suite.router, http.MethodGet, "/api/v1/ping", "", nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, pingResp.Error)
	assert.NotEmpty(t, pingResp.Data)

	email := integrationEmail(t.Name())
	password := "merchant1234"
	merchantUserID := e2eRegisterMerchant(t, suite.router, email, password)
	merchantTokens := e2ePasswordToken(t, suite.router, email, password)
	adminTokens := e2ePasswordToken(t, suite.router, "admin@sandbox.local", "admin1234")

	e2eAssertOAuth2Protocol(t, suite.router, merchantTokens, merchantUserID)

	merchantID := e2eAssertMerchantWallet(t, suite.router, merchantTokens.AccessToken, 0)
	e2eAssertAdminMerchants(t, suite.router, adminTokens.AccessToken, email, merchantID)
	e2eAssertLedgerAccount(t, suite.router, adminTokens.AccessToken, merchantID)

	topupID := e2eCreateTopupWithIdempotency(t, suite.router, merchantTokens.AccessToken, 1000)
	e2eListContainsID(t, suite.router, http.MethodGet, "/api/v1/merchant/topups?page=1&limit=10", merchantTokens.AccessToken, topupID)
	e2eListContainsID(t, suite.router, http.MethodGet, "/api/v1/admin/topups", adminTokens.AccessToken, topupID)
	e2eUpdateTopupStatus(t, suite.router, adminTokens.AccessToken, topupID, "SUCCESS", http.StatusOK, "")
	e2eUpdateTopupStatus(t, suite.router, adminTokens.AccessToken, topupID, "FAILED", http.StatusBadRequest, "topup_update_failed")
	e2eAssertMerchantWallet(t, suite.router, merchantTokens.AccessToken, 1000)

	dueDate := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	invoiceID, paymentToken := e2eCreateInvoiceWithIdempotency(t, suite.router, merchantTokens.AccessToken, 200, dueDate)
	e2eListContainsID(t, suite.router, http.MethodGet, "/api/v1/merchant/invoices?status=PENDING&page=1&limit=10", merchantTokens.AccessToken, invoiceID)
	e2eGetInvoice(t, suite.router, merchantTokens.AccessToken, invoiceID, "PENDING")
	e2eGetPublicInvoice(t, suite.router, paymentToken, invoiceID, "PENDING")

	paymentIntentID := e2eCreatePaymentIntent(t, suite.router, paymentToken, "WALLET")
	e2eListContainsID(t, suite.router, http.MethodGet, "/api/v1/admin/payment-intents?status=PENDING", adminTokens.AccessToken, paymentIntentID)
	e2eUpdatePaymentIntentStatus(t, suite.router, adminTokens.AccessToken, paymentIntentID, "SUCCESS", http.StatusOK, "")
	e2eUpdatePaymentIntentStatus(t, suite.router, adminTokens.AccessToken, paymentIntentID, "FAILED", http.StatusBadRequest, "payment_intent_update_failed")
	assertPaymentAndInvoiceStatus(t, suite.db, paymentIntentID, "SUCCESS", "PAID")
	e2eGetInvoice(t, suite.router, merchantTokens.AccessToken, invoiceID, "PAID")
	e2eAssertMerchantWallet(t, suite.router, merchantTokens.AccessToken, 1200)
	e2eAssertWalletTransactions(t, suite.router, merchantTokens.AccessToken, 2)
	e2eAssertAdminWalletTransactions(t, suite.router, adminTokens.AccessToken, merchantID, 2)

	refundID := e2eRequestRefundWithIdempotency(t, suite.router, merchantTokens.AccessToken, invoiceID)
	e2eListContainsID(t, suite.router, http.MethodGet, "/api/v1/merchant/refunds?status=REQUESTED", merchantTokens.AccessToken, refundID)
	e2eListContainsID(t, suite.router, http.MethodGet, "/api/v1/admin/refunds?status=REQUESTED", adminTokens.AccessToken, refundID)
	e2eReviewRefund(t, suite.router, adminTokens.AccessToken, refundID, "APPROVE", http.StatusOK, "")
	e2eProcessRefund(t, suite.router, adminTokens.AccessToken, refundID, "SUCCESS", http.StatusOK, "")
	e2eProcessRefund(t, suite.router, adminTokens.AccessToken, refundID, "FAILED", http.StatusBadRequest, "refund_process_failed")
	assertRefundAndBalance(t, suite.db, refundID, "SUCCESS", 1000)
	e2eAssertMerchantWallet(t, suite.router, merchantTokens.AccessToken, 1000)
	e2eAssertAdminStats(t, suite.router, adminTokens.AccessToken, merchantID, 1, 200, 200)
}

func TestE2EAPI_AllRouteNegativeCases(t *testing.T) {
	suite := setupIntegrationSuite(t)

	email := integrationEmail(t.Name())
	password := "merchant1234"
	e2eRegisterMerchant(t, suite.router, email, password)
	merchantTokens := e2ePasswordToken(t, suite.router, email, password)
	adminTokens := e2ePasswordToken(t, suite.router, "admin@sandbox.local", "admin1234")
	dueDate := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	invoiceID, paymentToken := e2eCreateInvoiceWithIdempotency(t, suite.router, merchantTokens.AccessToken, 200, dueDate)
	paymentIntentID := e2eCreatePaymentIntent(t, suite.router, paymentToken, "WALLET")

	status, noIDKeyResp := doE2EJSONRequest(t, suite.router, http.MethodPost, "/api/v1/merchant/topups", merchantTokens.AccessToken, map[string]any{"amount": 100}, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.NotNil(t, noIDKeyResp.Error)
	assert.Equal(t, "idempotency_key_required", noIDKeyResp.Error.Code)

	tests := []struct {
		name        string
		method      string
		path        string
		token       string
		body        any
		headers     map[string]string
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "missing bearer on merchant wallet",
			method:      http.MethodGet,
			path:        "/api/v1/merchant/wallet",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "auth_missing_bearer_token",
		},
		{
			name:        "malformed bearer on merchant wallet",
			method:      http.MethodGet,
			path:        "/api/v1/merchant/wallet",
			token:       "not-a-valid-token",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "auth_invalid_token",
		},
		{
			name:        "merchant forbidden from admin stats",
			method:      http.MethodGet,
			path:        "/api/v1/admin/stats",
			token:       merchantTokens.AccessToken,
			wantStatus:  http.StatusForbidden,
			wantErrCode: "auth_forbidden",
		},
		{
			name:        "admin forbidden from merchant wallet",
			method:      http.MethodGet,
			path:        "/api/v1/merchant/wallet",
			token:       adminTokens.AccessToken,
			wantStatus:  http.StatusForbidden,
			wantErrCode: "auth_forbidden",
		},
		{
			name:        "register invalid email",
			method:      http.MethodPost,
			path:        "/api/v1/users/register",
			body:        map[string]any{"name": "Bad Email", "email": "bad", "password": "merchant1234"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "register weak password",
			method:      http.MethodPost,
			path:        "/api/v1/users/register",
			body:        map[string]any{"name": "Weak Password", "email": integrationEmail(t.Name() + "_weak"), "password": "short"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "register duplicate email",
			method:      http.MethodPost,
			path:        "/api/v1/users/register",
			body:        map[string]any{"name": "Duplicate", "email": email, "password": password},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "oauth2 invalid client",
			method:      http.MethodPost,
			path:        "/api/v1/oauth2/token",
			body:        url.Values{"grant_type": {"password"}, "username": {email}, "password": {password}, "client_id": {e2eClientID}, "client_secret": {"wrong"}},
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "invalid_client",
		},
		{
			name:        "public invoice not found",
			method:      http.MethodGet,
			path:        "/api/v1/pay/not-found",
			wantStatus:  http.StatusNotFound,
			wantErrCode: "invoice_not_found",
		},
		{
			name:        "payment intent invalid method",
			method:      http.MethodPost,
			path:        "/api/v1/pay/" + paymentToken + "/intents",
			body:        map[string]any{"method": "CRYPTO"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "payment_intent_create_failed",
		},
		{
			name:        "payment status missing field",
			method:      http.MethodPatch,
			path:        "/api/v1/admin/payment-intents/" + paymentIntentID + "/status",
			token:       adminTokens.AccessToken,
			body:        map[string]any{},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "refund before payment success",
			method:      http.MethodPost,
			path:        "/api/v1/merchant/refunds",
			token:       merchantTokens.AccessToken,
			body:        map[string]any{"invoice_id": invoiceID, "reason": "too early"},
			headers:     map[string]string{"Idempotency-Key": uuid.NewString()},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "refund_request_failed",
		},
		{
			name:        "refund review invalid decision",
			method:      http.MethodPatch,
			path:        "/api/v1/admin/refunds/" + uuid.NewString() + "/review",
			token:       adminTokens.AccessToken,
			body:        map[string]any{"decision": "MAYBE"},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "refund_review_failed",
		},
		{
			name:        "admin stats invalid date",
			method:      http.MethodGet,
			path:        "/api/v1/admin/stats?start_date=2026-01-99",
			token:       adminTokens.AccessToken,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "stats_query_failed",
		},
		{
			name:        "ledger invalid merchant id",
			method:      http.MethodGet,
			path:        "/api/v1/admin/ledger/accounts/not-a-uuid",
			token:       adminTokens.AccessToken,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "invalid_merchant_id",
		},
		{
			name:        "wallet transactions invalid direction",
			method:      http.MethodGet,
			path:        "/api/v1/merchant/wallet/transactions?direction=X",
			token:       merchantTokens.AccessToken,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
		{
			name:        "merchant client invalid redirect uri",
			method:      http.MethodPost,
			path:        "/api/v1/merchant/clients",
			token:       merchantTokens.AccessToken,
			body:        map[string]any{"name": "Bad Client", "redirect_uris": []string{"not-url"}, "scopes": []string{"read"}},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "validation_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, resp := doE2ERequest(t, suite.router, tc.method, tc.path, tc.token, tc.body, tc.headers)
			require.Equal(t, tc.wantStatus, status)
			require.NotNil(t, resp.Error)
			assert.Equal(t, tc.wantErrCode, resp.Error.Code)
		})
	}
}

func TestE2EAPI_MerchantOwnershipAndIdempotencyConflict(t *testing.T) {
	suite := setupIntegrationSuite(t)

	firstEmail := integrationEmail(t.Name() + "_first")
	secondEmail := integrationEmail(t.Name() + "_second")
	e2eRegisterMerchant(t, suite.router, firstEmail, "merchant1234")
	e2eRegisterMerchant(t, suite.router, secondEmail, "merchant1234")
	firstToken := e2ePasswordToken(t, suite.router, firstEmail, "merchant1234").AccessToken
	secondToken := e2ePasswordToken(t, suite.router, secondEmail, "merchant1234").AccessToken

	dueDate := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	invoiceID, _ := e2eCreateInvoiceWithIdempotency(t, suite.router, firstToken, 150, dueDate)

	status, resp := doE2EJSONRequest(t, suite.router, http.MethodGet, "/api/v1/merchant/invoices/"+invoiceID, secondToken, nil, nil)
	require.Equal(t, http.StatusNotFound, status)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "invoice_not_found", resp.Error.Code)

	key := uuid.NewString()
	body := map[string]any{"amount": 321}
	status, firstResp := doE2EJSONRequest(t, suite.router, http.MethodPost, "/api/v1/merchant/topups", firstToken, body, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, firstResp.Error)
	firstTopupID := e2eStringField(t, mustMap(t, firstResp.Data), "id")

	status, replayResp := doE2EJSONRequest(t, suite.router, http.MethodPost, "/api/v1/merchant/topups", firstToken, body, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, replayResp.Error)
	replayedTopupID := e2eStringField(t, mustMap(t, replayResp.Data), "id")
	assert.Equal(t, firstTopupID, replayedTopupID)

	status, conflictResp := doE2EJSONRequest(t, suite.router, http.MethodPost, "/api/v1/merchant/topups", firstToken, map[string]any{"amount": 654}, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusConflict, status)
	require.NotNil(t, conflictResp.Error)
	assert.Equal(t, "idempotency_key_conflict", conflictResp.Error.Code)
}

func e2eRegisterMerchant(t *testing.T, router *gin.Engine, email, password string) string {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPost, "/api/v1/users/register", "", map[string]any{
		"name":     "E2E Merchant",
		"email":    email,
		"password": password,
	}, nil)
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	assert.Equal(t, email, data["email"])
	assert.Equal(t, "MERCHANT", data["role"])
	return e2eStringField(t, data, "id")
}

func e2ePasswordToken(t *testing.T, router *gin.Engine, email, password string) e2eTokenPair {
	t.Helper()
	status, resp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/token", "", url.Values{
		"grant_type":    {"password"},
		"username":      {email},
		"password":      {password},
		"client_id":     {e2eClientID},
		"client_secret": {e2eClientSecret},
		"scope":         {"read write admin"},
	}, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	return e2eTokenPair{
		AccessToken:  e2eStringField(t, data, "access_token"),
		RefreshToken: e2eOptionalStringField(t, data, "refresh_token"),
	}
}

func e2eAssertOAuth2Protocol(t *testing.T, router *gin.Engine, tokens e2eTokenPair, userID string) {
	t.Helper()

	status, userInfoResp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/oauth2/userinfo", tokens.AccessToken, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, userInfoResp.Error)
	userInfo := mustMap(t, userInfoResp.Data)
	assert.Equal(t, userID, userInfo["id"])
	assert.Equal(t, "MERCHANT", userInfo["role"])

	status, introspectResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/introspect", "", url.Values{"token": {tokens.AccessToken}}, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, introspectResp.Error)
	introspect := mustMap(t, introspectResp.Data)
	assert.Equal(t, true, introspect["active"])
	assert.Equal(t, userID, introspect["user_id"])
	assert.Equal(t, e2eClientID, introspect["client_id"])

	status, refreshResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/token", "", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokens.RefreshToken},
		"client_id":     {e2eClientID},
		"client_secret": {e2eClientSecret},
	}, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, refreshResp.Error)
	refreshed := mustMap(t, refreshResp.Data)
	assert.NotEmpty(t, e2eStringField(t, refreshed, "access_token"))
	assert.NotEmpty(t, e2eStringField(t, refreshed, "refresh_token"))

	clientID, clientSecret := e2eRegisterOAuthClient(t, router, tokens.AccessToken)
	authorizePath := "/api/v1/oauth2/authorize?response_type=code&client_id=" + url.QueryEscape(clientID) + "&redirect_uri=" + url.QueryEscape("http://localhost:3000/callback") + "&scope=read&state=e2e-state"
	status, authorizeGetResp := doE2EJSONRequest(t, router, http.MethodGet, authorizePath, "", nil, nil)
	require.Equal(t, http.StatusUnauthorized, status)
	require.NotNil(t, authorizeGetResp.Error)
	assert.Equal(t, "auth_unauthorized", authorizeGetResp.Error.Code)

	status, authorizePostResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/authorize", tokens.AccessToken, url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {"http://localhost:3000/callback"},
		"scope":         {"read"},
		"state":         {"e2e-state"},
	}, nil)
	require.Equal(t, http.StatusUnauthorized, status)
	require.NotNil(t, authorizePostResp.Error)
	assert.Equal(t, "auth_unauthorized", authorizePostResp.Error.Code)

	status, authCodeTokenResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/token", "", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"not-a-code"},
		"redirect_uri":  {"http://localhost:3000/callback"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.NotNil(t, authCodeTokenResp.Error)
	assert.Equal(t, "invalid_grant", authCodeTokenResp.Error.Code)

	status, clientCredentialsResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/token", "", url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {"read"},
	}, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, clientCredentialsResp.Error)
	assert.NotEmpty(t, e2eStringField(t, mustMap(t, clientCredentialsResp.Data), "access_token"))

	status, clientsResp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/merchant/clients", tokens.AccessToken, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, clientsResp.Error)
	assert.True(t, e2eArrayContainsID(mustSlice(t, clientsResp.Data), clientID))

	status, deleteResp := doE2EJSONRequest(t, router, http.MethodDelete, "/api/v1/merchant/clients/"+clientID, tokens.AccessToken, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, deleteResp.Error)
	assert.Equal(t, "deleted", e2eStringField(t, mustMap(t, deleteResp.Data), "status"))

	status, revokeResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/revoke", "", url.Values{
		"token":         {tokens.RefreshToken},
		"client_id":     {e2eClientID},
		"client_secret": {e2eClientSecret},
	}, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, revokeResp.Error)
	assert.Equal(t, "revoked", e2eStringField(t, mustMap(t, revokeResp.Data), "status"))
	tokens.RefreshToken = e2eStringField(t, refreshed, "refresh_token")

	status, inactiveResp := doE2EFormRequest(t, router, http.MethodPost, "/api/v1/oauth2/introspect", "", url.Values{"token": {"not-a-token"}}, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, inactiveResp.Error)
	assert.Equal(t, false, mustMap(t, inactiveResp.Data)["active"])
}

func e2eRegisterOAuthClient(t *testing.T, router *gin.Engine, token string) (string, string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/clients", token, map[string]any{
		"name":          "E2E Client",
		"redirect_uris": []string{"http://localhost:3000/callback"},
		"scopes":        []string{"read", "write"},
	}, nil)
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	client := mustMap(t, data["client"])
	return e2eStringField(t, client, "id"), e2eStringField(t, data, "client_secret")
}

func e2eAssertMerchantWallet(t *testing.T, router *gin.Engine, token string, expectedBalance int64) string {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/merchant/wallet", token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	assert.Equal(t, float64(expectedBalance), data["balance"])
	return e2eStringField(t, data, "id")
}

func e2eCreateTopupWithIdempotency(t *testing.T, router *gin.Engine, token string, amount int64) string {
	t.Helper()
	key := uuid.NewString()
	status, resp := doE2EJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/topups", token, map[string]any{"amount": amount}, map[string]string{"Idempotency-Key": key})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	return e2eStringField(t, mustMap(t, resp.Data), "id")
}

func e2eUpdateTopupStatus(t *testing.T, router *gin.Engine, token, topupID, nextStatus string, wantHTTP int, wantErr string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/topups/"+topupID+"/status", token, map[string]any{"status": nextStatus}, nil)
	require.Equal(t, wantHTTP, status)
	if wantHTTP >= 400 {
		require.NotNil(t, resp.Error)
		assert.Equal(t, wantErr, resp.Error.Code)
		return
	}
	require.Nil(t, resp.Error)
}

func e2eCreateInvoiceWithIdempotency(t *testing.T, router *gin.Engine, token string, amount int64, dueDate string) (string, string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/invoices", token, map[string]any{
		"customer_name":  "E2E Customer",
		"customer_email": "e2e.customer@example.com",
		"amount":         amount,
		"description":    "e2e invoice",
		"due_date":       dueDate,
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	return e2eStringField(t, data, "id"), e2eStringField(t, data, "payment_link_token")
}

func e2eGetInvoice(t *testing.T, router *gin.Engine, token, invoiceID, expectedStatus string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/merchant/invoices/"+invoiceID, token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	assert.Equal(t, invoiceID, e2eStringField(t, data, "id"))
	assert.Equal(t, expectedStatus, e2eStringField(t, data, "status"))
}

func e2eGetPublicInvoice(t *testing.T, router *gin.Engine, paymentToken, invoiceID, expectedStatus string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/pay/"+paymentToken, "", nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	assert.Equal(t, invoiceID, e2eStringField(t, data, "id"))
	assert.Equal(t, expectedStatus, e2eStringField(t, data, "status"))
}

func e2eCreatePaymentIntent(t *testing.T, router *gin.Engine, paymentToken, method string) string {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPost, "/api/v1/pay/"+paymentToken+"/intents", "", map[string]any{"method": method}, nil)
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	intent := mustMap(t, data["payment_intent"])
	assert.Equal(t, "PENDING", e2eStringField(t, intent, "status"))
	return e2eStringField(t, intent, "id")
}

func e2eUpdatePaymentIntentStatus(t *testing.T, router *gin.Engine, token, paymentIntentID, nextStatus string, wantHTTP int, wantErr string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/payment-intents/"+paymentIntentID+"/status", token, map[string]any{"status": nextStatus}, nil)
	require.Equal(t, wantHTTP, status)
	if wantHTTP >= 400 {
		require.NotNil(t, resp.Error)
		assert.Equal(t, wantErr, resp.Error.Code)
		return
	}
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	intent := mustMap(t, data["payment_intent"])
	invoice := mustMap(t, data["invoice"])
	assert.Equal(t, nextStatus, e2eStringField(t, intent, "status"))
	if nextStatus == "SUCCESS" {
		assert.Equal(t, "PAID", e2eStringField(t, invoice, "status"))
	}
}

func e2eRequestRefundWithIdempotency(t *testing.T, router *gin.Engine, token, invoiceID string) string {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPost, "/api/v1/merchant/refunds", token, map[string]any{
		"invoice_id": invoiceID,
		"reason":     "e2e refund",
	}, map[string]string{"Idempotency-Key": uuid.NewString()})
	require.Equal(t, http.StatusCreated, status)
	require.Nil(t, resp.Error)
	return e2eStringField(t, mustMap(t, resp.Data), "id")
}

func e2eReviewRefund(t *testing.T, router *gin.Engine, token, refundID, decision string, wantHTTP int, wantErr string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/refunds/"+refundID+"/review", token, map[string]any{"decision": decision}, nil)
	require.Equal(t, wantHTTP, status)
	if wantHTTP >= 400 {
		require.NotNil(t, resp.Error)
		assert.Equal(t, wantErr, resp.Error.Code)
		return
	}
	require.Nil(t, resp.Error)
	assert.Equal(t, "APPROVED", e2eStringField(t, mustMap(t, resp.Data), "status"))
}

func e2eProcessRefund(t *testing.T, router *gin.Engine, token, refundID, nextStatus string, wantHTTP int, wantErr string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodPatch, "/api/v1/admin/refunds/"+refundID+"/process", token, map[string]any{"status": nextStatus}, nil)
	require.Equal(t, wantHTTP, status)
	if wantHTTP >= 400 {
		require.NotNil(t, resp.Error)
		assert.Equal(t, wantErr, resp.Error.Code)
		return
	}
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	refund := mustMap(t, data["refund"])
	assert.Equal(t, nextStatus, e2eStringField(t, refund, "status"))
}

func e2eAssertAdminStats(t *testing.T, router *gin.Engine, token, merchantID string, wantInvoices int, wantPayments int64, wantRefunds int64) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/admin/stats?merchant_id="+url.QueryEscape(merchantID), token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	assert.Equal(t, float64(wantInvoices), data["total_invoice_created"])
	assert.Equal(t, float64(wantPayments), data["total_payment_nominal"])
	assert.Equal(t, float64(wantRefunds), data["total_refund_nominal"])
	byStatus := mustMap(t, data["total_by_status"])
	assert.Equal(t, float64(1), byStatus["PAID"])
}

func e2eAssertMerchantWalletTransactions(t *testing.T, router *gin.Engine, token string, minTotal int) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/merchant/wallet/transactions?page=1&limit=10", token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	assert.GreaterOrEqual(t, len(mustSlice(t, resp.Data)), minTotal)
}

func e2eAssertWalletTransactions(t *testing.T, router *gin.Engine, token string, minTotal int) {
	t.Helper()
	e2eAssertMerchantWalletTransactions(t, router, token, minTotal)
}

func e2eAssertAdminWalletTransactions(t *testing.T, router *gin.Engine, token, merchantID string, minTotal int) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/admin/wallet/transactions?merchant_id="+url.QueryEscape(merchantID)+"&page=1&limit=10", token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	assert.GreaterOrEqual(t, len(mustSlice(t, resp.Data)), minTotal)
}

func e2eAssertAdminMerchants(t *testing.T, router *gin.Engine, token, email, merchantID string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/admin/merchants?search="+url.QueryEscape("E2E")+"&page=1&limit=20", token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	items := mustSlice(t, resp.Data)
	found := false
	for _, item := range items {
		m := mustMap(t, item)
		if m["email"] == email && m["id"] == merchantID {
			found = true
		}
	}
	assert.True(t, found, "expected merchant %s in admin list", email)
}

func e2eAssertLedgerAccount(t *testing.T, router *gin.Engine, token, merchantID string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, http.MethodGet, "/api/v1/admin/ledger/accounts/"+merchantID, token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	data := mustMap(t, resp.Data)
	assert.Equal(t, merchantID, e2eStringField(t, data, "MerchantID"))
}

func e2eListContainsID(t *testing.T, router *gin.Engine, method, path, token, id string) {
	t.Helper()
	status, resp := doE2EJSONRequest(t, router, method, path, token, nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Error)
	assert.True(t, e2eArrayContainsID(mustSlice(t, resp.Data), id), "expected id %s in %s", id, path)
}

func doE2EJSONRequest(t *testing.T, router *gin.Engine, method, path, token string, body any, headers map[string]string) (int, apiEnvelope) {
	t.Helper()
	return doE2ERequest(t, router, method, path, token, body, headers)
}

func doE2EFormRequest(t *testing.T, router *gin.Engine, method, path, token string, values url.Values, headers map[string]string) (int, apiEnvelope) {
	t.Helper()
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	return doE2ERequest(t, router, method, path, token, values, headers)
}

func doE2ERequest(t *testing.T, router *gin.Engine, method, path, token string, body any, headers map[string]string) (int, apiEnvelope) {
	t.Helper()

	var reader *bytes.Reader
	contentType := "application/json"
	switch v := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case url.Values:
		reader = bytes.NewReader([]byte(v.Encode()))
		contentType = "application/x-www-form-urlencoded"
	default:
		payload, err := json.Marshal(v)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var envelope apiEnvelope
	if rec.Body.Len() > 0 && strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		err := json.Unmarshal(rec.Body.Bytes(), &envelope)
		require.NoError(t, err, "response body: %s", rec.Body.String())
	}

	return rec.Code, envelope
}

func e2eStringField(t *testing.T, data map[string]any, key string) string {
	t.Helper()
	value, ok := data[key].(string)
	require.True(t, ok, "field %s should be string in %#v", key, data)
	require.NotEmpty(t, value)
	return value
}

func e2eOptionalStringField(t *testing.T, data map[string]any, key string) string {
	t.Helper()
	value, ok := data[key].(string)
	if !ok {
		return ""
	}
	return value
}

func mustSlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	require.True(t, ok, "value should be []any: %#v", value)
	return items
}

func e2eArrayContainsID(items []any, id string) bool {
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["id"] == id {
			return true
		}
	}
	return false
}

func TestE2EAPI_RouteCoverageList(t *testing.T) {
	// This test is intentionally small: the route behavior is covered above, and
	// this table protects the E2E file from silently missing newly added routes.
	covered := map[string]bool{
		"GET /api/v1/ping":                               true,
		"POST /api/v1/users/register":                    true,
		"POST /api/v1/oauth2/token":                      true,
		"POST /api/v1/oauth2/introspect":                 true,
		"POST /api/v1/oauth2/revoke":                     true,
		"GET /api/v1/oauth2/authorize":                   true,
		"POST /api/v1/oauth2/authorize":                  true,
		"GET /api/v1/oauth2/userinfo":                    true,
		"GET /api/v1/pay/:token":                         true,
		"POST /api/v1/pay/:token/intents":                true,
		"GET /api/v1/merchant/wallet":                    true,
		"GET /api/v1/merchant/wallet/transactions":       true,
		"GET /api/v1/merchant/topups":                    true,
		"POST /api/v1/merchant/topups":                   true,
		"GET /api/v1/merchant/invoices":                  true,
		"POST /api/v1/merchant/invoices":                 true,
		"GET /api/v1/merchant/invoices/:id":              true,
		"GET /api/v1/merchant/refunds":                   true,
		"POST /api/v1/merchant/refunds":                  true,
		"POST /api/v1/merchant/clients":                  true,
		"GET /api/v1/merchant/clients":                   true,
		"DELETE /api/v1/merchant/clients/:id":            true,
		"GET /api/v1/admin/topups":                       true,
		"PATCH /api/v1/admin/topups/:id/status":          true,
		"GET /api/v1/admin/payment-intents":              true,
		"PATCH /api/v1/admin/payment-intents/:id/status": true,
		"GET /api/v1/admin/refunds":                      true,
		"PATCH /api/v1/admin/refunds/:id/review":         true,
		"PATCH /api/v1/admin/refunds/:id/process":        true,
		"GET /api/v1/admin/stats":                        true,
		"GET /api/v1/admin/wallet/transactions":          true,
		"GET /api/v1/admin/ledger/accounts/:merchant_id": true,
		"GET /api/v1/admin/merchants":                    true,
	}

	router := e2eRouteCoverageRouter()
	registered := routeMap(router.Routes())
	for key := range registered {
		if key == "GET /swagger/*any" {
			continue
		}
		assert.True(t, covered[key], "registered route missing E2E coverage declaration: %s", key)
	}
	for key := range covered {
		assert.True(t, registered[key], "E2E coverage declaration references unregistered route: %s", key)
	}
}

func e2eRouteCoverageRouter() *gin.Engine {
	cfg := config.Config{AppPort: "8080", JWTSecret: "test-secret", JWTDuration: time.Hour, ShutdownTTL: time.Second}
	auditLogger := audit.NewNoopLogger()
	idemMW := &idempotency.Middleware{Store: &idempotency.Store{TTL: time.Hour}, Cache: &idempotency.Cache{TTL: time.Hour}}
	return newRouter(
		cfg,
		idemMW,
		&userHandlers.UserHandler{},
		&adminHandlers.AdminHandler{},
		merchantHandlers.NewMerchantsHandler(nil),
		walletHandlers.NewWalletHandler(nil, auditLogger),
		invoiceHandlers.NewInvoiceHandler(nil, auditLogger),
		paymentHandlers.NewPaymentHandler(nil, auditLogger),
		refundHandlers.NewRefundHandler(nil, auditLogger),
		oauth2Handlers.NewOAuth2Handler(nil, cfg),
		ledgerHandlers.NewLedgerHandler(nil),
	)
}
