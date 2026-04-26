package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	serviceMocks "payment-sandbox/app/modules/wallet/handlers/mocks"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	journeylog "payment-sandbox/app/shared/journeylog"
	journeyMocks "payment-sandbox/app/shared/journeylog/mocks"

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
		setupMocks func(service *serviceMocks.MockWalletService, logger *journeyMocks.MockJourneyLogger)
		wantStatus int
		wantCode   string
		wantDataID string
	}{
		{
			name: "validation error",
			body: `{"amount":0}`,
			setupMocks: func(service *serviceMocks.MockWalletService, logger *journeyMocks.MockJourneyLogger) {
				service.AssertNotCalled(t, "CreateTopup")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure still returns business error",
			body: `{"amount":15000}`,
			setupMocks: func(service *serviceMocks.MockWalletService, logger *journeyMocks.MockJourneyLogger) {
				service.EXPECT().
					CreateTopup("user-1", 15000.0).
					Return(walletEntity.Topup{}, errors.New("invalid topup state"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event journeylog.Event) bool {
							return event.Module == "wallet" &&
								event.Action == "TOPUP_CREATE" &&
								event.Result == "FAILED" &&
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
			setupMocks: func(service *serviceMocks.MockWalletService, logger *journeyMocks.MockJourneyLogger) {
				service.EXPECT().
					CreateTopup("user-1", 25000.0).
					Return(walletEntity.Topup{
						ID:     "topup-1",
						Amount: 25000,
						Status: "PENDING",
					}, nil)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event journeylog.Event) bool {
							return event.Module == "wallet" &&
								event.Action == "TOPUP_CREATE" &&
								event.Result == "SUCCESS" &&
								event.EntityID == "topup-1"
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
			service := serviceMocks.NewMockWalletService(t)
			logger := journeyMocks.NewMockJourneyLogger(t)
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
