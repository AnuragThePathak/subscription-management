package services_test

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisRateLimiter_Allowed(t *testing.T) {
	// Spin up the blazing fast in-memory Redis clone
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	// Connect the real go-redis client to the miniredis server
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = rdb.Close() })

	// Setup our Service with a strict limit: 2 requests per minute
	limit := redis_rate.Limit{
		Rate:   2,
		Burst:  2,
		Period: time.Minute,
	}
	limiter := redis_rate.NewLimiter(rdb)
	svc := services.NewRateLimiterService(limiter, limit, "test_prefix")

	ctx := t.Context()
	ip := "192.168.1.100"

	// --- Hit 1: Allowed (1 token remaining) ---
	t.Run("Hit 1: Allowed with tokens remaining", func(t *testing.T) {
		isAllowed, remaining, retryAfter, err := svc.Allowed(ctx, ip)
		require.NoError(t, err)
		assert.True(t, isAllowed)
		assert.Equal(t, 1, remaining)
		assert.Equal(t, time.Duration(0), retryAfter) // No wait time needed yet
	})

	// --- Hit 2: Allowed (0 tokens remaining) ---
	// This proves our fix worked! It allows the last token.
	t.Run("Hit 2: Allowed but exhausts remaining tokens", func(t *testing.T) {
		isAllowed, remaining, retryAfter, err := svc.Allowed(ctx, ip)
		require.NoError(t, err)
		assert.True(t, isAllowed)
		assert.Equal(t, 0, remaining)
		assert.Equal(t, time.Duration(0), retryAfter)
	})

	// --- Hit 3: Blocked (Rate Limit Exceeded) ---
	t.Run("Hit 3: Blocked and requires retry", func(t *testing.T) {
		isAllowed, remaining, retryAfter, err := svc.Allowed(ctx, ip)
		require.NoError(t, err)
		assert.False(t, isAllowed)
		assert.Equal(t, 0, remaining)
		assert.Greater(t, retryAfter, time.Duration(0), "Should tell us to retry later")
	})
}

func TestRedisRateLimiter_Error_FailOpen(t *testing.T) {
	// Point to a dead port to simulate Redis crashing
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:9999",
	})

	limit := redis_rate.PerMinute(5)
	limiter := redis_rate.NewLimiter(rdb)
	svc := services.NewRateLimiterService(limiter, limit, "test_prefix")

	// Execute check
	isAllowed, remaining, retryAfter, err := svc.Allowed(t.Context(), "10.0.0.1")

	// Assert strict zero-value returns on error
	require.Error(t, err)
	assert.False(t, isAllowed)
	assert.Equal(t, 0, remaining)
	assert.Equal(t, time.Duration(0), retryAfter)
	assert.Contains(t, err.Error(), "error checking rate limit")
}
