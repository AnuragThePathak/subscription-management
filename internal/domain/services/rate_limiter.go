package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/go-redis/redis_rate/v10"
)

// RateLimiterService defines the interface for rate limiting operations.
type RateLimiterService interface {
	// Allowed checks if the given IP has not exceeded the rate limit.
	Allowed(ctx context.Context, ip string) (bool, int, time.Duration, error)
}

type redisRateLimiter struct {
	limiter *redis_rate.Limiter
	limit   redis_rate.Limit
	prefix  string
}

// NewRateLimiterService creates a new instance of the rate limiter service.
func NewRateLimiterService(
	redisClient *redis_rate.Limiter, limit redis_rate.Limit, prefix string,
) RateLimiterService {
	slog.Info("Rate limiter service created",
		logattr.Prefix(prefix),
		logattr.Rate(limit.Rate),
		logattr.Burst(limit.Burst),
		logattr.Period(limit.Period),
	)

	return &redisRateLimiter{
		limiter: redisClient,
		limit:   limit,
		prefix:  prefix,
	}
}

// Allowed checks if the given IP has not exceeded the rate limit.
func (r *redisRateLimiter) Allowed(
	ctx context.Context,
	ip string,
) (bool, int, time.Duration, error) {
	key := fmt.Sprintf("%s:%s", r.prefix, ip)
	res, err := r.limiter.Allow(ctx, key, r.limit)
	if err != nil {
		return false, 0, 0, fmt.Errorf("error checking rate limit: %w", err)
	}

	isAllowed := res.Allowed == 1
	retryAfter := max(res.RetryAfter, 0)
	return isAllowed, res.Remaining, retryAfter, nil
}
