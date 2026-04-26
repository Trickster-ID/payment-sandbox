package errors

import (
	stdErrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name       string
		input      error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "app error passthrough",
			input:      BadRequest("validation_error", "invalid payload", nil),
			wantStatus: 400,
			wantCode:   "validation_error",
		},
		{
			name:       "generic error maps to internal",
			input:      stdErrors.New("boom"),
			wantStatus: 500,
			wantCode:   "internal_error",
		},
		{
			name:       "nil input",
			input:      nil,
			wantStatus: 0,
			wantCode:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract(tc.input)
			if tc.input == nil {
				assert.Nil(t, got)
				return
			}

			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantCode, got.Code)
		})
	}
}
