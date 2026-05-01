package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Runner struct{ DB *sql.DB }

func NewRunner(db *sql.DB) *Runner { return &Runner{DB: db} }

// CheckLedgerIntegrity verifies that every account's balance equals the sum of its ledger entries.
func (r *Runner) CheckLedgerIntegrity(ctx context.Context) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT a.id, a.balance, a.type::text,
		       COALESCE(SUM(CASE WHEN le.direction='D' THEN le.amount ELSE -le.amount END), 0) AS computed
		FROM accounts a
		LEFT JOIN ledger_entries le ON le.account_id = a.id
		GROUP BY a.id, a.balance, a.type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bad []string
	for rows.Next() {
		var id, accType string
		var bal, computed int64
		_ = rows.Scan(&id, &bal, &accType, &computed)
		expected := computed
		if accType == "liability" || accType == "revenue" || accType == "equity" {
			expected = -computed
		}
		if bal != expected {
			bad = append(bad, fmt.Sprintf("account %s (type=%s): balance=%d expected=%d diff=%d",
				id, accType, bal, expected, bal-expected))
		}
	}
	return bad, nil
}

// CheckTransactionBalance verifies that every ledger_transaction has equal debits and credits.
func (r *Runner) CheckTransactionBalance(ctx context.Context) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT transaction_id::text,
		       SUM(CASE WHEN direction='D' THEN amount ELSE 0 END) AS debits,
		       SUM(CASE WHEN direction='C' THEN amount ELSE 0 END) AS credits
		FROM ledger_entries
		GROUP BY transaction_id
		HAVING SUM(CASE WHEN direction='D' THEN amount ELSE 0 END)
		     <> SUM(CASE WHEN direction='C' THEN amount ELSE 0 END)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bad []string
	for rows.Next() {
		var id string
		var d, c int64
		_ = rows.Scan(&id, &d, &c)
		bad = append(bad, fmt.Sprintf("tx %s unbalanced: D=%d C=%d", id, d, c))
	}
	return bad, nil
}

type ExternalRecord struct {
	Reference string
	Amount    int64
}

type Discrepancy struct {
	Category       string
	Reference      string
	InternalAmount int64
	ExternalAmount int64
}

// ReconcileWithProcessor compares the internal ledger to an external settlement file.
// Reports discrepancies only — never mutates data.
func (r *Runner) ReconcileWithProcessor(ctx context.Context, ext []ExternalRecord, day time.Time) ([]Discrepancy, error) {
	internal := map[string]int64{}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT lt.reference, COALESCE(SUM(le.amount), 0)
		FROM ledger_transactions lt
		JOIN ledger_entries le ON le.transaction_id = lt.id
		WHERE lt.created_at::date = $1::date AND le.direction = 'D'
		GROUP BY lt.reference
	`, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		var amt int64
		_ = rows.Scan(&ref, &amt)
		internal[ref] = amt
	}

	external := map[string]int64{}
	for _, e := range ext {
		external[e.Reference] = e.Amount
	}

	var out []Discrepancy
	for ref, intAmt := range internal {
		extAmt, ok := external[ref]
		if !ok {
			out = append(out, Discrepancy{Category: "missing_in_external", Reference: ref, InternalAmount: intAmt})
		} else if extAmt != intAmt {
			out = append(out, Discrepancy{Category: "amount_mismatch", Reference: ref, InternalAmount: intAmt, ExternalAmount: extAmt})
		}
	}
	for ref, extAmt := range external {
		if _, ok := internal[ref]; !ok {
			out = append(out, Discrepancy{Category: "missing_in_internal", Reference: ref, ExternalAmount: extAmt})
		}
	}
	return out, nil
}
