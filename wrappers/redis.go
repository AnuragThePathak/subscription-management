package wrappers

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// Redis wraps the Redis client to provide additional functionality.
type Redis struct {
	Client *redis.Client
}

// Shutdown gracefully shuts down the Redis client, respecting the provided context.
func (r *Redis) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	closeChan := make(chan error, 1)

	go func() {
		slog.Info("Closing Redis client", slog.String("component", "redis"))
		closeChan <- r.Client.Close()
	}()

	select {
	case err := <-closeChan:
		if err != nil {
			slog.Error("Failed to close Redis client", slog.String("component", "redis"), slog.Any("error", err))
		} else {
			slog.Info("Redis client closed successfully", slog.String("component", "redis"))
		}
		return err
	case <-ctx.Done():
		slog.Warn("Context expired while closing Redis client", slog.String("component", "redis"))
		return ctx.Err()
	}
}

// Ping checks the connection to the Redis server.
func (r *Redis) Ping(ctx context.Context) error {
	if err := r.Client.Ping(ctx).Err(); err != nil {
		slog.Error("Redis ping failed", slog.String("component", "redis"), slog.Any("error", err))
		return err
	}
	slog.Debug("Redis ping successful", slog.String("component", "redis"))
	return nil
}
