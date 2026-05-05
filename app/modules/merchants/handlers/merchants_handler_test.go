package handlers

// Branch analysis for MerchantsHandler.ListMerchants(c):
// ├── service.ListMerchants returns error → 400 merchants_list_failed
// └── service.ListMerchants returns data → 200 with data + meta {page, limit, total}
//
// Pagination edge-cases (via pagination.Parse):
// ├── page < 1 or non-numeric → defaults to page=1
// └── limit < 1, > 100, or non-numeric → defaults to limit=10

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/modules/merchants/models/entity"
	merchantServices "payment-sandbox/app/modules/merchants/services"
	serviceMocks "payment-sandbox/app/modules/merchants/services/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMerchantsHandler_ListMerchants(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service merchantServices.IMerchantsService
	}
	type args struct {
		query string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		body       string
	}

	merchant1 := entity.MerchantSummary{ID: "m1", Name: "Alice", Email: "alice@example.com"}
	merchant2 := entity.MerchantSummary{ID: "m2", Name: "Bob", Email: "bob@example.com"}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. no query params -> 200 default pagination page=1 limit=20",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: ""},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return([]entity.MerchantSummary{merchant1, merchant2}, 2, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[{"id":"m1","name":"Alice","email":"alice@example.com"},{"id":"m2","name":"Bob","email":"bob@example.com"}],"meta":{"limit":20,"page":1,"total":2}}`,
			},
		},
		{
			name:   "2. page=2 limit=5 -> 200 service called with page=2 limit=5",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: "?page=2&limit=5"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "", 2, 5).
					Return([]entity.MerchantSummary{merchant1}, 6, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[{"id":"m1","name":"Alice","email":"alice@example.com"}],"meta":{"limit":5,"page":2,"total":6}}`,
			},
		},
		{
			name:   "3. search=ali -> 200 search forwarded to service",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: "?search=ali"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "ali", 1, 20).
					Return([]entity.MerchantSummary{merchant1}, 1, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[{"id":"m1","name":"Alice","email":"alice@example.com"}],"meta":{"limit":20,"page":1,"total":1}}`,
			},
		},
		{
			name:   "4. service returns error -> 400 merchants_list_failed",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: ""},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return(nil, 0, errors.New("db error")).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusBadRequest,
				body:       `{"error":{"code":"merchants_list_failed","message":"db error"}}`,
			},
		},
		{
			name:   "5. search=nomatch zero results -> 200 empty array",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: "?search=nomatch"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "nomatch", 1, 20).
					Return([]entity.MerchantSummary{}, 0, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[],"meta":{"limit":20,"page":1,"total":0}}`,
			},
		},
		{
			name:   "6. page=0 (invalid) -> defaults to page=1 success",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: "?page=0"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return([]entity.MerchantSummary{merchant1}, 1, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[{"id":"m1","name":"Alice","email":"alice@example.com"}],"meta":{"limit":20,"page":1,"total":1}}`,
			},
		},
		{
			name:   "7. limit=200 (over max=100) -> defaults to limit=10 success",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: "?limit=200"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "", 1, 10).
					Return([]entity.MerchantSummary{merchant1}, 1, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[{"id":"m1","name":"Alice","email":"alice@example.com"}],"meta":{"limit":10,"page":1,"total":1}}`,
			},
		},
		{
			name:   "8. page=abc (non-numeric) -> defaults to page=1 success",
			fields: fields{service: serviceMocks.NewMockIMerchantsService(t)},
			args:   args{query: "?page=abc"},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIMerchantsService).EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return([]entity.MerchantSummary{merchant1}, 1, nil).
					Once()
			}},
			wants: wants{
				statusCode: http.StatusOK,
				body:       `{"data":[{"id":"m1","name":"Alice","email":"alice@example.com"}],"meta":{"limit":20,"page":1,"total":1}}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			handler := NewMerchantsHandler(tt.fields.service)
			router := gin.New()
			router.GET("/admin/merchants", handler.ListMerchants)

			req := httptest.NewRequest(http.MethodGet, "/admin/merchants"+tt.args.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")
			assert.JSONEq(t, tt.wants.body, rec.Body.String(), "response body")

			if m, ok := tt.fields.service.(*serviceMocks.MockIMerchantsService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
