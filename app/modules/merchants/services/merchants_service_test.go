package services

// Branch analysis for MerchantsService.ListMerchants:
// └── s.repo.ListMerchants(ctx, strings.TrimSpace(search), page, limit)
//     ├── search with whitespace → trimmed value forwarded to repo
//     ├── whitespace-only search → trimmed to "" forwarded to repo
//     └── repo error → propagated as-is

import (
	"context"
	"errors"
	"testing"

	"payment-sandbox/app/modules/merchants/models/entity"
	"payment-sandbox/app/modules/merchants/repositories"
	repoMocks "payment-sandbox/app/modules/merchants/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMerchantsService_ListMerchants(t *testing.T) {
	ctx := context.Background()

	type fields struct {
		repo repositories.IMerchantsRepository
	}
	type args struct {
		search string
		page   int
		limit  int
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		items  []entity.MerchantSummary
		total  int
		errMsg string
	}

	alice := entity.MerchantSummary{ID: "m1", Name: "Alice", Email: "alice@example.com"}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. search with surrounding whitespace -> trimmed before delegating to repo",
			fields: fields{repo: repoMocks.NewMockIMerchantsRepository(t)},
			args:   args{search: "  alice  ", page: 1, limit: 20},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIMerchantsRepository).EXPECT().
					ListMerchants(mock.Anything, "alice", 1, 20).
					Return([]entity.MerchantSummary{alice}, 1, nil).
					Once()
			}},
			wants: wants{
				items: []entity.MerchantSummary{alice},
				total: 1,
			},
		},
		{
			name:   "2. empty search passes through unchanged",
			fields: fields{repo: repoMocks.NewMockIMerchantsRepository(t)},
			args:   args{search: "", page: 1, limit: 20},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIMerchantsRepository).EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return([]entity.MerchantSummary{alice, {ID: "m2"}}, 2, nil).
					Once()
			}},
			wants: wants{
				items: []entity.MerchantSummary{alice, {ID: "m2"}},
				total: 2,
			},
		},
		{
			name:   "3. whitespace-only search -> trimmed to empty string",
			fields: fields{repo: repoMocks.NewMockIMerchantsRepository(t)},
			args:   args{search: "   ", page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIMerchantsRepository).EXPECT().
					ListMerchants(mock.Anything, "", 1, 10).
					Return([]entity.MerchantSummary{}, 0, nil).
					Once()
			}},
			wants: wants{
				items: []entity.MerchantSummary{},
				total: 0,
			},
		},
		{
			name:   "4. already-trimmed search -> forwarded unchanged to repo",
			fields: fields{repo: repoMocks.NewMockIMerchantsRepository(t)},
			args:   args{search: "bob", page: 2, limit: 5},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIMerchantsRepository).EXPECT().
					ListMerchants(mock.Anything, "bob", 2, 5).
					Return([]entity.MerchantSummary{{ID: "m3", Name: "Bob"}}, 1, nil).
					Once()
			}},
			wants: wants{
				items: []entity.MerchantSummary{{ID: "m3", Name: "Bob"}},
				total: 1,
			},
		},
		{
			name:   "5. repo returns error -> error propagated as-is",
			fields: fields{repo: repoMocks.NewMockIMerchantsRepository(t)},
			args:   args{search: "", page: 1, limit: 20},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIMerchantsRepository).EXPECT().
					ListMerchants(mock.Anything, "", 1, 20).
					Return(nil, 0, errors.New("db error")).
					Once()
			}},
			wants: wants{errMsg: "db error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.mocks.setup != nil {
				tt.mocks.setup(tt.fields, tt.args)
			}

			svc := NewMerchantsService(tt.fields.repo)
			items, total, err := svc.ListMerchants(ctx, tt.args.search, tt.args.page, tt.args.limit)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.total, total, "total")
				assert.Equal(t, tt.wants.items, items, "items")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIMerchantsRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
