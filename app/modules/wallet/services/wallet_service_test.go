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

func TestWalletService_WalletByUserID(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		setupMocks func(repo *repoMocks.MockIWalletRepository)
		wantErr    bool
	}{
		{
			name:   "success",
			userID: "user-1",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().GetMerchantWallet("user-1").Return(walletEntity.Merchant{ID: "m1"}, nil)
			},
			wantErr: false,
		},
		{
			name:   "not found",
			userID: "user-1",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().GetMerchantWallet("user-1").Return(walletEntity.Merchant{}, errors.New("not found"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIWalletRepository(t)
			tc.setupMocks(repo)
			service := NewWalletService(repo)
			_, err := service.WalletByUserID(tc.userID)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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
		{
			name:   "repository error",
			userID: "user-1",
			amount: 10000,
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().CreateTopup("merchant-1", 10000.0).Return(walletEntity.Topup{}, errors.New("db error"))
			},
			wantErr: "db error",
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
			name:    "success status update success",
			topupID: "topup-1",
			status:  "success",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess).
					Return(walletEntity.Topup{ID: "topup-1", Status: paymentEntity.PaymentSuccess}, nil)
			},
			wantStatus: paymentEntity.PaymentSuccess,
		},
		{
			name:    "success status update failed",
			topupID: "topup-1",
			status:  "failed",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentFailed).
					Return(walletEntity.Topup{ID: "topup-1", Status: paymentEntity.PaymentFailed}, nil)
			},
			wantStatus: paymentEntity.PaymentFailed,
		},
		{
			name:    "repository error",
			topupID: "topup-1",
			status:  "success",
			setupMocks: func(repo *repoMocks.MockIWalletRepository) {
				repo.EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess).
					Return(walletEntity.Topup{}, errors.New("db error"))
			},
			wantErr: "db error",
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
	t.Run("returns list", func(t *testing.T) {
		repo := repoMocks.NewMockIWalletRepository(t)
		service := NewWalletService(repo)

		expected := []walletEntity.Topup{{ID: "topup-1"}, {ID: "topup-2"}}
		repo.EXPECT().ListTopups().Return(expected)

		result := service.ListTopups()
		assert.Equal(t, expected, result)
	})

	t.Run("empty list", func(t *testing.T) {
		repo := repoMocks.NewMockIWalletRepository(t)
		service := NewWalletService(repo)

		repo.EXPECT().ListTopups().Return([]walletEntity.Topup{})

		result := service.ListTopups()
		assert.Empty(t, result)
	})
}
