package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	authEntity "payment-sandbox/app/modules/auth/models/entity"
	serviceMocks "payment-sandbox/app/modules/auth/services/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthHandler_RegisterMerchant(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockIAuthService)
		wantStatus int
		wantCode   string
		wantID     string
	}{
		{
			name: "validation error",
			body: `{"name":"","email":"invalid","password":"123"}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.AssertNotCalled(t, "RegisterMerchant")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "malformed json",
			body: `{invalid-json}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.AssertNotCalled(t, "RegisterMerchant")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "service validation error",
			body: `{"name":"Merchant","email":"merchant@example.com","password":"password123"}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.EXPECT().
					RegisterMerchant("Merchant", "merchant@example.com", "password123").
					Return(authEntity.User{}, errors.New("email already exists"))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "success",
			body: `{"name":"Merchant","email":"merchant@example.com","password":"password123"}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.EXPECT().
					RegisterMerchant("Merchant", "merchant@example.com", "password123").
					Return(authEntity.User{
						ID:    "user-1",
						Name:  "Merchant",
						Email: "merchant@example.com",
						Role:  authEntity.RoleMerchant,
					}, nil)
			},
			wantStatus: http.StatusCreated,
			wantID:     "user-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIAuthService(t)
			tc.setupMocks(service)

			handler := NewAuthHandler(service)
			router := gin.New()
			router.POST("/auth/register", handler.RegisterMerchant)

			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(tc.body))
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

func TestAuthHandler_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		setupMocks func(service *serviceMocks.MockIAuthService)
		wantStatus int
		wantCode   string
		wantToken  string
	}{
		{
			name: "validation error",
			body: `{"email":"invalid","password":""}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.AssertNotCalled(t, "Login")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "malformed json",
			body: `{invalid-json}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.AssertNotCalled(t, "Login")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name: "invalid credentials",
			body: `{"email":"merchant@example.com","password":"wrong-password"}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.EXPECT().
					Login("merchant@example.com", "wrong-password").
					Return("", authEntity.User{}, errors.New("invalid credentials"))
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "auth_invalid_credentials",
		},
		{
			name: "success",
			body: `{"email":"merchant@example.com","password":"password123"}`,
			setupMocks: func(service *serviceMocks.MockIAuthService) {
				service.EXPECT().
					Login("merchant@example.com", "password123").
					Return("token-1", authEntity.User{
						ID:    "user-1",
						Name:  "Merchant",
						Email: "merchant@example.com",
						Role:  authEntity.RoleMerchant,
					}, nil)
			},
			wantStatus: http.StatusOK,
			wantToken:  "token-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := serviceMocks.NewMockIAuthService(t)
			tc.setupMocks(service)

			handler := NewAuthHandler(service)
			router := gin.New()
			router.POST("/auth/login", handler.Login)

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(tc.body))
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
			assert.Equal(t, tc.wantToken, data["access_token"])
		})
	}
}
