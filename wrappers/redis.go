package wrappers

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

func (r *Redis) Shutdown(ctx context.Context) error {
	// Check if context is already done before we even start
    if ctx.Err() != nil {
        return ctx.Err()
    }
    
    // Since r.Client.Close() doesn't support context cancellation,
    // we need to run it in a goroutine and try to respect the context deadline
    closeChan := make(chan error, 1)
    
    go func() {
        closeChan <- r.Client.Close()
    }()
    
    // Wait for either Close() to complete or context to expire
    select {
    case err := <-closeChan:
        return err
    case <-ctx.Done():
        // Context expired, but we can't really abort Close() once started
        // Let the goroutine complete in the background
        return ctx.Err()
    }
}

func (r *Redis) Ping(ctx context.Context) error {
	if err := r.Client.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}