package handlers

// Branch analysis for WalletHandler.CreateTopup:
// ├── middleware.MustUserID fails (no user_id key) → 401 auth_unauthorized
// ├── c.ShouldBindJSON → validation error (amount=0 or malformed JSON) → 400 validation_error
// ├── service.CreateTopup → error → audit log (FAILED event), 400 topup_create_failed
// └── service.CreateTopup → success → audit log (SUCCESS event), 201 with topup data
//
// Branch analysis for WalletHandler.ListMerchantTopups:
// ├── middleware.MustUserID fails → 401 auth_unauthorized
// ├── service.ListMerchantTopups → error → 400 topup_list_failed
// └── service.ListMerchantTopups → success → 200 with data + meta {page, limit, total}

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	walletServices "payment-sandbox/app/modules/wallet/services"
	serviceMocks "payment-sandbox/app/modules/wallet/services/mocks"
	"payment-sandbox/app/shared/audit"
	auditMocks "payment-sandbox/app/shared/audit/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─── CreateTopup ─────────────────────────────────────────────────────────────

func TestWalletHandler_CreateTopup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     walletServices.IWalletService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		body   string
		userID string // empty → MustUserID missing
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errCode    string // non-empty → assert error response
		dataID     string // non-empty → assert data.id in success response
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name: "1. missing user_id in context -> 401 auth_unauthorized",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{body: `{"amount":10000}`, userID: ""},
			mocks: mocks{setup: func(f fields, a args) {
				// handler aborts at MustUserID — no service/audit calls expected
			}},
			wants: wants{statusCode: http.StatusUnauthorized, errCode: "auth_unauthorized"},
		},
		{
			name: "2. amount=0 fails binding validation -> 400 validation_error",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{body: `{"amount":0}`, userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				// ShouldBindJSON rejects before service call
			}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "validation_error"},
		},
		{
			name: "3. malformed JSON body -> 400 validation_error",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{body: `{invalid}`, userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "validation_error"},
		},
		{
			name: "4. service.CreateTopup error -> audit FAILED event 400 topup_create_failed",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{body: `{"amount":15000}`, userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					CreateTopup("user-1", int64(15000)).
					Return(walletEntity.Topup{}, errors.New("merchant not found")).
					Once()
				f.auditLogger.(*auditMocks.MockIAuditLogger).EXPECT().
					Log(mock.Anything, mock.MatchedBy(func(event audit.Event) bool {
						result, _ := event.Metadata["result"].(string)
						return event.EventType == "topup.created" && result == "FAILED"
					})).
					Return(errors.New("audit write failed")).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "topup_create_failed"},
		},
		{
			name: "5. service.CreateTopup success -> audit SUCCESS event 201 with topup ID",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{body: `{"amount":25000}`, userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					CreateTopup("user-1", int64(25000)).
					Return(walletEntity.Topup{ID: "topup-1", Amount: 25000}, nil).
					Once()
				f.auditLogger.(*auditMocks.MockIAuditLogger).EXPECT().
					Log(mock.Anything, mock.MatchedBy(func(event audit.Event) bool {
						result, _ := event.Metadata["result"].(string)
						return event.EventType == "topup.created" && result == "SUCCESS" &&
							event.ResourceID == "topup-1"
					})).
					Return(nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusCreated, dataID: "topup-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			handler := NewWalletHandler(tt.fields.service, tt.fields.auditLogger)
			router := gin.New()
			router.POST("/topups", func(c *gin.Context) {
				if tt.args.userID != "" {
					c.Set(middleware.ContextUserID, tt.args.userID)
				}
				c.Set(middleware.ContextRole, "MERCHANT")
				c.Set(middleware.ContextRequestID, "req-1")
				handler.CreateTopup(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/topups", bytes.NewBufferString(tt.args.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err, "response body must be valid JSON")

			if tt.wants.errCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok, "error key must be present")
				assert.Equal(t, tt.wants.errCode, errData["code"], "error code")
			} else {
				data, ok := payload["data"].(map[string]any)
				require.True(t, ok, "data key must be present")
				assert.Equal(t, tt.wants.dataID, data["id"], "data.id")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIWalletService); ok {
				m.AssertExpectations(t)
			}
			if m, ok := tt.fields.auditLogger.(*auditMocks.MockIAuditLogger); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListMerchantTopups ───────────────────────────────────────────────────────

func TestWalletHandler_ListMerchantTopups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     walletServices.IWalletService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		query  string
		userID string // empty → MustUserID missing
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errCode    string
		dataLen    int
		page       float64
		limit      float64
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
			name: "1. missing user_id in context -> 401 auth_unauthorized",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "", userID: ""},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusUnauthorized, errCode: "auth_unauthorized"},
		},
		{
			name: "2. service.ListMerchantTopups error -> 400 topup_list_failed",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "", userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListMerchantTopups("user-1", 1, 10).
					Return(nil, 0, errors.New("merchant not found")).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "topup_list_failed"},
		},
		{
			name: "3. success default pagination -> 200 data + meta page=1 limit=10",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "", userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListMerchantTopups("user-1", 1, 10).
					Return([]walletEntity.Topup{{ID: "t1"}, {ID: "t2"}}, 2, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 2, page: 1, limit: 10, total: 2},
		},
		{
			name: "4. page=2 limit=5 -> 200 service called with correct pagination",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?page=2&limit=5", userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListMerchantTopups("user-1", 2, 5).
					Return([]walletEntity.Topup{{ID: "t3"}}, 6, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 1, page: 2, limit: 5, total: 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			handler := NewWalletHandler(tt.fields.service, tt.fields.auditLogger)
			router := gin.New()
			router.GET("/merchant/topups", func(c *gin.Context) {
				if tt.args.userID != "" {
					c.Set(middleware.ContextUserID, tt.args.userID)
				}
				handler.ListMerchantTopups(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/topups"+tt.args.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err, "response body must be valid JSON")

			if tt.wants.errCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok, "error key must be present")
				assert.Equal(t, tt.wants.errCode, errData["code"], "error code")
			} else {
				data, ok := payload["data"].([]any)
				require.True(t, ok, "data must be an array")
				assert.Len(t, data, tt.wants.dataLen, "data length")
				meta, ok := payload["meta"].(map[string]any)
				require.True(t, ok, "meta must be present")
				assert.Equal(t, tt.wants.page, meta["page"], "meta.page")
				assert.Equal(t, tt.wants.limit, meta["limit"], "meta.limit")
				assert.Equal(t, tt.wants.total, meta["total"], "meta.total")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIWalletService); ok {
				m.AssertExpectations(t)
			}
			if m, ok := tt.fields.auditLogger.(*auditMocks.MockIAuditLogger); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
