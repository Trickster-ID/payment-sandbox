package handlers

// Branch map for CreateInvoice (Section 3.1 of the plan):
// ├── middleware.MustUserID not ok        -> 401 auth_unauthorized, abort
// ├── c.ShouldBindJSON fails (malformed)  -> 400 validation_error
// ├── c.ShouldBindJSON fails (binding)    -> 400 validation_error
// ├── service.CreateInvoice fails         -> 400 invoice_create_failed + audit FAILED
// └── service.CreateInvoice succeeds      -> 201 data.id + audit SUCCESS

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	invoiceServices "payment-sandbox/app/modules/invoice/services"
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

	type fields struct {
		service     invoiceServices.IInvoiceService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		withUserID bool
		body       string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errorCode  string
		invoiceID  string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name: "1. user ID not in context -> unauthorized",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{
				withUserID: false,
				body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusUnauthorized, errorCode: "auth_unauthorized"},
		},
		{
			name: "2. malformed JSON body -> validation_error bad request",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args:  args{withUserID: true, body: `{invalid-json}`},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errorCode: "validation_error"},
		},
		{
			name: "3. binding validation fails (invalid email, zero amount) -> validation_error bad request",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args:  args{withUserID: true, body: `{"customer_name":"Alice","customer_email":"invalid","amount":0,"due_date":""}`},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errorCode: "validation_error"},
		},
		{
			name: "4. service.CreateInvoice fails -> invoice_create_failed, audit log written with FAILED",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{
				withUserID: true,
				body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIInvoiceService).EXPECT().
					CreateInvoice("user-1", "Alice", "alice@example.com", int64(10000), "desc", "2026-05-01T10:00:00Z").
					Return(invoiceEntity.Invoice{}, errors.New("due_date must be today or future")).
					Once()
				f.auditLogger.(*auditMocks.MockIAuditLogger).EXPECT().
					Log(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
						result, _ := e.Metadata["result"].(string)
						return e.EventType == "invoice.created" && result == "FAILED"
					})).
					Return(nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errorCode: "invoice_create_failed"},
		},
		{
			name: "5. valid request, service succeeds -> 201 with invoice ID in data",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{
				withUserID: true,
				body:       `{"customer_name":"Alice","customer_email":"alice@example.com","amount":10000,"description":"desc","due_date":"2026-05-01T10:00:00Z"}`,
			},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIInvoiceService).EXPECT().
					CreateInvoice("user-1", "Alice", "alice@example.com", int64(10000), "desc", "2026-05-01T10:00:00Z").
					Return(invoiceEntity.Invoice{
						ID:     "inv-1",
						Status: invoiceEntity.InvoicePending,
						Amount: 10000,
					}, nil).
					Once()
				f.auditLogger.(*auditMocks.MockIAuditLogger).EXPECT().
					Log(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
						result, _ := e.Metadata["result"].(string)
						return e.EventType == "invoice.created" && result == "SUCCESS" && e.ResourceID == "inv-1"
					})).
					Return(nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusCreated, invoiceID: "inv-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: gin.SetMode is global
			tt.mocks.setup(tt.fields, tt.args)

			handler := NewInvoiceHandler(tt.fields.service, tt.fields.auditLogger)
			router := gin.New()
			router.POST("/merchant/invoices", func(c *gin.Context) {
				if tt.args.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
					c.Set(middleware.ContextRole, "MERCHANT")
				}
				c.Set(middleware.ContextRequestID, "req-1")
				handler.CreateInvoice(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/merchant/invoices", bytes.NewBufferString(tt.args.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), "response JSON parseable")

			if tt.wants.errorCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok, "error envelope present")
				assert.Equal(t, tt.wants.errorCode, errData["code"], "error code")
			} else {
				data, ok := payload["data"].(map[string]any)
				require.True(t, ok, "data envelope present")
				assert.Equal(t, tt.wants.invoiceID, data["id"], "invoice ID")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIInvoiceService); ok {
				m.AssertExpectations(t)
			}
			if m, ok := tt.fields.auditLogger.(*auditMocks.MockIAuditLogger); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
