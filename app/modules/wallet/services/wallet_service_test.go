package services

// Branch analysis for WalletService:
//
// WalletByUserID:
// └── s.repo.GetMerchantWallet → propagates result/error
//
// CreateTopup:
// ├── s.repo.MerchantIDByUserID → error → return empty Topup, err
// └── s.repo.CreateTopup(merchantID, amount) → propagates result/error
//
// ListTopups:
// └── s.repo.ListTopups() → propagates result
//
// ListMerchantTopups:
// ├── s.repo.MerchantIDByUserID → error → return nil, 0, err
// └── s.repo.ListMerchantTopups(merchantID, page, limit) → return topups, total, nil
//
// UpdateTopupStatus:
// ├── ParsePaymentStatus("PENDING") → error → return empty Topup, err  [invalid status branch]
// └── s.repo.UpdateTopupStatus(topupID, parsed) → propagates result/error
//
// ListWalletTransactions:
// ├── s.repo.MerchantIDByUserID → error → return nil, 0, err
// └── s.repo.ListTransactions(merchantID, filter, page, limit) → propagates result/error
//
// ListWalletTransactionsByMerchant:
// └── s.repo.ListTransactions(merchantID, filter, page, limit) → propagates result/error

import (
	"errors"
	"testing"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	"payment-sandbox/app/modules/wallet/repositories"
	repoMocks "payment-sandbox/app/modules/wallet/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─── WalletByUserID ───────────────────────────────────────────────────────────

func TestWalletService_WalletByUserID(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type args struct {
		userID string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		merchant walletEntity.Merchant
		errMsg   string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo returns merchant -> success",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					GetMerchantWallet(a.userID).
					Return(walletEntity.Merchant{ID: "merchant-1"}, nil).
					Once()
			}},
			wants: wants{merchant: walletEntity.Merchant{ID: "merchant-1"}},
		},
		{
			name:   "2. repo returns error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-unknown"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					GetMerchantWallet(a.userID).
					Return(walletEntity.Merchant{}, errors.New("merchant wallet not found")).
					Once()
			}},
			wants: wants{errMsg: "merchant wallet not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewWalletService(tt.fields.repo)
			got, err := svc.WalletByUserID(tt.args.userID)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.merchant, got, "merchant")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── CreateTopup ─────────────────────────────────────────────────────────────

func TestWalletService_CreateTopup(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type args struct {
		userID string
		amount int64
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		topupID string
		errMsg  string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. MerchantIDByUserID fails -> error propagated no CreateTopup call",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", amount: 10000},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("", errors.New("merchant not found")).
					Once()
			}},
			wants: wants{errMsg: "merchant not found"},
		},
		{
			name:   "2. CreateTopup repo fails -> error propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", amount: 50000},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					CreateTopup("merchant-1", a.amount).
					Return(walletEntity.Topup{}, errors.New("db error")).
					Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "3. all steps succeed -> topup returned",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", amount: 25000},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					CreateTopup("merchant-1", a.amount).
					Return(walletEntity.Topup{ID: "topup-1", Amount: a.amount}, nil).
					Once()
			}},
			wants: wants{topupID: "topup-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewWalletService(tt.fields.repo)
			got, err := svc.CreateTopup(tt.args.userID, tt.args.amount)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, got.ID, "topup ID must be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.topupID, got.ID, "topup ID")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListTopups ──────────────────────────────────────────────────────────────

func TestWalletService_ListTopups(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type mocks struct {
		setup func(f fields, _ struct{})
	}
	type wants struct {
		topups []walletEntity.Topup
	}

	tests := []struct {
		name   string
		fields fields
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo returns items -> items propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			mocks: mocks{setup: func(f fields, _ struct{}) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListTopups().
					Return([]walletEntity.Topup{{ID: "t1"}, {ID: "t2"}}).
					Once()
			}},
			wants: wants{topups: []walletEntity.Topup{{ID: "t1"}, {ID: "t2"}}},
		},
		{
			name:   "2. repo returns empty slice -> empty slice propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			mocks: mocks{setup: func(f fields, _ struct{}) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListTopups().
					Return([]walletEntity.Topup{}).
					Once()
			}},
			wants: wants{topups: []walletEntity.Topup{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, struct{}{})
			}

			svc := NewWalletService(tt.fields.repo)
			got := svc.ListTopups()

			assert.Equal(t, tt.wants.topups, got, "topups")

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListMerchantTopups ───────────────────────────────────────────────────────

func TestWalletService_ListMerchantTopups(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type args struct {
		userID string
		page   int
		limit  int
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		topups []walletEntity.Topup
		total  int
		errMsg string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. MerchantIDByUserID fails -> error propagated nil topups",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-unknown", page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("", errors.New("merchant not found")).
					Once()
			}},
			wants: wants{errMsg: "merchant not found"},
		},
		{
			name:   "2. success with results -> topups and total returned",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListMerchantTopups("merchant-1", a.page, a.limit).
					Return([]walletEntity.Topup{{ID: "t1"}}, 1).
					Once()
			}},
			wants: wants{topups: []walletEntity.Topup{{ID: "t1"}}, total: 1},
		},
		{
			name:   "3. success with no results -> empty slice zero total",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", page: 2, limit: 5},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListMerchantTopups("merchant-1", a.page, a.limit).
					Return([]walletEntity.Topup{}, 0).
					Once()
			}},
			wants: wants{topups: []walletEntity.Topup{}, total: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewWalletService(tt.fields.repo)
			topups, total, err := svc.ListMerchantTopups(tt.args.userID, tt.args.page, tt.args.limit)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Nil(t, topups, "topups must be nil on error")
				assert.Zero(t, total, "total must be zero on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.topups, topups, "topups")
				assert.Equal(t, tt.wants.total, total, "total")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── UpdateTopupStatus ────────────────────────────────────────────────────────

func TestWalletService_UpdateTopupStatus(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type args struct {
		topupID string
		status  string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		topupStatus paymentEntity.PaymentStatus
		errMsg      string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. invalid status (PENDING) -> parse error repo not called",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{topupID: "topup-1", status: "PENDING"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "invalid payment status: PENDING"},
		},
		{
			name:   "2. status=success repo.UpdateTopupStatus returns SUCCESS topup -> success",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{topupID: "topup-1", status: "success"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess).
					Return(walletEntity.Topup{ID: "topup-1", Status: paymentEntity.PaymentSuccess}, nil).
					Once()
			}},
			wants: wants{topupStatus: paymentEntity.PaymentSuccess},
		},
		{
			name:   "3. status=failed repo.UpdateTopupStatus returns FAILED topup -> success",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{topupID: "topup-1", status: "FAILED"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentFailed).
					Return(walletEntity.Topup{ID: "topup-1", Status: paymentEntity.PaymentFailed}, nil).
					Once()
			}},
			wants: wants{topupStatus: paymentEntity.PaymentFailed},
		},
		{
			name:   "4. valid status but repo returns error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{topupID: "topup-1", status: "SUCCESS"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess).
					Return(walletEntity.Topup{}, errors.New("topup already finalized")).
					Once()
			}},
			wants: wants{errMsg: "topup already finalized"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewWalletService(tt.fields.repo)
			got, err := svc.UpdateTopupStatus(tt.args.topupID, tt.args.status)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, got.ID, "topup ID must be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.topupStatus, got.Status, "topup status")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListWalletTransactions ───────────────────────────────────────────────────

func TestWalletService_ListWalletTransactions(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type args struct {
		userID string
		filter ledgerEntity.EntryFilter
		page   int
		limit  int
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		entries []ledgerEntity.EntryWithTxn
		total   int
		errMsg  string
	}

	entry := ledgerEntity.EntryWithTxn{ID: 1, Reference: "topup:abc"}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. MerchantIDByUserID fails -> error propagated nil entries",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-unknown", filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("", errors.New("merchant not found")).
					Once()
			}},
			wants: wants{errMsg: "merchant not found"},
		},
		{
			name:   "2. ListTransactions repo fails -> error propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListTransactions("merchant-1", a.filter, a.page, a.limit).
					Return(nil, 0, errors.New("db error")).
					Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "3. all steps succeed -> entries and total returned",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{userID: "user-1", filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListTransactions("merchant-1", a.filter, a.page, a.limit).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			}},
			wants: wants{entries: []ledgerEntity.EntryWithTxn{entry}, total: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewWalletService(tt.fields.repo)
			entries, total, err := svc.ListWalletTransactions(tt.args.userID, tt.args.filter, tt.args.page, tt.args.limit)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Nil(t, entries, "entries must be nil on error")
				assert.Zero(t, total, "total must be zero on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.entries, entries, "entries")
				assert.Equal(t, tt.wants.total, total, "total")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// ─── ListWalletTransactionsByMerchant ─────────────────────────────────────────

func TestWalletService_ListWalletTransactionsByMerchant(t *testing.T) {
	type fields struct {
		repo repositories.IWalletRepository
	}
	type args struct {
		merchantID string
		filter     ledgerEntity.EntryFilter
		page       int
		limit      int
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		entries []ledgerEntity.EntryWithTxn
		total   int
		errMsg  string
	}

	entry := ledgerEntity.EntryWithTxn{ID: 2, Reference: "refund:xyz"}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. repo returns error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{merchantID: "merchant-1", filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListTransactions(a.merchantID, a.filter, a.page, a.limit).
					Return(nil, 0, errors.New("db error")).
					Once()
			}},
			wants: wants{errMsg: "db error"},
		},
		{
			name:   "2. repo returns entries -> entries and total propagated",
			fields: fields{repo: repoMocks.NewMockIWalletRepository(t)},
			args:   args{merchantID: "merchant-1", filter: ledgerEntity.EntryFilter{}, page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIWalletRepository).EXPECT().
					ListTransactions(a.merchantID, a.filter, a.page, a.limit).
					Return([]ledgerEntity.EntryWithTxn{entry}, 1, nil).
					Once()
			}},
			wants: wants{entries: []ledgerEntity.EntryWithTxn{entry}, total: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewWalletService(tt.fields.repo)
			entries, total, err := svc.ListWalletTransactionsByMerchant(tt.args.merchantID, tt.args.filter, tt.args.page, tt.args.limit)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Nil(t, entries, "entries must be nil on error")
				assert.Zero(t, total, "total must be zero on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.entries, entries, "entries")
				assert.Equal(t, tt.wants.total, total, "total")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIWalletRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

// satisfy import so unused mock import doesn't fail compilation
var _ = mock.Anything
