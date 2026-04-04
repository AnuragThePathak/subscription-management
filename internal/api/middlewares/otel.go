package middlewares

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// OTel returns a chi middleware that instruments HTTP requests with OpenTelemetry.
// It creates a span per request, records metrics, sets status codes,
// and propagates trace context.
func OTel(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Wrap the next handler to rename the span to the actual chi route
		// after the routing has matched and the route pattern is available.
		fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			// Extract the route pattern from chi context and rename the span.
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				if pattern := routeCtx.RoutePattern(); pattern != "" {
					// Rename the span so Jaeger shows the route (e.g. "/api/v1/users/{id}")
					// instead of a generic "HTTP GET" or the base service name.
					trace.SpanFromContext(r.Context()).SetName(pattern)
				}
			}
		})

		// otelhttp.NewHandler wraps the internal handler and handles standard HTTP telemetry:
		// span creation, trace context propagation, and Prometheus metrics recording.
		return otelhttp.NewHandler(fn, serviceName)
	}
}
