package apperror

import "net/http"

// Generic
func NewInternalError(err error) AppError {
	return &appError{
		code:    ErrInternal,
		message: "Something went wrong",
		status:  http.StatusInternalServerError,
		err:     err,
	}
}

func NewTimeoutError(err error) AppError {
	return &appError{
		code:    ErrTimeout,
		message: "Request timed out",
		status:  http.StatusGatewayTimeout,
		err:     err,
	}
}

// Auth
func NewUnauthorizedError(msg string) AppError {
	return &appError{
		code:    ErrUnauthorized,
		message: msg,
		status:  http.StatusUnauthorized,
	}
}

func NewForbiddenError(msg string) AppError {
	return &appError{
		code:    ErrForbidden,
		message: msg,
		status:  http.StatusForbidden,
	}
}

// Validation
func NewValidationError(msg string) AppError {
	return &appError{
		code:    ErrValidation,
		message: msg,
		status:  http.StatusBadRequest,
	}
}

func NewUnprocessableEntity(msg string) AppError {
	return &appError{
		code:    ErrUnprocessable,
		message: msg,
		status:  http.StatusUnprocessableEntity,
	}
}

// DB & CRUD
func NewNotFoundError(msg string) AppError {
	return &appError{
		code:    ErrNotFound,
		message: msg,
		status:  http.StatusNotFound,
	}
}

func NewConflictError(msg string) AppError {
	return &appError{
		code:    ErrConflict,
		message: msg,
		status:  http.StatusConflict,
	}
}

func NewBadRequestError(msg string) AppError {
	return &appError{
		code:    ErrBadRequest,
		message: msg,
		status:  http.StatusBadRequest,
	}
}

func NewDBError(err error) AppError {
	return &appError{
		code:    ErrDB,
		message: "Database error",
		status:  http.StatusInternalServerError,
		err:     err,
	}
}

// Rate limit / throttling
func NewRateLimitError(msg string) AppError {
	return &appError{
		code:    ErrRateLimited,
		message: msg,
		status:  http.StatusTooManyRequests,
	}
}
