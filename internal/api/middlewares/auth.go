package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/lib"
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
			ctx := context.WithValue(r.Context(), lib.UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, lib.UserEmailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
