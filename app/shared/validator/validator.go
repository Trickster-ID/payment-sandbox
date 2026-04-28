package validator

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var iso4217CodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

var supportedISO4217Codes = map[string]struct{}{
	"IDR": {},
	"USD": {},
	"SGD": {},
	"EUR": {},
	"JPY": {},
}

func IsEmail(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return false
	}

	return addr.Address == trimmed
}

func IsPositiveAmount(amount float64) bool {
	return amount > 0
}

func ParseRFC3339(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, errors.New("must use RFC3339 format")
	}
	return parsed, nil
}

func IsTodayOrFuture(date time.Time, now time.Time) bool {
	y1, m1, d1 := date.Date()
	y2, m2, d2 := now.Date()
	dateOnly := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	nowOnly := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	return !dateOnly.Before(nowOnly)
}

func IsISO4217Code(code string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if !iso4217CodePattern.MatchString(normalized) {
		return false
	}
	_, ok := supportedISO4217Codes[normalized]
	return ok
}
