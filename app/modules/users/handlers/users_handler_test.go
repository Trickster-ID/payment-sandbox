package handlers

// Branch map for RegisterMerchant (Section 3.1 of the plan):
// ├── c.ShouldBindJSON fails (malformed JSON)          -> 400 validation_error
// ├── c.ShouldBindJSON fails (binding validation)      -> 400 validation_error
// ├── service.RegisterMerchant fails                   -> 400 validation_error
// └── service.RegisterMerchant succeeds                -> 201 data.id

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	userEntity "payment-sandbox/app/modules/users/models/entity"
	userServices "payment-sandbox/app/modules/users/services"
	serviceMocks "payment-sandbox/app/modules/users/services/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_RegisterMerchant(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type fields struct {
		service userServices.IUserService
	}
	type args struct {
		body string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		statusCode int
		errorCode  string
		userID     string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. malformed JSON body -> validation_error bad request",
			fields: fields{service: serviceMocks.NewMockIUserService(t)},
			args:   args{body: `{invalid-json}`},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest, errorCode: "validation_error"},
		},
		{
			name:   "2. binding validation fails (invalid email, short password) -> validation_error bad request",
			fields: fields{service: serviceMocks.NewMockIUserService(t)},
			args:   args{body: `{"name":"","email":"invalid","password":"123"}`},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{statusCode: http.StatusBadRequest, errorCode: "validation_error"},
		},
		{
			name:   "3. service.RegisterMerchant fails -> validation_error bad request",
			fields: fields{service: serviceMocks.NewMockIUserService(t)},
			args:   args{body: `{"name":"Merchant","email":"merchant@example.com","password":"password123"}`},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIUserService).EXPECT().
					RegisterMerchant("Merchant", "merchant@example.com", "password123").
					Return(userEntity.User{}, errors.New("email already registered")).
					Once()
			}},
			wants: wants{statusCode: http.StatusBadRequest, errorCode: "validation_error"},
		},
		{
			name:   "4. valid request, service succeeds -> 201 with user ID in data",
			fields: fields{service: serviceMocks.NewMockIUserService(t)},
			args:   args{body: `{"name":"Merchant","email":"merchant@example.com","password":"password123"}`},
			mocks: mocks{setup: func(f fields, a args) {
				f.service.(*serviceMocks.MockIUserService).EXPECT().
					RegisterMerchant("Merchant", "merchant@example.com", "password123").
					Return(userEntity.User{
						ID:    "user-1",
						Name:  "Merchant",
						Email: "merchant@example.com",
						Role:  userEntity.RoleMerchant,
					}, nil).
					Once()
			}},
			wants: wants{statusCode: http.StatusCreated, userID: "user-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: gin.SetMode is global
			tt.mocks.setup(tt.fields, tt.args)

			handler := NewUserHandler(tt.fields.service)
			router := gin.New()
			router.POST("/users/register", handler.RegisterMerchant)

			req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString(tt.args.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wants.statusCode, rec.Code, "status code")

			var payload map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), "response JSON parseable")

			if tt.wants.errorCode != "" {
				errData, ok := payload["error"].(map[string]any)
				require.True(t, ok, "error envelope present")
				assert.Equal(t, tt.wants.errorCode, errData["code"], "error code")
			} else {
				data, ok := payload["data"].(map[string]any)
				require.True(t, ok, "data envelope present")
				assert.Equal(t, tt.wants.userID, data["id"], "user ID")
			}

			if m, ok := tt.fields.service.(*serviceMocks.MockIUserService); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
