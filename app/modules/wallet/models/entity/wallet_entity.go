package entity

import (
	"time"

	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
)

type Merchant struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Balance   float64   `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Topup struct {
	ID         string                      `json:"id"`
	MerchantID string                      `json:"merchant_id"`
	Amount     float64                     `json:"amount"`
	Status     paymentEntity.PaymentStatus `json:"status"`
	CreatedAt  time.Time                   `json:"created_at"`
	UpdatedAt  time.Time                   `json:"updated_at"`
}
