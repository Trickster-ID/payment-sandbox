package services

import (
	"context"
	"errors"
	"testing"

	"payment-sandbox/app/modules/merchants/models/entity"
	repoMocks "payment-sandbox/app/modules/merchants/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerchantsService_ListMerchants(t *testing.T) {
	ctx := context.Background()

	t.Run("trims whitespace before delegating", func(t *testing.T) {
		repo := repoMocks.NewMockIMerchantsRepository(t)
		repo.EXPECT().ListMerchants(ctx, "alice", 1, 20).
			Return([]entity.MerchantSummary{{ID: "m1", Name: "Alice", Email: "alice@example.com"}}, 1, nil)

		svc := NewMerchantsService(repo)
		items, total, err := svc.ListMerchants(ctx, "  alice  ", 1, 20)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, items, 1)
		assert.Equal(t, "m1", items[0].ID)
	})

	t.Run("empty search passes through", func(t *testing.T) {
		repo := repoMocks.NewMockIMerchantsRepository(t)
		expected := []entity.MerchantSummary{{ID: "m1"}, {ID: "m2"}}
		repo.EXPECT().ListMerchants(ctx, "", 1, 20).Return(expected, 2, nil)

		svc := NewMerchantsService(repo)
		items, total, err := svc.ListMerchants(ctx, "", 1, 20)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, items, 2)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := repoMocks.NewMockIMerchantsRepository(t)
		repo.EXPECT().ListMerchants(ctx, "", 1, 20).Return(nil, 0, errors.New("db error"))

		svc := NewMerchantsService(repo)
		_, _, err := svc.ListMerchants(ctx, "", 1, 20)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}
