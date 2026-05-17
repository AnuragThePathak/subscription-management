package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeout_Middleware(t *testing.T) {
	tests := []struct {
		name            string
		timeoutDuration time.Duration
		handlerDelay    time.Duration
		expectTimeout   bool
	}{
		{
			name:            "success - context has deadline and finishes in time",
			timeoutDuration: 50 * time.Millisecond,
			handlerDelay:    0, // Finishes instantly
			expectTimeout:   false,
		},
		{
			name:            "success - context deadline expires when handler is too slow",
			timeoutDuration: 10 * time.Millisecond,
			handlerDelay:    20 * time.Millisecond, // Intentionally sleep past the timeout
			expectTimeout:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nextCalled bool
			var capturedCtx context.Context
			var capturedCtxErr error

			// Setup Dummy Handler to capture context and simulate work
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				capturedCtx = r.Context()

				// Simulate the downstream service doing heavy database work
				if tt.handlerDelay > 0 {
					time.Sleep(tt.handlerDelay)
				}

				// Capture error here, inside the handler, before defer cancel() fires
				capturedCtxErr = r.Context().Err()
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with the middleware
			middleware := middlewares.Timeout(tt.timeoutDuration)
			handler := middleware(nextHandler)

			// Execute Request
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Assert HTTP Wiring
			require.True(t, nextCalled, "Middleware must call next handler")

			// Assert Context Construction (The Vault Lock)
			deadline, ok := capturedCtx.Deadline()
			require.True(t, ok, "Context MUST have a deadline set by the middleware")
			require.False(t, deadline.IsZero(), "Deadline should not be zero")

			// Assert Time Mechanics
			// Note: capturedCtxErr is read inside the handler, before the middleware's
			// defer cancel() fires. Reading capturedCtx.Err() after ServeHTTP returns
			// would always yield context.Canceled due to the deferred cancel.
			if tt.expectTimeout {
				// If we slept longer than the timeout, the context should mathematically be dead
				assert.ErrorIs(t, capturedCtxErr, context.DeadlineExceeded, "Expected context to be canceled by timeout")
			} else {
				// If we finished fast, the context should still be alive
				assert.NoError(t, capturedCtxErr, "Expected context to remain alive")
			}
		})
	}
}