package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// ---------------------------------------------------------------------------
// Authentication middleware
// ---------------------------------------------------------------------------

func TestAuthentication(t *testing.T) {
	validUserID := "user_123"
	validEmail := "alice@example.com"
	validClaims := func() *models.Claims {
		return &models.Claims{
			UserID: validUserID,
			Email:  validEmail,
		}
	}

	tests := []struct {
		name         string
		token        string
		authHeader   string
		setupMocks   func(jwtSvc *mocks.MockJWTService, token string)
		wantStatus   int
		wantNextCall bool // Do we expect the next handler in the chain to be executed?
	}{
		{
			name:  "success - valid token injects context and calls next handler",
			token: "valid.jwt.token",
			setupMocks: func(jwtSvc *mocks.MockJWTService, token string) {
				jwtSvc.EXPECT().
					ValidateToken(token, models.AccessToken).
					Return(validClaims(), nil).
					Once()
			},
			wantStatus:   http.StatusOK, // Our dummy handler returns 200
			wantNextCall: true,
		},
		{
			name:  "error - missing authorization header",
			token: "",
			setupMocks: func(jwtSvc *mocks.MockJWTService, _ string) {
				// Service should never be called
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "error - invalid format (missing Bearer)",
			authHeader: "Basic some_random_token",
			setupMocks: func(jwtSvc *mocks.MockJWTService, _ string) {
				// Service should never be called
			},
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
		{
			name:  "error - expired token",
			token: "expired.jwt.token",
			setupMocks: func(jwtSvc *mocks.MockJWTService, token string) {
				jwtSvc.EXPECT().
					ValidateToken(token, models.AccessToken).
					Return(nil, jwt.ErrTokenExpired).
					Once()
			},
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
		{
			name:  "error - invalid token",
			token: "invalid.jwt.token",
			setupMocks: func(jwtSvc *mocks.MockJWTService, token string) {
				jwtSvc.EXPECT().
					ValidateToken(token, models.AccessToken).
					Return(nil, jwt.ErrSignatureInvalid).
					Once()
			},
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtSvc := mocks.NewMockJWTService(t)
			tt.setupMocks(jwtSvc, tt.token)

			// Setup the Dummy "Next" Handler to capture context
			var nextCalled bool
			var capturedCtx context.Context
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				capturedCtx = r.Context()
				// Represents a successful downstream controller
				w.WriteHeader(http.StatusOK)
			})

			// Wrap the dummy handler with our middleware
			middleware := middlewares.Authentication(jwtSvc)
			handler := middleware(nextHandler)

			// Execute Request
			req := httptest.NewRequest(http.MethodGet, "/protected-route", nil)
			if tt.token != "" || tt.authHeader != "" {
				var authHeader string
				if tt.authHeader != "" {
					authHeader = tt.authHeader
				} else {
					authHeader = "Bearer " + tt.token
				}

				req.Header.Set("Authorization", authHeader)
			}
			rr := httptest.NewRecorder()

			// Setup the In-Memory OTel Exporter (The Telemetry Trap)
			exporter := tracetest.NewInMemoryExporter()
			tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
			tracer := tp.Tracer("test-tracer")
			// Start a span and inject it into the request context BEFORE the middleware
			ctx, span := tracer.Start(t.Context(), "test-span")
			req = req.WithContext(ctx)

			handler.ServeHTTP(rr, req)

			// End the span
			span.End()

			// Assert Wiring & Status
			require.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, tt.wantNextCall, nextCalled, "Mismatch in expected execution of next handler")

			if tt.wantNextCall {
				// Assert Context Injection (The Vault Lock for Middlewares)
				require.NotNil(t, capturedCtx, "Context should have been captured")

				extractedUserID, ok := appctx.GetUserID(capturedCtx)
				require.True(t, ok)
				assert.Equal(t, validUserID, extractedUserID)

				extractedEmail, ok := appctx.GetUserEmail(capturedCtx)
				require.True(t, ok)
				assert.Equal(t, validEmail, extractedEmail)

				// Assert the Telemetry Lock
				spans := exporter.GetSpans()
				require.Len(t, spans, 1)

				var hasEnduserAttr bool
				for _, attr := range spans[0].Attributes {
					if attr.Key == semconv.EnduserIDKey {
						hasEnduserAttr = true
						assert.Equal(t, validUserID, attr.Value.AsString())
					}
				}
				assert.True(t, hasEnduserAttr, "Expected http.route attribute to be explicitly set on the span")
			}
		})
	}
}
