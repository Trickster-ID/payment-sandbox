package middleware

// Branch analysis for RequestIDMiddleware():
// ├── X-Request-ID header set and non-empty after TrimSpace → use existing ID
// ├── X-Request-ID header absent                           → generate UUID
// └── X-Request-ID header whitespace-only                  → TrimSpace→"" → generate UUID

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		requestHeaderID string // empty string means "do not set header"
		setHeader       bool
	}
	type wants struct {
		useProvidedID bool   // true → response header + ctx must equal requestHeaderID
		responseID    string // only checked when useProvidedID is true
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. X-Request-ID header provided -> used as-is in context and response header",
			args:  args{requestHeaderID: "req-abc-123", setHeader: true},
			wants: wants{useProvidedID: true, responseID: "req-abc-123"},
		},
		{
			name:  "2. X-Request-ID header absent -> UUID generated, set in context and response header",
			args:  args{requestHeaderID: "", setHeader: false},
			wants: wants{useProvidedID: false},
		},
		{
			name:  "3. X-Request-ID header whitespace-only -> trimmed to empty, UUID generated",
			args:  args{requestHeaderID: "   ", setHeader: true},
			wants: wants{useProvidedID: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := gin.New()
			router.Use(RequestIDMiddleware())
			router.GET("/test", func(c *gin.Context) {
				requestID, _ := c.Get(ContextRequestID)
				c.JSON(http.StatusOK, gin.H{"request_id": requestID})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.args.setHeader {
				req.Header.Set("X-Request-ID", tt.args.requestHeaderID)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "status code")

			// response header
			responseHeaderID := rec.Header().Get("X-Request-ID")
			assert.NotEmpty(t, responseHeaderID, "X-Request-ID response header must not be empty")

			// context value (from JSON body)
			var body map[string]string
			err := json.Unmarshal(rec.Body.Bytes(), &body)
			require.NoError(t, err, "body must be valid JSON")
			assert.NotEmpty(t, body["request_id"], "request_id in body must not be empty")

			if tt.wants.useProvidedID {
				// provided header must be echoed back unchanged
				assert.Equal(t, tt.wants.responseID, responseHeaderID, "response header matches provided ID")
				assert.Equal(t, tt.wants.responseID, body["request_id"], "body request_id matches provided ID")
			} else {
				// generated ID: header and body must agree
				assert.Equal(t, responseHeaderID, body["request_id"], "generated ID consistent across header and body")
				// whitespace-only must NOT be echoed as-is
				if tt.args.setHeader && tt.args.requestHeaderID != "" {
					assert.NotEqual(t, tt.args.requestHeaderID, responseHeaderID, "whitespace ID must not be used verbatim")
				}
			}
		})
	}
}
