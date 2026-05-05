package main

import (
	"context"
	"errors"
	"testing"

	runnerMocks "payment-sandbox/app/modules/reconciliation/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Branch analysis for executeReconcile(ctx, runner):
// ├── runner.CheckTransactionBalance returns error          → exitCode=2  [Case 1]
// ├── runner.CheckTransactionBalance returns bad len>0     → exitCode=1  [Case 2]
// ├── runner.CheckLedgerIntegrity returns error            → exitCode=2  [Case 3]
// ├── runner.CheckLedgerIntegrity returns bad len>0        → exitCode=1  [Case 4]
// ├── both checks pass (empty bad slices)                  → exitCode=0  [Case 5]
// ├── CheckTransactionBalance error + CheckLedgerIntegrity bad → exitCode=1 (ledger overrides)  [Case 6]
// └── CheckTransactionBalance bad + CheckLedgerIntegrity error → exitCode=2 (ledger error overrides) [Case 7]

func TestExecuteReconcile(t *testing.T) {
	type fields struct {
		runner *runnerMocks.MockIRunner
	}
	type args struct {
		ctx context.Context
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		exitCode int
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. CheckTransactionBalance returns error -> exitCode=2",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return(nil, errors.New("db connection refused")).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return(nil, nil).
						Once()
				},
			},
			wants: wants{exitCode: 2},
		},
		{
			name:   "2. CheckTransactionBalance returns unbalanced records -> exitCode=1",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return([]string{"tx abc unbalanced: D=100 C=50"}, nil).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return(nil, nil).
						Once()
				},
			},
			wants: wants{exitCode: 1},
		},
		{
			name:   "3. CheckLedgerIntegrity returns error -> exitCode=2",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return(nil, nil).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return(nil, errors.New("query timeout")).
						Once()
				},
			},
			wants: wants{exitCode: 2},
		},
		{
			name:   "4. CheckLedgerIntegrity returns mismatched accounts -> exitCode=1",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return(nil, nil).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return([]string{"account acc-1 (type=asset): balance=500 expected=600 diff=-100"}, nil).
						Once()
				},
			},
			wants: wants{exitCode: 1},
		},
		{
			name:   "5. both checks pass with empty results -> exitCode=0",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return(nil, nil).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return(nil, nil).
						Once()
				},
			},
			wants: wants{exitCode: 0},
		},
		{
			name:   "6. CheckTransactionBalance error + CheckLedgerIntegrity bad records -> exitCode=1 (ledger overrides)",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return(nil, errors.New("tx check failed")).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return([]string{"account acc-2 mismatch"}, nil).
						Once()
				},
			},
			wants: wants{exitCode: 1},
		},
		{
			name:   "7. CheckTransactionBalance bad records + CheckLedgerIntegrity error -> exitCode=2 (ledger error overrides)",
			fields: fields{runner: runnerMocks.NewMockIRunner(t)},
			args:   args{ctx: context.Background()},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.runner.EXPECT().
						CheckTransactionBalance(mock.Anything).
						Return([]string{"tx xyz unbalanced"}, nil).
						Once()
					f.runner.EXPECT().
						CheckLedgerIntegrity(mock.Anything).
						Return(nil, errors.New("integrity check failed")).
						Once()
				},
			},
			wants: wants{exitCode: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			got := executeReconcile(tt.args.ctx, tt.fields.runner)

			assert.Equal(t, tt.wants.exitCode, got, "exit code")

			tt.fields.runner.AssertExpectations(t)
		})
	}
}
