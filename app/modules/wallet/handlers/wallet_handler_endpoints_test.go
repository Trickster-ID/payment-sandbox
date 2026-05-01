package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	serviceMocks "payment-sandbox/app/modules/wallet/services/mocks"
	"payment-sandbox/app/shared/audit"
	auditMocks "payment-sandbox/app/shared/audit/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWalletHandler_Wallet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		withUserID bool
		setupMocks func(service *serviceMocks.MockIWalletService)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name:       "missing user context",
			withUserID: false,
			setupMocks: func(service *serviceMocks.MockIWalletService) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "auth_unauthorized",
		},
		{
			name:       "wallet not found",
			withUserID: true,
			setupMocks: func(service *serviceMocks.MockIWalletService) {
				service.EXPECT().WalletByUserID("user-1").Return(walletEntity.Merchant{}, errors.New("wallet not found"))
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "wallet_not_found",
		},
		{
			name:       "success",
			withUserID: true,
			setupMocks: func(service *serviceMocks.MockIWalletService) {
				service.EXPECT().WalletByUserID("user-1").Return(walletEntity.Merchant{ID: "merchant-1"}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     "merchant-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIWalletService(t)
			logger := auditMocks.NewMockIAuditLogger(t)
			tc.setupMocks(service)

			handler := NewWalletHandler(service, logger)
			router := gin.New()
			router.GET("/merchant/wallet", func(c *gin.Context) {
				if tc.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
				}
				handler.Wallet(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/wallet", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err)

			if tc.wantCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tc.wantCode, errData["code"])
				return
			}

			data, ok := payload["data"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantID, data["id"])
		})
	}
}

func TestWalletHandler_ListTopups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := serviceMocks.NewMockIWalletService(t)
	logger := auditMocks.NewMockIAuditLogger(t)
	service.EXPECT().ListTopups().Return([]walletEntity.Topup{{ID: "topup-1"}, {ID: "topup-2"}})

	handler := NewWalletHandler(service, logger)
	router := gin.New()
	router.GET("/admin/topups", handler.ListTopups)

	req := httptest.NewRequest(http.MethodGet, "/admin/topups", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &payload)
	require.NoError(t, err)

	data, ok := payload["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 2)
}

func TestWalletHandler_UpdateTopupStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name: "validation error",
			body: `{"status":""}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "UpdateTopupStatus")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure",
			body: `{"status":"SUCCESS"}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().UpdateTopupStatus("topup-1", "SUCCESS").Return(walletEntity.Topup{}, errors.New("topup already processed"))
				logger.EXPECT().Log(
					mock.Anything,
					mock.MatchedBy(func(event audit.Event) bool {
						result, _ := event.Metadata["result"].(string)
						return event.EventType == "topup.status_updated" && result == "FAILED"
					}),
				).Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "topup_update_failed",
		},
		{
			name: "success and logger failure",
			body: `{"status":"SUCCESS"}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().UpdateTopupStatus("topup-1", "SUCCESS").Return(walletEntity.Topup{ID: "topup-1"}, nil)
				logger.EXPECT().Log(
					mock.Anything,
					mock.MatchedBy(func(event audit.Event) bool {
						result, _ := event.Metadata["result"].(string)
						return event.EventType == "topup.status_updated" && result == "SUCCESS"
					}),
				).Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusOK,
			wantID:     "topup-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIWalletService(t)
			logger := auditMocks.NewMockIAuditLogger(t)
			tc.setupMocks(service, logger)

			handler := NewWalletHandler(service, logger)
			router := gin.New()
			router.PATCH("/admin/topups/:id/status", func(c *gin.Context) {
				c.Set(middleware.ContextRequestID, "req-1")
				c.Set(middleware.ContextRole, "ADMIN")
				handler.UpdateTopupStatus(c)
			})

			req := httptest.NewRequest(http.MethodPatch, "/admin/topups/topup-1/status", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err)

			if tc.wantCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tc.wantCode, errData["code"])
				return
			}

			data, ok := payload["data"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantID, data["id"])
		})
	}
}
