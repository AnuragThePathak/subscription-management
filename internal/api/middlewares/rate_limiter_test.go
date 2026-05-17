package middlewares_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	// clientIP is the IP that httptest.NewRequest sets as RemoteAddr (loopback
	// from Go 1.13+). ClientIP() will see this as a private/loopback address
	// and fall back to using it as-is.
	//
	// httptest.NewRequest sets RemoteAddr = "192.0.2.1:1234" (TEST-NET-1),
	// which is not private/loopback, so ClientIP returns it directly.
	// We capture that value and use it in mock expectations.
	const expectedIP = "192.0.2.1"

	tests := []struct {
		name           string
		remoteAddr     string // To simulate the IP address
		setupMocks     func(svc *mocks.MockRateLimiterService, ip string)
		wantStatus     int
		wantNextCall   bool
		wantRemaining  string
		wantRetryAfter string
	}{
		{
			name:       "success - request allowed with remaining quota",
			remoteAddr: "192.168.1.1:1234",
			setupMocks: func(svc *mocks.MockRateLimiterService, ip string) {
				// Assumes lib.ClientIP strips the port and returns "192.168.1.1"
				svc.EXPECT().
					Allowed(mock.Anything, ip).
					Return(5, nil).
					Once()
			},
			wantStatus:    http.StatusOK,
			wantNextCall:  true,
			wantRemaining: "5",
		},
		{
			name:       "success (fail-open) - service error allows request through",
			remoteAddr: "192.168.1.1:1234",
			setupMocks: func(svc *mocks.MockRateLimiterService, ip string) {
				svc.EXPECT().
					Allowed(mock.Anything, ip).
					Return(0, errors.New("redis connection refused")).
					Once()
			},
			// Expect 200 OK because the system gracefully fails OPEN
			wantStatus:   http.StatusOK,
			wantNextCall: true,
		},
		{
			name:       "error - malformed remote address",
			remoteAddr: "invalid-ip-format",
			setupMocks: func(svc *mocks.MockRateLimiterService, _ string) {
				// Service should never be called because IP extraction fails
			},
			wantStatus:   http.StatusBadRequest,
			wantNextCall: false,
		},
		{
			name:       "error - rate limit exceeded",
			remoteAddr: "192.168.1.1:1234",
			setupMocks: func(svc *mocks.MockRateLimiterService, ip string) {
				svc.EXPECT().
					Allowed(mock.Anything, ip).
					Return(0, nil).
					Once()
			},
			wantStatus:     http.StatusTooManyRequests,
			wantNextCall:   false,
			wantRemaining:  "0",
			wantRetryAfter: "60",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewMockRateLimiterService(t)
			tt.setupMocks(svc, strings.Split(tt.remoteAddr, ":")[0])

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

			// Assert HTTP Headers
			if tt.wantRemaining != "" {
				assert.Equal(t, tt.wantRemaining, rr.Header().Get("X-RateLimit-Remaining"))
			}
			if tt.wantRetryAfter != "" {
				assert.Equal(t, tt.wantRetryAfter, rr.Header().Get("Retry-After"))
			}
		})
	}
}
