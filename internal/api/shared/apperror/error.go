package apperror

import (
	"fmt"
)

// ErrorCode represents the type of error.
type ErrorCode string

const (
	ErrInternal      ErrorCode = "INTERNAL"
	ErrUnauthorized  ErrorCode = "UNAUTHORIZED"
	ErrForbidden     ErrorCode = "FORBIDDEN"
	ErrNotFound      ErrorCode = "NOT_FOUND"
	ErrConflict      ErrorCode = "CONFLICT"
	ErrBadRequest    ErrorCode = "BAD_REQUEST"
	ErrValidation    ErrorCode = "VALIDATION"
	ErrTimeout       ErrorCode = "TIMEOUT"
	ErrDB            ErrorCode = "DB_ERROR"
	ErrRateLimited   ErrorCode = "RATE_LIMITED"
)

// AppError defines a structured application error.
type AppError interface {
	error
	Code() ErrorCode
	Message() string
	Status() int
	Unwrap() error
}

type appError struct {
	code    ErrorCode
	message string
	status  int
	err     error
}

func (e *appError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.code, e.message, e.err)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

func (e *appError) Code() ErrorCode {
	return e.code
}

func (e *appError) Message() string {
	return e.message
}

func (e *appError) Status() int {
	return e.status
}

// Unwrap returns the underlying wrapped error, allowing standard library
// functions like errors.Is and errors.As to work seamlessly.
func (e *appError) Unwrap() error {
	return e.err
}
