package repositories

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	ledgerEntity "payment-sandbox/app/modules/ledger/models/entity"
	ledgerMocks "payment-sandbox/app/modules/ledger/repositories/mocks"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testMerchantUUID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testAccountUUID  = uuid.MustParse("00000000-0000-0000-0000-000000000010")
)

func TestWalletRepository_GetMerchantWallet(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewWalletRepository(db, nil)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "created_at", "updated_at"}).
				AddRow("merchant-1", "user-1", int64(1000), now, now))

		merchant, err := repo.GetMerchantWallet("user-1")
		require.NoError(t, err)
		assert.Equal(t, "merchant-1", merchant.ID)
	})

	t.Run("not found", func(t *testing.T) {
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetMerchantWallet("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "merchant wallet not found")
	})
}

func TestWalletRepository_CreateTopup(t *testing.T) {
	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewWalletRepository(db, nil)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		sqlMock.ExpectQuery(regexp.QuoteMeta("INSERT INTO topups")).
			WithArgs(testMerchantUUID.String(), int64(50000)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "amount", "status", "created_at", "updated_at"}).
				AddRow("topup-1", testMerchantUUID.String(), int64(50000), "PENDING", now, now))

		topup, err := repo.CreateTopup(testMerchantUUID.String(), int64(50000))
		require.NoError(t, err)
		assert.Equal(t, "topup-1", topup.ID)
	})
}

func TestWalletRepository_ListTransactions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		ledgerMock := ledgerMocks.NewMockIRepository(t)
		repo := NewWalletRepository(db, ledgerMock)

		ledgerMock.EXPECT().
			GetAccountByMerchantID(mock.Anything, testMerchantUUID).
			Return(ledgerEntity.Account{ID: testAccountUUID}, nil)

		expected := []ledgerEntity.EntryWithTxn{{ID: 1, Reference: "topup:t1"}}
		ledgerMock.EXPECT().
			ListEntriesByAccount(mock.Anything, testAccountUUID, ledgerEntity.EntryFilter{}, 1, 10).
			Return(expected, 1, nil)

		entries, total, err := repo.ListTransactions(testMerchantUUID.String(), ledgerEntity.EntryFilter{}, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, expected[0].Reference, entries[0].Reference)
	})

	t.Run("invalid merchant id", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewWalletRepository(db, nil)
		_, _, err = repo.ListTransactions("not-a-uuid", ledgerEntity.EntryFilter{}, 1, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid merchant id")
	})

	t.Run("account not found", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		ledgerMock := ledgerMocks.NewMockIRepository(t)
		repo := NewWalletRepository(db, ledgerMock)

		ledgerMock.EXPECT().
			GetAccountByMerchantID(mock.Anything, testMerchantUUID).
			Return(ledgerEntity.Account{}, errors.New("not found"))

		_, _, err = repo.ListTransactions(testMerchantUUID.String(), ledgerEntity.EntryFilter{}, 1, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "merchant ledger account not found")
	})
}

func TestWalletRepository_UpdateTopupStatus(t *testing.T) {
	merchantIDStr := testMerchantUUID.String()
	now := time.Now()

	t.Run("success SUCCESS", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		ledgerMock := ledgerMocks.NewMockIRepository(t)
		repo := NewWalletRepository(db, ledgerMock)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT merchant_id::text, amount, status::text FROM topups")).
			WithArgs("topup-1").
			WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
				AddRow(merchantIDStr, int64(100), "PENDING"))

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
			WithArgs("SUCCESS", "topup-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		ledgerMock.EXPECT().
			GetAccountByMerchantID(mock.Anything, testMerchantUUID).
			Return(ledgerEntity.Account{ID: testAccountUUID}, nil)

		ledgerMock.EXPECT().
			Post(mock.Anything, mock.Anything, mock.Anything).
			Return(uuid.New(), nil)

		sqlMock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance = (SELECT balance FROM accounts WHERE id=$1) WHERE id=$2")).
			WithArgs(testAccountUUID, merchantIDStr).
			WillReturnResult(sqlmock.NewResult(0, 1))

		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, amount, status::text, created_at, updated_at FROM topups WHERE id=$1")).
			WithArgs("topup-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "amount", "status", "created_at", "updated_at"}).
				AddRow("topup-1", merchantIDStr, int64(100), "SUCCESS", now, now))

		sqlMock.ExpectCommit()

		topup, err := repo.UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess)
		require.NoError(t, err)
		assert.Equal(t, paymentEntity.PaymentSuccess, topup.Status)
		assert.NoError(t, sqlMock.ExpectationsWereMet())
	})

	t.Run("already finalized", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewWalletRepository(db, nil)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
				AddRow(merchantIDStr, int64(100), "SUCCESS"))
		sqlMock.ExpectRollback()

		_, err = repo.UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topup already finalized")
	})

	t.Run("not found", func(t *testing.T) {
		db, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewWalletRepository(db, nil)

		sqlMock.ExpectBegin()
		sqlMock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnError(sql.ErrNoRows)
		sqlMock.ExpectRollback()

		_, err = repo.UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topup not found")
	})
}
