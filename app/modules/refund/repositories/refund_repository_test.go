package repositories

import (
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefundRepository_RequestRefund(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRefundRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT pi.status::text, inv.merchant_id::text, inv.amount::double precision")).
			WithArgs("intent-1").
			WillReturnRows(sqlmock.NewRows([]string{"status", "merchant_id", "amount"}).
				AddRow("SUCCESS", "m-1", 100.0))
		
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO refunds")).
			WithArgs("intent-1", "reason").
			WillReturnRows(sqlmock.NewRows([]string{"id", "payment_intent_id", "status", "created_at", "updated_at"}).
				AddRow("ref-1", "intent-1", "REQUESTED", now, now))
		
		mock.ExpectCommit()

		ref, err := repo.RequestRefund("m-1", "intent-1", "reason")
		require.NoError(t, err)
		assert.Equal(t, "ref-1", ref.ID)
	})

	t.Run("not successful payment", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnRows(sqlmock.NewRows([]string{"status", "merchant_id", "amount"}).
				AddRow("PENDING", "m-1", 100.0))
		mock.ExpectRollback()

		_, err := repo.RequestRefund("m-1", "intent-1", "reason")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "successful payment only")
	})
}

func TestRefundRepository_ReviewRefund(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRefundRepository(db)
	now := time.Now()

	t.Run("success APPROVED", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE refunds")).
			WithArgs("APPROVED", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, inv.amount::double precision, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "ca", "ua"}).
				AddRow("ref-1", "pi-1", "m-1", "r", "APPROVED", 100.0, now, now))

		ref, err := repo.ReviewRefund("ref-1", true)
		require.NoError(t, err)
		assert.Equal(t, refundEntity.RefundApproved, ref.Status)
	})
}

func TestRefundRepository_ProcessRefund(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRefundRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, inv.amount::double precision, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "ca", "ua"}).
				AddRow("ref-1", "pi-1", "m-1", "r", "APPROVED", 100.0, now, now))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance::double precision, created_at, updated_at FROM merchants")).
			WithArgs("m-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow("m-1", "u-1", 1000.0, now, now))
		
		mock.ExpectExec(regexp.QuoteMeta("UPDATE refunds SET status=$1 WHERE id=$2")).
			WithArgs("SUCCESS", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance = balance - $1 WHERE id=$2")).
			WithArgs(100.0, "m-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectCommit()

		// Final lookups
		mock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, inv.amount::double precision, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "ca", "ua"}).
				AddRow("ref-1", "pi-1", "m-1", "r", "SUCCESS", 100.0, now, now))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance::double precision, created_at, updated_at FROM merchants")).
			WithArgs("u-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow("m-1", "u-1", 900.0, now, now))

		ref, m, err := repo.ProcessRefund("ref-1", refundEntity.RefundSuccess)
		require.NoError(t, err)
		assert.Equal(t, refundEntity.RefundSuccess, ref.Status)
		assert.Equal(t, 900.0, m.Balance)
	})

	t.Run("insufficient balance", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "ca", "ua"}).
				AddRow("ref-1", "pi-1", "m-1", "r", "APPROVED", 1000.0, now, now))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow("m-1", "u-1", 100.0, now, now))
		mock.ExpectRollback()

		_, _, err := repo.ProcessRefund("ref-1", refundEntity.RefundSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient merchant balance")
	})
}
