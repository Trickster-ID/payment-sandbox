package services

// Branch analysis for AdminService.Stats(merchantID, startDate, endDate):
// ├── TrimSpace(startDate) != "" && time.Parse fails  → error "start_date must be YYYY-MM-DD"  [1]
// ├── TrimSpace(endDate)   != "" && time.Parse fails  → error "end_date must be YYYY-MM-DD"    [2]
// ├── TrimSpace(startDate) == "" (empty or whitespace) → filter.StartDate = nil               [3,4,7]
// ├── TrimSpace(endDate)   == "" (empty or whitespace) → filter.EndDate   = nil               [3,5,6]
// ├── startDate valid only → filter.StartDate set, filter.EndDate nil                         [6]
// ├── endDate valid only   → filter.EndDate = end-of-day, filter.StartDate nil                [7]
// ├── both valid dates + merchantID trimmed → all three filter fields set                     [8]
// └── all empty → repo called with zero filter                                                [3]

import (
	"testing"
	"time"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	"payment-sandbox/app/modules/admin/repositories"
	repoMocks "payment-sandbox/app/modules/admin/repositories/mocks"

	"github.com/stretchr/testify/assert"
)

func TestAdminService_Stats(t *testing.T) {
	type fields struct {
		repo repositories.IAdminRepository
	}
	type args struct {
		merchantID string
		startDate  string
		endDate    string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		errMsg string // empty → no error expected
		stats  adminEntity.DashboardStats
	}

	start0401 := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end0430   := time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC)
	start0401Only := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end0430Only   := time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC)

	fullStats := adminEntity.DashboardStats{
		TotalInvoiceCreated: 20,
		TotalByStatus:       map[string]int{"PAID": 15, "EXPIRED": 3},
		TotalPaymentNominal: 9_500,
		TotalRefundNominal:  200,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. invalid startDate format -> error start_date must be YYYY-MM-DD, repo not called",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "m-1", startDate: "2026/04/01", endDate: "2026-04-30"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "start_date must be YYYY-MM-DD"},
		},
		{
			name:   "2. invalid endDate format -> error end_date must be YYYY-MM-DD, repo not called",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "m-1", startDate: "2026-04-01", endDate: "2026/04/30"},
			mocks:  mocks{setup: func(f fields, a args) {}},
			wants:  wants{errMsg: "end_date must be YYYY-MM-DD"},
		},
		{
			name:   "3. all empty strings -> repo called with zero filter, stats returned",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "", startDate: "", endDate: ""},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIAdminRepository).EXPECT().
					DashboardStats(adminEntity.StatsFilter{MerchantID: "", StartDate: nil, EndDate: nil}).
					Return(fullStats).
					Once()
			}},
			wants: wants{stats: fullStats},
		},
		{
			name:   "4. whitespace-only startDate -> treated as empty, repo called without StartDate",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "", startDate: "   ", endDate: ""},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIAdminRepository).EXPECT().
					DashboardStats(adminEntity.StatsFilter{MerchantID: "", StartDate: nil, EndDate: nil}).
					Return(fullStats).
					Once()
			}},
			wants: wants{stats: fullStats},
		},
		{
			name:   "5. whitespace-only endDate -> treated as empty, repo called without EndDate",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "", startDate: "", endDate: "   "},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIAdminRepository).EXPECT().
					DashboardStats(adminEntity.StatsFilter{MerchantID: "", StartDate: nil, EndDate: nil}).
					Return(fullStats).
					Once()
			}},
			wants: wants{stats: fullStats},
		},
		{
			name:   "6. valid startDate only, no endDate -> repo called with StartDate set, EndDate nil",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "", startDate: "2026-04-01", endDate: ""},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIAdminRepository).EXPECT().
					DashboardStats(adminEntity.StatsFilter{MerchantID: "", StartDate: &start0401Only, EndDate: nil}).
					Return(fullStats).
					Once()
			}},
			wants: wants{stats: fullStats},
		},
		{
			name:   "7. valid endDate only, no startDate -> repo called with EndDate set to end-of-day, StartDate nil",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "", startDate: "", endDate: "2026-04-30"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIAdminRepository).EXPECT().
					DashboardStats(adminEntity.StatsFilter{MerchantID: "", StartDate: nil, EndDate: &end0430Only}).
					Return(fullStats).
					Once()
			}},
			wants: wants{stats: fullStats},
		},
		{
			name:   "8. both valid dates + whitespace-padded merchantID -> all three filter fields set, merchantID trimmed",
			fields: fields{repo: repoMocks.NewMockIAdminRepository(t)},
			args:   args{merchantID: "  merchant-1  ", startDate: "2026-04-01", endDate: "2026-04-30"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIAdminRepository).EXPECT().
					DashboardStats(adminEntity.StatsFilter{
						MerchantID: "merchant-1",
						StartDate:  &start0401,
						EndDate:    &end0430,
					}).
					Return(fullStats).
					Once()
			}},
			wants: wants{stats: fullStats},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewAdminService(tt.fields.repo)
			gotStats, err := svc.Stats(tt.args.merchantID, tt.args.startDate, tt.args.endDate)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Zero(t, gotStats.TotalInvoiceCreated, "TotalInvoiceCreated must be zero on error")
				assert.Zero(t, gotStats.TotalPaymentNominal, "TotalPaymentNominal must be zero on error")
				assert.Zero(t, gotStats.TotalRefundNominal, "TotalRefundNominal must be zero on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.stats.TotalInvoiceCreated, gotStats.TotalInvoiceCreated, "TotalInvoiceCreated")
				assert.Equal(t, tt.wants.stats.TotalByStatus, gotStats.TotalByStatus, "TotalByStatus")
				assert.Equal(t, tt.wants.stats.TotalPaymentNominal, gotStats.TotalPaymentNominal, "TotalPaymentNominal")
				assert.Equal(t, tt.wants.stats.TotalRefundNominal, gotStats.TotalRefundNominal, "TotalRefundNominal")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIAdminRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}

	_ = start0401
	_ = end0430
}
