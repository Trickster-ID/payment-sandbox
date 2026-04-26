package entity

import "time"

type InvoiceStatus string

const (
	InvoicePending InvoiceStatus = "PENDING"
	InvoicePaid    InvoiceStatus = "PAID"
	InvoiceExpired InvoiceStatus = "EXPIRED"
)

type Invoice struct {
	ID               string        `json:"id"`
	MerchantID       string        `json:"merchant_id"`
	InvoiceNumber    string        `json:"invoice_number"`
	CustomerName     string        `json:"customer_name"`
	CustomerEmail    string        `json:"customer_email"`
	Amount           float64       `json:"amount"`
	Description      string        `json:"description"`
	DueDate          time.Time     `json:"due_date"`
	Status           InvoiceStatus `json:"status"`
	PaymentLinkToken string        `json:"payment_link_token"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type ListOptions struct {
	Page  int
	Limit int
}
