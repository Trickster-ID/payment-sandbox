package entity

import (
	"time"

	"github.com/google/uuid"
)

type Direction string

const (
	Debit  Direction = "D"
	Credit Direction = "C"
)

type AccountType string

const (
	Asset     AccountType = "asset"
	Liability AccountType = "liability"
	Revenue   AccountType = "revenue"
	Expense   AccountType = "expense"
	Equity    AccountType = "equity"
)

type Account struct {
	ID         uuid.UUID
	MerchantID *uuid.UUID
	Name       string
	Type       AccountType
	Currency   string
	Balance    int64
	Version    int64
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Entry struct {
	AccountID uuid.UUID
	Direction Direction
	Amount    int64
	Currency  string
}

type Posting struct {
	Reference   string
	Description string
	Entries     []Entry
	Metadata    map[string]any
	CreatedBy   uuid.UUID
}

type LedgerEntry struct {
	ID            int64
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	Direction     Direction
	Amount        int64
	Currency      string
	BalanceAfter  int64
	CreatedAt     time.Time
}
