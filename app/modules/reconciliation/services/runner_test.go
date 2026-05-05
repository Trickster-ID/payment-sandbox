package services

// Branch analysis for Runner.CheckLedgerIntegrity:
// ├── r.DB.QueryContext → error → return nil, err
// └── rows.Next() loop:
//     ├── type in {"liability","revenue","equity"} → expected = -computed
//     │   ├── bal == expected → no discrepancy
//     │   └── bal != expected → append entry
//     └── other type (e.g. "asset") → expected = computed
//         ├── bal == expected → no discrepancy
//         └── bal != expected → append entry
//
// Branch analysis for Runner.CheckTransactionBalance:
// ├── r.DB.QueryContext → error → return nil, err
// └── rows.Next() loop (HAVING filters to only unbalanced rows):
//     └── each row → append entry to bad
//
// Branch analysis for Runner.ReconcileWithProcessor:
// ├── r.DB.QueryContext → error → return nil, err
// └── internal map built from rows; external map built from ext slice
//     ├── ref in internal, not in external → "missing_in_external"
//     ├── ref in both, amounts differ → "amount_mismatch"
//     ├── ref in both, amounts equal → no discrepancy
//     └── ref in external, not in internal → "missing_in_internal"

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── CheckLedgerIntegrity ─────────────────────────────────────────────────────

func TestRunner_CheckLedgerIntegrity(t *testing.T) {
	// not parallel: each subtest creates its own db; kept serial by convention.

	ctx := context.Background()
	cols := []string{"id", "balance", "type", "computed"}

	type args struct{}
	type wants struct {
		bad         []string
		errContains string
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. query error -> nil bad nil returned with error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errContains: "sql: connection is already closed"},
		},
		{
			name: "2. no rows -> nil bad slice no error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols))
			},
			wants: wants{bad: nil},
		},
		{
			name: "3. asset account balanced (bal==computed) -> no discrepancy",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc1", int64(100), "asset", int64(100)))
			},
			wants: wants{bad: nil},
		},
		{
			name: "4. asset account unbalanced (bal!=computed) -> discrepancy entry",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc1", int64(100), "asset", int64(80)))
			},
			// expected=80, diff=100-80=20
			wants: wants{bad: []string{"account acc1 (type=asset): balance=100 expected=80 diff=20"}},
		},
		{
			name: "5. liability account balanced (bal==-computed) -> no discrepancy",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc2", int64(-100), "liability", int64(100)))
			},
			// expected=-computed=-100, bal=-100 → matches
			wants: wants{bad: nil},
		},
		{
			name: "6. liability account unbalanced -> discrepancy entry",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc2", int64(-80), "liability", int64(100)))
			},
			// expected=-100, diff=-80-(-100)=20
			wants: wants{bad: []string{"account acc2 (type=liability): balance=-80 expected=-100 diff=20"}},
		},
		{
			name: "7. revenue account unbalanced -> discrepancy entry",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc3", int64(-30), "revenue", int64(50)))
			},
			// expected=-50, diff=-30-(-50)=20
			wants: wants{bad: []string{"account acc3 (type=revenue): balance=-30 expected=-50 diff=20"}},
		},
		{
			name: "8. equity account unbalanced -> discrepancy entry",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc4", int64(-180), "equity", int64(200)))
			},
			// expected=-200, diff=-180-(-200)=20
			wants: wants{bad: []string{"account acc4 (type=equity): balance=-180 expected=-200 diff=20"}},
		},
		{
			name: "9. mixed accounts (balanced and unbalanced) -> only unbalanced in result",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT a.id").
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("acc1", int64(100), "asset", int64(100)). // balanced
						AddRow("acc2", int64(90), "asset", int64(100)).  // unbalanced, diff=-10
						AddRow("acc3", int64(-100), "liability", int64(100))) // balanced
			},
			// acc2: expected=100, diff=90-100=-10
			wants: wants{bad: []string{"account acc2 (type=asset): balance=90 expected=100 diff=-10"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			r := NewRunner(db)
			bad, err := r.CheckLedgerIntegrity(ctx)

			if tt.wants.errContains != "" {
				require.Error(t, err, "expected an error")
				assert.ErrorContains(t, err, tt.wants.errContains, "error message")
				assert.Nil(t, bad, "bad must be nil on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.bad, bad, "bad entries")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── CheckTransactionBalance ──────────────────────────────────────────────────

func TestRunner_CheckTransactionBalance(t *testing.T) {
	// not parallel: each subtest creates its own db; kept serial by convention.

	ctx := context.Background()
	cols := []string{"transaction_id", "debits", "credits"}

	type wants struct {
		bad         []string
		errContains string
	}

	tests := []struct {
		name      string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. query error -> nil bad returned with error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT transaction_id::text")).
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errContains: "sql: connection is already closed"},
		},
		{
			name: "2. no unbalanced transactions -> nil bad slice no error",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT transaction_id::text")).
					WillReturnRows(sqlmock.NewRows(cols))
			},
			wants: wants{bad: nil},
		},
		{
			name: "3. one unbalanced transaction -> entry in bad",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT transaction_id::text")).
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("tx1", int64(100), int64(80)))
			},
			wants: wants{bad: []string{"tx tx1 unbalanced: D=100 C=80"}},
		},
		{
			name: "4. multiple unbalanced transactions -> all entries in bad",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT transaction_id::text")).
					WillReturnRows(sqlmock.NewRows(cols).
						AddRow("tx1", int64(100), int64(80)).
						AddRow("tx2", int64(200), int64(150)))
			},
			wants: wants{bad: []string{
				"tx tx1 unbalanced: D=100 C=80",
				"tx tx2 unbalanced: D=200 C=150",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			r := NewRunner(db)
			bad, err := r.CheckTransactionBalance(ctx)

			if tt.wants.errContains != "" {
				require.Error(t, err, "expected an error")
				assert.ErrorContains(t, err, tt.wants.errContains, "error message")
				assert.Nil(t, bad, "bad must be nil on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.bad, bad, "bad entries")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── ReconcileWithProcessor ───────────────────────────────────────────────────

func TestRunner_ReconcileWithProcessor(t *testing.T) {
	// not parallel: each subtest creates its own db; kept serial by convention.

	ctx := context.Background()
	day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	queryCols := []string{"reference", "amount"}

	type args struct {
		ext []ExternalRecord
		day time.Time
	}
	type wants struct {
		out         []Discrepancy
		errContains string
	}

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. query error -> nil discrepancies returned with error",
			args: args{ext: nil, day: day},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errContains: "sql: connection is already closed"},
		},
		{
			name: "2. internal empty external empty -> no discrepancies",
			args: args{ext: []ExternalRecord{}, day: day},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(queryCols))
			},
			wants: wants{out: nil},
		},
		{
			name: "3. ref in internal not in external -> missing_in_external",
			args: args{ext: []ExternalRecord{}, day: day},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(queryCols).
						AddRow("REF1", int64(100)))
			},
			wants: wants{out: []Discrepancy{
				{Category: "missing_in_external", Reference: "REF1", InternalAmount: 100},
			}},
		},
		{
			name: "4. ref in both with matching amounts -> no discrepancy",
			args: args{ext: []ExternalRecord{{Reference: "REF1", Amount: 100}}, day: day},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(queryCols).
						AddRow("REF1", int64(100)))
			},
			wants: wants{out: nil},
		},
		{
			name: "5. ref in both with differing amounts -> amount_mismatch",
			args: args{ext: []ExternalRecord{{Reference: "REF1", Amount: 120}}, day: day},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(queryCols).
						AddRow("REF1", int64(100)))
			},
			wants: wants{out: []Discrepancy{
				{Category: "amount_mismatch", Reference: "REF1", InternalAmount: 100, ExternalAmount: 120},
			}},
		},
		{
			name: "6. ref in external not in internal -> missing_in_internal",
			args: args{ext: []ExternalRecord{{Reference: "REF2", Amount: 200}}, day: day},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(queryCols))
			},
			wants: wants{out: []Discrepancy{
				{Category: "missing_in_internal", Reference: "REF2", ExternalAmount: 200},
			}},
		},
		{
			// REF1: internal=100, external=100 → match (no discrepancy)
			// REF2: internal=200, external absent → missing_in_external
			// REF3: internal=300, external=350 → amount_mismatch
			// REF4: internal absent, external=400 → missing_in_internal
			name: "7. mixed: match, missing_in_external, amount_mismatch, missing_in_internal -> 3 discrepancies",
			args: args{
				ext: []ExternalRecord{
					{Reference: "REF1", Amount: 100},
					{Reference: "REF3", Amount: 350},
					{Reference: "REF4", Amount: 400},
				},
				day: day,
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT lt.reference").
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(queryCols).
						AddRow("REF1", int64(100)).
						AddRow("REF2", int64(200)).
						AddRow("REF3", int64(300)))
			},
			wants: wants{out: []Discrepancy{
				{Category: "missing_in_external", Reference: "REF2", InternalAmount: 200},
				{Category: "amount_mismatch", Reference: "REF3", InternalAmount: 300, ExternalAmount: 350},
				{Category: "missing_in_internal", Reference: "REF4", ExternalAmount: 400},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			r := NewRunner(db)
			out, err := r.ReconcileWithProcessor(ctx, tt.args.ext, tt.args.day)

			if tt.wants.errContains != "" {
				require.Error(t, err, "expected an error")
				assert.ErrorContains(t, err, tt.wants.errContains, "error message")
				assert.Nil(t, out, "out must be nil on error")
			} else {
				require.NoError(t, err, "unexpected error")
				// ElementsMatch: map iteration order is non-deterministic
				assert.ElementsMatch(t, tt.wants.out, out, "discrepancies")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}
