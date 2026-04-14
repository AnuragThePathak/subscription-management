package lib

import (
	"context"
)

type contextKey string

const (
	UserIDKey         contextKey = "userID"         // Context key for authenticated user ID.
	UserEmailKey      contextKey = "userEmail"      // Context key for authenticated user email.
	SubscriptionIDKey contextKey = "subscriptionID" // Context key for subscription ID.
)

// GetUserID retrieves the authenticated user ID from the context.
func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDKey).(string)
	return id, ok
}

// GetUserEmail retrieves the authenticated user email from the context.
func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(UserEmailKey).(string)
	return email, ok
}

// GetSubscriptionID retrieves the subscription ID from the context.
func GetSubscriptionID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(SubscriptionIDKey).(string)
	return id, ok
}