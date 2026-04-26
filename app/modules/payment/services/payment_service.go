package services

import (
	"errors"
	"strings"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	"payment-sandbox/app/modules/payment/repositories"
)

type PaymentService struct {
	repo repositories.PaymentRepository
}

type Service interface {
	PublicInvoiceByToken(token string) (invoiceEntity.Invoice, error)
	CreatePaymentIntent(token, method string) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
	ListPaymentIntents(status string) []paymentEntity.PaymentIntent
	UpdatePaymentIntentStatus(paymentID, status string) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
}

var _ Service = (*PaymentService)(nil)

func NewPaymentService(repo repositories.PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) PublicInvoiceByToken(token string) (invoiceEntity.Invoice, error) {
	invoice, found := s.repo.GetInvoiceByToken(token)
	if !found {
		return invoiceEntity.Invoice{}, errors.New("invoice not found")
	}
	return invoice, nil
}

func (s *PaymentService) CreatePaymentIntent(token, method string) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error) {
	return s.repo.CreatePaymentIntent(token, paymentEntity.PaymentMethod(strings.ToUpper(strings.TrimSpace(method))))
}

func (s *PaymentService) ListPaymentIntents(status string) []paymentEntity.PaymentIntent {
	return s.repo.ListPaymentIntents(strings.ToUpper(strings.TrimSpace(status)))
}

func (s *PaymentService) UpdatePaymentIntentStatus(paymentID, status string) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error) {
	parsed, err := paymentEntity.ParsePaymentStatus(status)
	if err != nil {
		return paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, err
	}
	return s.repo.UpdatePaymentStatus(paymentID, parsed)
}
