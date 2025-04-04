package middlewares

import (
	"context"
	"fmt"
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
	// UserIDKey is the context key for the authenticated user ID
	UserIDKey contextKey = "userID"
	// UserEmailKey is the context key for the authenticated user email
	UserEmailKey contextKey = "userEmail"
)

// Authentication returns middleware that validates JWT tokens
func Authentication(jwtService services.JWTService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Authorization header required"})
				return
			}

			// Split "Bearer token"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid authorization format"})
				return
			}

			tokenString := parts[1]
			slog.Debug(fmt.Sprintf("Token: %s", tokenString))
			claims, err := jwtService.ValidateToken(tokenString, models.AccessToken)
			if err != nil {
				endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
				return
			}

			// Add user claims to context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user ID from context
func GetUserID(ctx context.Context) (string, error) {
	id, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return "", apperror.NewUnauthorizedError("User ID not found in context")
	}
	return id, nil
}

// GetUserEmail retrieves the authenticated user email from context
func GetUserEmail(ctx context.Context) (string, error) {
	email, ok := ctx.Value(UserEmailKey).(string)
	if !ok {
		return "", apperror.NewUnauthorizedError("User email not found in context")
	}
	return email, nil
}

// RequireRole checks if a user has a specific role
// This is a placeholder for role-based authorization
// You would need to implement user roles in your User model first
func RequireRole(role string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Example implementation - modify based on your role structure
			// userID, err := GetUserID(r.Context())
			// if err != nil {
			//     endpoint.WriteAPIResponse(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			//     return
			// }

			// Here you would check the user's role from the database
			// For now, we'll just continue to the next handler
			next.ServeHTTP(w, r)
		})
	}
}