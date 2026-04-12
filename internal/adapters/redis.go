package adapters

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
		slog.InfoContext(ctx, "Closing Redis client")
		closeChan <- r.Client.Close()
	}()

	select {
	case err := <-closeChan:
		if err != nil {
			slog.ErrorContext(ctx, "Failed to close Redis client", slog.Any("error", err))
		} else {
			slog.InfoContext(ctx, "Redis client closed successfully")
		}
		return err
	case <-ctx.Done():
		slog.WarnContext(ctx, "Context expired while closing Redis client")
		return ctx.Err()
	}
}

// Ping checks the connection to the Redis server.
func (r *Redis) Ping(ctx context.Context) error {
	if err := r.Client.Ping(ctx).Err(); err != nil {
		slog.ErrorContext(ctx, "Redis ping failed", slog.Any("error", err))
		return err
	}
	slog.DebugContext(ctx, "Redis ping successful")
	return nil
}
