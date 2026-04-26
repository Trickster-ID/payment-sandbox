package errors

import (
	"errors"
	"net/http"
)

type AppError struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e *AppError) Error() string {
	return e.Message
}

func New(status int, code, message string, details any) *AppError {
	return &AppError{
		Status:  status,
		Code:    code,
		Message: message,
		Details: details,
	}
}

func BadRequest(code, message string, details any) *AppError {
	return New(http.StatusBadRequest, code, message, details)
}

func Unauthorized(code, message string, details any) *AppError {
	return New(http.StatusUnauthorized, code, message, details)
}

func Forbidden(code, message string, details any) *AppError {
	return New(http.StatusForbidden, code, message, details)
}

func NotFound(code, message string, details any) *AppError {
	return New(http.StatusNotFound, code, message, details)
}

func Conflict(code, message string, details any) *AppError {
	return New(http.StatusConflict, code, message, details)
}

func Internal(code, message string, details any) *AppError {
	return New(http.StatusInternalServerError, code, message, details)
}

func Extract(err error) *AppError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return Internal("internal_error", "internal server error", nil)
}
