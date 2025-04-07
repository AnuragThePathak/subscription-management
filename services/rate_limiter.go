package services

import (
	"context"
	"fmt"

	"github.com/go-redis/redis_rate/v10"
)

type RateLimiterService interface {
	// IsAllowed checks if the given IP has not exceeded the rate limit
	Allowed(ctx context.Context, ip string) (int, error)
}

type redisRateLimiter struct {
	limiter *redis_rate.Limiter
	limit   *redis_rate.Limit
	prefix  string
}

// NewRateLimiterService creates a new instance of the rate limiter service
func NewRateLimiterService(redisClient *redis_rate.Limiter, limit *redis_rate.Limit, prefix string) RateLimiterService {
	return &redisRateLimiter{
		limiter: redisClient,
		limit:   limit,
		prefix:  prefix,
	}
}

func (r *redisRateLimiter) Allowed(ctx context.Context, ip string) (int, error) {
	res, err := r.limiter.Allow(ctx, fmt.Sprint(r.prefix, ":", ip), *r.limit)
	if err != nil {
		return 0, err
	}

	return res.Remaining, nil
}
