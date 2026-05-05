package repositories

// Branch map (used to derive test cases per Section 3.1 of the plan):
//
// MerchantIDByUserID:
// ├── getMerchantByUserID returns error  -> "merchant not found"
// └── getMerchantByUserID succeeds       -> merchant.ID, nil
//
// CreateInvoice:
// ├── db INSERT fails  -> Invoice{}, error
// └── db INSERT ok     -> populated Invoice, nil
//
// ListInvoices:
// ├── count query fails                  -> []Invoice{}, 0
// ├── list query fails                   -> []Invoice{}, total (count preserved)
// ├── with status filter                 -> WHERE includes AND status=$2
// ├── without status filter              -> WHERE omits status clause
// ├── page < 1                           -> sanitized to page 1, offset 0
// ├── limit = 0                          -> sanitized to 10
// └── limit > 100                        -> sanitized to 10
//
// MerchantInvoiceByID:
// ├── db SELECT fails  -> Invoice{}, "invoice not found"
// └── db SELECT ok     -> populated Invoice, nil

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceRepository_MerchantIDByUserID(t *testing.T) {
	now := time.Now()

	type args struct {
		userID string
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		merchantID string
		err        string
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. valid user ID -> merchant ID returned",
			args: args{userID: "user-1"},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
						WithArgs("user-1").
						WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "created_at", "updated_at"}).
							AddRow("merchant-1", "user-1", 0.0, now, now))
				},
			},
			wants: wants{merchantID: "merchant-1"},
		},
		{
			name: "2. user has no merchant record -> merchant not found error",
			args: args{userID: "unknown"},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, user_id::text, balance, created_at, updated_at FROM merchants")).
						WithArgs("unknown").
						WillReturnError(sql.ErrNoRows)
				},
			},
			wants: wants{err: "merchant not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			repo := NewInvoiceRepository(db)
			tt.mocks.setup(sqlMock)

			merchantID, err := repo.MerchantIDByUserID(tt.args.userID)

			if tt.wants.err != "" {
				assert.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, merchantID, "merchant ID should be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.merchantID, merchantID, "merchant ID")
			}
			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}

func TestInvoiceRepository_CreateInvoice(t *testing.T) {
	now := time.Now()
	dueDate := now.Add(24 * time.Hour)

	type args struct {
		merchantID    string
		customerName  string
		customerEmail string
		amount        int64
		description   string
		dueDate       time.Time
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		invoiceID string
		err       string
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. valid invoice data -> invoice created and returned",
			args: args{
				merchantID:    "merchant-1",
				customerName:  "Alice",
				customerEmail: "alice@example.com",
				amount:        1000,
				description:   "desc",
				dueDate:       dueDate,
			},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO invoices")).
						WithArgs("merchant-1", sqlmock.AnyArg(), "Alice", "alice@example.com", int64(1000), "desc", dueDate, sqlmock.AnyArg()).
						WillReturnRows(sqlmock.NewRows([]string{
							"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
							"amount", "description", "due_date", "status", "payment_link_token",
							"created_at", "updated_at",
						}).AddRow("inv-1", "merchant-1", "INV-ABC", "Alice", "alice@example.com",
							int64(1000), "desc", dueDate, "PENDING", "token-1", now, now))
				},
			},
			wants: wants{invoiceID: "inv-1"},
		},
		{
			name: "2. db insert fails -> empty invoice and error returned",
			args: args{
				merchantID:    "merchant-1",
				customerName:  "Alice",
				customerEmail: "alice@example.com",
				amount:        1000,
				description:   "desc",
				dueDate:       dueDate,
			},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectQuery(regexp.QuoteMeta("INSERT INTO invoices")).
						WithArgs("merchant-1", sqlmock.AnyArg(), "Alice", "alice@example.com", int64(1000), "desc", dueDate, sqlmock.AnyArg()).
						WillReturnError(sql.ErrConnDone)
				},
			},
			wants: wants{err: "sql: connection is already closed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			repo := NewInvoiceRepository(db)
			tt.mocks.setup(sqlMock)

			invoice, err := repo.CreateInvoice(
				tt.args.merchantID, tt.args.customerName, tt.args.customerEmail,
				tt.args.amount, tt.args.description, tt.args.dueDate,
			)

			if tt.wants.err != "" {
				assert.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, invoice.ID, "invoice ID should be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.invoiceID, invoice.ID, "invoice ID")
			}
			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}

func TestInvoiceRepository_ListInvoices(t *testing.T) {
	now := time.Now()
	invoiceCols := []string{
		"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
		"amount", "description", "due_date", "status", "token", "created_at", "updated_at",
	}

	type args struct {
		merchantID string
		status     string
		options    invoiceEntity.ListOptions
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		total int
		len   int
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. status filter with matching rows -> items and total returned",
			args: args{merchantID: "merchant-1", status: "PAID", options: invoiceEntity.ListOptions{Page: 1, Limit: 10}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL AND status=$2")).
						WithArgs("merchant-1", "PAID").
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text")).
						WithArgs("merchant-1", "PAID", 10, 0).
						WillReturnRows(sqlmock.NewRows(invoiceCols).
							AddRow("inv-1", "merchant-1", "INV-1", "Alice", "alice@example.com", int64(1000), "", now, "PAID", "tok", now, now))
				},
			},
			wants: wants{total: 1, len: 1},
		},
		{
			name: "2. no status filter -> all invoices for merchant returned",
			args: args{merchantID: "merchant-1", status: "", options: invoiceEntity.ListOptions{Page: 1, Limit: 5}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL")).
						WithArgs("merchant-1").
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text")).
						WithArgs("merchant-1", 5, 0).
						WillReturnRows(sqlmock.NewRows(invoiceCols).
							AddRow("inv-1", "merchant-1", "INV-1", "Alice", "alice@example.com", int64(1000), "", now, "PAID", "tok", now, now).
							AddRow("inv-2", "merchant-1", "INV-2", "Bob", "bob@example.com", int64(2000), "", now, "PENDING", "tok2", now, now))
				},
			},
			wants: wants{total: 2, len: 2},
		},
		{
			name: "3. count query fails -> empty slice, zero total",
			args: args{merchantID: "merchant-1", status: "PENDING", options: invoiceEntity.ListOptions{Page: 1, Limit: 10}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices")).
						WillReturnError(sql.ErrConnDone)
				},
			},
			wants: wants{total: 0, len: 0},
		},
		{
			name: "4. list query fails -> empty slice, count preserved",
			args: args{merchantID: "merchant-1", status: "PAID", options: invoiceEntity.ListOptions{Page: 1, Limit: 10}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL AND status=$2")).
						WithArgs("merchant-1", "PAID").
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text")).
						WillReturnError(sql.ErrConnDone)
				},
			},
			wants: wants{total: 3, len: 0},
		},
		{
			name: "5. page 0 -> sanitized to page 1, offset 0",
			args: args{merchantID: "merchant-1", status: "", options: invoiceEntity.ListOptions{Page: 0, Limit: 10}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL")).
						WithArgs("merchant-1").
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
					// offset = (1-1)*10 = 0 confirms page was sanitized to 1
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text")).
						WithArgs("merchant-1", 10, 0).
						WillReturnRows(sqlmock.NewRows(invoiceCols))
				},
			},
			wants: wants{total: 0, len: 0},
		},
		{
			name: "6. limit exceeds max (200) -> sanitized to 10",
			args: args{merchantID: "merchant-1", status: "", options: invoiceEntity.ListOptions{Page: 1, Limit: 200}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL")).
						WithArgs("merchant-1").
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
					// limit=10 confirms 200 was clamped down
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text")).
						WithArgs("merchant-1", 10, 0).
						WillReturnRows(sqlmock.NewRows(invoiceCols))
				},
			},
			wants: wants{total: 0, len: 0},
		},
		{
			name: "7. limit 0 -> sanitized to 10",
			args: args{merchantID: "merchant-1", status: "", options: invoiceEntity.ListOptions{Page: 1, Limit: 0}},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM invoices WHERE merchant_id=$1 AND deleted_at IS NULL")).
						WithArgs("merchant-1").
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
					// limit=10 confirms 0 was replaced with default
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text")).
						WithArgs("merchant-1", 10, 0).
						WillReturnRows(sqlmock.NewRows(invoiceCols))
				},
			},
			wants: wants{total: 0, len: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			repo := NewInvoiceRepository(db)
			tt.mocks.setup(sqlMock)

			items, total := repo.ListInvoices(tt.args.merchantID, tt.args.status, tt.args.options)

			assert.Equal(t, tt.wants.total, total, "total count")
			assert.Len(t, items, tt.wants.len, "items count")
			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}

func TestInvoiceRepository_MerchantInvoiceByID(t *testing.T) {
	now := time.Now()

	type args struct {
		invoiceID  string
		merchantID string
	}
	type mocks struct {
		setup func(m sqlmock.Sqlmock)
	}
	type wants struct {
		invoiceID string
		err       string
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		wants wants
	}{
		{
			name: "1. invoice exists for merchant -> invoice returned",
			args: args{invoiceID: "inv-1", merchantID: "merchant-1"},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name")).
						WithArgs("inv-1", "merchant-1").
						WillReturnRows(sqlmock.NewRows([]string{
							"id", "merchant_id", "invoice_number", "customer_name", "customer_email",
							"amount", "description", "due_date", "status", "payment_link_token",
							"created_at", "updated_at",
						}).AddRow("inv-1", "merchant-1", "INV-1", "Alice", "alice@example.com",
							int64(1000), "", now, "PENDING", "token", now, now))
				},
			},
			wants: wants{invoiceID: "inv-1"},
		},
		{
			name: "2. invoice not found for merchant -> invoice not found error",
			args: args{invoiceID: "inv-99", merchantID: "merchant-1"},
			mocks: mocks{
				setup: func(m sqlmock.Sqlmock) {
					m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(regexp.QuoteMeta("SELECT id::text, merchant_id::text, invoice_number, customer_name")).
						WithArgs("inv-99", "merchant-1").
						WillReturnError(sql.ErrNoRows)
				},
			},
			wants: wants{err: "invoice not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: sqlmock expectations are ordered per instance
			db, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })

			repo := NewInvoiceRepository(db)
			tt.mocks.setup(sqlMock)

			invoice, err := repo.MerchantInvoiceByID(tt.args.invoiceID, tt.args.merchantID)

			if tt.wants.err != "" {
				assert.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, invoice.ID, "invoice ID should be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.invoiceID, invoice.ID, "invoice ID")
			}
			assert.NoError(t, sqlMock.ExpectationsWereMet(), "all sql expectations met")
		})
	}
}
