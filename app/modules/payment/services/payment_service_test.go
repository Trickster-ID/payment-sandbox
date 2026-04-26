package services

import (
	"errors"
	"testing"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	repoMocks "payment-sandbox/app/modules/payment/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentService_PublicInvoiceByToken(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		setupMocks func(repo *repoMocks.MockPaymentRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:  "not found",
			token: "token-1",
			setupMocks: func(repo *repoMocks.MockPaymentRepository) {
				repo.EXPECT().GetInvoiceByToken("token-1").Return(invoiceEntity.Invoice{}, false)
			},
			wantErr: "invoice not found",
		},
		{
			name:  "success",
			token: "token-1",
			setupMocks: func(repo *repoMocks.MockPaymentRepository) {
				repo.EXPECT().GetInvoiceByToken("token-1").Return(invoiceEntity.Invoice{ID: "inv-1"}, true)
			},
			wantID: "inv-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockPaymentRepository(t)
			tc.setupMocks(repo)
			service := NewPaymentService(repo)

			result, err := service.PublicInvoiceByToken(tc.token)

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

func TestPaymentService_CreatePaymentIntent(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		method     string
		setupMocks func(repo *repoMocks.MockPaymentRepository)
		wantID     string
		wantErr    string
	}{
		{
			name:   "repository error",
			token:  "token-1",
			method: "wallet",
			setupMocks: func(repo *repoMocks.MockPaymentRepository) {
				repo.EXPECT().
					CreatePaymentIntent("token-1", paymentEntity.MethodWallet).
					Return(paymentEntity.PaymentIntent{}, invoiceEntity.Invoice{}, errors.New("invoice already paid"))
			},
			wantErr: "invoice already paid",
		},
		{
			name:   "success with uppercase normalization",
			token:  "token-1",
			method: " va_dummy ",
			setupMocks: func(repo *repoMocks.MockPaymentRepository) {
				repo.EXPECT().
					CreatePaymentIntent("token-1", paymentEntity.MethodVADummy).
					Return(
						paymentEntity.PaymentIntent{ID: "pi-1", Method: paymentEntity.MethodVADummy},
						invoiceEntity.Invoice{ID: "inv-1"},
						nil,
					)
			},
			wantID: "pi-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockPaymentRepository(t)
			tc.setupMocks(repo)
			service := NewPaymentService(repo)

			intent, _, err := service.CreatePaymentIntent(tc.token, tc.method)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, intent.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, intent.ID)
		})
	}
}

func TestPaymentService_UpdatePaymentIntentStatus(t *testing.T) {
	tests := []struct {
		name       string
		paymentID  string
		status     string
		setupMocks func(repo *repoMocks.MockPaymentRepository)
		wantStatus paymentEntity.PaymentStatus
		wantErr    string
	}{
		{
			name:      "invalid status",
			paymentID: "pi-1",
			status:    "PENDING",
			setupMocks: func(repo *repoMocks.MockPaymentRepository) {
				repo.AssertNotCalled(t, "UpdatePaymentStatus")
			},
			wantErr: "invalid payment status",
		},
		{
			name:      "success mapping",
			paymentID: "pi-1",
			status:    "failed",
			setupMocks: func(repo *repoMocks.MockPaymentRepository) {
				repo.EXPECT().
					UpdatePaymentStatus("pi-1", paymentEntity.PaymentFailed).
					Return(
						paymentEntity.PaymentIntent{ID: "pi-1", Status: paymentEntity.PaymentFailed},
						invoiceEntity.Invoice{ID: "inv-1"},
						nil,
					)
			},
			wantStatus: paymentEntity.PaymentFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoMocks.NewMockPaymentRepository(t)
			tc.setupMocks(repo)
			service := NewPaymentService(repo)

			intent, _, err := service.UpdatePaymentIntentStatus(tc.paymentID, tc.status)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Empty(t, intent.ID)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, intent.Status)
		})
	}
}

func TestPaymentService_ListPaymentIntents(t *testing.T) {
	repo := repoMocks.NewMockPaymentRepository(t)
	service := NewPaymentService(repo)

	expected := []paymentEntity.PaymentIntent{{ID: "pi-1"}, {ID: "pi-2"}}
	repo.EXPECT().ListPaymentIntents("SUCCESS").Return(expected)

	result := service.ListPaymentIntents(" success ")
	assert.Equal(t, expected, result)
}
