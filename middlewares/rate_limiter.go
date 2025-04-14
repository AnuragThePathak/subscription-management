package middlewares

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/services"
)

// RateLimiter returns a middleware that limits requests by IP address.
func RateLimiter(rateLimiterService services.RateLimiterService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the client's IP address.
			ip, err := getClientIP(r)
			if err != nil {
				slog.Error("Failed to get client IP",
					slog.String("component", "ratelimiter"),
					slog.Any("error", err),
				)
				endpoint.WriteAPIResponse(w, http.StatusInternalServerError, nil)
				return
			}

			// Check if the request is allowed.
			remaining, err := rateLimiterService.Allowed(r.Context(), ip)
			if err != nil {
				slog.Error("Rate limiter service error",
					slog.String("component", "ratelimiter"),
					slog.String("ip", ip),
					slog.Any("error", err),
				)
				endpoint.WriteAPIResponse(w, http.StatusInternalServerError, nil)
				return
			}

			// Set the rate limit headers.
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if remaining <= 0 {
				// Set retry header.
				w.Header().Set("Retry-After", "60") // Suggest retry after 60 seconds.

				slog.Info("Rate limit exceeded",
					slog.String("component", "ratelimiter"),
					slog.String("ip", ip),
					slog.Int("remaining", remaining),
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

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) (string, error) {
	// Try X-Forwarded-For header first.
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For can contain multiple IPs; use the first one (client).
		ips := strings.Split(ip, ",")
		ip = strings.TrimSpace(ips[0])

		if parsedIP := net.ParseIP(ip); parsedIP != nil {
			return ip, nil
		}
	}

	// Try X-Real-IP header.
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		if parsedIP := net.ParseIP(ip); parsedIP != nil {
			return ip, nil
		}
	}

	// Fall back to RemoteAddr.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	return ip, nil
}
