package errors

import (
	stdErrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			appErr *AppError
		}
		wants struct {
			message string
		}
	}{
		{
			name: "1. message field is returned -> no nil",
			args: struct {
				appErr *AppError
			}{
				appErr: &AppError{Message: "test error"},
			},
			wants: struct {
				message string
			}{
				message: "test error",
			},
		},
		{
			name: "2. empty message -> returns empty string",
			args: struct {
				appErr *AppError
			}{
				appErr: &AppError{Message: ""},
			},
			wants: struct {
				message string
			}{
				message: "",
			},
		},
		{
			name: "3. message with special chars -> returns exactly",
			args: struct {
				appErr *AppError
			}{
				appErr: &AppError{Message: "error: invalid\ndata"},
			},
			wants: struct {
				message string
			}{
				message: "error: invalid\ndata",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.args.appErr.Error()
			assert.Equal(t, tc.wants.message, got)
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			status  int
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. all fields set -> creates AppError with exact values",
			args: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  400,
				code:    "bad_req",
				message: "bad request",
				details: "extra",
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  400,
				code:    "bad_req",
				message: "bad request",
				details: "extra",
			},
		},
		{
			name: "2. empty strings and nil details -> valid AppError",
			args: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  500,
				code:    "",
				message: "",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  500,
				code:    "",
				message: "",
				details: nil,
			},
		},
		{
			name: "3. large status code -> stored correctly",
			args: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  999,
				code:    "custom",
				message: "msg",
				details: 123,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  999,
				code:    "custom",
				message: "msg",
				details: 123,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := New(tc.args.status, tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestBadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. sets correct status code -> 400",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "val_err",
				message: "bad",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  400,
				code:    "val_err",
				message: "bad",
				details: nil,
			},
		},
		{
			name: "2. forwards parameters correctly",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "code",
				message: "message",
				details: map[string]any{"key": "value"},
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  400,
				code:    "code",
				message: "message",
				details: map[string]any{"key": "value"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BadRequest(tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestUnauthorized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. sets status to 401",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "auth_fail",
				message: "no auth",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  401,
				code:    "auth_fail",
				message: "no auth",
				details: nil,
			},
		},
		{
			name: "2. forwards message and details",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "auth_fail",
				message: "unauthorized access",
				details: 123,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  401,
				code:    "auth_fail",
				message: "unauthorized access",
				details: 123,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Unauthorized(tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestForbidden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. sets status to 403",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "no_access",
				message: "forbidden",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  403,
				code:    "no_access",
				message: "forbidden",
				details: nil,
			},
		},
		{
			name: "2. complex details object",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "no_access",
				message: "forbidden",
				details: map[string]any{"reason": "permission_denied"},
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  403,
				code:    "no_access",
				message: "forbidden",
				details: map[string]any{"reason": "permission_denied"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Forbidden(tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. sets status to 404",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "not_found",
				message: "resource missing",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  404,
				code:    "not_found",
				message: "resource missing",
				details: nil,
			},
		},
		{
			name: "2. message and code",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "missing_user",
				message: "user not found",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  404,
				code:    "missing_user",
				message: "user not found",
				details: nil,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NotFound(tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. sets status to 409",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "conflict",
				message: "resource exists",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  409,
				code:    "conflict",
				message: "resource exists",
				details: nil,
			},
		},
		{
			name: "2. with details",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "conflict",
				message: "resource exists",
				details: map[string]any{"existing_id": "123"},
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  409,
				code:    "conflict",
				message: "resource exists",
				details: map[string]any{"existing_id": "123"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Conflict(tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestInternal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			code    string
			message string
			details any
		}
		wants struct {
			status  int
			code    string
			message string
			details any
		}
	}{
		{
			name: "1. sets status to 500",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "internal",
				message: "server error",
				details: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  500,
				code:    "internal",
				message: "server error",
				details: nil,
			},
		},
		{
			name: "2. with error details",
			args: struct {
				code    string
				message string
				details any
			}{
				code:    "internal",
				message: "server error",
				details: "database connection failed",
			},
			wants: struct {
				status  int
				code    string
				message string
				details any
			}{
				status:  500,
				code:    "internal",
				message: "server error",
				details: "database connection failed",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Internal(tc.args.code, tc.args.message, tc.args.details)

			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
			assert.Equal(t, tc.wants.details, got.Details)
		})
	}
}

func TestExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args struct {
			err error
		}
		wants struct {
			status  int
			code    string
			message string
			isNil   bool
		}
	}{
		{
			name: "1. nil input -> returns nil",
			args: struct {
				err error
			}{
				err: nil,
			},
			wants: struct {
				status  int
				code    string
				message string
				isNil   bool
			}{
				isNil: true,
			},
		},
		{
			name: "2. AppError input -> returns same",
			args: struct {
				err error
			}{
				err: BadRequest("validation_error", "invalid payload", nil),
			},
			wants: struct {
				status  int
				code    string
				message string
				isNil   bool
			}{
				status:  400,
				code:    "validation_error",
				message: "invalid payload",
				isNil:   false,
			},
		},
		{
			name: "3. generic error -> maps to internal",
			args: struct {
				err error
			}{
				err: stdErrors.New("boom"),
			},
			wants: struct {
				status  int
				code    string
				message string
				isNil   bool
			}{
				status:  500,
				code:    "internal_error",
				message: "internal server error",
				isNil:   false,
			},
		},
		{
			name: "4. wrapped AppError via errors.As -> extracted",
			args: struct {
				err error
			}{
				err: BadRequest("bad_data", "invalid input", map[string]any{"field": "email"}),
			},
			wants: struct {
				status  int
				code    string
				message string
				isNil   bool
			}{
				status:  400,
				code:    "bad_data",
				message: "invalid input",
				isNil:   false,
			},
		},
		{
			name: "5. default message for unmapped error",
			args: struct {
				err error
			}{
				err: stdErrors.New("unknown failure"),
			},
			wants: struct {
				status  int
				code    string
				message string
				isNil   bool
			}{
				status:  500,
				code:    "internal_error",
				message: "internal server error",
				isNil:   false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract(tc.args.err)

			if tc.wants.isNil {
				assert.Nil(t, got)
				return
			}

			assert.NotNil(t, got)
			assert.Equal(t, tc.wants.status, got.Status)
			assert.Equal(t, tc.wants.code, got.Code)
			assert.Equal(t, tc.wants.message, got.Message)
		})
	}
}
