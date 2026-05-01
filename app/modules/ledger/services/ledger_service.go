package services

import (
	"errors"
	"fmt"

	"payment-sandbox/app/modules/ledger/models/entity"
)

func ValidatePosting(p entity.Posting) error {
	if len(p.Entries) < 2 {
		return errors.New("posting requires at least 2 entries")
	}
	if p.Reference == "" {
		return errors.New("posting requires a reference")
	}
	totals := map[string]int64{}
	for _, e := range p.Entries {
		if e.Amount <= 0 {
			return errors.New("entry amount must be positive")
		}
		switch e.Direction {
		case entity.Debit:
			totals[e.Currency] += e.Amount
		case entity.Credit:
			totals[e.Currency] -= e.Amount
		default:
			return fmt.Errorf("invalid direction: %s", e.Direction)
		}
	}
	for ccy, diff := range totals {
		if diff != 0 {
			return fmt.Errorf("posting unbalanced for %s: debits - credits = %d", ccy, diff)
		}
	}
	return nil
}
