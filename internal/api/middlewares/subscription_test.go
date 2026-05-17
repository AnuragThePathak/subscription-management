package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			// Execute Request
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

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
		})
	}
}
