package services

import (
	"errors"
	"testing"
	"time"

	"payment-sandbox/app/middleware"
	authEntity "payment-sandbox/app/modules/auth/models/entity"
	repoMocks "payment-sandbox/app/modules/auth/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_RegisterMerchant(t *testing.T) {
	jwtService := middleware.JWTService{Secret: "test-secret", Duration: time.Hour}

	tests := []struct {
		name       string
		input      struct{ name, email, password string }
		setupMocks func(repo *repoMocks.MockAuthRepository)
		wantID     string
		wantErr    string
	}{
		{
			name: "name required",
			input: struct{ name, email, password string }{
				name:     " ",
				email:    "merchant@example.com",
				password: "password123",
			},
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.AssertNotCalled(t, "CreateUser")
			},
			wantErr: "name is required",
		},
		{
			name: "email invalid",
			input: struct{ name, email, password string }{
				name:     "Merchant",
				email:    "merchant.example.com",
				password: "password123",
			},
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.AssertNotCalled(t, "CreateUser")
			},
			wantErr: "email is invalid",
		},
		{
			name: "password too short",
			input: struct{ name, email, password string }{
				name:     "Merchant",
				email:    "merchant@example.com",
				password: "short",
			},
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.AssertNotCalled(t, "CreateUser")
			},
			wantErr: "password minimum length is 8",
		},
		{
			name: "repository error",
			input: struct{ name, email, password string }{
				name:     "Merchant",
				email:    "merchant@example.com",
				password: "password123",
			},
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.EXPECT().
					CreateUser("Merchant", "merchant@example.com", mock.AnythingOfType("string"), authEntity.RoleMerchant).
					Return(authEntity.User{}, errors.New("email already exists"))
			},
			wantErr: "email already exists",
		},
		{
			name: "success with normalized input",
			input: struct{ name, email, password string }{
				name:     " Merchant ",
				email:    "Merchant@Example.COM ",
				password: "password123",
			},
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.EXPECT().
					CreateUser("Merchant", "merchant@example.com", mock.AnythingOfType("string"), authEntity.RoleMerchant).
					Return(authEntity.User{ID: "user-1"}, nil)
			},
			wantID: "user-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockAuthRepository(t)
			tc.setupMocks(repo)

			service := NewAuthService(repo, jwtService)
			user, err := service.RegisterMerchant(tc.input.name, tc.input.email, tc.input.password)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, user.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, user.ID)
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	jwtService := middleware.JWTService{Secret: "test-secret", Duration: time.Hour}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	require.NoError(t, err)

	tests := []struct {
		name       string
		email      string
		password   string
		setupMocks func(repo *repoMocks.MockAuthRepository)
		wantUserID string
		wantToken  bool
		wantErr    string
	}{
		{
			name:     "user not found",
			email:    "merchant@example.com",
			password: "password123",
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.EXPECT().FindUserByEmail("merchant@example.com").Return(authEntity.User{}, false)
			},
			wantErr: "invalid credentials",
		},
		{
			name:     "invalid password",
			email:    "merchant@example.com",
			password: "wrong-password",
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.EXPECT().
					FindUserByEmail("merchant@example.com").
					Return(authEntity.User{ID: "user-1", Role: authEntity.RoleMerchant, PasswordHash: string(passwordHash)}, true)
			},
			wantErr: "invalid credentials",
		},
		{
			name:     "success",
			email:    " Merchant@Example.COM ",
			password: "password123",
			setupMocks: func(repo *repoMocks.MockAuthRepository) {
				repo.EXPECT().
					FindUserByEmail("merchant@example.com").
					Return(authEntity.User{ID: "user-1", Role: authEntity.RoleMerchant, PasswordHash: string(passwordHash)}, true)
			},
			wantUserID: "user-1",
			wantToken:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockAuthRepository(t)
			tc.setupMocks(repo)

			service := NewAuthService(repo, jwtService)
			token, user, loginErr := service.Login(tc.email, tc.password)

			if tc.wantErr != "" {
				require.Error(t, loginErr)
				assert.ErrorContains(t, loginErr, tc.wantErr)
				assert.Empty(t, token)
				assert.Empty(t, user.ID)
				return
			}

			require.NoError(t, loginErr)
			assert.Equal(t, tc.wantUserID, user.ID)
			if tc.wantToken {
				assert.NotEmpty(t, token)
			}
		})
	}
}
