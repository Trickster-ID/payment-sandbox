// Package sagas demonstrates the saga pattern for the payment settlement flow.
// ValidatePaymentStep → PostLedgerStep → MarkPaymentSuccessStep run in sequence;
// if any step fails its predecessors' Compensate methods run in reverse order.
package sagas

import (
	"context"
	"database/sql"
	"errors"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerRepo "payment-sandbox/app/modules/ledger/repositories"
	ledgerSvc "payment-sandbox/app/modules/ledger/services"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"

	"github.com/google/uuid"
)

// ValidatePaymentStep loads and validates the payment intent, storing results in state.
type ValidatePaymentStep struct {
	DB *sql.DB
}

func (s *ValidatePaymentStep) Name() string { return "validate_payment" }

func (s *ValidatePaymentStep) Execute(ctx context.Context, state map[string]any) error {
	paymentID, _ := state["payment_id"].(string)

	var invoiceID, merchantID string
	var amount int64
	var status string
	err := s.DB.QueryRowContext(ctx, `
		SELECT pi.invoice_id::text, inv.merchant_id::text, inv.amount, pi.status::text
		FROM payment_intents pi
		JOIN invoices inv ON inv.id = pi.invoice_id AND inv.deleted_at IS NULL
		WHERE pi.id=$1 AND pi.deleted_at IS NULL
	`, paymentID).Scan(&invoiceID, &merchantID, &amount, &status)
	if err != nil {
		return errors.New("payment intent not found")
	}
	if paymentEntity.PaymentStatus(status) != paymentEntity.PaymentPending {
		return errors.New("payment intent already finalized")
	}

	state["invoice_id"] = invoiceID
	state["merchant_id"] = merchantID
	state["amount"] = amount
	return nil
}

func (s *ValidatePaymentStep) Compensate(_ context.Context, _ map[string]any) error {
	return nil // read-only, nothing to undo
}

// PostLedgerStep posts double-entry ledger entries for the payment settlement.
// Compensate reverses the ledger and marks the payment FAILED.
type PostLedgerStep struct {
	DB         *sql.DB
	LedgerRepo ledgerRepo.IRepository
}

func (s *PostLedgerStep) Name() string { return "post_ledger" }

func (s *PostLedgerStep) Execute(ctx context.Context, state map[string]any) error {
	paymentID, _ := state["payment_id"].(string)
	merchantIDStr, _ := state["merchant_id"].(string)
	amount, _ := state["amount"].(int64)

	merchantUUID, err := uuid.Parse(merchantIDStr)
	if err != nil {
		return errors.New("invalid merchant id")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	walletAcct, err := s.LedgerRepo.GetAccountByMerchantID(ctx, merchantUUID)
	if err != nil {
		return errors.New("merchant ledger account not found")
	}

	posting := ledgerEntity.Posting{
		Reference:   "payment_" + paymentID,
		Description: "Payment settlement",
		Entries: []ledgerEntity.Entry{
			{AccountID: walletAcct.ID, Direction: ledgerEntity.Debit, Amount: amount, Currency: "IDR"},
			{AccountID: ledgerEntity.PendingPaymentsAccountID, Direction: ledgerEntity.Credit, Amount: amount, Currency: "IDR"},
		},
	}
	if err := ledgerSvc.ValidatePosting(posting); err != nil {
		return err
	}
	if _, err := s.LedgerRepo.Post(ctx, tx, posting); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2 AND deleted_at IS NULL`,
		walletAcct.ID, merchantIDStr); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	state["ledger_ref"] = posting.Reference
	state["wallet_acct_id"] = walletAcct.ID.String()
	return nil
}

func (s *PostLedgerStep) Compensate(ctx context.Context, state map[string]any) error {
	paymentID, _ := state["payment_id"].(string)
	ref, _ := state["ledger_ref"].(string)
	if ref == "" {
		return nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := s.LedgerRepo.Reverse(ctx, tx, ref, "payment saga compensation", uuid.Nil); err != nil {
		return err
	}

	merchantIDStr, _ := state["merchant_id"].(string)
	walletAcctIDStr, _ := state["wallet_acct_id"].(string)
	if walletAcctIDStr != "" {
		walletAcctID, _ := uuid.Parse(walletAcctIDStr)
		_, _ = tx.ExecContext(ctx,
			`UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2 AND deleted_at IS NULL`,
			walletAcctID, merchantIDStr)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE payment_intents SET status='FAILED' WHERE id=$1 AND deleted_at IS NULL`, paymentID); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkPaymentSuccessStep updates payment_intents and invoices to their final states.
type MarkPaymentSuccessStep struct {
	DB *sql.DB
}

func (s *MarkPaymentSuccessStep) Name() string { return "mark_payment_success" }

func (s *MarkPaymentSuccessStep) Execute(ctx context.Context, state map[string]any) error {
	paymentID, _ := state["payment_id"].(string)
	invoiceID, _ := state["invoice_id"].(string)

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE payment_intents SET status='SUCCESS' WHERE id=$1 AND deleted_at IS NULL`, paymentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE invoices SET status='PAID' WHERE id=$1 AND deleted_at IS NULL`, invoiceID); err != nil {
		return err
	}
	return tx.Commit()
}

// Compensate is a noop — by this point the ledger is already reversed and the payment is FAILED.
func (s *MarkPaymentSuccessStep) Compensate(_ context.Context, _ map[string]any) error {
	return nil
}
