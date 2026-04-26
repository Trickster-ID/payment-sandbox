package entity

import (
	"fmt"
	"strings"
	"time"
)

type RefundStatus string

const (
	RefundRequested RefundStatus = "REQUESTED"
	RefundApproved  RefundStatus = "APPROVED"
	RefundRejected  RefundStatus = "REJECTED"
	RefundSuccess   RefundStatus = "SUCCESS"
	RefundFailed    RefundStatus = "FAILED"
)

type Refund struct {
	ID              string       `json:"id"`
	PaymentIntentID string       `json:"payment_intent_id"`
	MerchantID      string       `json:"merchant_id"`
	Reason          string       `json:"reason"`
	Status          RefundStatus `json:"status"`
	Amount          float64      `json:"amount"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

func ParseRefundProcessStatus(value string) (RefundStatus, error) {
	status := RefundStatus(strings.ToUpper(strings.TrimSpace(value)))
	if status != RefundSuccess && status != RefundFailed {
		return "", fmt.Errorf("invalid refund status: %s", value)
	}
	return status, nil
}
