package services

import (
	"errors"
	"strings"
	"time"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	"payment-sandbox/app/modules/admin/repositories"
)

type AdminService struct {
	repo repositories.IAdminRepository
}

type IAdminService interface {
	Stats(merchantID, startDate, endDate string) (adminEntity.DashboardStats, error)
}

func NewAdminService(repo repositories.IAdminRepository) *AdminService {
	return &AdminService{repo: repo}
}

func (s *AdminService) Stats(merchantID, startDate, endDate string) (adminEntity.DashboardStats, error) {
	filter := adminEntity.StatsFilter{
		MerchantID: strings.TrimSpace(merchantID),
	}

	if strings.TrimSpace(startDate) != "" {
		parsed, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return adminEntity.DashboardStats{}, errors.New("start_date must be YYYY-MM-DD")
		}
		filter.StartDate = &parsed
	}
	if strings.TrimSpace(endDate) != "" {
		parsed, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return adminEntity.DashboardStats{}, errors.New("end_date must be YYYY-MM-DD")
		}
		end := parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		filter.EndDate = &end
	}

	return s.repo.DashboardStats(filter), nil
}
