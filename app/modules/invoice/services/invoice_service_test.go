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

func TestInvoiceService_ListInvoices(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		status     string
		page       int
		limit      int
		setupMocks func(repo *repoMocks.MockIInvoiceRepository)
		wantTotal  int
		wantLen    int
		wantErr    string
	}{
		{
			name:   "merchant lookup failed",
			userID: "user-1",
			status: "PENDING",
			page:   1,
			limit:  10,
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found"))
			},
			wantErr: "merchant not found",
		},
		{
			name:   "success",
			userID: "user-1",
			status: "PAID",
			page:   2,
			limit:  5,
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().
					ListInvoices("merchant-1", "PAID", invoiceEntity.ListOptions{Page: 2, Limit: 5}).
					Return([]invoiceEntity.Invoice{{ID: "inv-1"}, {ID: "inv-2"}}, 11)
			},
			wantTotal: 11,
			wantLen:   2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIInvoiceRepository(t)
			tc.setupMocks(repo)
			service := NewInvoiceService(repo)

			items, total, err := service.ListInvoices(tc.userID, tc.status, tc.page, tc.limit)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, items)
				assert.Zero(t, total)
				return
			}

			require.NoError(t, err)
			assert.Len(t, items, tc.wantLen)
			assert.Equal(t, tc.wantTotal, total)
		})
	}
}

func TestInvoiceService_InvoiceByID(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		invoiceID  string
		setupMocks func(repo *repoMocks.MockIInvoiceRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:      "merchant lookup failed",
			userID:    "user-1",
			invoiceID: "inv-1",
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("", errors.New("merchant not found"))
			},
			wantErr: "merchant not found",
		},
		{
			name:      "invoice not found for merchant",
			userID:    "user-1",
			invoiceID: "inv-1",
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().MerchantInvoiceByID("inv-1", "merchant-1").Return(invoiceEntity.Invoice{}, errors.New("invoice not found"))
			},
			wantErr: "invoice not found",
		},
		{
			name:      "success",
			userID:    "user-1",
			invoiceID: "inv-1",
			setupMocks: func(repo *repoMocks.MockIInvoiceRepository) {
				repo.EXPECT().MerchantIDByUserID("user-1").Return("merchant-1", nil)
				repo.EXPECT().MerchantInvoiceByID("inv-1", "merchant-1").Return(invoiceEntity.Invoice{ID: "inv-1"}, nil)
			},
			wantID: "inv-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockIInvoiceRepository(t)
			tc.setupMocks(repo)
			service := NewInvoiceService(repo)

			result, err := service.InvoiceByID(tc.userID, tc.invoiceID)

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
