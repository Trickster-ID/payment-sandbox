package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	serviceMocks "payment-sandbox/app/modules/payment/services/mocks"
	journeylog "payment-sandbox/app/shared/journeylog"
	journeyMocks "payment-sandbox/app/shared/journeylog/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPaymentHandler_PublicInvoice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		setupMocks func(service *serviceMocks.MockIPaymentService)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name: "invoice not found",
			setupMocks: func(service *serviceMocks.MockIPaymentService) {
				service.EXPECT().PublicInvoiceByToken("token-1").Return(invoiceEntity.Invoice{}, errors.New("invoice not found"))
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "invoice_not_found",
		},
		{
			name: "success",
			setupMocks: func(service *serviceMocks.MockIPaymentService) {
				service.EXPECT().PublicInvoiceByToken("token-1").Return(invoiceEntity.Invoice{ID: "inv-1"}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     "inv-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIPaymentService(t)
			logger := journeyMocks.NewMockIJourneyLogger(t)
			tc.setupMocks(service)

			handler := NewPaymentHandler(service, logger)
			router := gin.New()
			router.GET("/pay/:token", handler.PublicInvoice)

			req := httptest.NewRequest(http.MethodGet, "/pay/token-1", nil)
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

func TestPaymentHandler_ListPaymentIntents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := serviceMocks.NewMockIPaymentService(t)
	logger := journeyMocks.NewMockIJourneyLogger(t)
	service.EXPECT().ListPaymentIntents("SUCCESS").Return([]paymentEntity.PaymentIntent{{ID: "pi-1"}})

	handler := NewPaymentHandler(service, logger)
	router := gin.New()
	router.GET("/admin/payment-intents", handler.ListPaymentIntents)

	req := httptest.NewRequest(http.MethodGet, "/admin/payment-intents?status=SUCCESS", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &payload)
	require.NoError(t, err)
	data, ok := payload["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}

func TestPaymentHandler_UpdatePaymentIntentStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockIPaymentService, logger *journeyMocks.MockIJourneyLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name: "validation error",
			body: `{"status":""}`,
			setupMocks: func(service *serviceMocks.MockIPaymentService, logger *journeyMocks.MockIJourneyLogger) {
				service.AssertNotCalled(t, "UpdatePaymentIntentStatus")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure",
			body: `{"status":"FAILED"}`,
			setupMocks: func(service *serviceMocks.MockIPaymentService, logger *journeyMocks.MockIJourneyLogger) {
				service.EXPECT().
					UpdatePaymentIntentStatus("pi-1", "FAILED").
					Return(paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invalid transition"))
				logger.EXPECT().Log(
					mock.Anything,
					mock.MatchedBy(func(event journeylog.Event) bool {
						return event.Module == "payment" && event.Action == "PAYMENT_INTENT_STATUS_UPDATE" && event.Result == "FAILED"
					}),
				).Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "payment_intent_update_failed",
		},
		{
			name: "success and logger failure",
			body: `{"status":"FAILED"}`,
			setupMocks: func(service *serviceMocks.MockIPaymentService, logger *journeyMocks.MockIJourneyLogger) {
				service.EXPECT().
					UpdatePaymentIntentStatus("pi-1", "FAILED").
					Return(paymentEntity.PaymentIntent{ID: "pi-1", Status: paymentEntity.PaymentFailed}, invoiceEntity.Invoice{ID: "inv-1"}, nil)
				logger.EXPECT().Log(
					mock.Anything,
					mock.MatchedBy(func(event journeylog.Event) bool {
						return event.Module == "payment" && event.Action == "PAYMENT_INTENT_STATUS_UPDATE" && event.Result == "SUCCESS"
					}),
				).Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusOK,
			wantID:     "pi-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIPaymentService(t)
			logger := journeyMocks.NewMockIJourneyLogger(t)
			tc.setupMocks(service, logger)

			handler := NewPaymentHandler(service, logger)
			router := gin.New()
			router.PATCH("/admin/payment-intents/:id/status", func(c *gin.Context) {
				c.Set(middleware.ContextRequestID, "req-1")
				c.Set(middleware.ContextRole, "ADMIN")
				handler.UpdatePaymentIntentStatus(c)
			})

			req := httptest.NewRequest(http.MethodPatch, "/admin/payment-intents/pi-1/status", bytes.NewBufferString(tc.body))
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
			intent, ok := data["payment_intent"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantID, intent["id"])
		})
	}
}
