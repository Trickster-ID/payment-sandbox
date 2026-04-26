package services

import (
	"errors"
	"testing"

	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	repoMocks "payment-sandbox/app/modules/wallet/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletService_CreateTopup(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		amount     float64
		setupMocks func(repo *repoMocks.MockIWalletRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:   "merchant lookup failed",
			userID: "user-1",
			amount: 10000,
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found"))
			},
			wantErr: "merchant not found",
		},
		{
			name:   "success",
			userID: "user-1",
			amount: 10000,
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().CreateTopup("merchant-1", 10000.0).Return(walletEntity.Topup{ID: "topup-1"}, nil)
			},
			wantID: "topup-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIWalletRepository(t)
			tc.setupMocks(repo)
			service := NewWalletService(repo)

			result, err := service.CreateTopup(tc.userID, tc.amount)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, result.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, result.ID)
		})
	}
}

func TestWalletService_UpdateTopupStatus(t *testing.T) {
	tests := []struct {
		name       string
		topupID    string
		status     string
		setupMocks func(repo *repoMocks.MockIWalletRepository)
		wantStatus paymentEntity.PaymentStatus
		wantErr    string
	}{
		{
			name:    "invalid status",
			topupID: "topup-1",
			status:  "PENDING",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.AssertNotCalled(t, "UpdateTopupStatus")
			},
			wantErr: "invalid payment status",
		},
		{
			name:    "success status update",
			topupID: "topup-1",
			status:  "success",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess).
					Return(walletEntity.Topup{ID: "topup-1", Status: paymentEntity.PaymentSuccess}, nil)
			},
			wantStatus: paymentEntity.PaymentSuccess,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIWalletRepository(t)
			tc.setupMocks(repo)
			service := NewWalletService(repo)

			result, err := service.UpdateTopupStatus(tc.topupID, tc.status)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, result.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, result.Status)
		})
	}
}

func TestWalletService_ListTopups(t *testing.T) {
	repo := repoMocks.NewMockIWalletRepository(t)
	service := NewWalletService(repo)

	expected := []walletEntity.Topup{{ID: "topup-1"}, {ID: "topup-2"}}
	repo.EXPECT().ListTopups().Return(expected)

	result := service.ListTopups()
	assert.Equal(t, expected, result)
}
