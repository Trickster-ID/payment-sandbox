package services

import (
	"errors"
	"strings"

	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	"payment-sandbox/app/modules/refund/repositories"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
)

type RefundService struct {
	repo repositories.IRefundRepository
}

type IRefundService interface {
	RequestRefund(userID, invoiceID, reason string) (refundEntity.Refund, error)
	MerchantListRefunds(userID, status string) []refundEntity.Refund
	ListRefunds(status string) []refundEntity.Refund
	ReviewRefund(refundID, decision string) (refundEntity.Refund, error)
	ProcessRefund(refundID, status string) (refundEntity.Refund, walletEntity.Merchant, error)
}

func NewRefundService(repo repositories.IRefundRepository) *RefundService {
	return &RefundService{repo: repo}
}

func (s *RefundService) RequestRefund(userID, invoiceID, reason string) (refundEntity.Refund, error) {
	if strings.TrimSpace(reason) == "" {
		return refundEntity.Refund{}, errors.New("reason is required")
	}
	merchantID, err := s.repo.MerchantIDByUserID(userID)
	if err != nil {
		return refundEntity.Refund{}, err
	}
	return s.repo.RequestRefund(merchantID, invoiceID, reason)
}

func (s *RefundService) MerchantListRefunds(userID, status string) []refundEntity.Refund {
	merchantID, err := s.repo.MerchantIDByUserID(userID)
	if err != nil {
		return []refundEntity.Refund{}
	}
	return s.repo.ListMerchantRefunds(merchantID, strings.ToUpper(strings.TrimSpace(status)))
}

func (s *RefundService) ListRefunds(status string) []refundEntity.Refund {
	return s.repo.ListRefunds(status)
}

func (s *RefundService) ReviewRefund(refundID, decision string) (refundEntity.Refund, error) {
	value := strings.ToUpper(strings.TrimSpace(decision))
	switch value {
	case "APPROVE":
		return s.repo.ReviewRefund(refundID, true)
	case "REJECT":
		return s.repo.ReviewRefund(refundID, false)
	default:
		return refundEntity.Refund{}, errors.New("decision must be APPROVE or REJECT")
	}
}

func (s *RefundService) ProcessRefund(refundID, status string) (refundEntity.Refund, walletEntity.Merchant, error) {
	parsed, err := refundEntity.ParseRefundProcessStatus(status)
	if err != nil {
		return refundEntity.Refund{}, walletEntity.Merchant{}, err
	}
	return s.repo.ProcessRefund(refundID, parsed)
}
