package entity

import "github.com/google/uuid"

var (
	TopupClearingAccountID   = uuid.MustParse("00000000-0000-4000-8000-000000000010")
	PendingPaymentsAccountID = uuid.MustParse("00000000-0000-4000-8000-000000000011")
	FeesRevenueAccountID     = uuid.MustParse("00000000-0000-4000-8000-000000000012")
	RefundsExpenseAccountID  = uuid.MustParse("00000000-0000-4000-8000-000000000013")
)
