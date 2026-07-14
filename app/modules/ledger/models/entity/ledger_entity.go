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
	ID         uuid.UUID   `json:"id"`
	MerchantID *uuid.UUID  `json:"merchant_id,omitempty"`
	Name       string      `json:"name"`
	Type       AccountType `json:"type"`
	Currency   string      `json:"currency"`
	Balance    int64       `json:"balance"`
	Version    int64       `json:"version"`
	IsActive   bool        `json:"is_active"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
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

type EntryFilter struct {
	From            *time.Time
	To              *time.Time
	Direction       *string
	ReferencePrefix *string
}

type EntryWithTxn struct {
	ID          int64
	TxnID       uuid.UUID
	AccountID   uuid.UUID
	Direction   Direction
	Amount      int64
	Currency    string
	BalanceAfter int64
	CreatedAt   time.Time
	Reference   string
	Description string
	Metadata    map[string]any
}
