// Branch map for GetInvoiceByID:
// ├── repo.GetInvoiceByID returns false  -> "invoice not found"  [Case 1]
// └── repo.GetInvoiceByID returns true   -> success              [Case 2]
package services

import (
	"testing"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	repoMocks "payment-sandbox/app/modules/payment/repositories/mocks"

	"github.com/stretchr/testify/assert"
)

func TestPaymentService_GetInvoiceByID(t *testing.T) {
	type fields struct {
		repo *repoMocks.MockIPaymentRepository
	}
	type args struct {
		id string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		invoiceID string
		errMsg    string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. invoice id not found -> invoice not found error",
			fields: fields{repo: repoMocks.NewMockIPaymentRepository(t)},
			args:   args{id: "inv-missing"},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().
						GetInvoiceByID(a.id).
						Return(invoiceEntity.Invoice{}, false).
						Once()
				},
			},
			wants: wants{errMsg: "invoice not found"},
		},
		{
			name:   "2. invoice id exists -> success",
			fields: fields{repo: repoMocks.NewMockIPaymentRepository(t)},
			args:   args{id: "inv-1"},
			mocks: mocks{
				setup: func(f fields, a args) {
					f.repo.EXPECT().
						GetInvoiceByID(a.id).
						Return(invoiceEntity.Invoice{ID: "inv-1", Amount: 50000}, true).
						Once()
				},
			},
			wants: wants{invoiceID: "inv-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewPaymentService(tt.fields.repo)
			invoice, err := svc.GetInvoiceByID(tt.args.id)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Empty(t, invoice.ID, "invoice ID should be empty on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.invoiceID, invoice.ID, "invoice ID")
			}

			tt.fields.repo.AssertExpectations(t)
		})
	}
}
