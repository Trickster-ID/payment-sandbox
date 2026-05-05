package services

// Branch map for RegisterMerchant (Section 3.1 of the plan):
// ├── name is blank after trim                  -> "name is required"
// ├── email fails IsEmail validation             -> "email is invalid"
// ├── password length < 8                       -> "password minimum length is 8"
// ├── bcrypt.GenerateFromPassword fails          -> propagate error
// │   NOTE: unreachable in practice — bcrypt.DefaultCost (10) is always valid;
// │         testing this branch would require injecting a cost that is out of range [4,31],
// │         which is not possible without changing the function signature.
// ├── repo.CreateUser fails                     -> propagate error
// └── all validations pass, repo succeeds       -> return User, nil (input is normalized)

import (
	"errors"
	"testing"

	"payment-sandbox/app/modules/users/models/entity"
	"payment-sandbox/app/modules/users/repositories"
	repoMocks "payment-sandbox/app/modules/users/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserService_RegisterMerchant(t *testing.T) {
	type fields struct {
		repo repositories.IUserRepository
	}
	type args struct {
		name, email, password string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		userID string
		err    string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. name blank after trim -> name is required",
			fields: fields{repo: repoMocks.NewMockIUserRepository(t)},
			args:   args{name: "  ", email: "merchant@example.com", password: "password123"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{err: "name is required"},
		},
		{
			name:   "2. email fails validation -> email is invalid",
			fields: fields{repo: repoMocks.NewMockIUserRepository(t)},
			args:   args{name: "Merchant", email: "merchant.example.com", password: "password123"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{err: "email is invalid"},
		},
		{
			name:   "3. password shorter than 8 chars -> password minimum length is 8",
			fields: fields{repo: repoMocks.NewMockIUserRepository(t)},
			args:   args{name: "Merchant", email: "merchant@example.com", password: "short"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{err: "password minimum length is 8"},
		},
		{
			name:   "4. repo.CreateUser fails -> error propagated",
			fields: fields{repo: repoMocks.NewMockIUserRepository(t)},
			args:   args{name: "Merchant", email: "merchant@example.com", password: "password123"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIUserRepository).EXPECT().
					CreateUser("Merchant", "merchant@example.com", mock.AnythingOfType("string"), entity.RoleMerchant).
					Return(entity.User{}, errors.New("email already registered")).
					Once()
			}},
			wants: wants{err: "email already registered"},
		},
		{
			name:   "5. valid request with whitespace and mixed-case email -> user returned, input normalized",
			fields: fields{repo: repoMocks.NewMockIUserRepository(t)},
			args:   args{name: " Merchant ", email: "Merchant@Example.COM ", password: "password123"},
			mocks: mocks{setup: func(f fields, a args) {
				// verifies trim+lowercase normalization is applied before the repo call
				f.repo.(*repoMocks.MockIUserRepository).EXPECT().
					CreateUser("Merchant", "merchant@example.com", mock.AnythingOfType("string"), entity.RoleMerchant).
					Return(entity.User{ID: "user-1", Name: "Merchant", Email: "merchant@example.com", Role: entity.RoleMerchant}, nil).
					Once()
			}},
			wants: wants{userID: "user-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewUserService(tt.fields.repo)
			user, err := svc.RegisterMerchant(tt.args.name, tt.args.email, tt.args.password)

			if tt.wants.err != "" {
				require.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, user.ID, "user ID should be empty on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.userID, user.ID, "user ID")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIUserRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
