package services

import (
	"errors"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	"payment-sandbox/app/modules/invoice/repositories"
	"payment-sandbox/app/shared/validator"
)

type InvoiceService struct {
	repo repositories.InvoiceRepository
}

type Service interface {
	CreateInvoice(userID, customerName, customerEmail string, amount float64, description, dueDate string) (invoiceEntity.Invoice, error)
	ListInvoices(userID, status string, page, limit int) ([]invoiceEntity.Invoice, int, error)
	InvoiceByID(userID, invoiceID string) (invoiceEntity.Invoice, error)
}

var _ Service = (*InvoiceService)(nil)

func NewInvoiceService(repo repositories.InvoiceRepository) *InvoiceService {
	return &InvoiceService{repo: repo}
}

func (s *InvoiceService) CreateInvoice(userID, customerName, customerEmail string, amount float64, description, dueDate string) (invoiceEntity.Invoice, error) {
	merchantID, err := s.repo.MerchantIDByUserID(userID)
	if err != nil {
		return invoiceEntity.Invoice{}, err
	}

	parsedDueDate, err := validator.ParseRFC3339(dueDate)
	if err != nil {
		return invoiceEntity.Invoice{}, errors.New("due_date must use RFC3339 format")
	}

	if !validator.IsEmail(customerEmail) {
		return invoiceEntity.Invoice{}, errors.New("customer_email is invalid")
	}

	return s.repo.CreateInvoice(merchantID, customerName, customerEmail, amount, description, parsedDueDate)
}

func (s *InvoiceService) ListInvoices(userID, status string, page, limit int) ([]invoiceEntity.Invoice, int, error) {
	merchantID, err := s.repo.MerchantIDByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	items, total := s.repo.ListInvoices(merchantID, status, invoiceEntity.ListOptions{Page: page, Limit: limit})
	return items, total, nil
}

func (s *InvoiceService) InvoiceByID(userID, invoiceID string) (invoiceEntity.Invoice, error) {
	merchantID, err := s.repo.MerchantIDByUserID(userID)
	if err != nil {
		return invoiceEntity.Invoice{}, err
	}
	return s.repo.MerchantInvoiceByID(invoiceID, merchantID)
}
