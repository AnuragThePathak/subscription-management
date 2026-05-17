package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/otelattr"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ---------------------------------------------------------------------------
// WithSubscriptionID middleware
// ---------------------------------------------------------------------------

func TestWithSubscriptionID(t *testing.T) {
	tests := []struct {
		name           string
		routePattern   string
		requestPath    string
		wantNextCalled bool
		wantSubID      string
	}{
		{
			// Normal path: URL param is present and gets injected into context.
			name:         "success - extracts ID from URL and injects into context",
			routePattern: "/{subscriptionID}",
			requestPath:  "/sub_12345abc",
			wantSubID:    "sub_12345abc",
		},
		{
			// Route does not carry the param → middleware passes through without injecting.
			name:         "success - skips injection gracefully if parameter is missing",
			routePattern: "/static-path",
			requestPath:  "/static-path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Dummy Handler to capture context
			var nextCalled bool
			var capturedCtx context.Context

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				capturedCtx = r.Context()
				w.WriteHeader(http.StatusOK)
			})

			// Setup a real Chi router to populate URL Params correctly
			r := chi.NewRouter()
			r.With(middlewares.WithSubscriptionID).
				Get(tt.routePattern, nextHandler)

			// Execute the request
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rr := httptest.NewRecorder()

			// Setup the In-Memory OTel Exporter (The Telemetry Trap)
			exporter := tracetest.NewInMemoryExporter()
			tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
			tracer := tp.Tracer("test-tracer")
			// Start a span and inject it into the request context BEFORE the middleware
			ctx, span := tracer.Start(t.Context(), "test-span")
			req = req.WithContext(ctx)

			// Run the middleware
			r.ServeHTTP(rr, req)

			// End the span
			span.End()

			// Assert Wiring
			require.True(t, nextCalled, "Middleware must always call the next handler")
			require.Equal(t, http.StatusOK, rr.Code)

			// Assert Context Injection (The Vault Lock)
			extractedID, ok := appctx.GetSubscriptionID(capturedCtx)
			if tt.wantSubID != "" {
				require.True(t, ok, "Expected subscription ID to be in context")
				assert.Equal(t, tt.wantSubID, extractedID)
			} else {
				assert.False(t, ok, "Did not expect subscription ID in context")
				assert.Empty(t, extractedID)
			}

			// Assert the Telemetry Lock
			if tt.wantSubID != "" {
				spans := exporter.GetSpans()
				require.Len(t, spans, 1)

				expectedKey := otelattr.SubscriptionID("").Key

				var found bool
				for _, attr := range spans[0].Attributes {
					if attr.Key == expectedKey {
						assert.Equal(t, tt.wantSubID, attr.Value.AsString())
						found = true
					}
				}
				assert.True(t, found, "Expected subscription.id attribute on OTel span")
			}
		})
	}
}
