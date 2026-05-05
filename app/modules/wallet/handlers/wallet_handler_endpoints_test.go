package handlers

// Branch analysis for WalletHandler.Wallet:
// ├── middleware.MustUserID fails → 401 auth_unauthorized
// ├── service.WalletByUserID → error → 404 wallet_not_found
// └── service.WalletByUserID → success → 200 with merchant data
//
// Branch analysis for WalletHandler.ListTopups:
// └── service.ListTopups → always 200 (no error path)
//
// Branch analysis for WalletHandler.ListWalletTransactions:
// ├── middleware.MustUserID fails → 401 auth_unauthorized
// ├── non-admin role with merchant_id param → 403 auth_forbidden
// ├── invalid 'from' date (not RFC3339) → 400 validation_error
// ├── invalid 'to' date (not RFC3339) → 400 validation_error
// ├── direction != "D" && direction != "C" → 400 validation_error
// ├── valid direction "D" filter → forwarded in EntryFilter
// ├── valid reference_prefix filter → forwarded in EntryFilter
// ├── targetMerchantID != "" (admin) → ListWalletTransactionsByMerchant
// ├── targetMerchantID == "" (merchant) → ListWalletTransactions
// ├── service error → 400 transactions_list_failed
// └── success → 200 with data + meta
//
// Branch analysis for WalletHandler.UpdateTopupStatus:
// ├── c.ShouldBindJSON → validation error (empty status) → 400 validation_error
// ├── service.UpdateTopupStatus → error → audit log (FAILED event), 400 topup_update_failed
// └── service.UpdateTopupStatus → success → audit log (SUCCESS event), 200 with topup

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"
	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
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

// ─── Wallet ──────────────────────────────────────────────────────────────────

func TestWalletHandler_Wallet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     walletServices.IWalletService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		userID string // empty → MustUserID missing
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errCode    string
		merchantID string
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
			args: args{userID: ""},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusUnauthorized, errCode: "auth_unauthorized"},
		},
		{
			name: "2. service.WalletByUserID error -> 404 wallet_not_found",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					WalletByUserID("user-1").
					Return(walletEntity.Merchant{}, errors.New("merchant wallet not found")).
					Once()
			}},
			wants: wants{statusCode: http.StatusNotFound, errCode: "wallet_not_found"},
		},
		{
			name: "3. service.WalletByUserID success -> 200 with merchant data",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					WalletByUserID("user-1").
					Return(walletEntity.Merchant{ID: "merchant-1", Balance: 50000}, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, merchantID: "merchant-1"},
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
			router.GET("/merchant/wallet", func(c *gin.Context) {
				if tt.args.userID != "" {
					c.Set(middleware.ContextUserID, tt.args.userID)
				}
				handler.Wallet(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/wallet", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.errCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok, "error key must be present")
				assert.Equal(t, tt.wants.errCode, errData["code"], "error code")
			} else {
				data, ok := payload["data"].(map[string]any)
				require.True(t, ok, "data key must be present")
				assert.Equal(t, tt.wants.merchantID, data["id"], "data.id")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIWalletService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListTopups ──────────────────────────────────────────────────────────────

func TestWalletHandler_ListTopups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     walletServices.IWalletService
		auditLogger audit.IAuditLogger
	}
	type mocks struct {
		setup func(f fields, _ struct{})
	}
	type wants struct {
		statusCode int
		dataLen    int
	}

	tests := []struct {
		name   string
		fields fields
		mocks  mocks
		wants  wants
	}{
		{
			name: "1. service.ListTopups returns items -> 200 all items returned",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			mocks: mocks{setup: func(f fields, _ struct{}) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListTopups().
					Return([]walletEntity.Topup{{ID: "t1"}, {ID: "t2"}}).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 2},
		},
		{
			name: "2. service.ListTopups returns empty slice -> 200 empty array",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			mocks: mocks{setup: func(f fields, _ struct{}) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListTopups().
					Return([]walletEntity.Topup{}).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, struct{}{})
			}

			handler := NewWalletHandler(tt.fields.service, tt.fields.auditLogger)
			router := gin.New()
			router.GET("/admin/topups", handler.ListTopups)

			req := httptest.NewRequest(http.MethodGet, "/admin/topups", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")
			data, ok := payload["data"].([]any)
			require.True(t, ok, "data must be an array")
			assert.Len(t, data, tt.wants.dataLen, "data length")

			if m, ok := tt.fields.service.(*serviceMocks.MockIWalletService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListWalletTransactions ───────────────────────────────────────────────────

func TestWalletHandler_ListWalletTransactions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	entry := ledgerEntity.EntryWithTxn{ID: 1, Reference: "topup:t1", Direction: ledgerEntity.Debit, Amount: 50000, Currency: "IDR"}

	dirD := "D"
	prefix := "topup:"

	type fields struct {
		service     walletServices.IWalletService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		query  string
		userID string // empty → MustUserID missing
		role   string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errCode    string
		dataLen    int
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
			args: args{query: "", userID: "", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusUnauthorized, errCode: "auth_unauthorized"},
		},
		{
			name: "2. non-admin uses merchant_id param -> 403 auth_forbidden",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?merchant_id=some-uuid", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusForbidden, errCode: "auth_forbidden"},
		},
		{
			name: "3. invalid 'from' date param -> 400 validation_error",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?from=not-a-date", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "validation_error"},
		},
		{
			name: "4. invalid 'to' date param -> 400 validation_error",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?to=not-a-date", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "validation_error"},
		},
		{
			name: "5. invalid direction param (not D or C) -> 400 validation_error",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?direction=X", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "validation_error"},
		},
		{
			name: "6. service.ListWalletTransactions error -> 400 transactions_list_failed",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListWalletTransactions("user-1", ledgerEntity.EntryFilter{}, 1, 10).
					Return(nil, 0, errors.New("db error")).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "transactions_list_failed"},
		},
		{
			name: "7. merchant success no filters -> 200 with entries and meta",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListWalletTransactions("user-1", ledgerEntity.EntryFilter{}, 1, 10).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 1},
		},
		{
			name: "8. direction=D filter -> ListWalletTransactions called with direction filter",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?direction=D", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListWalletTransactions("user-1", ledgerEntity.EntryFilter{Direction: &dirD}, 1, 10).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 1},
		},
		{
			name: "9. reference_prefix filter -> ListWalletTransactions called with prefix filter",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?reference_prefix=topup:", userID: "user-1", role: "MERCHANT"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListWalletTransactions("user-1", ledgerEntity.EntryFilter{ReferencePrefix: &prefix}, 1, 10).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 1},
		},
		{
			name: "10. admin with merchant_id param -> ListWalletTransactionsByMerchant called",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{query: "?merchant_id=merchant-99", userID: "admin-1", role: "ADMIN"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					ListWalletTransactionsByMerchant("merchant-99", ledgerEntity.EntryFilter{}, 1, 10).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataLen: 1},
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
			router.GET("/merchant/wallet/transactions", func(c *gin.Context) {
				if tt.args.userID != "" {
					c.Set(middleware.ContextUserID, tt.args.userID)
				}
				c.Set(middleware.ContextRole, tt.args.role)
				handler.ListWalletTransactions(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/merchant/wallet/transactions"+tt.args.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.errCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok, "error key must be present")
				assert.Equal(t, tt.wants.errCode, errData["code"], "error code")
			} else {
				data, ok := payload["data"].([]any)
				require.True(t, ok, "data must be an array")
				assert.Len(t, data, tt.wants.dataLen, "data length")
				_, hasMeta := payload["meta"]
				assert.True(t, hasMeta, "meta must be present on success")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIWalletService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── UpdateTopupStatus ────────────────────────────────────────────────────────

func TestWalletHandler_UpdateTopupStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service     walletServices.IWalletService
		auditLogger audit.IAuditLogger
	}
	type args struct {
		topupID string
		body    string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errCode    string
		dataID     string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name: "1. empty status fails binding validation -> 400 validation_error",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{topupID: "topup-1", body: `{"status":""}`},
			mocks: mocks{setup: func(f fields, a args) {}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "validation_error"},
		},
		{
			name: "2. service.UpdateTopupStatus error -> audit FAILED event 400 topup_update_failed",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{topupID: "topup-1", body: `{"status":"SUCCESS"}`},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					UpdateTopupStatus("topup-1", "SUCCESS").
					Return(walletEntity.Topup{}, errors.New("topup already finalized")).
					Once()
				f.auditLogger.(*auditMocks.MockIAuditLogger).EXPECT().
					Log(mock.Anything, mock.MatchedBy(func(event audit.Event) bool {
						result, _ := event.Metadata["result"].(string)
						return event.EventType == "topup.status_updated" && result == "FAILED" &&
							event.ResourceID == "topup-1"
					})).
					Return(nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errCode: "topup_update_failed"},
		},
		{
			name: "3. service.UpdateTopupStatus success -> audit SUCCESS event 200 with topup",
			fields: fields{
				service:     serviceMocks.NewMockIWalletService(t),
				auditLogger: auditMocks.NewMockIAuditLogger(t),
			},
			args: args{topupID: "topup-1", body: `{"status":"SUCCESS"}`},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIWalletService).EXPECT().
					UpdateTopupStatus("topup-1", "SUCCESS").
					Return(walletEntity.Topup{ID: "topup-1"}, nil).
					Once()
				f.auditLogger.(*auditMocks.MockIAuditLogger).EXPECT().
					Log(mock.Anything, mock.MatchedBy(func(event audit.Event) bool {
						result, _ := event.Metadata["result"].(string)
						return event.EventType == "topup.status_updated" && result == "SUCCESS" &&
							event.ResourceID == "topup-1"
					})).
					Return(nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusOK, dataID: "topup-1"},
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
			router.PATCH("/admin/topups/:id/status", func(c *gin.Context) {
				c.Set(middleware.ContextRequestID, "req-1")
				c.Set(middleware.ContextRole, "ADMIN")
				handler.UpdateTopupStatus(c)
			})

			req := httptest.NewRequest(http.MethodPatch, "/admin/topups/"+tt.args.topupID+"/status",
				bytes.NewBufferString(tt.args.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

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
