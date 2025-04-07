package wrappers

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

func (r *Redis) Shutdown(ctx context.Context) error {
	if err := r.Client.Close(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) Ping(ctx context.Context) error {
	if err := r.Client.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}