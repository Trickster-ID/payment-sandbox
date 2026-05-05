package services

// Branch analysis for Orchestrator.Run:
// ├── INSERT INTO sagas fails → return err
// └── INSERT succeeds, iterate steps:
//     ├── step.Execute fails:
//     │   ├── logStep(forward, err!=nil) → status="failed"       [logStep err!=nil branch]
//     │   ├── update("compensating", err) → 3-param SQL          [update lastErr!="" branch]
//     │   ├── compensation loop (reverse over executed):
//     │   │   ├── executed empty → loop skipped                  [loop-body never runs]
//     │   │   ├── Compensate succeeds → logStep(success), continue
//     │   │   └── Compensate fails   → logStep(failed), update("failed"), return cerr
//     │   └── all compensations succeed → update("compensated","") → return original err
//     └── step.Execute succeeds:
//         ├── logStep(forward, err==nil) → status="success"      [logStep err==nil branch]
//         ├── UPDATE current_step
//         └── all steps done → update("completed","")            [update lastErr=="" branch]
//
// Branch analysis for StartRecoveryLoop:
// ├── <-ctx.Done() → goroutine exits                             [testable]
// └── <-t.C (ticker fires) → DB query + row logging             [requires ticker injection; not tested]

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"payment-sandbox/app/modules/saga/models/entity"
)

// mockStep is a minimal test double for entity.Step.
// No mockery-generated mock exists for this interface; a simple struct double suffices.
type mockStep struct {
	name          string
	executeErr    error
	compensateErr error
}

func (m *mockStep) Name() string                                          { return m.name }
func (m *mockStep) Execute(_ context.Context, _ map[string]any) error    { return m.executeErr }
func (m *mockStep) Compensate(_ context.Context, _ map[string]any) error { return m.compensateErr }

// expectIgnoredExec registers a sqlmock Exec expectation whose return value the SUT ignores (_, _ =).
func expectIgnoredExec(m sqlmock.Sqlmock, queryPattern string) {
	m.ExpectExec(queryPattern).WillReturnResult(sqlmock.NewResult(0, 0))
}

// steps converts a variadic list of *mockStep to []entity.Step.
func steps(ms ...*mockStep) []entity.Step {
	out := make([]entity.Step, len(ms))
	for i, m := range ms {
		out[i] = m
	}
	return out
}

// ─── Orchestrator.Run ─────────────────────────────────────────────────────────

func TestOrchestrator_Run(t *testing.T) {
	// not parallel: sqlmock expectations are strictly sequential; each subtest creates its own db.

	ctx := context.Background()

	type args struct {
		sagaType string
		steps    []entity.Step
		payload  map[string]any
	}
	type wants struct {
		errMsg string // empty → no error expected
	}

	errStep := errors.New("step execute failed")
	errComp := errors.New("compensate failed")

	tests := []struct {
		name      string
		args      args
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. INSERT INTO sagas fails -> error returned",
			args: args{sagaType: "pay", steps: nil, payload: map[string]any{}},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnError(sql.ErrConnDone)
			},
			wants: wants{errMsg: "sql: connection is already closed"},
		},
		{
			name: "2. empty steps -> saga completed nil error",
			args: args{sagaType: "pay", steps: steps(), payload: map[string]any{}},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnResult(sqlmock.NewResult(1, 1))
				expectIgnoredExec(m, "UPDATE sagas SET status") // update("completed","")
			},
			wants: wants{},
		},
		{
			name: "3. single step succeeds -> saga completed nil error",
			args: args{
				sagaType: "pay",
				steps:    steps(&mockStep{name: "step0"}),
				payload:  map[string]any{},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnResult(sqlmock.NewResult(1, 1))
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step0 forward success
				expectIgnoredExec(m, "UPDATE sagas SET current_step") // current_step=1
				expectIgnoredExec(m, "UPDATE sagas SET status")       // update("completed","")
			},
			wants: wants{},
		},
		{
			name: "4. first step fails no prior executed steps -> compensation loop skipped saga compensated returns step error",
			args: args{
				sagaType: "pay",
				steps:    steps(&mockStep{name: "step0", executeErr: errStep}),
				payload:  map[string]any{},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnResult(sqlmock.NewResult(1, 1))
				expectIgnoredExec(m, "INSERT INTO saga_step_log") // logStep: step0 forward failed
				expectIgnoredExec(m, "UPDATE sagas SET status")   // update("compensating", err)
				// executed=[] → for-loop condition false immediately, body never runs
				expectIgnoredExec(m, "UPDATE sagas SET status") // update("compensated","")
			},
			wants: wants{errMsg: "step execute failed"},
		},
		{
			name: "5. step0 succeeds step1 fails compensation of step0 succeeds -> saga compensated returns step1 error",
			args: args{
				sagaType: "pay",
				steps: steps(
					&mockStep{name: "step0"},
					&mockStep{name: "step1", executeErr: errStep},
				),
				payload: map[string]any{},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnResult(sqlmock.NewResult(1, 1))
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step0 forward success
				expectIgnoredExec(m, "UPDATE sagas SET current_step") // current_step=1
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step1 forward failed
				expectIgnoredExec(m, "UPDATE sagas SET status")       // update("compensating")
				// compensation loop: j=0 → compensate step0 → success
				expectIgnoredExec(m, "INSERT INTO saga_step_log") // logStep: step0 compensate success
				expectIgnoredExec(m, "UPDATE sagas SET status")   // update("compensated","")
			},
			wants: wants{errMsg: "step execute failed"},
		},
		{
			name: "6. step0 succeeds step1 fails compensation of step0 fails -> saga failed returns compensation error",
			args: args{
				sagaType: "pay",
				steps: steps(
					&mockStep{name: "step0", compensateErr: errComp},
					&mockStep{name: "step1", executeErr: errStep},
				),
				payload: map[string]any{},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnResult(sqlmock.NewResult(1, 1))
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step0 forward success
				expectIgnoredExec(m, "UPDATE sagas SET current_step") // current_step=1
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step1 forward failed
				expectIgnoredExec(m, "UPDATE sagas SET status")       // update("compensating")
				// compensation loop: j=0 → compensate step0 → fails
				expectIgnoredExec(m, "INSERT INTO saga_step_log") // logStep: step0 compensate failed
				expectIgnoredExec(m, "UPDATE sagas SET status")   // update("failed","compensation failed at step 0: ...")
			},
			wants: wants{errMsg: "compensate failed"},
		},
		{
			name: "7. two steps both succeed -> saga completed nil error (covers multi-step current_step path)",
			args: args{
				sagaType: "pay",
				steps:    steps(&mockStep{name: "step0"}, &mockStep{name: "step1"}),
				payload:  map[string]any{},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO sagas").WillReturnResult(sqlmock.NewResult(1, 1))
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step0 forward success
				expectIgnoredExec(m, "UPDATE sagas SET current_step") // current_step=1
				expectIgnoredExec(m, "INSERT INTO saga_step_log")     // logStep: step1 forward success
				expectIgnoredExec(m, "UPDATE sagas SET current_step") // current_step=2
				expectIgnoredExec(m, "UPDATE sagas SET status")       // update("completed","")
			},
			wants: wants{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			tt.setupMock(dbMock)

			o := NewOrchestrator(db)
			runErr := o.Run(ctx, tt.args.sagaType, tt.args.steps, tt.args.payload)

			if tt.wants.errMsg != "" {
				require.Error(t, runErr, "expected an error")
				assert.EqualError(t, runErr, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, runErr, "unexpected error")
			}

			assert.NoError(t, dbMock.ExpectationsWereMet(), "all sqlmock expectations must be met")
		})
	}
}

// ─── Orchestrator.StartRecoveryLoop ───────────────────────────────────────────

func TestOrchestrator_StartRecoveryLoop(t *testing.T) {
	// not parallel: each subtest creates its own db; kept serial for consistency.
	//
	// NOTE: the ticker path (case <-t.C) is not tested here because the ticker
	// interval is hardcoded to time.Minute. Testing it would require injecting a
	// clock or ticker — a code change outside the scope of this plan.

	type wants struct {
		noDBCalls bool
	}

	tests := []struct {
		name     string
		setupCtx func() (context.Context, context.CancelFunc)
		wants    wants
	}{
		{
			name: "1. context already cancelled -> goroutine exits immediately no DB calls made",
			setupCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // pre-cancel so the first select picks ctx.Done()
				return ctx, cancel
			},
			wants: wants{noDBCalls: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: see note above

			db, dbMock, err := sqlmock.New()
			require.NoError(t, err, "sqlmock.New")
			t.Cleanup(func() { db.Close() })

			ctx, cancel := tt.setupCtx()
			defer cancel()

			o := NewOrchestrator(db)
			o.StartRecoveryLoop(ctx) // returns immediately; goroutine exits asynchronously

			// brief sleep so the goroutine is scheduled and observes ctx.Done()
			time.Sleep(50 * time.Millisecond)

			if tt.wants.noDBCalls {
				assert.NoError(t, dbMock.ExpectationsWereMet(), "no DB calls expected when context already cancelled")
			}
		})
	}
}
