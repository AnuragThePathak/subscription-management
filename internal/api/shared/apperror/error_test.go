package apperror_test

import (
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Factory Contract & HTTP Mapping
// ---------------------------------------------------------------------------

func TestAppError_CodeAndStatus(t *testing.T) {
	cause := errors.New("root cause")
	const msg = "custom message"

	tests := []struct {
		name       string
		err        apperror.AppError
		wantCode   apperror.ErrorCode
		wantStatus int
		wantMsg    string
		wantCause  error
	}{
		{
			name:       "NewInternalError",
			err:        apperror.NewInternalError(cause),
			wantCode:   apperror.ErrInternal,
			wantStatus: http.StatusInternalServerError,
			wantCause:  cause,
		},
		{
			name:       "NewTimeoutError",
			err:        apperror.NewTimeoutError(cause),
			wantCode:   apperror.ErrTimeout,
			wantStatus: http.StatusGatewayTimeout,
			wantCause:  cause,
		},
		{
			name:       "NewUnauthorizedError",
			err:        apperror.NewUnauthorizedError(msg),
			wantCode:   apperror.ErrUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantMsg:    msg,
		},
		{
			name:       "NewForbiddenError",
			err:        apperror.NewForbiddenError(msg),
			wantCode:   apperror.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantMsg:    msg,
		},
		{
			name:       "NewValidationError",
			err:        apperror.NewValidationError(msg),
			wantCode:   apperror.ErrValidation,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "NewNotFoundError",
			err:        apperror.NewNotFoundError(msg),
			wantCode:   apperror.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    msg,
		},
		{
			name:       "NewConflictError",
			err:        apperror.NewConflictError(msg),
			wantCode:   apperror.ErrConflict,
			wantStatus: http.StatusConflict,
			wantMsg:    msg,
		},
		{
			name:       "NewBadRequestError",
			err:        apperror.NewBadRequestError(msg),
			wantCode:   apperror.ErrBadRequest,
			wantStatus: http.StatusBadRequest,
			wantMsg:    msg,
		},
		{
			name:       "NewDBError",
			err:        apperror.NewDBError(cause),
			wantCode:   apperror.ErrDB,
			wantStatus: http.StatusInternalServerError,
			wantCause:  cause,
		},
		{
			name:       "NewRateLimitError",
			err:        apperror.NewRateLimitError(msg),
			wantCode:   apperror.ErrRateLimited,
			wantStatus: http.StatusTooManyRequests,
			wantMsg:    msg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantCode, tt.err.Code())
			assert.Equal(t, tt.wantStatus, tt.err.Status())

			// Error specific assertion
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, tt.err.Message())
			}
			if tt.wantCause != nil {
				assert.ErrorIs(t, tt.err, tt.wantCause,
					"errors.Is should unwrap and find the root cause")
				assert.Equal(t, tt.wantCause, tt.err.Unwrap())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error() string format
// ---------------------------------------------------------------------------

func TestAppError_ErrorString(t *testing.T) {
	t.Run("with wrapped cause", func(t *testing.T) {
		cause := errors.New("disk full")
		err := apperror.NewDBError(cause)
		s := err.Error()

		assert.Contains(t, s, string(apperror.ErrDB))
		assert.Contains(t, s, "disk full")
	})

	t.Run("without wrapped cause", func(t *testing.T) {
		err := apperror.NewNotFoundError("user not found")
		s := err.Error()

		assert.Contains(t, s, string(apperror.ErrNotFound))
		assert.Contains(t, s, "user not found")
		// No parenthesized cause section.
		assert.NotContains(t, s, "(",
			"Should not have parenthesis if no wrapped error")
	})
}

// ---------------------------------------------------------------------------
// WithLogAttributes
// ---------------------------------------------------------------------------

func TestAppError_WithLogAttributes(t *testing.T) {
	attr1 := slog.String("user_id", "abc123")
	attr2 := slog.Int("attempt", 3)

	err := apperror.NewUnauthorizedError("bad token").
		WithLogAttributes(attr1, attr2)

	attrs := err.LogAttributes()
	require.Len(t, attrs, 2)
	assert.Equal(t, attr1, attrs[0])
	assert.Equal(t, attr2, attrs[1])
}
