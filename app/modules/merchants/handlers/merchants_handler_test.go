package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/modules/merchants/models/entity"
	serviceMocks "payment-sandbox/app/modules/merchants/services/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMerchantsHandler_ListMerchants(t *testing.T) {
	gin.SetMode(gin.TestMode)

	merchant1 := entity.MerchantSummary{ID: "m1", Name: "Alice", Email: "alice@example.com"}
	merchant2 := entity.MerchantSummary{ID: "m2", Name: "Bob", Email: "bob@example.com"}

	tests := []struct {
		name       string
		query      string
		setupMocks func(svc *serviceMocks.MockIMerchantsService)
		wantStatus int
		wantCode   string
		wantLen    int
		wantTotal  float64
	}{
		{
			name:  "default pagination — no params",
			query: "",
			setupMocks: func(svc *serviceMocks.MockIMerchantsService) {
				svc.EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return([]entity.MerchantSummary{merchant1, merchant2}, 2, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    2,
			wantTotal:  2,
		},
		{
			name:  "explicit page and limit",
			query: "?page=2&limit=5",
			setupMocks: func(svc *serviceMocks.MockIMerchantsService) {
				svc.EXPECT().
					ListMerchants(mock.Anything, "", 2, 5).
					Return([]entity.MerchantSummary{merchant1}, 6, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    1,
			wantTotal:  6,
		},
		{
			name:  "search param forwarded",
			query: "?search=ali",
			setupMocks: func(svc *serviceMocks.MockIMerchantsService) {
				svc.EXPECT().
					ListMerchants(mock.Anything, "ali", 1, 20).
					Return([]entity.MerchantSummary{merchant1}, 1, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    1,
			wantTotal:  1,
		},
		{
			name:  "service error returns 400",
			query: "",
			setupMocks: func(svc *serviceMocks.MockIMerchantsService) {
				svc.EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return(nil, 0, errors.New("db error"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "merchants_list_failed",
		},
		{
			name:  "empty result — returns empty array",
			query: "?search=nomatch",
			setupMocks: func(svc *serviceMocks.MockIMerchantsService) {
				svc.EXPECT().
					ListMerchants(mock.Anything, "nomatch", 1, 20).
					Return([]entity.MerchantSummary{}, 0, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    0,
			wantTotal:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := serviceMocks.NewMockIMerchantsService(t)
			tc.setupMocks(svc)

			handler := NewMerchantsHandler(svc)
			router := gin.New()
			router.GET("/admin/merchants", handler.ListMerchants)

			req := httptest.NewRequest(http.MethodGet, "/admin/merchants"+tc.query, nil)
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

			data, ok := payload["data"].([]any)
			require.True(t, ok)
			assert.Len(t, data, tc.wantLen)

			meta, ok := payload["meta"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantTotal, meta["total"])
		})
	}
}
