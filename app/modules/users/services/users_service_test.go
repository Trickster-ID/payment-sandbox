package services

import (
	"errors"
	"testing"

	"payment-sandbox/app/modules/users/models/entity"
	repoMocks "payment-sandbox/app/modules/users/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserService_RegisterMerchant(t *testing.T) {
	tests := []struct {
		name       string
		input      struct{ name, email, password string }
		setupMocks func(repo *repoMocks.MockIUserRepository)
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
			setupMocks: func(repo *repoMocks.MockIUserRepository) {
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
			setupMocks: func(repo *repoMocks.MockIUserRepository) {
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
			setupMocks: func(repo *repoMocks.MockIUserRepository) {
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
			setupMocks: func(repo *repoMocks.MockIUserRepository) {
				repo.EXPECT().
					CreateUser("Merchant", "merchant@example.com", mock.AnythingOfType("string"), entity.RoleMerchant).
					Return(entity.User{}, errors.New("email already exists"))
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
			setupMocks: func(repo *repoMocks.MockIUserRepository) {
				repo.EXPECT().
					CreateUser("Merchant", "merchant@example.com", mock.AnythingOfType("string"), entity.RoleMerchant).
					Return(entity.User{ID: "user-1"}, nil)
			},
			wantID: "user-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIUserRepository(t)
			tc.setupMocks(repo)

			service := NewUserService(repo)
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
