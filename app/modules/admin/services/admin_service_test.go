package services

import (
	"testing"
	"time"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	repoMocks "payment-sandbox/app/modules/admin/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminService_Stats(t *testing.T) {
	tests := []struct {
		name       string
		merchantID string
		startDate  string
		endDate    string
		setupMocks func(repo *repoMocks.MockIAdminRepository)
		wantTotal  float64
		wantErr    string
	}{
		{
			name:       "invalid start date format",
			merchantID: "merchant-1",
			startDate:  "2026/04/01",
			endDate:    "2026-04-30",
			setupMocks: func(repo *repoMocks.MockIAdminRepository) {
				repo.AssertNotCalled(t, "DashboardStats")
			},
			wantErr: "start_date must be YYYY-MM-DD",
		},
		{
			name:       "invalid end date format",
			merchantID: "merchant-1",
			startDate:  "2026-04-01",
			endDate:    "2026/04/30",
			setupMocks: func(repo *repoMocks.MockIAdminRepository) {
				repo.AssertNotCalled(t, "DashboardStats")
			},
			wantErr: "end_date must be YYYY-MM-DD",
		},
		{
			name:       "success with trimmed merchant and date window",
			merchantID: "  merchant-1  ",
			startDate:  "2026-04-01",
			endDate:    "2026-04-30",
			setupMocks: func(repo *repoMocks.MockIAdminRepository) {
				start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC)
				repo.EXPECT().DashboardStats(adminEntity.StatsFilter{
					MerchantID: "merchant-1",
					StartDate:  &start,
					EndDate:    &end,
				}).Return(adminEntity.DashboardStats{
					TotalPaymentNominal: 5000,
				})
			},
			wantTotal: 5000,
		},
		{
			name:       "success without date filter",
			merchantID: "",
			startDate:  "",
			endDate:    "",
			setupMocks: func(repo *repoMocks.MockIAdminRepository) {
				repo.EXPECT().DashboardStats(adminEntity.StatsFilter{
					MerchantID: "",
					StartDate:  nil,
					EndDate:    nil,
				}).Return(adminEntity.DashboardStats{
					TotalPaymentNominal: 9000,
				})
			},
			wantTotal: 9000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIAdminRepository(t)
			tc.setupMocks(repo)
			service := NewAdminService(repo)

			result, err := service.Stats(tc.merchantID, tc.startDate, tc.endDate)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Zero(t, result.TotalPaymentNominal)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantTotal, result.TotalPaymentNominal)
		})
	}
}
