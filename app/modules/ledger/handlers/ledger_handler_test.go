// Branch map for GetMerchantAccount:
// ├── uuid.Parse(merchant_id) fails                 -> 400 bad request     [Case 1]
// ├── repo.GetAccountByMerchantID returns error     -> 404 not found       [Case 2]
// └── repo.GetAccountByMerchantID returns account   -> 200 success         [Case 3]
package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/modules/ledger/handlers"
	"payment-sandbox/app/modules/ledger/models/entity"
	"payment-sandbox/app/modules/ledger/repositories"
	"payment-sandbox/app/modules/ledger/repositories/mocks"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestLedgerHandler_GetMerchantAccount(t *testing.T) {
	type fields struct {
		repo repositories.IRepository
	}
	type args struct {
		merchantID string
	}
	type mockSetup struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		bodyJSON   string // non-empty: exact JSONEq check
		hasDataKey bool   // true: assert "data" key present in response
	}

	validMerchantID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	account := entity.Account{
		ID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Name:     "Merchant Wallet",
		Type:     entity.Asset,
		Currency: "USD",
		Balance:  5000,
		IsActive: true,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mockSetup
		wants  wants
	}{
		{
			name: "1. invalid merchant_id (not a UUID) -> 400 bad request",
			fields: fields{
				repo: mocks.NewMockIRepository(t),
			},
			args: args{merchantID: "not-a-uuid"},
			mocks: mockSetup{
				setup: func(f fields, a args) {
					// repo is not reached; no expectations set
				},
			},
			wants: wants{
				statusCode: http.StatusBadRequest,
				bodyJSON:   `{"error":{"code":"invalid_merchant_id","message":"merchant_id must be a valid UUID"}}`,
			},
		},
		{
			name: "2. valid merchant_id, repo returns error -> 404 not found",
			fields: fields{
				repo: mocks.NewMockIRepository(t),
			},
			args: args{merchantID: validMerchantID.String()},
			mocks: mockSetup{
				setup: func(f fields, a args) {
					m := f.repo.(*mocks.MockIRepository)
					m.EXPECT().
						GetAccountByMerchantID(mock.Anything, validMerchantID).
						Return(entity.Account{}, errors.New("sql: no rows")).
						Once()
				},
			},
			wants: wants{
				statusCode: http.StatusNotFound,
				bodyJSON:   `{"error":"account not found"}`,
			},
		},
		{
			name: "3. valid merchant_id, repo returns account -> 200 success",
			fields: fields{
				repo: mocks.NewMockIRepository(t),
			},
			args: args{merchantID: validMerchantID.String()},
			mocks: mockSetup{
				setup: func(f fields, a args) {
					m := f.repo.(*mocks.MockIRepository)
					m.EXPECT().
						GetAccountByMerchantID(mock.Anything, validMerchantID).
						Return(account, nil).
						Once()
				},
			},
			wants: wants{
				statusCode: http.StatusOK,
				hasDataKey: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				if _, ok := tt.fields.repo.(*mocks.MockIRepository); ok {
					tt.mocks.setup(tt.fields, tt.args)
				}
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
			c.Params = gin.Params{{Key: "merchant_id", Value: tt.args.merchantID}}

			h := handlers.NewLedgerHandler(tt.fields.repo)
			h.GetMerchantAccount(c)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			assert.False(t, c.IsAborted(), "handler should not abort (uses c.JSON, not c.Abort)")

			if tt.wants.bodyJSON != "" {
				assert.JSONEq(t, tt.wants.bodyJSON, w.Body.String(), "response body")
			}

			if tt.wants.hasDataKey {
				var resp map[string]any
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "body must be valid JSON")
				assert.Contains(t, resp, "data", "successful response must contain 'data' key")
			}

			if m, ok := tt.fields.repo.(*mocks.MockIRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
