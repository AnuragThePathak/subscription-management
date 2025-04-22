package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-redis/redis_rate/v10"
)

// RateLimiterService defines the interface for rate limiting operations.
type RateLimiterService interface {
	// Allowed checks if the given IP has not exceeded the rate limit.
	Allowed(ctx context.Context, ip string) (int, error)
}

type redisRateLimiter struct {
	limiter *redis_rate.Limiter
	limit   *redis_rate.Limit
	prefix  string
}

// NewRateLimiterService creates a new instance of the rate limiter service.
func NewRateLimiterService(redisClient *redis_rate.Limiter, limit *redis_rate.Limit, prefix string) RateLimiterService {
	return &redisRateLimiter{
		limiter: redisClient,
		limit:   limit,
		prefix:  prefix,
	}
}

// Allowed checks if the given IP has not exceeded the rate limit.
func (r *redisRateLimiter) Allowed(ctx context.Context, ip string) (int, error) {
	key := fmt.Sprintf("%s:%s", r.prefix, ip)
	res, err := r.limiter.Allow(ctx, key, *r.limit)
	if err != nil {
		slog.Error("Rate limiter error", slog.String("key", key), slog.Any("error", err))
		return 0, err
	}

	slog.Debug("Rate limiter check", slog.String("key", key), slog.Int("remaining", res.Remaining))
	return res.Remaining, nil
}
