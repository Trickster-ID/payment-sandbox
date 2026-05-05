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
		setupMocks func(repo *repoMocks.MockIRefundRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:      "reason required",
			userID:    "user-1",
			paymentID: "pi-1",
			reason:    " ",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
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
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found"))
			},
			wantErr: "merchant not found",
		},
		{
			name:      "success",
			userID:    "user-1",
			paymentID: "pi-1",
			reason:    "duplicate payment",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().
					RequestRefund("merchant-1", "pi-1", "duplicate payment").
					Return(refundEntity.Refund{ID: "refund-1"}, nil)
			},
			wantID: "refund-1",
		},
		{
			name:      "repository error",
			userID:    "user-1",
			paymentID: "pi-1",
			reason:    "duplicate payment",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().
					RequestRefund("merchant-1", "pi-1", "duplicate payment").
					Return(refundEntity.Refund{}, errors.New("db error"))
			},
			wantErr: "db error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIRefundRepository(t)
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
		setupMocks func(repo *repoMocks.MockIRefundRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:     "approve review",
			refundID: "refund-1",
			decision: "APPROVE",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().ReviewRefund("refund-1", true).Return(refundEntity.Refund{ID: "refund-1"}, nil)
			},
			wantID: "refund-1",
		},
		{
			name:     "reject review",
			refundID: "refund-1",
			decision: "REJECT",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().ReviewRefund("refund-1", false).Return(refundEntity.Refund{ID: "refund-1"}, nil)
			},
			wantID: "refund-1",
		},
		{
			name:     "invalid decision",
			refundID: "refund-1",
			decision: "WAIT",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.AssertNotCalled(t, "ReviewRefund")
			},
			wantErr: "decision must be APPROVE or REJECT",
		},
		{
			name:     "repository error",
			refundID: "refund-1",
			decision: "APPROVE",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().ReviewRefund("refund-1", true).Return(refundEntity.Refund{}, errors.New("db error"))
			},
			wantErr: "db error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIRefundRepository(t)
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
		setupMocks func(repo *repoMocks.MockIRefundRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:     "invalid status",
			refundID: "refund-1",
			status:   "UNKNOWN",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.AssertNotCalled(t, "ProcessRefund")
			},
			wantErr: "invalid refund status",
		},
		{
			name:     "success status mapping",
			refundID: "refund-1",
			status:   "success",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
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
		{
			name:     "failed status mapping",
			refundID: "refund-1",
			status:   "failed",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().
					ProcessRefund("refund-1", refundEntity.RefundFailed).
					Return(
						refundEntity.Refund{ID: "refund-1", Status: refundEntity.RefundFailed},
						walletEntity.Merchant{},
						nil,
					)
			},
			wantID: "refund-1",
		},
		{
			name:     "repository error",
			refundID: "refund-1",
			status:   "success",
			setupMocks: func(repo *repoMocks.MockIRefundRepository) {
				repo.EXPECT().
					ProcessRefund("refund-1", refundEntity.RefundSuccess).
					Return(refundEntity.Refund{}, walletEntity.Merchant{}, errors.New("db error"))
			},
			wantErr: "db error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIRefundRepository(t)
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
			if tc.status == "success" {
				assert.Equal(t, "merchant-1", merchant.ID)
			}
		})
	}
}

func TestRefundService_ListRefunds(t *testing.T) {
	type fields struct {
		repo *repoMocks.MockIRefundRepository
	}
	type args struct {
		status string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		dataLen int
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. with status filter -> returns filtered list",
			fields: fields{repo: repoMocks.NewMockIRefundRepository(t)},
			args:   args{status: "PENDING"},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().ListRefunds("PENDING").Return([]refundEntity.Refund{{ID: "r1"}}).Once()
				},
			},
			wants: wants{dataLen: 1},
		},
		{
			name:   "2. empty status -> returns all refunds",
			fields: fields{repo: repoMocks.NewMockIRefundRepository(t)},
			args:   args{status: ""},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().ListRefunds("").Return([]refundEntity.Refund{{ID: "r1"}, {ID: "r2"}}).Once()
				},
			},
			wants: wants{dataLen: 2},
		},
		{
			name:   "3. no matching records -> empty slice",
			fields: fields{repo: repoMocks.NewMockIRefundRepository(t)},
			args:   args{status: "PENDING"},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().ListRefunds("PENDING").Return([]refundEntity.Refund{}).Once()
				},
			},
			wants: wants{dataLen: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			service := NewRefundService(tt.fields.repo)
			res := service.ListRefunds(tt.args.status)

			assert.Len(t, res, tt.wants.dataLen)
			tt.fields.repo.AssertExpectations(t)
		})
	}
}

func TestRefundService_MerchantListRefunds(t *testing.T) {
	type fields struct {
		repo *repoMocks.MockIRefundRepository
	}
	type args struct {
		userID string
		status string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		result []refundEntity.Refund
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. merchant not found -> empty slice",
			fields: fields{repo: repoMocks.NewMockIRefundRepository(t)},
			args:   args{userID: "user-1", status: ""},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found")).Once()
				},
			},
			wants: wants{result: []refundEntity.Refund{}},
		},
		{
			name:   "2. valid user, empty status -> returns all refunds",
			fields: fields{repo: repoMocks.NewMockIRefundRepository(t)},
			args:   args{userID: "user-1", status: ""},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil).Once()
					f.repo.EXPECT().ListMerchantRefunds("merchant-1", "").Return([]refundEntity.Refund{{ID: "r-1"}, {ID: "r-2"}}).Once()
				},
			},
			wants: wants{result: []refundEntity.Refund{{ID: "r-1"}, {ID: "r-2"}}},
		},
		{
			name:   "3. status trimmed and uppercased -> normalized status forwarded",
			fields: fields{repo: repoMocks.NewMockIRefundRepository(t)},
			args:   args{userID: "user-1", status: " requested "},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil).Once()
					f.repo.EXPECT().ListMerchantRefunds("merchant-1", "REQUESTED").Return([]refundEntity.Refund{{ID: "r-3"}}).Once()
				},
			},
			wants: wants{result: []refundEntity.Refund{{ID: "r-3"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			service := NewRefundService(tt.fields.repo)
			res := service.MerchantListRefunds(tt.args.userID, tt.args.status)

			assert.Equal(t, tt.wants.result, res)
			tt.fields.repo.AssertExpectations(t)
		})
	}
}
