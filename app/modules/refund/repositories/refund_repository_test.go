package repositories

import (
	"regexp"
	"testing"
	"time"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerMocks "payment-sandbox/app/modules/ledger/repositories/mocks"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testRefundMerchantUUID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testRefundAccountUUID  = uuid.MustParse("00000000-0000-0000-0000-000000000020")
)

func TestRefundRepository_RequestRefund(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRefundRepository(db, nil)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.id::text, pi.status::text, inv.merchant_id::text, inv.amount")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "status", "merchant_id", "amount"}).
				AddRow("pi-1", "SUCCESS", "m-1", int64(100)))

		sqlMock.ExpectQuery(regexp.QuoteMeta("INSERT INTO refunds")).
			WithArgs("pi-1", "reason", int64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "payment_intent_id", "amount", "status", "created_at", "updated_at"}).
				AddRow("ref-1", "pi-1", int64(100), "REQUESTED", now, now))

		sqlMock.ExpectCommit()

		ref, err := repo.RequestRefund("m-1", "inv-1", "reason")
		require.NoError(t, err)
		assert.Equal(t, "ref-1", ref.ID)
	})

	t.Run("not successful payment", func(t *testing.T) {
		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.id::text, pi.status::text, inv.merchant_id::text, inv.amount")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "status", "merchant_id", "amount"}).
				AddRow("pi-1", "PENDING", "m-1", int64(100)))
		sqlMock.ExpectRollback()

		_, err := repo.RequestRefund("m-1", "inv-1", "reason")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "successful payment only")
	})
}

func TestRefundRepository_ReviewRefund(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRefundRepository(db, nil)
	now := time.Now()

	t.Run("success APPROVED", func(t *testing.T) {
		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE refunds")).
			WithArgs("APPROVED", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua", "m_name"}).
				AddRow("ref-1", "pi-1", "m-1", "r", "APPROVED", int64(100), "INV-001", now, now, "Test Merchant"))

		ref, err := repo.ReviewRefund("ref-1", true)
		require.NoError(t, err)
		assert.Equal(t, refundEntity.RefundApproved, ref.Status)
	})
}

func TestRefundRepository_ProcessRefund(t *testing.T) {
	merchantIDStr := testRefundMerchantUUID.String()
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		ledgerMock := ledgerMocks.NewMockIRepository(t)
		repo := NewRefundRepository(db, ledgerMock)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua"}).
				AddRow("ref-1", "pi-1", merchantIDStr, "r", "APPROVED", int64(100), "INV-001", now, now))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
			WithArgs(merchantIDStr).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow(merchantIDStr, "u-1", int64(1000), now, now))

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE refunds SET status=$1 WHERE id=$2")).
			WithArgs("SUCCESS", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		ledgerMock.EXPECT().
			GetAccountByMerchantID(mock.Anything, testRefundMerchantUUID).
			Return(ledgerEntity.Account{ID: testRefundAccountUUID}, nil)

		ledgerMock.EXPECT().
			Post(mock.Anything, mock.Anything, mock.Anything).
			Return(uuid.New(), nil)

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2")).
			WithArgs(testRefundAccountUUID, merchantIDStr).
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectCommit()

		// Final lookups after commit
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua", "m_name"}).
				AddRow("ref-1", "pi-1", merchantIDStr, "r", "SUCCESS", int64(100), "INV-001", now, now, "Test Merchant"))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
			WithArgs("u-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow(merchantIDStr, "u-1", int64(900), now, now))

		ref, m, err := repo.ProcessRefund("ref-1", refundEntity.RefundSuccess)
		require.NoError(t, err)
		assert.Equal(t, refundEntity.RefundSuccess, ref.Status)
		assert.Equal(t, int64(900), m.Balance)
		assert.NoError(t, sqlMock.ExpectationsWereMet())
	})

	t.Run("insufficient balance", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewRefundRepository(db, nil)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua"}).
				AddRow("ref-1", "pi-1", merchantIDStr, "r", "APPROVED", int64(1000), "INV-002", now, now))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
			WithArgs(merchantIDStr).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow(merchantIDStr, "u-1", int64(100), now, now))
		sqlMock.ExpectRollback()

		_, _, err = repo.ProcessRefund("ref-1", refundEntity.RefundSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient merchant balance")
	})
}
