package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
)

type contextKey string

const (
	UserIDKey    contextKey = "userID"    // Context key for authenticated user ID.
	UserEmailKey contextKey = "userEmail" // Context key for authenticated user email.
)

// Authentication validates JWT tokens and adds user claims to the request context.
func Authentication(jwtService services.JWTService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Authorization header required"})
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid authorization format"})
				return
			}

			tokenString := parts[1]
			claims, err := jwtService.ValidateToken(tokenString, models.AccessToken)
			if err != nil {
				slog.Warn("Invalid token", slog.String("error", err.Error()))
				endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
				return
			}

			// Add user claims to context.
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user ID from the context.
func GetUserID(ctx context.Context) (string, error) {
	id, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return "", apperror.NewUnauthorizedError("User ID not found in context")
	}
	return id, nil
}

// GetUserEmail retrieves the authenticated user email from the context.
func GetUserEmail(ctx context.Context) (string, error) {
	email, ok := ctx.Value(UserEmailKey).(string)
	if !ok {
		return "", apperror.NewUnauthorizedError("User email not found in context")
	}
	return email, nil
}

// RequireRole is a placeholder for role-based authorization.
func RequireRole(role string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Example implementation for role-based checks.
			next.ServeHTTP(w, r)
		})
	}
}
