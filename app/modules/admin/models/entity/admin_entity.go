package entity

import "time"

type StatsFilter struct {
	MerchantID string
	StartDate  *time.Time
	EndDate    *time.Time
}

type DashboardStats struct {
	TotalInvoiceCreated int            `json:"total_invoice_created"`
	TotalByStatus       map[string]int `json:"total_by_status"`
	TotalPaymentNominal float64        `json:"total_payment_nominal"`
	TotalRefundNominal  float64        `json:"total_refund_nominal"`
}
