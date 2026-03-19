package middlewares

import (
	"context"
	"net/http"
	"time"
)

// Timeout returns a middleware that sets a timeout for incoming requests.
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			// Replace the request with the new context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
