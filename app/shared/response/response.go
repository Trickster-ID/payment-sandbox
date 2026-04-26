package response

import (
	"payment-sandbox/app/shared/errors"

	"github.com/gin-gonic/gin"
)

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Envelope struct {
	Data  any           `json:"data,omitempty"`
	Meta  any           `json:"meta,omitempty"`
	Error *ErrorPayload `json:"error,omitempty"`
}

func JSON(c *gin.Context, status int, data any, meta any) {
	c.JSON(status, Envelope{
		Data: data,
		Meta: meta,
	})
}

func OK(c *gin.Context, data any) {
	JSON(c, 200, data, nil)
}

func OKWithMeta(c *gin.Context, data any, meta any) {
	JSON(c, 200, data, meta)
}

func Created(c *gin.Context, data any) {
	JSON(c, 201, data, nil)
}

func Fail(c *gin.Context, appErr *errors.AppError) {
	if appErr == nil {
		appErr = errors.Internal("internal_error", "internal server error", nil)
	}

	c.JSON(appErr.Status, Envelope{
		Error: &ErrorPayload{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
}

func FailFromError(c *gin.Context, err error) {
	Fail(c, errors.Extract(err))
}
