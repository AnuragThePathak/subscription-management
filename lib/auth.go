package lib

import (
	"context"

	"github.com/anuragthepathak/subscription-management/apperror"
)

type contextKey string

const (
	UserIDKey    contextKey = "userID"    // Context key for authenticated user ID.
	UserEmailKey contextKey = "userEmail" // Context key for authenticated user email.
)

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