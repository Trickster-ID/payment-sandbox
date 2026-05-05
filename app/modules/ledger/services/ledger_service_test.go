// Branch map for ValidatePosting:
// ├── len(entries) < 2                              -> "posting requires at least 2 entries"   [Cases 1-2]
// ├── reference == ""                               -> "posting requires a reference"           [Case 3]
// ├── entry.Amount <= 0 (Amount == 0)               -> "entry amount must be positive"          [Case 4]
// ├── entry.Amount <= 0 (Amount < 0)                -> "entry amount must be positive"          [Case 5]
// ├── entry.Direction invalid (not D or C)          -> "invalid direction: <dir>"              [Case 6]
// ├── balanced posting, single currency             -> nil / success                           [Case 7]
// ├── unbalanced posting, single currency           -> "posting unbalanced for <ccy>: ..."    [Case 8]
// ├── all debits, no credit entries                 -> "posting unbalanced for <ccy>: ..."    [Case 9]
// └── multi-currency, all balanced                 -> nil / success                           [Case 10]
package services_test

import (
	"testing"

	"payment-sandbox/app/modules/ledger/models/entity"
	"payment-sandbox/app/modules/ledger/services"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestValidatePosting(t *testing.T) {
	type args struct {
		posting entity.Posting
	}
	type wants struct {
		errMsg string // empty means no error expected
	}

	accountA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	accountB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. zero entries -> posting requires at least 2 entries",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries:   []entity.Entry{},
				},
			},
			wants: wants{errMsg: "posting requires at least 2 entries"},
		},
		{
			name: "2. one entry -> posting requires at least 2 entries",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 100, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "posting requires at least 2 entries"},
		},
		{
			name: "3. two entries, empty reference -> posting requires a reference",
			args: args{
				posting: entity.Posting{
					Reference: "",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 100, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 100, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "posting requires a reference"},
		},
		{
			name: "4. entry with zero amount -> entry amount must be positive",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 0, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 100, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "entry amount must be positive"},
		},
		{
			name: "5. entry with negative amount -> entry amount must be positive",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: -50, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 100, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "entry amount must be positive"},
		},
		{
			name: "6. entry with invalid direction -> invalid direction error",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Direction("X"), Amount: 100, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 100, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "invalid direction: X"},
		},
		{
			name: "7. single currency balanced debit and credit -> success",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 500, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 500, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: ""},
		},
		{
			name: "8. single currency debit does not equal credit -> posting unbalanced error",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 700, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 500, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "posting unbalanced for USD: debits - credits = 200"},
		},
		{
			name: "9. all debit entries, no credit -> posting unbalanced error",
			args: args{
				posting: entity.Posting{
					Reference: "ref-001",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 300, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Debit, Amount: 200, Currency: "USD"},
					},
				},
			},
			wants: wants{errMsg: "posting unbalanced for USD: debits - credits = 500"},
		},
		{
			name: "10. multi-currency both balanced -> success",
			args: args{
				posting: entity.Posting{
					Reference: "ref-multi",
					Entries: []entity.Entry{
						{AccountID: accountA, Direction: entity.Debit, Amount: 1000, Currency: "USD"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 1000, Currency: "USD"},
						{AccountID: accountA, Direction: entity.Debit, Amount: 200, Currency: "EUR"},
						{AccountID: accountB, Direction: entity.Credit, Amount: 200, Currency: "EUR"},
					},
				},
			},
			wants: wants{errMsg: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := services.ValidatePosting(tt.args.posting)

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "expected no error")
			}
		})
	}
}
