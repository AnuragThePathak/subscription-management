package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func TestOTel_Middleware(t *testing.T) {
	// Setup the In-Memory OTel Exporter (The Telemetry Trap)
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	
	// Temporarily override the global tracer provider so otelhttp picks it up
	traceProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(traceProvider) }) // Restore it after the test

	// Setup Dummy Handler
	var nextCalled bool
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Setup Chi Router with the Middleware
	r := chi.NewRouter()
	r.Use(middlewares.OTel())
	
	// We specifically test a route with a parameter to ensure Chi resolves it correctly
	routePattern := "/users/{userID}/subscriptions"
	r.Get(routePattern, nextHandler)

	// Execute Request
	// Notice we send the RAW URL with real data, NOT the pattern
	req := httptest.NewRequest(http.MethodGet, "/users/alice_123/subscriptions", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	// Assert HTTP Wiring
	require.True(t, nextCalled, "Middleware must call next handler")
	require.Equal(t, http.StatusOK, rr.Code)

	// Assert Telemetry Accuracy (The High-Cardinality Lock)
	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span to be created by the middleware")

	span := spans[0]
	
	// Proof 1: The Span Name Formatter worked
	// It must be the pattern ("/users/{userID}/subscriptions"), NOT the raw URL
	assert.Equal(t, routePattern, span.Name)

	// Proof 2: The http.route attribute was injected correctly for the metrics labeler
	var hasRouteAttr bool
	for _, attr := range span.Attributes {
		if attr.Key == semconv.HTTPRouteKey {
			hasRouteAttr = true
			assert.Equal(t, routePattern, attr.Value.AsString())
		}
	}
	assert.True(t, hasRouteAttr, "Expected http.route attribute to be explicitly set on the span")
}