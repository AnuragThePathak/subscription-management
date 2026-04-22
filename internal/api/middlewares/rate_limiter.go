package middlewares

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// failOpenLogInterval specifies the minimum time in seconds between consecutive
// error logs when the rate limiter service fails. This prevents log flooding
// while the middleware is failing open.
const failOpenLogInterval = 60

// RateLimiter returns a middleware that limits requests by IP address.
func RateLimiter(rateLimiterService services.RateLimiterService) func(http.Handler) http.Handler {
	var lastErrLog atomic.Int64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the client's IP address.
			ip, err := lib.ClientIP(r)
			if err != nil {
				slog.WarnContext(r.Context(), "Failed to get client IP",
					logattr.Error(err),
				)
				endpoint.WriteAPIResponse(w, http.StatusBadRequest,
					map[string]string{
						"error": "Malformed request environment",
					},
				)
				return
			}

			// Check if the request is allowed.
			remaining, err := rateLimiterService.Allowed(r.Context(), ip)
			if err != nil {
				span := trace.SpanFromContext(r.Context())
				span.RecordError(err)
				span.SetStatus(codes.Error, "Rate limiter service error. Failing OPEN")

				now := time.Now().Unix()
				last := lastErrLog.Load()
				if now-last > failOpenLogInterval { // Log at most once per failOpenLogInterval
					if lastErrLog.CompareAndSwap(last, now) {
						slog.ErrorContext(r.Context(), "Rate limiter service error. Failing OPEN",
							logattr.IP(ip),
							logattr.Error(err),
						)
					}
				}

				next.ServeHTTP(w, r)
				return
			}

			// Set the rate limit headers.
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if remaining <= 0 {
				// Set retry header.
				w.Header().Set("Retry-After", "60") // Suggest retry after 60 seconds.

				slog.WarnContext(r.Context(), "Rate limit exceeded",
					logattr.IP(ip),
					logattr.Remaining(remaining),
					logattr.Method(r.Method),
					logattr.Path(r.URL.Path),
				)

				endpoint.WriteAPIResponse(w, http.StatusTooManyRequests, map[string]string{
					"error": "Rate limit exceeded. Please try again later.",
				})
				return
			}

			// Call the next handler.
			next.ServeHTTP(w, r)
		})
	}
}
