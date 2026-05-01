package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type Middleware struct {
	Store *Store
	Cache *Cache
}

type recorder struct {
	gin.ResponseWriter
	body bytes.Buffer
	code int
}

func (r *recorder) Write(b []byte) (int, error)       { r.body.Write(b); return r.ResponseWriter.Write(b) }
func (r *recorder) WriteString(s string) (int, error) { return r.Write([]byte(s)) }
func (r *recorder) WriteHeader(code int)              { r.code = code; r.ResponseWriter.WriteHeader(code) }

func (m *Middleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			response.Fail(c, appErrors.BadRequest("idempotency_key_required", "Idempotency-Key header is required", nil))
			c.Abort()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			response.Fail(c, appErrors.BadRequest("invalid_request_body", "cannot read body", nil))
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		hash := hashBytes(body)

		if m.Cache != nil && m.Cache.Client != nil {
			if cached, _ := m.Cache.Get(c.Request.Context(), key); cached != nil {
				if cached.RequestHash != hash {
					response.Fail(c, appErrors.Conflict("idempotency_key_conflict", "idempotency key reused with different payload", nil))
					c.Abort()
					return
				}
				c.Data(cached.Code, "application/json", cached.Body)
				c.Abort()
				return
			}
		}

		if m.Store == nil || m.Store.DB == nil {
			c.Next()
			return
		}

		userID, _ := c.Get("user_id")
		userIDStr, _ := userID.(string)
		err = m.Store.Claim(c.Request.Context(), key, userIDStr, hash)
		if err != nil {
			rec, ferr := m.Store.Fetch(c.Request.Context(), key)
			if ferr != nil || rec == nil {
				response.Fail(c, appErrors.Internal("idempotency_lookup_failed", "idempotency lookup failed", nil))
				c.Abort()
				return
			}
			if rec.RequestHash != hash {
				response.Fail(c, appErrors.Conflict("idempotency_key_conflict", "idempotency key reused with different payload", nil))
				c.Abort()
				return
			}
			if rec.Status == "in_progress" {
				response.Fail(c, appErrors.Conflict("idempotency_in_progress", "request still processing", nil))
				c.Abort()
				return
			}
			c.Data(rec.ResponseCode, "application/json", rec.ResponseBody)
			c.Abort()
			return
		}

		rec := &recorder{ResponseWriter: c.Writer, code: http.StatusOK}
		c.Writer = rec
		c.Next()

		_ = m.Store.Complete(c.Request.Context(), key, rec.code, rec.body.Bytes())
		if m.Cache != nil && m.Cache.Client != nil {
			_ = m.Cache.Set(c.Request.Context(), key, CachedResponse{
				RequestHash: hash, Code: rec.code, Body: rec.body.Bytes(),
			})
		}
	}
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
