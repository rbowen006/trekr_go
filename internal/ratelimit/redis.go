// Package ratelimit provides the Redis-backed fixed-window rate limiter used in
// production for the AI endpoints (ADR-0010). It implements
// middleware.RateLimiter; tests and CI use the in-memory limiter instead, so no
// Redis is required to run the suite.
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter is a fixed-window counter backed by Redis: INCR the key, and on
// the first hit of a window set the key to expire after the window. This mirrors
// how Rails' rate_limit stores counters in its cache store.
type RedisLimiter struct {
	client *redis.Client
}

// NewRedisLimiter connects to Redis at redisURL (redis://host:port/db).
func NewRedisLimiter(redisURL string) (*RedisLimiter, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &RedisLimiter{client: redis.NewClient(opt)}, nil
}

// Close releases the underlying Redis connection.
func (l *RedisLimiter) Close() error { return l.client.Close() }

// Increment records one hit for key and returns the new count within the current
// window. On the first hit (count == 1) it sets the key to expire after window,
// so the counter resets automatically.
func (l *RedisLimiter) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := l.client.Expire(ctx, key, window).Err(); err != nil {
			return count, err
		}
	}
	return count, nil
}
