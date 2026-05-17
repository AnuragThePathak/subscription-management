package middlewares_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// RateLimiter middleware
// ---------------------------------------------------------------------------

func TestRateLimiter(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string

		// The Shared Truth
		isAllowed  bool
		remaining  int
		retryAfter time.Duration

		// Directives
		setupMocks func(
			svc *mocks.MockRateLimiterService,
			ip string,
			allowed bool,
			remaining int,
			retry time.Duration,
		)
		wantStatus    int
		wantNextCall  bool
		expectHeaders bool
	}{
		{
			name:       "success - request allowed with remaining quota",
			remoteAddr: "192.168.1.1:1234",
			isAllowed:  true,
			remaining:  5,
			retryAfter: 0,
			setupMocks: func(svc *mocks.MockRateLimiterService, ip string, allowed bool, rem int, retry time.Duration) {
				svc.EXPECT().
					Allowed(mock.Anything, ip).
					Return(allowed, rem, retry, nil).
					Once()
			},
			wantStatus:    http.StatusOK,
			wantNextCall:  true,
			expectHeaders: true,
		},
		{
			name:       "success (fail-open) - service error allows request through",
			remoteAddr: "192.168.1.1:1234",
			setupMocks: func(svc *mocks.MockRateLimiterService, ip string, allowed bool, rem int, retry time.Duration) {
				svc.EXPECT().
					Allowed(mock.Anything, ip).
					Return(allowed, rem, retry, errors.New("redis connection refused")).
					Once()
			},
			wantStatus:    http.StatusOK,
			wantNextCall:  true,
			expectHeaders: false, // Middleware skips headers and fails open on error
		},
		{
			name:       "error - malformed remote address",
			remoteAddr: "invalid-ip-format",
			setupMocks: func(svc *mocks.MockRateLimiterService, _ string, _ bool, _ int, _ time.Duration) {
				// Service should never be called
			},
			wantStatus:    http.StatusBadRequest,
			wantNextCall:  false,
			expectHeaders: false,
		},
		{
			name:       "error - rate limit exceeded",
			remoteAddr: "192.168.1.1:1234",
			isAllowed:  false,
			remaining:  0,
			retryAfter: 60 * time.Second, // Represents 60 seconds
			setupMocks: func(svc *mocks.MockRateLimiterService, ip string, allowed bool, rem int, retry time.Duration) {
				svc.EXPECT().
					Allowed(mock.Anything, ip).
					Return(allowed, rem, retry, nil).
					Once()
			},
			wantStatus:    http.StatusTooManyRequests,
			wantNextCall:  false,
			expectHeaders: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewMockRateLimiterService(t)

			ip := strings.Split(tt.remoteAddr, ":")[0]
			tt.setupMocks(svc, ip, tt.isAllowed, tt.remaining, tt.retryAfter)

			// Setup Dummy Handler
			var nextCalled bool
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with the middleware
			middleware := middlewares.RateLimiter(svc)
			handler := middleware(nextHandler)

			// Execute Request
			req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}
			rr := httptest.NewRecorder()

			// Skipping Otel here as non-essential actions only
			handler.ServeHTTP(rr, req)

			// Assert HTTP Wiring
			require.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, tt.wantNextCall, nextCalled, "Mismatch in expected execution of next handler")

			// Assert HTTP Headers using the Shared Truth
			if tt.expectHeaders {
				assert.Equal(t, strconv.Itoa(tt.remaining), rr.Header().Get("X-RateLimit-Remaining"))

				if !tt.isAllowed {
					assert.Equal(t, strconv.Itoa(int(tt.retryAfter.Seconds())), rr.Header().Get("Retry-After"))
				} else {
					assert.Empty(t, rr.Header().Get("Retry-After"), "Retry-After should not be set for allowed requests")
				}
			}
		})
	}
}
