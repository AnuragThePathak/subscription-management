package middlewares

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// OTel returns a chi middleware that instruments HTTP requests with OpenTelemetry.
// It creates a span per request, records metrics, sets status codes,
// and propagates trace context.
func OTel() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Wrap the next handler to inject the http.route label into the
		// OTel labeler after chi has resolved the route pattern.
		fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			if pattern := resolveRoutePattern(r.Context()); pattern != "" {
				// Set http.route directly on the span so Jaeger displays
				// the correct route. The labeler only affects metrics.
				trace.SpanFromContext(r.Context()).SetAttributes(
					semconv.HTTPRoute(pattern),
				)

				if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
					labeler.Add(semconv.HTTPRoute(pattern))
				}
			}
		})

		// otelhttp.NewHandler creates the span and records HTTP metrics.
		//
		// WithSpanNameFormatter overrides the default span naming. otelhttp
		// calls the formatter twice: once at span creation (before routing)
		// and once after the handler returns (when r.Pattern is set).
		// On the second call chi's RoutePatterns are populated, so we
		// return the fully resolved route (e.g. "/api/v1/auth/login").
		return otelhttp.NewHandler(fn, "http.request",
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				if pattern := resolveRoutePattern(r.Context()); pattern != "" {
					return pattern
				}
				return operation
			}),
		)
	}
}

// resolveRoutePattern resolves the route pattern for the given context.
// It returns the route pattern if found, otherwise returns an empty string.
func resolveRoutePattern(ctx context.Context) string {
	routeCtx := chi.RouteContext(ctx)
	if routeCtx == nil {
		return ""
	}
	return routeCtx.RoutePattern()
}
