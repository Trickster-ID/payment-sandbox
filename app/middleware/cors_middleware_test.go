package middleware

// Branch analysis for CORSMiddleware():
// ├── c.Request.Method == "OPTIONS" → sets CORS headers, AbortWithStatus(204)
// └── any other method              → sets CORS headers, c.Next()

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type args struct {
		method         string
		originHeader   string
		allowedOrigins []string
	}
	type wants struct {
		statusCode       int
		aborted          bool
		allowOrigin      string
		allowMethods     string
		allowHeaders     string
		allowCredentials string
		exposeHeaders    string
		maxAge           string
		vary             string
	}

	const (
		prodOrigin      = "https://payment.pikri.my.id"
		localOrigin     = "http://localhost"
		wantMaxAge      = "86400"
		wantMethods     = "POST, GET, OPTIONS, PUT, DELETE, UPDATE, PATCH"
		wantHeaders     = "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Channel, X-Request-Id, Idempotency-Key"
		wantExpose      = "Content-Length"
		wantCredentials = "true"
	)
	multiOrigins := []string{prodOrigin, localOrigin}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. OPTIONS request with matching origin -> 204 aborted, origin echoed",
			args: args{method: http.MethodOptions, originHeader: prodOrigin, allowedOrigins: []string{prodOrigin}},
			wants: wants{
				statusCode:       http.StatusNoContent,
				aborted:          true,
				allowOrigin:      prodOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
				vary:             "Origin",
			},
		},
		{
			name: "2. GET request with matching origin -> not aborted, origin echoed",
			args: args{method: http.MethodGet, originHeader: prodOrigin, allowedOrigins: []string{prodOrigin}},
			wants: wants{
				statusCode:       http.StatusOK,
				aborted:          false,
				allowOrigin:      prodOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
				vary:             "Origin",
			},
		},
		{
			name: "3. POST request with matching origin -> not aborted, origin echoed",
			args: args{method: http.MethodPost, originHeader: prodOrigin, allowedOrigins: []string{prodOrigin}},
			wants: wants{
				statusCode:       http.StatusOK,
				aborted:          false,
				allowOrigin:      prodOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
				vary:             "Origin",
			},
		},
		{
			name: "4. multi-origin config: second allowed origin (local compose FE) is echoed back",
			args: args{method: http.MethodGet, originHeader: localOrigin, allowedOrigins: multiOrigins},
			wants: wants{
				statusCode:       http.StatusOK,
				allowOrigin:      localOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
				vary:             "Origin",
			},
		},
		{
			name: "5. unrecognized origin -> falls back to first configured origin (not reflected)",
			args: args{method: http.MethodGet, originHeader: "https://evil.example.com", allowedOrigins: multiOrigins},
			wants: wants{
				statusCode:       http.StatusOK,
				allowOrigin:      prodOrigin,
				allowMethods:     wantMethods,
				allowHeaders:     wantHeaders,
				allowCredentials: wantCredentials,
				exposeHeaders:    wantExpose,
				maxAge:           wantMaxAge,
				vary:             "Origin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, c := ginCtx(tt.args.method, "/")
			require.NotNil(t, w)
			require.NotNil(t, c)
			c.Request.Header.Set("Origin", tt.args.originHeader)

			CORSMiddleware(tt.args.allowedOrigins)(c)

			assert.Equal(t, tt.wants.statusCode, w.Code, "status code")
			assert.Equal(t, tt.wants.aborted, c.IsAborted(), "aborted")
			assert.Equal(t, tt.wants.allowOrigin, w.Header().Get("Access-Control-Allow-Origin"), "Allow-Origin header")
			assert.Equal(t, tt.wants.maxAge, w.Header().Get("Access-Control-Max-Age"), "Max-Age header")
			assert.Equal(t, tt.wants.allowMethods, w.Header().Get("Access-Control-Allow-Methods"), "Allow-Methods header")
			assert.Equal(t, tt.wants.allowHeaders, w.Header().Get("Access-Control-Allow-Headers"), "Allow-Headers header")
			assert.Equal(t, tt.wants.exposeHeaders, w.Header().Get("Access-Control-Expose-Headers"), "Expose-Headers header")
			assert.Equal(t, tt.wants.allowCredentials, w.Header().Get("Access-Control-Allow-Credentials"), "Allow-Credentials header")
			assert.Equal(t, tt.wants.vary, w.Header().Get("Vary"), "Vary header")
		})
	}
}
