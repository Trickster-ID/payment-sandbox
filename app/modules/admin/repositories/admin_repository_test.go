package repositories

// Branch analysis for buildFilter(merchantColumn, dateColumn, filter):
// ├── filter.MerchantID != "" → appends " AND merchantCol = $N"
// ├── filter.MerchantID == "" → no merchant clause
// ├── filter.StartDate != nil → appends " AND dateCol >= $N"
// ├── filter.StartDate == nil → no start clause
// ├── filter.EndDate != nil   → appends " AND dateCol <= $N"
// └── filter.EndDate == nil   → no end clause
//
// Branch analysis for AdminRepository.DashboardStats(filter):
// ├── expireDueInvoices → UPDATE invoices (errors silently ignored)
// ├── 6 SELECT queries (errors silently ignored, scan fills stats)
// └── stats assembled and returned
//     Sub-cases: empty filter / merchant filter / date filter

import (
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	adminEntity "payment-sandbox/app/modules/admin/models/entity"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── buildFilter ─────────────────────────────────────────────────────────────

func TestBuildFilter(t *testing.T) {
	// not parallel: pure function with no shared state, kept serial for simplicity

	now := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC)

	type args struct {
		merchantColumn string
		dateColumn     string
		filter         adminEntity.StatsFilter
	}
	type wants struct {
		clause string
		args   []any
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. all fields empty/nil -> empty clause and empty args",
			args: args{
				merchantColumn: "i.merchant_id::text",
				dateColumn:     "i.created_at",
				filter:         adminEntity.StatsFilter{},
			},
			wants: wants{clause: "", args: []any{}},
		},
		{
			name: "2. MerchantID only -> merchant clause with $1",
			args: args{
				merchantColumn: "i.merchant_id::text",
				dateColumn:     "i.created_at",
				filter:         adminEntity.StatsFilter{MerchantID: "m-1"},
			},
			wants: wants{
				clause: " AND i.merchant_id::text = $1",
				args:   []any{"m-1"},
			},
		},
		{
			name: "3. StartDate only -> start clause with $1",
			args: args{
				merchantColumn: "i.merchant_id::text",
				dateColumn:     "i.created_at",
				filter:         adminEntity.StatsFilter{StartDate: &now},
			},
			wants: wants{
				clause: " AND i.created_at >= $1",
				args:   []any{now},
			},
		},
		{
			name: "4. EndDate only -> end clause with $1",
			args: args{
				merchantColumn: "i.merchant_id::text",
				dateColumn:     "i.created_at",
				filter:         adminEntity.StatsFilter{EndDate: &end},
			},
			wants: wants{
				clause: " AND i.created_at <= $1",
				args:   []any{end},
			},
		},
		{
			name: "5. all three fields set -> three clauses with $1 $2 $3 in order",
			args: args{
				merchantColumn: "i.merchant_id::text",
				dateColumn:     "i.created_at",
				filter: adminEntity.StatsFilter{
					MerchantID: "m-1",
					StartDate:  &now,
					EndDate:    &end,
				},
			},
			wants: wants{
				clause: " AND i.merchant_id::text = $1 AND i.created_at >= $2 AND i.created_at <= $3",
				args:   []any{"m-1", now, end},
			},
		},
		{
			name: "6. MerchantID + StartDate, no EndDate -> two clauses $1 $2",
			args: args{
				merchantColumn: "inv.merchant_id::text",
				dateColumn:     "pi.created_at",
				filter: adminEntity.StatsFilter{
					MerchantID: "m-2",
					StartDate:  &now,
				},
			},
			wants: wants{
				clause: " AND inv.merchant_id::text = $1 AND pi.created_at >= $2",
				args:   []any{"m-2", now},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: no shared state here, kept serial for consistency with DB tests below

			gotClause, gotArgs := buildFilter(tt.args.merchantColumn, tt.args.dateColumn, tt.args.filter)

			assert.Equal(t, tt.wants.clause, gotClause, "clause")
			assert.Equal(t, tt.wants.args, gotArgs, "args")
		})
	}
}

// ─── AdminRepository.DashboardStats ──────────────────────────────────────────

// expectDashboardQueries registers the 7 ordered sqlmock expectations for
// DashboardStats: 1 Exec (expireDueInvoices) + 6 Queries.
func expectDashboardQueries(
	m sqlmock.Sqlmock,
	totalInvoice, paid, expired, failed int,
	paymentNominal, refundNominal int64,
	extraArgs ...interface{}, // appended to each query when filter present
) {
	m.ExpectExec(regexp.QuoteMeta("UPDATE invoices")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	invoiceBase := `SELECT COUNT(*) FROM invoices i WHERE i.deleted_at IS NULL`
	invoicePaid := invoiceBase + ` AND i.status='PAID'`
	invoiceExp  := invoiceBase + ` AND i.status='EXPIRED'`

	driverArgs := make([]driver.Value, len(extraArgs))
	for i, v := range extraArgs {
		driverArgs[i] = v
	}
	withArgs := func(q *sqlmock.ExpectedQuery) *sqlmock.ExpectedQuery {
		if len(driverArgs) > 0 {
			return q.WithArgs(driverArgs...)
		}
		return q
	}

	withArgs(m.ExpectQuery(regexp.QuoteMeta(invoiceBase))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(totalInvoice))
	withArgs(m.ExpectQuery(regexp.QuoteMeta(invoicePaid))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(paid))
	withArgs(m.ExpectQuery(regexp.QuoteMeta(invoiceExp))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(expired))
	withArgs(m.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*)`))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(failed))
	withArgs(m.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(inv.amount), 0)`))).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(paymentNominal))
	withArgs(m.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(inv.amount), 0)`))).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(refundNominal))
}

func TestAdminRepository_DashboardStats(t *testing.T) {
	// not parallel: sqlmock expectations are sequential per db instance;
	// each subtest creates its own db to avoid cross-contamination.

	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end   := time.Date(2026, time.April, 30, 23, 59, 59, 0, time.UTC)

	type args struct {
		filter adminEntity.StatsFilter
	}
	type wants struct {
		totalInvoice   int
		paid           int
		expired        int
		failed         int
		paymentNominal int64
		refundNominal  int64
	}

	tests := []struct {
		name       string
		args       args
		setupMock  func(m sqlmock.Sqlmock)
		wants      wants
	}{
		{
			name: "1. empty filter -> all 7 queries executed without args, stats aggregated",
			args: args{filter: adminEntity.StatsFilter{}},
			setupMock: func(m sqlmock.Sqlmock) {
				expectDashboardQueries(m, 10, 5, 2, 1, 5_000, 100)
			},
			wants: wants{
				totalInvoice:   10,
				paid:           5,
				expired:        2,
				failed:         1,
				paymentNominal: 5_000,
				refundNominal:  100,
			},
		},
		{
			name: "2. merchant filter -> all 7 queries executed with merchant $1 arg",
			args: args{filter: adminEntity.StatsFilter{MerchantID: "m-1"}},
			setupMock: func(m sqlmock.Sqlmock) {
				expectDashboardQueries(m, 3, 2, 0, 0, 2_000, 0, "m-1")
			},
			wants: wants{
				totalInvoice:   3,
				paid:           2,
				expired:        0,
				failed:         0,
				paymentNominal: 2_000,
				refundNominal:  0,
			},
		},
		{
			name: "3. date filter (start + end) -> queries executed with $1 $2 date args",
			args: args{filter: adminEntity.StatsFilter{StartDate: &start, EndDate: &end}},
			setupMock: func(m sqlmock.Sqlmock) {
				expectDashboardQueries(m, 7, 4, 1, 2, 8_000, 500, start, end)
			},
			wants: wants{
				totalInvoice:   7,
				paid:           4,
				expired:        1,
				failed:         2,
				paymentNominal: 8_000,
				refundNominal:  500,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, mock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(mock)

			repo := NewAdminRepository(db)
			stats := repo.DashboardStats(tt.args.filter)

			assert.Equal(t, tt.wants.totalInvoice, stats.TotalInvoiceCreated, "TotalInvoiceCreated")
			assert.Equal(t, tt.wants.paid, stats.TotalByStatus["PAID"], "TotalByStatus[PAID]")
			assert.Equal(t, tt.wants.expired, stats.TotalByStatus["EXPIRED"], "TotalByStatus[EXPIRED]")
			assert.Equal(t, tt.wants.failed, stats.TotalByStatus["FAILED"], "TotalByStatus[FAILED]")
			assert.Equal(t, tt.wants.paymentNominal, stats.TotalPaymentNominal, "TotalPaymentNominal")
			assert.Equal(t, tt.wants.refundNominal, stats.TotalRefundNominal, "TotalRefundNominal")

			assert.NoError(t, mock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}
