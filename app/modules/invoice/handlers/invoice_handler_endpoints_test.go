package handlers

// Branch map for ListInvoices and GetInvoice (Section 3.1 of the plan):
//
// ListInvoices:
// ├── middleware.MustUserID not ok    -> 401 auth_unauthorized
// ├── service.ListInvoices fails      -> 400 invoice_list_failed
// └── service.ListInvoices succeeds   -> 200 with pagination meta
//
// GetInvoice:
// ├── middleware.MustUserID not ok    -> 401 auth_unauthorized
// ├── service.InvoiceByID fails       -> 404 invoice_not_found
// └── service.InvoiceByID succeeds    -> 200 data.id

import (
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
	"github.com/stretchr/testify/require"
)

func TestInvoiceHandler_ListInvoices(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     invoiceServices.IInvoiceService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		withUserID bool
		query      string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errorCode  string
		total      float64
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
			args:  args{withUserID: false, query: "status=PENDING&page=1&limit=10"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusUnauthorized, errorCode: "auth_unauthorized"},
		},
		{
			name: "2. service.ListInvoices fails -> invoice_list_failed bad request",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, query: "status=PENDING&page=2&limit=20"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIInvoiceService).EXPECT().
					ListInvoices("user-1", "PENDING", 2, 20).
					Return(nil, 0, errors.New("query failed")).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errorCode: "invoice_list_failed"},
		},
		{
			name: "3. valid request, service succeeds -> 200 with pagination meta",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, query: "status=PENDING&page=2&limit=20"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIInvoiceService).EXPECT().
					ListInvoices("user-1", "PENDING", 2, 20).
					Return([]invoiceEntity.Invoice{{ID: "inv-1"}}, 42, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, total: 42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: gin.SetMode is global
			tt.mocks.setup(tt.fields, tt.args)

			handler := NewInvoiceHandler(tt.fields.service, tt.fields.auditLogger)
			router := gin.New()
			router.GET("/merchant/invoices", func(c *gin.Context) {
				if tt.args.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
				}
				handler.ListInvoices(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/invoices?"+tt.args.query, nil)
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
				meta, ok := payload["meta"].(map[string]any)
				require.True(t, ok, "meta envelope present")
				assert.Equal(t, tt.wants.total, meta["total"], "total in meta")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIInvoiceService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestInvoiceHandler_GetInvoice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     invoiceServices.IInvoiceService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		withUserID bool
		invoiceID  string
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
			args:  args{withUserID: false, invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusUnauthorized, errorCode: "auth_unauthorized"},
		},
		{
			name: "2. invoice not found -> invoice_not_found 404",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIInvoiceService).EXPECT().
					InvoiceByID("user-1", a.invoiceID).
					Return(invoiceEntity.Invoice{}, errors.New("not found")).
					Once()
			}},
			wants: wants{statusCode: http.StatusNotFound, errorCode: "invoice_not_found"},
		},
		{
			name: "3. invoice exists -> 200 with invoice data",
			fields: fields{
				service:     serviceMocks.NewMockIInvoiceService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIInvoiceService).EXPECT().
					InvoiceByID("user-1", a.invoiceID).
					Return(invoiceEntity.Invoice{ID: "inv-1"}, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, invoiceID: "inv-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: gin.SetMode is global
			tt.mocks.setup(tt.fields, tt.args)

			handler := NewInvoiceHandler(tt.fields.service, tt.fields.auditLogger)
			router := gin.New()
			router.GET("/merchant/invoices/:id", func(c *gin.Context) {
				if tt.args.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
				}
				handler.GetInvoice(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/invoices/"+tt.args.invoiceID, nil)
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
		})
	}
}
