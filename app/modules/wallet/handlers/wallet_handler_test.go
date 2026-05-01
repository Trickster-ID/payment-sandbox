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

func TestWalletHandler_CreateTopup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger)
		wantStatus int
		wantCode   string
		wantDataID string
	}{
		{
			name: "validation error",
			body: `{"amount":0}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "CreateTopup")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "malformed json",
			body: `{invalid-json}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "CreateTopup")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure still returns business error",
			body: `{"amount":15000}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					CreateTopup("user-1", int64(15000)).
					Return(walletEntity.Topup{}, errors.New("invalid topup state"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "topup.created" &&
								result == "FAILED" &&
								event.RequestID == "req-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "topup_create_failed",
		},
		{
			name: "success and logger failure still returns created",
			body: `{"amount":25000}`,
			setupMocks: func(service *serviceMocks.MockIWalletService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					CreateTopup("user-1", int64(25000)).
					Return(walletEntity.Topup{
						ID:     "topup-1",
						Amount: 25000,
						Status: "PENDING",
					}, nil)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "topup.created" &&
								result == "SUCCESS" &&
								event.ResourceID == "topup-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusCreated,
			wantDataID: "topup-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIWalletService(t)
			logger := auditMocks.NewMockIAuditLogger(t)
			tc.setupMocks(service, logger)

			handler := NewWalletHandler(service, logger)
			router := gin.New()
			router.POST("/topups", func(c *gin.Context) {
				c.Set(middleware.ContextUserID, "user-1")
				c.Set(middleware.ContextRole, "MERCHANT")
				c.Set(middleware.ContextRequestID, "req-1")
				handler.CreateTopup(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/topups", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err)

			if tc.wantCode != "" {
				errorData, ok := payload["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tc.wantCode, errorData["code"])
				return
			}

			data, ok := payload["data"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantDataID, data["id"])
		})
	}
}
