package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	serviceMocks "payment-sandbox/app/modules/admin/services/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHandler_Healthz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := serviceMocks.NewMockIAdminService(t)
	service.AssertNotCalled(t, "Stats")

	handler := NewAdminHandler(service)
	router := gin.New()
	router.GET("/api/v1/ping", handler.Healthz)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &payload)
	require.NoError(t, err)

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", data["status"])
}

func TestAdminHandler_DashboardStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		setupMocks func(service *serviceMocks.MockIAdminService)
		wantStatus int
		wantCode   string
		wantTotal  float64
	}{
		{
			name:  "service error",
			query: "merchant_id=merchant-1&start_date=2026-04-01&end_date=2026-04-30",
			setupMocks: func(service *serviceMocks.MockIAdminService) {
				service.EXPECT().
					Stats("merchant-1", "2026-04-01", "2026-04-30").
					Return(adminEntity.DashboardStats{}, errors.New("invalid date range"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "stats_query_failed",
		},
		{
			name:  "success",
			query: "merchant_id=merchant-1&start_date=2026-04-01&end_date=2026-04-30",
			setupMocks: func(service *serviceMocks.MockIAdminService) {
				service.EXPECT().
					Stats("merchant-1", "2026-04-01", "2026-04-30").
					Return(adminEntity.DashboardStats{
						TotalInvoiceCreated: 10,
						TotalByStatus: map[string]int{
							"PAID": 8,
						},
						TotalPaymentNominal: 100000,
						TotalRefundNominal:  5000,
					}, nil)
			},
			wantStatus: http.StatusOK,
			wantTotal:  100000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIAdminService(t)
			tc.setupMocks(service)

			handler := NewAdminHandler(service)
			router := gin.New()
			router.GET("/api/v1/admin/stats", handler.DashboardStats)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats?"+tc.query, nil)
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
			assert.Equal(t, tc.wantTotal, data["total_payment_nominal"])
		})
	}
}
