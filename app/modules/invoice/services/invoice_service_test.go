package services

import (
	"errors"
	"testing"
	"time"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	repoMocks "payment-sandbox/app/modules/invoice/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceService_CreateInvoice(t *testing.T) {
	dueDate := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	dueDateRFC3339 := dueDate.Format(time.RFC3339)

	tests := []struct {
		name  string
		input struct {
			userID, customerName, customerEmail, description, dueDate string
			amount                                                    float64
		}
		setupMocks func(repo *repoMocks.MockIInvoiceRepository)
		wantID     string
		wantErr    string
	}{
		{
			name: "merchant lookup failed",
			input: struct {
				userID, customerName, customerEmail, description, dueDate string
				amount                                                    float64
			}{
				userID:        "user-1",
				customerName:  "Alice",
				customerEmail: "alice@example.com",
				description:   "invoice 1",
				dueDate:       dueDateRFC3339,
				amount:        10000,
			},
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found"))
			},
			wantErr: "merchant not found",
		},
		{
			name: "invalid due date format",
			input: struct {
				userID, customerName, customerEmail, description, dueDate string
				amount                                                    float64
			}{
				userID:        "user-1",
				customerName:  "Alice",
				customerEmail: "alice@example.com",
				description:   "invoice 1",
				dueDate:       "2026-04-30",
				amount:        10000,
			},
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
			},
			wantErr: "due_date must use RFC3339 format",
		},
		{
			name: "invalid customer email",
			input: struct {
				userID, customerName, customerEmail, description, dueDate string
				amount                                                    float64
			}{
				userID:        "user-1",
				customerName:  "Alice",
				customerEmail: "alice.example.com",
				description:   "invoice 1",
				dueDate:       dueDateRFC3339,
				amount:        10000,
			},
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
			},
			wantErr: "customer_email is invalid",
		},
		{
			name: "success",
			input: struct {
				userID, customerName, customerEmail, description, dueDate string
				amount                                                    float64
			}{
				userID:        "user-1",
				customerName:  "Alice",
				customerEmail: "alice@example.com",
				description:   "invoice 1",
				dueDate:       dueDateRFC3339,
				amount:        10000,
			},
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().
					CreateInvoice("merchant-1", "Alice", "alice@example.com", 10000.0, "invoice 1", dueDate).
					Return(invoiceEntity.Invoice{ID: "inv-1"}, nil)
			},
			wantID: "inv-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIInvoiceRepository(t)
			tc.setupMocks(repo)
			service := NewInvoiceService(repo)

			result, err := service.CreateInvoice(
				tc.input.userID,
				tc.input.customerName,
				tc.input.customerEmail,
				tc.input.amount,
				tc.input.description,
				tc.input.dueDate,
			)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, result.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, result.ID)
		})
	}
}
