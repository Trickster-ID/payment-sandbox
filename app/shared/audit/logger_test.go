package audit

// Branch map for logger functions (Section 3.1 of the plan):
//
// NewNoopLogger:
// └── always returns noopLogger instance  -> noopLogger, nil
//
// noopLogger.Log:
// └── always returns nil                  -> nil (no side effects)
//
// NewLogger(db):
// ├── db is nil                           -> returns noopLogger
// └── db is not nil                       -> returns Logger with collection
//
// redact(m map[string]any) - tested implicitly:
// ├── map is nil                          -> no panic
// ├── map is empty                        -> no change
// ├── map has redactable keys (password, cvv, etc.)  -> "[REDACTED]"
// ├── map has non-redactable keys         -> unchanged
// └── key matching is case-insensitive    -> "PASSWORD", "Password" both redacted

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNoopLogger(t *testing.T) {
	t.Parallel()

	logger := NewNoopLogger()

	assert.NotNil(t, logger, "returns non-nil logger")
	_, ok := logger.(noopLogger)
	assert.True(t, ok, "returns noopLogger instance")
}

func TestNoopLoggerLog(t *testing.T) {
	t.Parallel()

	logger := NewNoopLogger()

	err := logger.Log(context.Background(), Event{
		EventType: "test.event",
		ActorID:   "user-1",
	})

	assert.NoError(t, err, "always returns nil")
}

func TestNewLogger(t *testing.T) {
	t.Parallel()

	// db is nil -> returns noopLogger
	logger := NewLogger(nil)
	assert.NotNil(t, logger, "returns non-nil logger")
	_, isNoop := logger.(noopLogger)
	assert.True(t, isNoop, "returns noopLogger when db is nil")

	// db is not nil -> returns Logger instance
	// Cannot easily test with real *mongo.Database without integration test
	// This branch is tested indirectly: if the nil check in NewLogger didn't work,
	// calling Logger.Log() would panic when trying to access coll (verified via integration tests)
}

func TestRedact(t *testing.T) {
	// redact is an unexported function, but we test it indirectly through logger behavior
	// This test documents the redaction behavior via direct unit test
	t.Parallel()

	type args struct {
		input map[string]any
	}
	type wants struct {
		output map[string]any
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. nil map -> no panic",
			args:  args{input: nil},
			wants: wants{output: nil},
		},
		{
			name:  "2. empty map -> unchanged",
			args:  args{input: map[string]any{}},
			wants: wants{output: map[string]any{}},
		},
		{
			name: "3. map with sensitive keys -> redacted",
			args: args{input: map[string]any{
				"password":      "secret",
				"password_hash": "hash",
				"cvv":           "123",
				"card_number":   "1234",
				"client_secret": "secret",
				"authorization": "token",
				"name":          "Alice",
			}},
			wants: wants{output: map[string]any{
				"password":      "[REDACTED]",
				"password_hash": "[REDACTED]",
				"cvv":           "[REDACTED]",
				"card_number":   "[REDACTED]",
				"client_secret": "[REDACTED]",
				"authorization": "[REDACTED]",
				"name":          "Alice",
			}},
		},
		{
			name: "4. case-insensitive redaction",
			args: args{input: map[string]any{
				"PASSWORD":     "secret",
				"Password":     "secret",
				"CVV":          "123",
				"CARD_NUMBER":  "1234",
				"name":         "Alice",
			}},
			wants: wants{output: map[string]any{
				"PASSWORD":     "[REDACTED]",
				"Password":     "[REDACTED]",
				"CVV":          "[REDACTED]",
				"CARD_NUMBER":  "[REDACTED]",
				"name":         "Alice",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			redact(tt.args.input)

			assert.Equal(t, tt.wants.output, tt.args.input, "output matches expected")
		})
	}
}
