package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	serviceMocks "payment-sandbox/app/modules/refund/services/mocks"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	"payment-sandbox/app/shared/audit"
	auditMocks "payment-sandbox/app/shared/audit/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRefundHandler_RequestRefund(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		withUserID bool
		body       string
		setupMocks func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name:       "missing user context",
			withUserID: false,
			body:       `{"invoice_id":"inv-1","reason":"duplicate payment"}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "RequestRefund")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "auth_unauthorized",
		},
		{
			name:       "validation error",
			withUserID: true,
			body:       `{"invoice_id":"","reason":""}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "RequestRefund")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "malformed json",
			withUserID: true,
			body:       `{invalid-json}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "RequestRefund")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "service error and logger failure",
			withUserID: true,
			body:       `{"invoice_id":"inv-1","reason":"duplicate payment"}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					RequestRefund("user-1", "inv-1", "duplicate payment").
					Return(refundEntity.Refund{}, errors.New("refund can be requested for successful payment only"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "refund.requested" &&
								result == "FAILED" &&
								event.RequestID == "req-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "refund_request_failed",
		},
		{
			name:       "success and logger failure",
			withUserID: true,
			body:       `{"invoice_id":"inv-1","reason":"duplicate payment"}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					RequestRefund("user-1", "inv-1", "duplicate payment").
					Return(refundEntity.Refund{
						ID:              "refund-1",
						PaymentIntentID: "pi-1",
						Status:          refundEntity.RefundRequested,
					}, nil)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "refund.requested" &&
								result == "SUCCESS" &&
								event.ResourceID == "refund-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusCreated,
			wantID:     "refund-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIRefundService(t)
			logger := auditMocks.NewMockIAuditLogger(t)
			tc.setupMocks(service, logger)

			handler := NewRefundHandler(service, logger)
			router := gin.New()
			router.POST("/merchant/refunds", func(c *gin.Context) {
				if tc.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
					c.Set(middleware.ContextRole, "MERCHANT")
				}
				c.Set(middleware.ContextRequestID, "req-1")
				handler.RequestRefund(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/merchant/refunds", bytes.NewBufferString(tc.body))
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

func TestRefundHandler_MerchantListRefunds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service *serviceMocks.MockIRefundService
		logger  *auditMocks.MockIAuditLogger
	}
	type args struct {
		withUserID bool
		status     string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		status  int
		errCode string
		dataLen int
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name: "1. no user in context -> unauthorized",
			fields: fields{
				service: serviceMocks.NewMockIRefundService(t),
				logger:  auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: false, status: ""},
			mocks: mocks{
				setup: nil,
			},
			wants: wants{status: http.StatusUnauthorized, errCode: "auth_unauthorized"},
		},
		{
			name: "2. valid user, no status param -> success returns list",
			fields: fields{
				service: serviceMocks.NewMockIRefundService(t),
				logger:  auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, status: ""},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.service.EXPECT().
						MerchantListRefunds("user-1", "").
						Return([]refundEntity.Refund{{ID: "refund-1"}}).
						Once()
				},
			},
			wants: wants{status: http.StatusOK, dataLen: 1},
		},
		{
			name: "3. valid user, with status param -> success returns filtered list",
			fields: fields{
				service: serviceMocks.NewMockIRefundService(t),
				logger:  auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, status: "REQUESTED"},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.service.EXPECT().
						MerchantListRefunds("user-1", "REQUESTED").
						Return([]refundEntity.Refund{{ID: "refund-1"}, {ID: "refund-2"}}).
						Once()
				},
			},
			wants: wants{status: http.StatusOK, dataLen: 2},
		},
		{
			name: "4. valid user, service returns empty -> success with empty list",
			fields: fields{
				service: serviceMocks.NewMockIRefundService(t),
				logger:  auditMocks.NewMockIAuditLogger(t),
			},
			args: args{withUserID: true, status: "SUCCESS"},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.service.EXPECT().
						MerchantListRefunds("user-1", "SUCCESS").
						Return([]refundEntity.Refund{}).
						Once()
				},
			},
			wants: wants{status: http.StatusOK, dataLen: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			handler := NewRefundHandler(tt.fields.service, tt.fields.logger)
			router := gin.New()
			router.GET("/merchant/refunds", func(c *gin.Context) {
				if tt.args.withUserID {
					c.Set(middleware.ContextUserID, "user-1")
					c.Set(middleware.ContextRole, "MERCHANT")
				}
				handler.MerchantListRefunds(c)
			})

			url := "/merchant/refunds"
			if tt.args.status != "" {
				url += "?status=" + tt.args.status
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.status, rec.Code)

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err)

			if tt.wants.errCode != "" {
				errorData, ok := payload["error"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tt.wants.errCode, errorData["code"])
			} else {
				data, ok := payload["data"].([]any)
				require.True(t, ok)
				assert.Len(t, data, tt.wants.dataLen)
			}

			tt.fields.service.AssertExpectations(t)
		})
	}
}

func TestRefundHandler_ProcessRefund(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name: "validation error",
			body: `{"status":""}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.AssertNotCalled(t, "ProcessRefund")
				logger.AssertNotCalled(t, "Log")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service error and logger failure",
			body: `{"status":"SUCCESS"}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					ProcessRefund("refund-1", "SUCCESS").
					Return(refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("refund must be approved before processing"))

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "refund.processed" &&
								result == "FAILED" &&
								event.ResourceID == "refund-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "refund_process_failed",
		},
		{
			name: "success and logger failure",
			body: `{"status":"SUCCESS"}`,
			setupMocks: func(service *serviceMocks.MockIRefundService, logger *auditMocks.MockIAuditLogger) {
				service.EXPECT().
					ProcessRefund("refund-1", "SUCCESS").
					Return(
						refundEntity.Refund{ID: "refund-1", Status: refundEntity.RefundSuccess},
						walletEntity.Merchant{ID: "merchant-1"},
						nil,
					)

				logger.EXPECT().
					Log(
						mock.Anything,
						mock.MatchedBy(func(event audit.Event) bool {
							result, _ := event.Metadata["result"].(string)
							return event.EventType == "refund.processed" &&
								result == "SUCCESS" &&
								event.ResourceID == "refund-1"
						}),
					).
					Return(errors.New("mongo write failed"))
			},
			wantStatus: http.StatusOK,
			wantID:     "refund-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIRefundService(t)
			logger := auditMocks.NewMockIAuditLogger(t)
			tc.setupMocks(service, logger)

			handler := NewRefundHandler(service, logger)
			router := gin.New()
			router.PATCH("/admin/refunds/:id/process", func(c *gin.Context) {
				c.Set(middleware.ContextRequestID, "req-1")
				c.Set(middleware.ContextRole, "ADMIN")
				handler.ProcessRefund(c)
			})

			req := httptest.NewRequest(http.MethodPatch, "/admin/refunds/refund-1/process", bytes.NewBufferString(tc.body))
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
			refundData, ok := data["refund"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.wantID, refundData["id"])
		})
	}
}
