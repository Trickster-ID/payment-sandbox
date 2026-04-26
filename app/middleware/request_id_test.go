package middleware

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

	tests := []struct {
		name            string
		requestHeaderID string
	}{
		{
			name:            "uses incoming request id",
			requestHeaderID: "req-123",
		},
		{
			name:            "generates request id when header missing",
			requestHeaderID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestIDMiddleware())
			router.GET("/test", func(c *gin.Context) {
				requestID, _ := c.Get(ContextRequestID)
				c.JSON(http.StatusOK, gin.H{"request_id": requestID})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.requestHeaderID != "" {
				req.Header.Set("X-Request-ID", tc.requestHeaderID)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			responseHeaderID := rec.Header().Get("X-Request-ID")
			assert.NotEmpty(t, responseHeaderID)
			if tc.requestHeaderID != "" {
				assert.Equal(t, tc.requestHeaderID, responseHeaderID)
			}

			var body map[string]string
			err := json.Unmarshal(rec.Body.Bytes(), &body)
			require.NoError(t, err)
			assert.NotEmpty(t, body["request_id"])
			if tc.requestHeaderID != "" {
				assert.Equal(t, tc.requestHeaderID, body["request_id"])
			}
		})
	}
}
