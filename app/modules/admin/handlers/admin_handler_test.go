package handlers

// Branch analysis for AdminHandler.Healthz(c):
// └── single path: response.OK(c, {"status":"ok"}) → 200
//
// Branch analysis for AdminHandler.DashboardStats(c):
// ├── service.Stats returns error → 400, {"error":{"code":"stats_query_failed","message":"<err>"}}
// └── service.Stats returns stats → 200, {"data":{...full stats...}}

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	adminServices "payment-sandbox/app/modules/admin/services"
	serviceMocks "payment-sandbox/app/modules/admin/services/mocks"

	"errors"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// serveAdminRouter builds a minimal router for the given handler under test.
func serveAdminRouter(handler *AdminHandler) *gin.Engine {
	r := gin.New()
	r.GET("/ping", handler.Healthz)
	r.GET("/admin/stats", handler.DashboardStats)
	return r
}

// ─── AdminHandler.Healthz ─────────────────────────────────────────────────────

func TestAdminHandler_Healthz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service adminServices.IAdminService
	}
	type wants struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name   string
		fields fields
		wants  wants
	}{
		{
			name:   "1. GET /ping -> 200 {\"data\":{\"status\":\"ok\"}}",
			fields: fields{service: serviceMocks.NewMockIAdminService(t)},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":{"status":"ok"}}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Healthz must never call service.Stats
			handler := NewAdminHandler(tt.fields.service)
			rec := httptest.NewRecorder()
			serveAdminRouter(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")

			if m, ok := tt.fields.service.(*serviceMocks.MockIAdminService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── AdminHandler.DashboardStats ─────────────────────────────────────────────

func TestAdminHandler_DashboardStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service adminServices.IAdminService
	}
	type args struct {
		query string // raw URL query string, e.g. "merchant_id=x&start_date=y"
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		body       string
	}

	fullStats := adminEntity.DashboardStats{
		TotalInvoiceCreated: 10,
		TotalByStatus:       map[string]int{"PAID": 8, "EXPIRED": 2, "FAILED": 1},
		TotalPaymentNominal: 100_000,
		TotalRefundNominal:  5_000,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. service returns error -> 400 stats_query_failed",
			fields: fields{service: serviceMocks.NewMockIAdminService(t)},
			args:   args{query: "merchant_id=m-1&start_date=2026-04-01&end_date=2026-04-30"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIAdminService).EXPECT().
					Stats("m-1", "2026-04-01", "2026-04-30").
					Return(adminEntity.DashboardStats{}, errors.New("invalid date range")).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusBadRequest,
				body:       `{"error":{"code":"stats_query_failed","message":"invalid date range"}}`,
			},
		},
		{
			name:   "2. no query params -> service called with empty strings, 200 full stats",
			fields: fields{service: serviceMocks.NewMockIAdminService(t)},
			args:   args{query: ""},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIAdminService).EXPECT().
					Stats("", "", "").
					Return(fullStats, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body: `{
					"data": {
						"total_invoice_created": 10,
						"total_by_status": {"PAID":8,"EXPIRED":2,"FAILED":1},
						"total_payment_nominal": 100000,
						"total_refund_nominal": 5000
					}
				}`,
			},
		},
		{
			name:   "3. all query params provided -> service forwarded correct args, 200 full stats",
			fields: fields{service: serviceMocks.NewMockIAdminService(t)},
			args:   args{query: "merchant_id=m-2&start_date=2026-01-01&end_date=2026-03-31"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIAdminService).EXPECT().
					Stats("m-2", "2026-01-01", "2026-03-31").
					Return(fullStats, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body: `{
					"data": {
						"total_invoice_created": 10,
						"total_by_status": {"PAID":8,"EXPIRED":2,"FAILED":1},
						"total_payment_nominal": 100000,
						"total_refund_nominal": 5000
					}
				}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			handler := NewAdminHandler(tt.fields.service)
			url := "/admin/stats"
			if tt.args.query != "" {
				url += "?" + tt.args.query
			}
			rec := httptest.NewRecorder()
			serveAdminRouter(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")

			if m, ok := tt.fields.service.(*serviceMocks.MockIAdminService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
