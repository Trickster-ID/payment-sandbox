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
	serviceMocks "payment-sandbox/app/modules/payment/handlers/mocks"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	journeylog "payment-sandbox/app/shared/journeylog"
	journeyMocks "payment-sandbox/app/shared/journeylog/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPaymentHandler_CreatePaymentIntent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockPaymentService, logger *journeyMocks.MockJourneyLogger)
		wantStatus int
		wantCode   string
		wantDataID string
	}{
		{
			name: "validation error",
			body: `{}`,
			setupMocks: func(service *serviceMocks.MockPaymentService, logger *journeyMocks.MockJourneyLogger) {
				service.AssertNotCalled(t, "CreatePaymentIntent")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure still returns business error",
			body: `{"method":"WALLET"}`,
			setupMocks: func(service *serviceMocks.MockPaymentService, logger *journeyMocks.MockJourneyLogger) {
				service.EXPECT().
					CreatePaymentIntent("token-1", "WALLET").
					Return(paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invoice already paid"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event journeylog.Event) bool {
							return event.Module == "payment" &&
								event.Action == "PAYMENT_INTENT_CREATE" &&
								event.Result == "FAILED" &&
								event.RequestID == "req-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "payment_intent_create_failed",
		},
		{
			name: "success and logger failure still returns created",
			body: `{"method":"VA_DUMMY"}`,
			setupMocks: func(service *serviceMocks.MockPaymentService, logger *journeyMocks.MockJourneyLogger) {
				service.EXPECT().
					CreatePaymentIntent("token-1", "VA_DUMMY").
					Return(
						paymentEntity.PaymentIntent{
							ID:        "pi-1",
							InvoiceID: "inv-1",
							Method:    paymentEntity.MethodVADummy,
							Status:    paymentEntity.PaymentPending,
						},
						invoiceEntity.Invoice{
							ID: "inv-1",
						},
						nil,
					)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event journeylog.Event) bool {
							return event.Module == "payment" &&
								event.Action == "PAYMENT_INTENT_CREATE" &&
								event.Result == "SUCCESS" &&
								event.EntityID == "pi-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusCreated,
			wantDataID: "pi-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockPaymentService(t)
			logger := journeyMocks.NewMockJourneyLogger(t)
			tc.setupMocks(service, logger)

			handler := NewPaymentHandler(service, logger)
			router := gin.New()
			router.POST("/pay/:token/intents", func(c *gin.Context) {
				c.Set(middleware.ContextRequestID, "req-1")
				c.Set(middleware.ContextRole, "PUBLIC")
				handler.CreatePaymentIntent(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/pay/token-1/intents", bytes.NewBufferString(tc.body))
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
			paymentIntent, ok := data["payment_intent"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantDataID, paymentIntent["id"])
		})
	}
}
