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
	"payment-sandbox/app/shared/audit"
	auditMocks "payment-sandbox/app/shared/audit/mocks"

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
		setupMocks func(service *serviceMocks.MockIInvoiceService, logger *auditMocks.MockIAuditLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name:       "missing user context",
			withUserID: false,
			body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *auditMocks.MockIAuditLogger) {
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
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "CreateInvoice")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "malformed json",
			withUserID: true,
			body:       `{invalid-json}`,
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *auditMocks.MockIAuditLogger) {
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
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					CreateInvoice("user-1", "Alice", "alice@example.com", int64(10000), "desc", "2026-05-01T10:00:00Z").
					Return(invoiceEntity.Invoice{}, errors.New("due_date must be today or future"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "invoice.created" &&
								result == "FAILED" &&
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
			setupMocks: func(service *serviceMocks.MockIInvoiceService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					CreateInvoice("user-1", "Alice", "alice@example.com", int64(10000), "desc", "2026-05-01T10:00:00Z").
					Return(invoiceEntity.Invoice{
						ID:     "inv-1",
						Status: invoiceEntity.InvoicePending,
						Amount: 10000,
					}, nil)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "invoice.created" &&
								result == "SUCCESS" &&
								event.ResourceID == "inv-1"
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
			logger := auditMocks.NewMockIAuditLogger(t)
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
