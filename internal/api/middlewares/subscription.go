package middlewares

import (
	"log/slog"
	"net/http"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/traceattr"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace"
)

// WithSubscriptionID extracts the subscriptionID from the URL path and adds it to the request context.
// It also adds the subscription ID as an attribute to the current OpenTelemetry span.
func WithSubscriptionID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subscriptionID := chi.URLParam(r, "subscriptionID")
		if subscriptionID == "" {
			// This middleware should only be used on routes that have the subscriptionID parameter.
			slog.DebugContext(r.Context(),
				"WithSubscriptionID middleware skipped: {subscriptionID} missing from URL")
			next.ServeHTTP(w, r)
			return
		}

		// Inject into context
		ctx := appctx.WithSubscriptionID(r.Context(), subscriptionID)

		// Update OpenTelemetry span
		trace.SpanFromContext(ctx).SetAttributes(
			traceattr.SubscriptionID(subscriptionID),
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
