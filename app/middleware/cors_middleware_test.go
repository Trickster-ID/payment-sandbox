package middleware

// Branch analysis for CORSMiddleware():
// ├── c.Request.Method == "OPTIONS" → sets CORS headers, AbortWithStatus(204)
// └── any other method              → sets CORS headers, c.Next()

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		method string
	}
	type wants struct {
		statusCode              int
		aborted                 bool
		allowOrigin             string
		allowMethods            string
		allowHeaders            string
		allowCredentials        string
		exposeHeaders           string
		maxAge                  string
	}

	const (
		wantOrigin      = "*"
		wantMaxAge      = "86400"
		wantMethods     = "POST, GET, OPTIONS, PUT, DELETE, UPDATE, PATCH"
		wantHeaders     = "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Channel, X-Request-Id, Idempotency-Key"
		wantExpose      = "Content-Length"
		wantCredentials = "true"
	)

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. OPTIONS request -> 204 aborted, all CORS headers set",
			args: args{method: http.MethodOptions},
			wants: wants{
				statusCode:       http.StatusNoContent,
				aborted:          true,
				allowOrigin:      wantOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
			},
		},
		{
			name: "2. GET request -> not aborted, all CORS headers set",
			args: args{method: http.MethodGet},
			wants: wants{
				statusCode:       http.StatusOK,
				aborted:          false,
				allowOrigin:      wantOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
			},
		},
		{
			name: "3. POST request -> not aborted, all CORS headers set",
			args: args{method: http.MethodPost},
			wants: wants{
				statusCode:       http.StatusOK,
				aborted:          false,
				allowOrigin:      wantOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, c := ginCtx(tt.args.method, "/")

			CORSMiddleware()(c)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			assert.Equal(t, tt.wants.aborted, c.IsAborted(), "aborted")
			assert.Equal(t, tt.wants.allowOrigin, w.Header().Get("Access-Control-Allow-Origin"), "Allow-Origin header")
			assert.Equal(t, tt.wants.maxAge, w.Header().Get("Access-Control-Max-Age"), "Max-Age header")
			assert.Equal(t, tt.wants.allowMethods, w.Header().Get("Access-Control-Allow-Methods"), "Allow-Methods header")
			assert.Equal(t, tt.wants.allowHeaders, w.Header().Get("Access-Control-Allow-Headers"), "Allow-Headers header")
			assert.Equal(t, tt.wants.exposeHeaders, w.Header().Get("Access-Control-Expose-Headers"), "Expose-Headers header")
			assert.Equal(t, tt.wants.allowCredentials, w.Header().Get("Access-Control-Allow-Credentials"), "Allow-Credentials header")
		})
	}
}
