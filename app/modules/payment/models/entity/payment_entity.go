package entity

import (
	"fmt"
	"strings"
	"time"
)

type PaymentStatus string
type PaymentMethod string

const (
	PaymentPending PaymentStatus = "PENDING"
	PaymentSuccess PaymentStatus = "SUCCESS"
	PaymentFailed  PaymentStatus = "FAILED"

	MethodWallet       PaymentMethod = "WALLET"
	MethodVADummy      PaymentMethod = "VA_DUMMY"
	MethodEWalletDummy PaymentMethod = "EWALLET_DUMMY"
)

type PaymentIntent struct {
	ID        string        `json:"id"`
	InvoiceID string        `json:"invoice_id"`
	Method    PaymentMethod `json:"method"`
	Status    PaymentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func ParsePaymentStatus(value string) (PaymentStatus, error) {
	status := PaymentStatus(strings.ToUpper(strings.TrimSpace(value)))
	if status != PaymentSuccess && status != PaymentFailed {
		return "", fmt.Errorf("invalid payment status: %s", value)
	}
	return status, nil
}
