package validator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsEmail(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "valid email", value: "merchant@example.com", want: true},
		{name: "invalid email", value: "merchant.example.com", want: false},
		{name: "empty", value: " ", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsEmail(tc.value))
		})
	}
}

func TestIsPositiveAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   bool
	}{
		{name: "positive", amount: 1, want: true},
		{name: "zero", amount: 0, want: false},
		{name: "negative", amount: -1, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsPositiveAmount(tc.amount))
		})
	}
}

func TestParseRFC3339(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "2026-04-26T10:00:00Z", wantErr: false},
		{name: "invalid", value: "2026-04-26", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseRFC3339(tc.value)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, parsed.IsZero())
				return
			}

			require.NoError(t, err)
			assert.False(t, parsed.IsZero())
		})
	}
}

func TestIsTodayOrFuture(t *testing.T) {
	now := time.Date(2026, time.April, 26, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		date time.Time
		want bool
	}{
		{name: "today", date: time.Date(2026, time.April, 26, 23, 59, 0, 0, time.UTC), want: true},
		{name: "future", date: time.Date(2026, time.April, 27, 0, 0, 0, 0, time.UTC), want: true},
		{name: "past", date: time.Date(2026, time.April, 25, 23, 59, 0, 0, time.UTC), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsTodayOrFuture(tc.date, now))
		})
	}
}

func TestIsISO4217Code(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "supported code", code: "IDR", want: true},
		{name: "supported code lower case", code: "usd", want: true},
		{name: "unsupported but valid format", code: "AUD", want: false},
		{name: "invalid format with number", code: "U5D", want: false},
		{name: "invalid length", code: "US", want: false},
		{name: "empty", code: " ", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsISO4217Code(tc.code))
		})
	}
}
