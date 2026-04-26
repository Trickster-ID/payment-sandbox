package services

import (
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	"payment-sandbox/app/modules/wallet/repositories"
)

type WalletService struct {
	repo repositories.WalletRepository
}

type Service interface {
	WalletByUserID(userID string) (walletEntity.Merchant, error)
	CreateTopup(userID string, amount float64) (walletEntity.Topup, error)
	ListTopups() []walletEntity.Topup
	UpdateTopupStatus(topupID, status string) (walletEntity.Topup, error)
}

var _ Service = (*WalletService)(nil)

func NewWalletService(repo repositories.WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

func (s *WalletService) WalletByUserID(userID string) (walletEntity.Merchant, error) {
	return s.repo.GetMerchantWallet(userID)
}

func (s *WalletService) CreateTopup(userID string, amount float64) (walletEntity.Topup, error) {
	merchantID, err := s.repo.MerchantIDByUserID(userID)
	if err != nil {
		return walletEntity.Topup{}, err
	}
	return s.repo.CreateTopup(merchantID, amount)
}

func (s *WalletService) ListTopups() []walletEntity.Topup {
	return s.repo.ListTopups()
}

func (s *WalletService) UpdateTopupStatus(topupID, status string) (walletEntity.Topup, error) {
	parsed, err := paymentEntity.ParsePaymentStatus(status)
	if err != nil {
		return walletEntity.Topup{}, err
	}
	return s.repo.UpdateTopupStatus(topupID, parsed)
}
