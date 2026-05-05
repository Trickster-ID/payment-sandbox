package services

// Branch map (used to derive test cases per Section 3.1 of the plan):
//
// CreateInvoice:
// ├── repo.MerchantIDByUserID fails           -> return Invoice{}, error
// ├── validator.ParseRFC3339 fails            -> return Invoice{}, "due_date must use RFC3339 format"
// ├── validator.IsEmail fails                 -> return Invoice{}, "customer_email is invalid"
// ├── repo.CreateInvoice fails                -> return Invoice{}, error
// └── all validations pass, repo succeeds    -> return Invoice, nil
//
// ListInvoices:
// ├── repo.MerchantIDByUserID fails  -> return nil, 0, error
// ├── repo returns empty list        -> return [], 0, nil
// └── repo returns items             -> return items, total, nil
//
// InvoiceByID:
// ├── repo.MerchantIDByUserID fails      -> return Invoice{}, error
// ├── repo.MerchantInvoiceByID fails     -> return Invoice{}, error
// └── repo.MerchantInvoiceByID succeeds  -> return Invoice, nil

import (
	"errors"
	"testing"
	"time"

	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	"payment-sandbox/app/modules/invoice/repositories"
	repoMocks "payment-sandbox/app/modules/invoice/repositories/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceService_CreateInvoice(t *testing.T) {
	dueDate := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	dueDateRFC3339 := dueDate.Format(time.RFC3339)

	type fields struct {
		repo repositories.IInvoiceRepository
	}
	type args struct {
		userID, customerName, customerEmail, description, dueDate string
		amount                                                    int64
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		invoiceID string
		err       string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. merchant lookup fails -> error propagated",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", customerName: "Alice", customerEmail: "alice@example.com", amount: 10000, description: "invoice 1", dueDate: dueDateRFC3339},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIInvoiceRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("", errors.New("merchant not found")).
					Once()
			}},
			wants: wants{err: "merchant not found"},
		},
		{
			name:   "2. invalid due date format -> due_date must use RFC3339 format",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", customerName: "Alice", customerEmail: "alice@example.com", amount: 10000, description: "invoice 1", dueDate: "2026-04-30"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIInvoiceRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
			}},
			wants: wants{err: "due_date must use RFC3339 format"},
		},
		{
			name:   "3. invalid customer email -> customer_email is invalid",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", customerName: "Alice", customerEmail: "alice.example.com", amount: 10000, description: "invoice 1", dueDate: dueDateRFC3339},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIInvoiceRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("merchant-1", nil).
					Once()
			}},
			wants: wants{err: "customer_email is invalid"},
		},
		{
			name:   "4. repo.CreateInvoice fails -> error propagated",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", customerName: "Alice", customerEmail: "alice@example.com", amount: 10000, description: "invoice 1", dueDate: dueDateRFC3339},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().CreateInvoice("merchant-1", a.customerName, a.customerEmail, a.amount, a.description, dueDate).
					Return(invoiceEntity.Invoice{}, errors.New("db error")).
					Once()
			}},
			wants: wants{err: "db error"},
		},
		{
			name:   "5. valid request, all steps succeed -> invoice returned",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", customerName: "Alice", customerEmail: "alice@example.com", amount: 10000, description: "invoice 1", dueDate: dueDateRFC3339},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().CreateInvoice("merchant-1", a.customerName, a.customerEmail, a.amount, a.description, dueDate).
					Return(invoiceEntity.Invoice{ID: "inv-1"}, nil).
					Once()
			}},
			wants: wants{invoiceID: "inv-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewInvoiceService(tt.fields.repo)
			result, err := svc.CreateInvoice(tt.args.userID, tt.args.customerName, tt.args.customerEmail, tt.args.amount, tt.args.description, tt.args.dueDate)

			if tt.wants.err != "" {
				require.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, result.ID, "invoice ID should be empty on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.invoiceID, result.ID, "invoice ID")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIInvoiceRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestInvoiceService_ListInvoices(t *testing.T) {
	type fields struct {
		repo repositories.IInvoiceRepository
	}
	type args struct {
		userID string
		status string
		page   int
		limit  int
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		total int
		len   int
		err   string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. merchant lookup fails -> error propagated, empty results",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", status: "PENDING", page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIInvoiceRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("", errors.New("merchant not found")).
					Once()
			}},
			wants: wants{err: "merchant not found"},
		},
		{
			name:   "2. merchant found, repo returns items -> items and total returned",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", status: "PAID", page: 2, limit: 5},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().
					ListInvoices("merchant-1", a.status, invoiceEntity.ListOptions{Page: a.page, Limit: a.limit}).
					Return([]invoiceEntity.Invoice{{ID: "inv-1"}, {ID: "inv-2"}}, 11).
					Once()
			}},
			wants: wants{total: 11, len: 2},
		},
		{
			name:   "3. merchant found, repo returns empty list -> zero items and total",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", status: "PENDING", page: 1, limit: 10},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().
					ListInvoices("merchant-1", a.status, invoiceEntity.ListOptions{Page: a.page, Limit: a.limit}).
					Return([]invoiceEntity.Invoice{}, 0).
					Once()
			}},
			wants: wants{total: 0, len: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewInvoiceService(tt.fields.repo)
			items, total, err := svc.ListInvoices(tt.args.userID, tt.args.status, tt.args.page, tt.args.limit)

			if tt.wants.err != "" {
				require.EqualError(t, err, tt.wants.err, "error message")
				assert.Nil(t, items, "items should be nil on error")
				assert.Zero(t, total, "total should be zero on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Len(t, items, tt.wants.len, "items count")
				assert.Equal(t, tt.wants.total, total, "total count")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIInvoiceRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestInvoiceService_InvoiceByID(t *testing.T) {
	type fields struct {
		repo repositories.IInvoiceRepository
	}
	type args struct {
		userID    string
		invoiceID string
	}
	type mocks struct {
		setup func(f fields, a args)
	}
	type wants struct {
		invoiceID string
		err       string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		mocks  mocks
		wants  wants
	}{
		{
			name:   "1. merchant lookup fails -> error propagated",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {
				f.repo.(*repoMocks.MockIInvoiceRepository).EXPECT().
					MerchantIDByUserID(a.userID).
					Return("", errors.New("merchant not found")).
					Once()
			}},
			wants: wants{err: "merchant not found"},
		},
		{
			name:   "2. invoice not found for merchant -> error propagated",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().MerchantInvoiceByID(a.invoiceID, "merchant-1").
					Return(invoiceEntity.Invoice{}, errors.New("invoice not found")).
					Once()
			}},
			wants: wants{err: "invoice not found"},
		},
		{
			name:   "3. repo returns db error -> error propagated",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().MerchantInvoiceByID(a.invoiceID, "merchant-1").
					Return(invoiceEntity.Invoice{}, errors.New("db error")).
					Once()
			}},
			wants: wants{err: "db error"},
		},
		{
			name:   "4. invoice exists for merchant -> invoice returned",
			fields: fields{repo: repoMocks.NewMockIInvoiceRepository(t)},
			args:   args{userID: "user-1", invoiceID: "inv-1"},
			mocks: mocks{setup: func(f fields, a args) {
				m := f.repo.(*repoMocks.MockIInvoiceRepository)
				m.EXPECT().MerchantIDByUserID(a.userID).Return("merchant-1", nil).Once()
				m.EXPECT().MerchantInvoiceByID(a.invoiceID, "merchant-1").
					Return(invoiceEntity.Invoice{ID: "inv-1"}, nil).
					Once()
			}},
			wants: wants{invoiceID: "inv-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mocks.setup(tt.fields, tt.args)

			svc := NewInvoiceService(tt.fields.repo)
			result, err := svc.InvoiceByID(tt.args.userID, tt.args.invoiceID)

			if tt.wants.err != "" {
				require.EqualError(t, err, tt.wants.err, "error message")
				assert.Empty(t, result.ID, "invoice ID should be empty on error")
			} else {
				require.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.invoiceID, result.ID, "invoice ID")
			}

			if m, ok := tt.fields.repo.(*repoMocks.MockIInvoiceRepository); ok {
				m.AssertExpectations(t)
			}
		})
	}
}
