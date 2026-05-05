package validator

// Branch map for validator functions (Section 3.1 of the plan):
//
// IsEmail(value):
// ├── trimmed is empty                      -> false
// ├── mail.ParseAddress fails               -> false
// ├── addr.Address != trimmed (display name)-> false
// └── all checks pass                       -> true
//
// IsPositiveAmount(amount):
// ├── amount > 0                            -> true
// └── amount <= 0                           -> false
//
// ParseRFC3339(value):
// ├── invalid RFC3339 format                -> time.Time{}, "must use RFC3339 format"
// └── valid RFC3339 format                  -> parsed time, nil
//
// IsTodayOrFuture(date, now):
// ├── date is before now                    -> false
// ├── date equals now                       -> true
// └── date is after now                     -> true
//
// IsISO4217Code(code):
// ├── empty code                            -> false
// ├── code pattern invalid (not [A-Z]{3})  -> false
// ├── code supported in map                 -> true
// └── code not supported in map             -> false

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsEmail(t *testing.T) {
	type args struct {
		value string
	}
	type wants struct {
		valid bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. empty string -> invalid",
			args:  args{value: ""},
			wants: wants{valid: false},
		},
		{
			name:  "2. whitespace only -> invalid",
			args:  args{value: "   "},
			wants: wants{valid: false},
		},
		{
			name:  "3. valid email -> valid",
			args:  args{value: "user@example.com"},
			wants: wants{valid: true},
		},
		{
			name:  "4. email with leading/trailing whitespace -> valid",
			args:  args{value: "  user@example.com  "},
			wants: wants{valid: true},
		},
		{
			name:  "5. invalid email format (no @) -> invalid",
			args:  args{value: "notanemail"},
			wants: wants{valid: false},
		},
		{
			name:  "6. invalid email format (double @) -> invalid",
			args:  args{value: "user@@example.com"},
			wants: wants{valid: false},
		},
		{
			name:  "7. email with display name -> invalid (addr.Address != trimmed)",
			args:  args{value: "John Doe <john@example.com>"},
			wants: wants{valid: false},
		},
		{
			name:  "8. email with special characters -> valid",
			args:  args{value: "user+tag@example.co.uk"},
			wants: wants{valid: true},
		},
		{
			name:  "9. single char local part -> valid",
			args:  args{value: "a@b.co"},
			wants: wants{valid: true},
		},
		{
			name:  "10. email with only local-part and @ -> invalid",
			args:  args{value: "user@"},
			wants: wants{valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsEmail(tt.args.value)

			assert.Equal(t, tt.wants.valid, result, "email validation result")
		})
	}
}

func TestIsPositiveAmount(t *testing.T) {
	type args struct {
		amount float64
	}
	type wants struct {
		positive bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. positive amount -> true",
			args:  args{amount: 100.50},
			wants: wants{positive: true},
		},
		{
			name:  "2. small positive amount (0.01) -> true",
			args:  args{amount: 0.01},
			wants: wants{positive: true},
		},
		{
			name:  "3. zero -> false",
			args:  args{amount: 0},
			wants: wants{positive: false},
		},
		{
			name:  "4. negative amount -> false",
			args:  args{amount: -50.25},
			wants: wants{positive: false},
		},
		{
			name:  "5. very small negative -> false",
			args:  args{amount: -0.01},
			wants: wants{positive: false},
		},
		{
			name:  "6. large positive amount -> true",
			args:  args{amount: 999999.99},
			wants: wants{positive: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsPositiveAmount(tt.args.amount)

			assert.Equal(t, tt.wants.positive, result, "positive amount check")
		})
	}
}

func TestParseRFC3339(t *testing.T) {
	type args struct {
		value string
	}
	type wants struct {
		errMsg    string
		hasErr    bool
		year      int
		month     time.Month
		day       int
		hour      int
		minute    int
		second    int
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. valid RFC3339 timestamp -> success",
			args:  args{value: "2025-05-05T16:30:00Z"},
			wants: wants{hasErr: false, year: 2025, month: 5, day: 5, hour: 16, minute: 30, second: 0},
		},
		{
			name:  "2. valid RFC3339 with timezone offset -> success",
			args:  args{value: "2025-05-05T16:30:00+07:00"},
			wants: wants{hasErr: false, year: 2025, month: 5, day: 5, hour: 16, minute: 30, second: 0},
		},
		{
			name:  "3. valid RFC3339 with whitespace -> success (trimmed)",
			args:  args{value: "  2025-05-05T16:30:00Z  "},
			wants: wants{hasErr: false, year: 2025, month: 5, day: 5, hour: 16, minute: 30, second: 0},
		},
		{
			name:  "4. invalid format (missing T) -> error",
			args:  args{value: "2025-05-05 16:30:00Z"},
			wants: wants{errMsg: "must use RFC3339 format", hasErr: true},
		},
		{
			name:  "5. invalid format (no Z or offset) -> error",
			args:  args{value: "2025-05-05T16:30:00"},
			wants: wants{errMsg: "must use RFC3339 format", hasErr: true},
		},
		{
			name:  "6. completely invalid date string -> error",
			args:  args{value: "not-a-date"},
			wants: wants{errMsg: "must use RFC3339 format", hasErr: true},
		},
		{
			name:  "7. empty string -> error",
			args:  args{value: ""},
			wants: wants{errMsg: "must use RFC3339 format", hasErr: true},
		},
		{
			name:  "8. invalid day (32) -> error",
			args:  args{value: "2025-05-32T16:30:00Z"},
			wants: wants{errMsg: "must use RFC3339 format", hasErr: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseRFC3339(tt.args.value)

			if tt.wants.hasErr {
				assert.Error(t, err, "expected error")
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
				assert.Equal(t, time.Time{}, result, "time should be zero on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.wants.year, result.Year(), "year")
				assert.Equal(t, tt.wants.month, result.Month(), "month")
				assert.Equal(t, tt.wants.day, result.Day(), "day")
				assert.Equal(t, tt.wants.hour, result.Hour(), "hour")
				assert.Equal(t, tt.wants.minute, result.Minute(), "minute")
				assert.Equal(t, tt.wants.second, result.Second(), "second")
			}
		})
	}
}

func TestIsTodayOrFuture(t *testing.T) {
	type args struct {
		date time.Time
		now  time.Time
	}
	type wants struct {
		isTodayOrFuture bool
	}

	// Use fixed time for testing
	baseTime := time.Date(2025, 5, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. date is today -> true",
			args: args{
				date: time.Date(2025, 5, 5, 10, 30, 0, 0, time.UTC),
				now:  baseTime,
			},
			wants: wants{isTodayOrFuture: true},
		},
		{
			name: "2. date is tomorrow -> true",
			args: args{
				date: time.Date(2025, 5, 6, 10, 30, 0, 0, time.UTC),
				now:  baseTime,
			},
			wants: wants{isTodayOrFuture: true},
		},
		{
			name: "3. date is yesterday -> false",
			args: args{
				date: time.Date(2025, 5, 4, 10, 30, 0, 0, time.UTC),
				now:  baseTime,
			},
			wants: wants{isTodayOrFuture: false},
		},
		{
			name: "4. date is far in future -> true",
			args: args{
				date: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
				now:  baseTime,
			},
			wants: wants{isTodayOrFuture: true},
		},
		{
			name: "5. date is far in past -> false",
			args: args{
				date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				now:  baseTime,
			},
			wants: wants{isTodayOrFuture: false},
		},
		{
			name: "6. date has time, ignores time difference -> true (same date)",
			args: args{
				date: time.Date(2025, 5, 5, 0, 0, 0, 0, time.UTC),
				now:  time.Date(2025, 5, 5, 23, 59, 59, 0, time.UTC),
			},
			wants: wants{isTodayOrFuture: true},
		},
		{
			name: "7. different timezones, same calendar date -> true",
			args: args{
				date: time.Date(2025, 5, 5, 10, 0, 0, 0, time.FixedZone("UTC+7", 7*3600)),
				now:  time.Date(2025, 5, 5, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)),
			},
			wants: wants{isTodayOrFuture: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsTodayOrFuture(tt.args.date, tt.args.now)

			assert.Equal(t, tt.wants.isTodayOrFuture, result, "today or future check")
		})
	}
}

func TestIsISO4217Code(t *testing.T) {
	type args struct {
		code string
	}
	type wants struct {
		valid bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. supported code USD -> true",
			args:  args{code: "USD"},
			wants: wants{valid: true},
		},
		{
			name:  "2. supported code IDR -> true",
			args:  args{code: "IDR"},
			wants: wants{valid: true},
		},
		{
			name:  "3. supported code EUR -> true",
			args:  args{code: "EUR"},
			wants: wants{valid: true},
		},
		{
			name:  "4. supported code SGD -> true",
			args:  args{code: "SGD"},
			wants: wants{valid: true},
		},
		{
			name:  "5. supported code JPY -> true",
			args:  args{code: "JPY"},
			wants: wants{valid: true},
		},
		{
			name:  "6. unsupported code GBP -> false",
			args:  args{code: "GBP"},
			wants: wants{valid: false},
		},
		{
			name:  "7. unsupported code AUD -> false",
			args:  args{code: "AUD"},
			wants: wants{valid: false},
		},
		{
			name:  "8. lowercase supported code (usd) -> true (normalized)",
			args:  args{code: "usd"},
			wants: wants{valid: true},
		},
		{
			name:  "9. mixed case (UsD) -> true (normalized)",
			args:  args{code: "UsD"},
			wants: wants{valid: true},
		},
		{
			name:  "10. code with whitespace -> true (trimmed)",
			args:  args{code: "  USD  "},
			wants: wants{valid: true},
		},
		{
			name:  "11. too short (2 chars) -> false",
			args:  args{code: "US"},
			wants: wants{valid: false},
		},
		{
			name:  "12. too long (4 chars) -> false",
			args:  args{code: "USDA"},
			wants: wants{valid: false},
		},
		{
			name:  "13. empty string -> false",
			args:  args{code: ""},
			wants: wants{valid: false},
		},
		{
			name:  "14. contains lowercase letter (uSd) -> true (normalized to upper)",
			args:  args{code: "uSd"},
			wants: wants{valid: true},
		},
		{
			name:  "15. contains number (US1) -> false (not [A-Z]{3})",
			args:  args{code: "US1"},
			wants: wants{valid: false},
		},
		{
			name:  "16. contains special char (US-D) -> false",
			args:  args{code: "US-D"},
			wants: wants{valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsISO4217Code(tt.args.code)

			assert.Equal(t, tt.wants.valid, result, "ISO4217 code validation")
		})
	}
}
