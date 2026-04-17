package lib

import (
	"context"
)

type contextKey string

const (
	keyUserID         contextKey = "userID"         // Context key for authenticated user ID.
	keyUserEmail      contextKey = "userEmail"      // Context key for authenticated user email.
	keySubscriptionID contextKey = "subscriptionID" // Context key for subscription ID.
	keyTaskType       contextKey = "taskType"       // Context key for scheduler/worker task type.
)

// WithUserID returns a new context with the given user ID.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

// GetUserID retrieves the authenticated user ID from the context.
func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(keyUserID).(string)
	return id, ok
}

// WithUserEmail returns a new context with the given user email.
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, keyUserEmail, email)
}

// GetUserEmail retrieves the authenticated user email from the context.
func GetUserEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(keyUserEmail).(string)
	return email, ok
}

// WithSubscriptionID returns a new context with the given subscription ID.
func WithSubscriptionID(ctx context.Context, subscriptionID string) context.Context {
	return context.WithValue(ctx, keySubscriptionID, subscriptionID)
}

// GetSubscriptionID retrieves the subscription ID from the context.
func GetSubscriptionID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(keySubscriptionID).(string)
	return id, ok
}

// WithTaskType returns a new context with the given task type.
func WithTaskType(ctx context.Context, taskType string) context.Context {
	return context.WithValue(ctx, keyTaskType, taskType)
}

// GetTaskType retrieves the task type from the context.
func GetTaskType(ctx context.Context) (string, bool) {
	taskType, ok := ctx.Value(keyTaskType).(string)
	return taskType, ok
}
