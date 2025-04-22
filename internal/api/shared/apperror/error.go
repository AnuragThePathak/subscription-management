package apperror

import (
	"errors"
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
	ErrUnprocessable ErrorCode = "UNPROCESSABLE"
	ErrRateLimited   ErrorCode = "RATE_LIMITED"
)

// AppError defines a structured application error.
type AppError interface {
	error
	Code() ErrorCode
	Message() string
	Status() int
	Unwrap() error
	Is(target error) bool
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

func (e *appError) Unwrap() error {
	return e.err
}

func (e *appError) Is(target error) bool {
	return errors.Is(e, target)
}
