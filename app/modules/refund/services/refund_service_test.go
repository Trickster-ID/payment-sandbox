package services

import (
	"errors"
	"testing"

	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	repoMocks "payment-sandbox/app/modules/refund/repositories/mocks"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefundService_RequestRefund(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		paymentID  string
		reason     string
		setupMocks func(repo *repoMocks.MockRefundRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:      "reason required",
			userID:    "user-1",
			paymentID: "pi-1",
			reason:    " ",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.AssertNotCalled(t, "MerchantIDByUserID")
				repo.AssertNotCalled(t, "RequestRefund")
			},
			wantErr: "reason is required",
		},
		{
			name:      "merchant lookup failed",
			userID:    "user-1",
			paymentID: "pi-1",
			reason:    "duplicate payment",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found"))
			},
			wantErr: "merchant not found",
		},
		{
			name:      "success",
			userID:    "user-1",
			paymentID: "pi-1",
			reason:    "duplicate payment",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().
					RequestRefund("merchant-1", "pi-1", "duplicate payment").
					Return(refundEntity.Refund{ID: "refund-1"}, nil)
			},
			wantID: "refund-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockRefundRepository(t)
			tc.setupMocks(repo)
			service := NewRefundService(repo)

			result, err := service.RequestRefund(tc.userID, tc.paymentID, tc.reason)

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

func TestRefundService_ReviewRefund(t *testing.T) {
	tests := []struct {
		name       string
		refundID   string
		decision   string
		setupMocks func(repo *repoMocks.MockRefundRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:     "approve review",
			refundID: "refund-1",
			decision: "APPROVE",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.EXPECT().ReviewRefund("refund-1", true).Return(refundEntity.Refund{ID: "refund-1"}, nil)
			},
			wantID: "refund-1",
		},
		{
			name:     "reject review",
			refundID: "refund-1",
			decision: "REJECT",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.EXPECT().ReviewRefund("refund-1", false).Return(refundEntity.Refund{ID: "refund-1"}, nil)
			},
			wantID: "refund-1",
		},
		{
			name:     "invalid decision",
			refundID: "refund-1",
			decision: "WAIT",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.AssertNotCalled(t, "ReviewRefund")
			},
			wantErr: "decision must be APPROVE or REJECT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockRefundRepository(t)
			tc.setupMocks(repo)
			service := NewRefundService(repo)

			result, err := service.ReviewRefund(tc.refundID, tc.decision)

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

func TestRefundService_ProcessRefund(t *testing.T) {
	tests := []struct {
		name       string
		refundID   string
		status     string
		setupMocks func(repo *repoMocks.MockRefundRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:     "invalid status",
			refundID: "refund-1",
			status:   "UNKNOWN",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.AssertNotCalled(t, "ProcessRefund")
			},
			wantErr: "invalid refund status",
		},
		{
			name:     "success status mapping",
			refundID: "refund-1",
			status:   "success",
			setupMocks: func(repo *repoMocks.MockRefundRepository) {
				repo.EXPECT().
					ProcessRefund("refund-1", refundEntity.RefundSuccess).
					Return(
						refundEntity.Refund{ID: "refund-1", Status: refundEntity.RefundSuccess},
						walletEntity.Merchant{ID: "merchant-1"},
						nil,
					)
			},
			wantID: "refund-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockRefundRepository(t)
			tc.setupMocks(repo)
			service := NewRefundService(repo)

			result, merchant, err := service.ProcessRefund(tc.refundID, tc.status)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, result.ID)
				assert.Empty(t, merchant.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, result.ID)
			assert.Equal(t, "merchant-1", merchant.ID)
		})
	}
}
