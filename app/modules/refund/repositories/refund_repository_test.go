package repositories

import (
	"database/sql"
	"errors"
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

	t.Run("begin tx fails", func(t *testing.T) {
		sqlMock.ExpectBegin().WillReturnError(errors.New("begin error"))

		_, err := repo.RequestRefund("m-1", "inv-1", "reason")
		assert.Error(t, err)
	})

	t.Run("invoice not found", func(t *testing.T) {
		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.id::text, pi.status::text, inv.merchant_id::text, inv.amount")).
			WithArgs("inv-1").
			WillReturnError(sql.ErrNoRows)
		sqlMock.ExpectRollback()

		_, err := repo.RequestRefund("m-1", "inv-1", "reason")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invoice not found or payment not created")
	})

	t.Run("merchant id mismatch", func(t *testing.T) {
		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT pi.id::text, pi.status::text, inv.merchant_id::text, inv.amount")).
			WithArgs("inv-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "status", "merchant_id", "amount"}).
				AddRow("pi-1", "SUCCESS", "other-merchant", int64(100)))
		sqlMock.ExpectRollback()

		_, err := repo.RequestRefund("m-1", "inv-1", "reason")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invoice does not belong to merchant")
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

	t.Run("exec error -> error", func(t *testing.T) {
		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE refunds")).
			WithArgs("APPROVED", "ref-1").
			WillReturnError(errors.New("db error"))

		_, err := repo.ReviewRefund("ref-1", true)
		assert.Error(t, err)
	})

	t.Run("zero rows affected -> not found error", func(t *testing.T) {
		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE refunds")).
			WithArgs("APPROVED", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		_, err := repo.ReviewRefund("ref-1", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already reviewed or not found")
	})

	t.Run("reject -> REJECTED status", func(t *testing.T) {
		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE refunds")).
			WithArgs("REJECTED", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua", "m_name"}).
				AddRow("ref-1", "pi-1", "m-1", "r", "REJECTED", int64(100), "INV-001", now, now, "Test Merchant"))

		ref, err := repo.ReviewRefund("ref-1", false)
		require.NoError(t, err)
		assert.Equal(t, refundEntity.RefundRejected, ref.Status)
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

	t.Run("refund not found", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewRefundRepository(db, nil)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnError(sql.ErrNoRows)
		sqlMock.ExpectRollback()

		_, _, err = repo.ProcessRefund("ref-1", refundEntity.RefundSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "refund not found")
	})

	t.Run("refund not approved -> error", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewRefundRepository(db, nil)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua"}).
				AddRow("ref-1", "pi-1", merchantIDStr, "r", "REQUESTED", int64(100), "INV-001", now, now))
		sqlMock.ExpectRollback()

		_, _, err = repo.ProcessRefund("ref-1", refundEntity.RefundSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be approved before processing")
	})

	t.Run("failed status -> updates without ledger", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewRefundRepository(db, nil)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua"}).
				AddRow("ref-1", "pi-1", merchantIDStr, "r", "APPROVED", int64(50), "INV-001", now, now))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
			WithArgs(merchantIDStr).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow(merchantIDStr, "u-1", int64(200), now, now))

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE refunds SET status=$1 WHERE id=$2")).
			WithArgs("FAILED", "ref-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectCommit()

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text, r.amount, inv.invoice_number, r.created_at, r.updated_at, u.name::text FROM refunds")).
			WithArgs("ref-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua", "m_name"}).
				AddRow("ref-1", "pi-1", merchantIDStr, "r", "FAILED", int64(50), "INV-001", now, now, "Test Merchant"))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
			WithArgs("u-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
				AddRow(merchantIDStr, "u-1", int64(200), now, now))

		ref, m, err := repo.ProcessRefund("ref-1", refundEntity.RefundFailed)
		require.NoError(t, err)
		assert.Equal(t, refundEntity.RefundFailed, ref.Status)
		assert.Equal(t, int64(200), m.Balance)
		assert.NoError(t, sqlMock.ExpectationsWereMet())
	})
}

func TestRefundRepository_MerchantIDByUserID(t *testing.T) {
	type args struct {
		userID string
	}
	type wants struct {
		merchantID string
		wantErr    string
	}

	now := time.Now()

	tests := []struct {
		name  string
		args  args
		wants wants
		setup func(sqlMock sqlmock.Sqlmock)
	}{
		{
			name:  "1. merchant exists -> returns merchant ID",
			args:  args{userID: "user-1"},
			wants: wants{merchantID: "merchant-1"},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
					WithArgs("user-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "ca", "ua"}).
						AddRow("merchant-1", "user-1", int64(0), now, now))
			},
		},
		{
			name:  "2. merchant not found -> error",
			args:  args{userID: "user-1"},
			wants: wants{merchantID: "", wantErr: "merchant not found"},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
					WithArgs("user-1").
					WillReturnError(sql.ErrNoRows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.setup(sqlMock)

			repo := NewRefundRepository(db, nil)
			merchantID, err := repo.MerchantIDByUserID(tt.args.userID)

			if tt.wants.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wants.wantErr)
				assert.Empty(t, merchantID)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wants.merchantID, merchantID)
			}
		})
	}
}

func TestRefundRepository_ListMerchantRefunds(t *testing.T) {
	type args struct {
		merchantID string
		status     string
	}
	type wants struct {
		dataLen int
	}

	now := time.Now()
	rowCols := []string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua", "m_name"}

	tests := []struct {
		name  string
		args  args
		wants wants
		setup func(sqlMock sqlmock.Sqlmock)
	}{
		{
			name:  "1. without status filter -> returns all refunds",
			args:  args{merchantID: "m-1", status: ""},
			wants: wants{dataLen: 2},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text")).
					WithArgs("m-1").
					WillReturnRows(sqlmock.NewRows(rowCols).
						AddRow("ref-1", "pi-1", "m-1", "reason", "REQUESTED", int64(100), "INV-001", now, now, "Test Merchant").
						AddRow("ref-2", "pi-2", "m-1", "reason", "REQUESTED", int64(200), "INV-002", now, now, "Test Merchant"))
			},
		},
		{
			name:  "2. with status filter -> returns filtered refunds",
			args:  args{merchantID: "m-1", status: "REQUESTED"},
			wants: wants{dataLen: 1},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text")).
					WithArgs("m-1", "REQUESTED").
					WillReturnRows(sqlmock.NewRows(rowCols).
						AddRow("ref-1", "pi-1", "m-1", "reason", "REQUESTED", int64(100), "INV-001", now, now, "Test Merchant"))
			},
		},
		{
			name:  "3. db error -> returns empty slice",
			args:  args{merchantID: "m-1", status: ""},
			wants: wants{dataLen: 0},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason, r.status::text")).
					WithArgs("m-1").
					WillReturnError(errors.New("db error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.setup(sqlMock)

			repo := NewRefundRepository(db, nil)
			result := repo.ListMerchantRefunds(tt.args.merchantID, tt.args.status)

			assert.Len(t, result, tt.wants.dataLen)
		})
	}
}

func TestRefundRepository_ListRefunds(t *testing.T) {
	type args struct {
		status string
	}
	type wants struct {
		dataLen int
	}

	now := time.Now()
	rowCols := []string{"id", "pi_id", "m_id", "reason", "status", "amount", "inv_num", "ca", "ua", "m_name"}

	tests := []struct {
		name  string
		args  args
		wants wants
		setup func(sqlMock sqlmock.Sqlmock)
	}{
		{
			name:  "1. without status filter -> returns all refunds",
			args:  args{status: ""},
			wants: wants{dataLen: 2},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason")).
					WillReturnRows(sqlmock.NewRows(rowCols).
						AddRow("ref-1", "pi-1", "m-1", "reason", "REQUESTED", int64(100), "INV-001", now, now, "Test Merchant").
						AddRow("ref-2", "pi-2", "m-1", "reason", "APPROVED", int64(200), "INV-002", now, now, "Test Merchant"))
			},
		},
		{
			name:  "2. with status filter -> returns filtered refunds",
			args:  args{status: "REQUESTED"},
			wants: wants{dataLen: 1},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason")).
					WithArgs("REQUESTED").
					WillReturnRows(sqlmock.NewRows(rowCols).
						AddRow("ref-1", "pi-1", "m-1", "reason", "REQUESTED", int64(100), "INV-001", now, now, "Test Merchant"))
			},
		},
		{
			name:  "3. db error -> returns empty slice",
			args:  args{status: ""},
			wants: wants{dataLen: 0},
			setup: func(sqlMock sqlmock.Sqlmock) {
				sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT r.id::text, r.payment_intent_id::text, inv.merchant_id::text, r.reason")).
					WillReturnError(errors.New("db error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			tt.setup(sqlMock)

			repo := NewRefundRepository(db, nil)
			result := repo.ListRefunds(tt.args.status)

			assert.Len(t, result, tt.wants.dataLen)
		})
	}
}
