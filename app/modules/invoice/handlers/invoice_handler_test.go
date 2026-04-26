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
	serviceMocks "payment-sandbox/app/modules/invoice/services/mocks"
	journeylog "payment-sandbox/app/shared/journeylog"
	journeyMocks "payment-sandbox/app/shared/journeylog/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInvoiceHandler_CreateInvoice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		withUserID bool
		body       string
		setupMocks func(service *serviceMocks.MockIInvoiceService, logger *journeyMocks.MockIJourneyLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name:       "missing user context",
			withUserID: false,
			body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *journeyMocks.MockIJourneyLogger) {
				service.AssertNotCalled(t, "CreateInvoice")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "auth_unauthorized",
		},
		{
			name:       "validation error",
			withUserID: true,
			body:       `{"customer_name":"Alice","customer_email":"invalid","amount":0,"description":"desc","due_date":""}`,
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *journeyMocks.MockIJourneyLogger) {
				service.AssertNotCalled(t, "CreateInvoice")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "service error and logger failure",
			withUserID: true,
			body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *journeyMocks.MockIJourneyLogger) {
				service.EXPECT().
					CreateInvoice("user-1", "Alice", "alice@example.com", 10000.0, "desc", "2026-05-01T10:00:00Z").
					Return(invoiceEntity.Invoice{}, errors.New("due_date must be today or future"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event journeylog.Event) bool {
							return event.Module == "invoice" &&
								event.Action == "INVOICE_CREATE" &&
								event.Result == "FAILED" &&
								event.RequestID == "req-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invoice_create_failed",
		},
		{
			name:       "success and logger failure",
			withUserID: true,
			body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *journeyMocks.MockIJourneyLogger) {
				service.EXPECT().
					CreateInvoice("user-1", "Alice", "alice@example.com", 10000.0, "desc", "2026-05-01T10:00:00Z").
					Return(invoiceEntity.Invoice{
						ID:     "inv-1",
						Status: invoiceEntity.InvoicePending,
						Amount: 10000,
					}, nil)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event journeylog.Event) bool {
							return event.Module == "invoice" &&
								event.Action == "INVOICE_CREATE" &&
								event.Result == "SUCCESS" &&
								event.EntityID == "inv-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusCreated,
			wantID:     "inv-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIInvoiceService(t)
			logger := journeyMocks.NewMockIJourneyLogger(t)
			tc.setupMocks(service, logger)

			handler := NewInvoiceHandler(service, logger)
			router := gin.New()
			router.POST("/merchant/invoices", func(c *gin.Context) {
				if tc.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
					c.Set(middleware.ContextRole, "MERCHANT")
				}
				c.Set(middleware.ContextRequestID, "req-1")
				handler.CreateInvoice(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/merchant/invoices", bytes.NewBufferString(tc.body))
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
			assert.Equal(t, tc.wantID, data["id"])
		})
	}
}
