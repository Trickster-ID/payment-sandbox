package services

import (
	"context"
	"strings"

	"payment-sandbox/app/modules/merchants/models/entity"
	"payment-sandbox/app/modules/merchants/repositories"
)

//go:generate mockery --name IMerchantsService --output mocks --outpkg mocks --case underscore

type IMerchantsService interface {
	ListMerchants(ctx context.Context, search string, page, limit int) ([]entity.MerchantSummary, int, error)
}

type MerchantsService struct {
	repo repositories.IMerchantsRepository
}

func NewMerchantsService(repo repositories.IMerchantsRepository) *MerchantsService {
	return &MerchantsService{repo: repo}
}

func (s *MerchantsService) ListMerchants(ctx context.Context, search string, page, limit int) ([]entity.MerchantSummary, int, error) {
	return s.repo.ListMerchants(ctx, strings.TrimSpace(search), page, limit)
}
