package repositories

import (
	"database/sql"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletRepository_GetMerchantWallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewWalletRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance::double precision, created_at, updated_at FROM merchants")).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "created_at", "updated_at"}).
				AddRow("merchant-1", "user-1", 1000.0, now, now))

		merchant, err := repo.GetMerchantWallet("user-1")
		require.NoError(t, err)
		assert.Equal(t, "merchant-1", merchant.ID)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetMerchantWallet("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "merchant wallet not found")
	})
}

func TestWalletRepository_CreateTopup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewWalletRepository(db)
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO topups")).
			WithArgs("merchant-1", 50000.0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "amount", "status", "created_at", "updated_at"}).
				AddRow("topup-1", "merchant-1", 50000.0, "PENDING", now, now))

		topup, err := repo.CreateTopup("merchant-1", 50000.0)
		require.NoError(t, err)
		assert.Equal(t, "topup-1", topup.ID)
	})
}

func TestWalletRepository_UpdateTopupStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewWalletRepository(db)
	now := time.Now()

	t.Run("success SUCCESS", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT merchant_id::text, amount::double precision, status::text FROM topups")).
			WithArgs("topup-1").
			WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
				AddRow("merchant-1", 100.0, "PENDING"))
		
		mock.ExpectExec(regexp.QuoteMeta("UPDATE topups SET status=$1")).
			WithArgs("SUCCESS", "topup-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectExec(regexp.QuoteMeta("UPDATE merchants SET balance = balance + $1")).
			WithArgs(100.0, "merchant-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, amount::double precision, status::text, created_at, updated_at FROM topups WHERE id=$1")).
			WithArgs("topup-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "merchant_id", "amount", "status", "created_at", "updated_at"}).
				AddRow("topup-1", "merchant-1", 100.0, "SUCCESS", now, now))
		
		mock.ExpectCommit()

		topup, err := repo.UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess)
		require.NoError(t, err)
		assert.Equal(t, paymentEntity.PaymentSuccess, topup.Status)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("already finalized", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "amount", "status"}).
				AddRow("merchant-1", 100.0, "SUCCESS"))
		mock.ExpectRollback()

		_, err := repo.UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topup already finalized")
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		_, err := repo.UpdateTopupStatus("topup-1", paymentEntity.PaymentSuccess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topup not found")
	})
}
