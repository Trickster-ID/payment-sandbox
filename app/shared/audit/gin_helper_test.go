package audit

// Branch map for gin helper functions (Section 3.1 of the plan):
//
// ActorFromContext(c):
// ├── ContextUserID not in context             -> actorID=""
// ├── ContextUserID in context, wrong type     -> actorID=""
// ├── ContextUserID in context, correct type   -> actorID=value
// ├── ContextRole not in context               -> actorType="public"
// ├── ContextRole in context, wrong type       -> actorType="public"
// ├── ContextRole in context, empty string     -> actorType="public"
// └── ContextRole in context, correct value    -> actorType=lowercase(role)
//
// RequestIDFromContext(c):
// ├── ContextRequestID not in context          -> return ""
// ├── ContextRequestID in context, wrong type  -> return ""
// └── ContextRequestID in context, correct type -> return value
//
// LogBestEffort(c, logger, event):
// ├── logger.Log succeeds                      -> no error log
// └── logger.Log fails                         -> logs error to log.Printf

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"payment-sandbox/app/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestActorFromContext(t *testing.T) {
	type fields struct {
		userID, role, otherUserIDType, otherRoleType any
	}
	type wants struct {
		actorID   string
		actorType string
	}

	tests := []struct {
		name   string
		fields fields
		wants  wants
	}{
		{
			name:   "1. ContextUserID not in context -> actorID empty",
			fields: fields{userID: nil},
			wants:  wants{actorID: "", actorType: "public"},
		},
		{
			name:   "2. ContextUserID wrong type (int) -> actorID empty",
			fields: fields{userID: 123, otherUserIDType: true},
			wants:  wants{actorID: "", actorType: "public"},
		},
		{
			name:   "3. ContextUserID correct, ContextRole not in context -> actorType public",
			fields: fields{userID: "user-1", role: nil},
			wants:  wants{actorID: "user-1", actorType: "public"},
		},
		{
			name:   "4. ContextRole wrong type (int) -> actorType public",
			fields: fields{userID: "user-1", role: 999, otherRoleType: true},
			wants:  wants{actorID: "user-1", actorType: "public"},
		},
		{
			name:   "5. ContextRole empty string -> actorType public",
			fields: fields{userID: "user-1", role: ""},
			wants:  wants{actorID: "user-1", actorType: "public"},
		},
		{
			name:   "6. ContextUserID and ContextRole both present, lowercase applied to role",
			fields: fields{userID: "user-1", role: "MERCHANT"},
			wants:  wants{actorID: "user-1", actorType: "merchant"},
		},
		{
			name:   "7. mixed case role -> lowercased",
			fields: fields{userID: "user-1", role: "MeRcHaNt"},
			wants:  wants{actorID: "user-1", actorType: "merchant"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			if tt.fields.userID != nil {
				c.Set(middleware.ContextUserID, tt.fields.userID)
			}
			if tt.fields.role != nil {
				c.Set(middleware.ContextRole, tt.fields.role)
			}

			actorID, actorType := ActorFromContext(c)

			assert.Equal(t, tt.wants.actorID, actorID, "actor ID")
			assert.Equal(t, tt.wants.actorType, actorType, "actor type")
		})
	}
}

func TestRequestIDFromContext(t *testing.T) {
	type fields struct {
		requestID any
	}
	type wants struct {
		value string
	}

	tests := []struct {
		name   string
		fields fields
		wants  wants
	}{
		{
			name:   "1. ContextRequestID not in context -> empty string",
			fields: fields{requestID: nil},
			wants:  wants{value: ""},
		},
		{
			name:   "2. ContextRequestID wrong type (int) -> empty string",
			fields: fields{requestID: 12345},
			wants:  wants{value: ""},
		},
		{
			name:   "3. ContextRequestID correct type -> return value",
			fields: fields{requestID: "req-123"},
			wants:  wants{value: "req-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			if tt.fields.requestID != nil {
				c.Set(middleware.ContextRequestID, tt.fields.requestID)
			}

			value := RequestIDFromContext(c)

			assert.Equal(t, tt.wants.value, value, "request ID")
		})
	}
}

type testMockLogger struct {
	mock.Mock
}

func (m *testMockLogger) Log(ctx context.Context, event Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func TestLogBestEffort(t *testing.T) {

	type args struct {
		event Event
	}
	type mockSetup struct {
		setup func(m *testMockLogger)
	}

	tests := []struct {
		name      string
		args      args
		mockSetup mockSetup
	}{
		{
			name: "1. logger.Log succeeds -> no error",
			args: args{event: Event{EventType: "test"}},
			mockSetup: mockSetup{setup: func(m *testMockLogger) {
				m.On("Log", mock.Anything, mock.Anything).Return(nil).Once()
			}},
		},
		{
			name: "2. logger.Log fails -> logs error (function doesn't return error)",
			args: args{event: Event{EventType: "test"}},
			mockSetup: mockSetup{setup: func(m *testMockLogger) {
				m.On("Log", mock.Anything, mock.Anything).Return(errors.New("write failed")).Once()
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request, _ = http.NewRequest("POST", "/test", nil)

			logger := &testMockLogger{}
			tt.mockSetup.setup(logger)

			// LogBestEffort should not panic regardless of logger result
			assert.NotPanics(t, func() {
				LogBestEffort(c, logger, tt.args.event)
			}, "should not panic")

			logger.AssertExpectations(t)
		})
	}
}
