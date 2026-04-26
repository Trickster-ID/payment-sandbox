package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	serviceMocks "payment-sandbox/app/modules/refund/handlers/mocks"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	journeylog "payment-sandbox/app/shared/journeylog"
	journeyMocks "payment-sandbox/app/shared/journeylog/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRefundHandler_ListRefunds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := serviceMocks.NewMockRefundService(t)
	logger := journeyMocks.NewMockJourneyLogger(t)
	service.EXPECT().ListRefunds("REQUESTED").Return([]refundEntity.Refund{{ID: "refund-1"}})

	handler := NewRefundHandler(service, logger)
	router := gin.New()
	router.GET("/admin/refunds", handler.ListRefunds)

	req := httptest.NewRequest(http.MethodGet, "/admin/refunds?status=REQUESTED", nil)
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

func TestRefundHandler_ReviewRefund(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockRefundService, logger *journeyMocks.MockJourneyLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name: "validation error",
			body: `{"decision":""}`,
			setupMocks: func(service *serviceMocks.MockRefundService, logger *journeyMocks.MockJourneyLogger) {
				service.AssertNotCalled(t, "ReviewRefund")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure",
			body: `{"decision":"APPROVE"}`,
			setupMocks: func(service *serviceMocks.MockRefundService, logger *journeyMocks.MockJourneyLogger) {
				service.EXPECT().ReviewRefund("refund-1", "APPROVE").Return(refundEntity.Refund{}, errors.New("already reviewed"))
				logger.EXPECT().Log(
					mock.Anything,
					mock.MatchedBy(func(event journeylog.Event) bool {
						return event.Module == "refund" && event.Action == "REFUND_REVIEW" && event.Result == "FAILED"
					}),
				).Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "refund_review_failed",
		},
		{
			name: "success and logger failure",
			body: `{"decision":"APPROVE"}`,
			setupMocks: func(service *serviceMocks.MockRefundService, logger *journeyMocks.MockJourneyLogger) {
				service.EXPECT().ReviewRefund("refund-1", "APPROVE").Return(refundEntity.Refund{ID: "refund-1", Status: refundEntity.RefundApproved}, nil)
				logger.EXPECT().Log(
					mock.Anything,
					mock.MatchedBy(func(event journeylog.Event) bool {
						return event.Module == "refund" && event.Action == "REFUND_REVIEW" && event.Result == "SUCCESS"
					}),
				).Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusOK,
			wantID:     "refund-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockRefundService(t)
			logger := journeyMocks.NewMockJourneyLogger(t)
			tc.setupMocks(service, logger)

			handler := NewRefundHandler(service, logger)
			router := gin.New()
			router.PATCH("/admin/refunds/:id/review", func(c *gin.Context) {
				c.Set(middleware.ContextRequestID, "req-1")
				c.Set(middleware.ContextRole, "ADMIN")
				handler.ReviewRefund(c)
			})

			req := httptest.NewRequest(http.MethodPatch, "/admin/refunds/refund-1/review", bytes.NewBufferString(tc.body))
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
			assert.Equal(t, tc.wantID, data["id"])
		})
	}
}
