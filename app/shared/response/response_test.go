package response

import (
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appErrors "payment-sandbox/app/shared/errors"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		handler    gin.HandlerFunc
		wantStatus int
		wantHasKey string
	}{
		{
			name: "ok response",
			path: "/ok",
			handler: func(c *gin.Context) {
				OK(c, gin.H{"id": "1"})
			},
			wantStatus: http.StatusOK,
			wantHasKey: "data",
		},
		{
			name: "created response",
			path: "/created",
			handler: func(c *gin.Context) {
				Created(c, gin.H{"id": "2"})
			},
			wantStatus: http.StatusCreated,
			wantHasKey: "data",
		},
		{
			name: "ok with meta response",
			path: "/meta",
			handler: func(c *gin.Context) {
				OKWithMeta(c, []any{}, gin.H{"page": 1})
			},
			wantStatus: http.StatusOK,
			wantHasKey: "meta",
		},
		{
			name: "fail from app error",
			path: "/fail-app",
			handler: func(c *gin.Context) {
				Fail(c, appErrors.BadRequest("validation_error", "invalid payload", nil))
			},
			wantStatus: http.StatusBadRequest,
			wantHasKey: "error",
		},
		{
			name: "fail from generic error",
			path: "/fail-generic",
			handler: func(c *gin.Context) {
				FailFromError(c, stdErrors.New("boom"))
			},
			wantStatus: http.StatusInternalServerError,
			wantHasKey: "error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.GET(tc.path, tc.handler)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)

			var payload map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &payload)
			require.NoError(t, err)
			_, ok := payload[tc.wantHasKey]
			assert.True(t, ok)
		})
	}
}
