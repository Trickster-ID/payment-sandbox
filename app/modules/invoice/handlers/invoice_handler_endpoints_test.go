package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	serviceMocks "payment-sandbox/app/modules/invoice/services/mocks"
	journeyMocks "payment-sandbox/app/shared/journeylog/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceHandler_ListInvoices(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		withUserID bool
		query      string
		setupMocks func(service *serviceMocks.MockIInvoiceService)
		wantStatus int
		wantCode   string
		wantTotal  float64
	}{
		{
			name:       "missing user context",
			withUserID: false,
			query:      "status=PENDING&page=2&limit=20",
			setupMocks: func(service *serviceMocks.MockIInvoiceService) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "auth_unauthorized",
		},
		{
			name:       "service error",
			withUserID: true,
			query:      "status=PENDING&page=2&limit=20",
			setupMocks: func(service *serviceMocks.MockIInvoiceService) {
				service.EXPECT().ListInvoices("user-1", "PENDING", 2, 20).Return(nil, 0, errors.New("query failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invoice_list_failed",
		},
		{
			name:       "success with meta",
			withUserID: true,
			query:      "status=PENDING&page=2&limit=20",
			setupMocks: func(service *serviceMocks.MockIInvoiceService) {
				service.EXPECT().ListInvoices("user-1", "PENDING", 2, 20).Return([]invoiceEntity.Invoice{{ID: "inv-1"}}, 42, nil)
			},
			wantStatus: http.StatusOK,
			wantTotal:  42,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIInvoiceService(t)
			logger := journeyMocks.NewMockIJourneyLogger(t)
			tc.setupMocks(service)

			handler := NewInvoiceHandler(service, logger)
			router := gin.New()
			router.GET("/merchant/invoices", func(c *gin.Context) {
				if tc.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
				}
				handler.ListInvoices(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/invoices?"+tc.query, nil)
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

			meta, ok := payload["meta"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantTotal, meta["total"])
		})
	}
}

func TestInvoiceHandler_GetInvoice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		withUserID bool
		setupMocks func(service *serviceMocks.MockIInvoiceService)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name:       "missing user context",
			withUserID: false,
			setupMocks: func(service *serviceMocks.MockIInvoiceService) {},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "auth_unauthorized",
		},
		{
			name:       "invoice not found",
			withUserID: true,
			setupMocks: func(service *serviceMocks.MockIInvoiceService) {
				service.EXPECT().InvoiceByID("user-1", "inv-1").Return(invoiceEntity.Invoice{}, errors.New("not found"))
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "invoice_not_found",
		},
		{
			name:       "success",
			withUserID: true,
			setupMocks: func(service *serviceMocks.MockIInvoiceService) {
				service.EXPECT().InvoiceByID("user-1", "inv-1").Return(invoiceEntity.Invoice{ID: "inv-1"}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     "inv-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIInvoiceService(t)
			logger := journeyMocks.NewMockIJourneyLogger(t)
			tc.setupMocks(service)

			handler := NewInvoiceHandler(service, logger)
			router := gin.New()
			router.GET("/merchant/invoices/:id", func(c *gin.Context) {
				if tc.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
				}
				handler.GetInvoice(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/invoices/inv-1", nil)
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
