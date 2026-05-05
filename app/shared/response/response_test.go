package response

// Branch analysis for response helpers:
//
// JSON(c, status, data, meta):
// └── c.JSON(status, Envelope{Data: data, Meta: meta})
//
// OK(c, data):
// └── JSON(c, 200, data, nil)
//
// OKWithMeta(c, data, meta):
// └── JSON(c, 200, data, meta)
//
// Created(c, data):
// └── JSON(c, 201, data, nil)
//
// Fail(c, appErr):
// ├── appErr == nil → creates Internal error 500 → writes envelope with error
// └── appErr != nil → writes appErr.Status with error payload
//
// FailFromError(c, err):
// ├── errors.Extract(err) == nil → Fail(c, nil)
// ├── errors.Extract(err) == appErr → Fail(c, appErr)
// └── errors.Extract(std error) → Internal error → Fail(c, Internal(...))

import (
	"encoding/json"
	stdErrors "errors"
	"net/http/httptest"
	"testing"

	appErrors "payment-sandbox/app/shared/errors"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── JSON ─────────────────────────────────────────────────────────────────────

func TestJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		status int
		data   any
		meta   any
	}
	type wants struct {
		statusCode int
		hasData    bool
		hasMeta    bool
		hasError   bool
		dataValue  string // for simple assertions
		metaKey    string // key to check in meta object
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. status 200 with data only -> 200 response data present meta absent",
			args: args{status: 200, data: gin.H{"id": "123"}, meta: nil},
			wants: wants{
				statusCode: 200,
				hasData:    true,
				hasMeta:    false,
				hasError:   false,
			},
		},
		{
			name: "2. status 200 with data and meta -> 200 response both present",
			args: args{status: 200, data: []string{"a"}, meta: gin.H{"page": 1}},
			wants: wants{
				statusCode: 200,
				hasData:    true,
				hasMeta:    true,
				hasError:   false,
				metaKey:    "page",
			},
		},
		{
			name: "3. status 200 with nil data -> 200 response data absent",
			args: args{status: 200, data: nil, meta: nil},
			wants: wants{
				statusCode: 200,
				hasData:    false,
				hasMeta:    false,
				hasError:   false,
			},
		},
		{
			name: "4. status 201 with data -> 201 created response",
			args: args{status: 201, data: gin.H{"id": "new"}, meta: nil},
			wants: wants{
				statusCode: 201,
				hasData:    true,
				hasMeta:    false,
				hasError:   false,
			},
		},
		{
			name: "5. status 400 with data -> 400 response",
			args: args{status: 400, data: gin.H{"error": "bad"}, meta: nil},
			wants: wants{
				statusCode: 400,
				hasData:    true,
				hasMeta:    false,
				hasError:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			JSON(c, tt.args.status, tt.args.data, tt.args.meta)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")

			var payload Envelope
			err := json.Unmarshal(w.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.hasData {
				assert.NotNil(t, payload.Data, "data must be present")
			} else {
				assert.Nil(t, payload.Data, "data must be absent")
			}

			if tt.wants.hasMeta {
				assert.NotNil(t, payload.Meta, "meta must be present")
				if tt.wants.metaKey != "" {
					metaMap, ok := payload.Meta.(map[string]any)
					require.True(t, ok, "meta must be a map")
					_, exists := metaMap[tt.wants.metaKey]
					assert.True(t, exists, "meta key %s must exist", tt.wants.metaKey)
				}
			} else {
				assert.Nil(t, payload.Meta, "meta must be absent")
			}

			if tt.wants.hasError {
				assert.NotNil(t, payload.Error, "error must be present")
			} else {
				assert.Nil(t, payload.Error, "error must be absent")
			}
		})
	}
}

// ─── OK ───────────────────────────────────────────────────────────────────────

func TestOK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		data any
	}
	type wants struct {
		statusCode int
		hasData    bool
		hasMeta    bool
		hasError   bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. data provided -> 200 with data no meta",
			args: args{data: gin.H{"id": "user-1"}},
			wants: wants{statusCode: 200, hasData: true, hasMeta: false, hasError: false},
		},
		{
			name: "2. nil data -> 200 data absent",
			args: args{data: nil},
			wants: wants{statusCode: 200, hasData: false, hasMeta: false, hasError: false},
		},
		{
			name: "3. empty slice -> 200 with empty array",
			args: args{data: []any{}},
			wants: wants{statusCode: 200, hasData: true, hasMeta: false, hasError: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			OK(c, tt.args.data)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")

			var payload Envelope
			err := json.Unmarshal(w.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.hasData {
				assert.NotNil(t, payload.Data, "data must be present")
			} else {
				assert.Nil(t, payload.Data, "data must be absent")
			}
			assert.Nil(t, payload.Meta, "meta must always be absent")
			assert.Nil(t, payload.Error, "error must be absent")
		})
	}
}

// ─── OKWithMeta ───────────────────────────────────────────────────────────────

func TestOKWithMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		data any
		meta any
	}
	type wants struct {
		statusCode int
		hasData    bool
		hasMeta    bool
		hasError   bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. data and meta both provided -> 200 with both",
			args: args{data: []string{"a", "b"}, meta: gin.H{"page": 1, "total": 2}},
			wants: wants{statusCode: 200, hasData: true, hasMeta: true, hasError: false},
		},
		{
			name: "2. data only meta nil -> 200 data without meta",
			args: args{data: gin.H{"id": "1"}, meta: nil},
			wants: wants{statusCode: 200, hasData: true, hasMeta: false, hasError: false},
		},
		{
			name: "3. meta only data nil -> 200 meta without data",
			args: args{data: nil, meta: gin.H{"count": 5}},
			wants: wants{statusCode: 200, hasData: false, hasMeta: true, hasError: false},
		},
		{
			name: "4. both nil -> 200 with neither",
			args: args{data: nil, meta: nil},
			wants: wants{statusCode: 200, hasData: false, hasMeta: false, hasError: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			OKWithMeta(c, tt.args.data, tt.args.meta)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")

			var payload Envelope
			err := json.Unmarshal(w.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.hasData {
				assert.NotNil(t, payload.Data, "data must be present")
			} else {
				assert.Nil(t, payload.Data, "data must be absent")
			}

			if tt.wants.hasMeta {
				assert.NotNil(t, payload.Meta, "meta must be present")
			} else {
				assert.Nil(t, payload.Meta, "meta must be absent")
			}

			assert.Nil(t, payload.Error, "error must be absent")
		})
	}
}

// ─── Created ──────────────────────────────────────────────────────────────────

func TestCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		data any
	}
	type wants struct {
		statusCode int
		hasData    bool
		hasMeta    bool
		hasError   bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. data provided -> 201 created with data",
			args: args{data: gin.H{"id": "new-resource-1"}},
			wants: wants{statusCode: 201, hasData: true, hasMeta: false, hasError: false},
		},
		{
			name: "2. nil data -> 201 data absent",
			args: args{data: nil},
			wants: wants{statusCode: 201, hasData: false, hasMeta: false, hasError: false},
		},
		{
			name: "3. complex object -> 201 with object",
			args: args{data: gin.H{"id": "id", "name": "test", "status": "active"}},
			wants: wants{statusCode: 201, hasData: true, hasMeta: false, hasError: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			Created(c, tt.args.data)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")

			var payload Envelope
			err := json.Unmarshal(w.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.hasData {
				assert.NotNil(t, payload.Data, "data must be present")
			} else {
				assert.Nil(t, payload.Data, "data must be absent")
			}
			assert.Nil(t, payload.Meta, "meta must be absent")
			assert.Nil(t, payload.Error, "error must be absent")
		})
	}
}

// ─── Fail ─────────────────────────────────────────────────────────────────────

func TestFail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		appErr *appErrors.AppError
	}
	type wants struct {
		statusCode int
		errCode    string
		errMsg     string
		hasData    bool
		hasMeta    bool
		hasError   bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. nil appErr -> 500 internal error default error response",
			args: args{appErr: nil},
			wants: wants{
				statusCode: 500,
				errCode:    "internal_error",
				errMsg:     "internal server error",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "2. 400 BadRequest -> 400 error response",
			args: args{appErr: appErrors.BadRequest("validation_error", "invalid input", nil)},
			wants: wants{
				statusCode: 400,
				errCode:    "validation_error",
				errMsg:     "invalid input",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "3. 401 Unauthorized -> 401 error response",
			args: args{appErr: appErrors.Unauthorized("auth_failed", "invalid credentials", nil)},
			wants: wants{
				statusCode: 401,
				errCode:    "auth_failed",
				errMsg:     "invalid credentials",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "4. 403 Forbidden -> 403 error response",
			args: args{appErr: appErrors.Forbidden("forbidden_action", "not authorized", nil)},
			wants: wants{
				statusCode: 403,
				errCode:    "forbidden_action",
				errMsg:     "not authorized",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "5. 404 NotFound -> 404 error response",
			args: args{appErr: appErrors.NotFound("resource_not_found", "user does not exist", nil)},
			wants: wants{
				statusCode: 404,
				errCode:    "resource_not_found",
				errMsg:     "user does not exist",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "6. 409 Conflict -> 409 error response",
			args: args{appErr: appErrors.Conflict("duplicate_entry", "already exists", nil)},
			wants: wants{
				statusCode: 409,
				errCode:    "duplicate_entry",
				errMsg:     "already exists",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "7. 500 Internal -> 500 error response",
			args: args{appErr: appErrors.Internal("database_error", "query failed", nil)},
			wants: wants{
				statusCode: 500,
				errCode:    "database_error",
				errMsg:     "query failed",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
		{
			name: "8. appErr with Details -> error response includes details",
			args: args{appErr: appErrors.BadRequest("validation_error", "invalid input", gin.H{"field": "name", "reason": "too short"})},
			wants: wants{
				statusCode: 400,
				errCode:    "validation_error",
				errMsg:     "invalid input",
				hasData:    false,
				hasMeta:    false,
				hasError:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			Fail(c, tt.args.appErr)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")

			var payload Envelope
			err := json.Unmarshal(w.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			assert.Nil(t, payload.Data, "data must be absent")
			assert.Nil(t, payload.Meta, "meta must be absent")

			if tt.wants.hasError {
				require.NotNil(t, payload.Error, "error must be present")
				assert.Equal(t, tt.wants.errCode, payload.Error.Code, "error code")
				assert.Equal(t, tt.wants.errMsg, payload.Error.Message, "error message")
			} else {
				assert.Nil(t, payload.Error, "error must be absent")
			}
		})
	}
}

// ─── FailFromError ────────────────────────────────────────────────────────────

func TestFailFromError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		err error
	}
	type wants struct {
		statusCode int
		errCode    string
		hasError   bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. nil error -> 500 internal error via Extract",
			args: args{err: nil},
			wants: wants{
				statusCode: 500,
				errCode:    "internal_error",
				hasError:   true,
			},
		},
		{
			name: "2. standard error (not AppError) -> 500 internal error",
			args: args{err: stdErrors.New("something went wrong")},
			wants: wants{
				statusCode: 500,
				errCode:    "internal_error",
				hasError:   true,
			},
		},
		{
			name: "3. AppError as error -> uses original status and code",
			args: args{err: appErrors.BadRequest("custom_error", "custom message", nil)},
			wants: wants{
				statusCode: 400,
				errCode:    "custom_error",
				hasError:   true,
			},
		},
		{
			name: "4. AppError 401 Unauthorized -> 401 status preserved",
			args: args{err: appErrors.Unauthorized("auth_failed", "invalid token", nil)},
			wants: wants{
				statusCode: 401,
				errCode:    "auth_failed",
				hasError:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			FailFromError(c, tt.args.err)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")

			var payload Envelope
			err := json.Unmarshal(w.Body.Bytes(), &payload)
			require.NoError(t, err, "response must be valid JSON")

			if tt.wants.hasError {
				require.NotNil(t, payload.Error, "error must be present")
				assert.Equal(t, tt.wants.errCode, payload.Error.Code, "error code")
			}

			assert.Nil(t, payload.Data, "data must be absent")
			assert.Nil(t, payload.Meta, "meta must be absent")
		})
	}
}
