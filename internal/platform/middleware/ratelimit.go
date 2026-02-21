// Package middleware provides HTTP middleware for the DeploySentry API,
// including rate limiting and CORS configuration.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig holds configuration for the rate limiting middleware.
type RateLimitConfig struct {
	// RequestsPerWindow is the maximum number of requests allowed within the window.
	RequestsPerWindow int `json:"requests_per_window"`

	// Window is the duration of the sliding window.
	Window time.Duration `json:"window"`

	// KeyPrefix is the Redis key prefix for rate limit counters.
	KeyPrefix string `json:"key_prefix"`
}

// DefaultRateLimitConfig returns a sensible default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerWindow: 100,
		Window:            1 * time.Minute,
		KeyPrefix:         "ratelimit:",
	}
}

// RateLimiter provides rate limiting using a Redis sliding window algorithm.
type RateLimiter struct {
	client *redis.Client
	config RateLimitConfig
}

// NewRateLimiter creates a new RateLimiter backed by Redis.
func NewRateLimiter(client *redis.Client, config RateLimitConfig) *RateLimiter {
	if config.KeyPrefix == "" {
		config.KeyPrefix = "ratelimit:"
	}
	return &RateLimiter{
		client: client,
		config: config,
	}
}

// Middleware returns a Gin middleware function that enforces rate limits.
// The rate limit key is derived from the client IP address by default.
// When a limit is exceeded, the middleware responds with HTTP 429.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.keyFor(c)
		allowed, remaining, err := rl.allow(c.Request.Context(), key)
		if err != nil {
			// On Redis errors, allow the request but log the issue.
			c.Next()
			return
		}

		// Set rate limit headers.
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerWindow))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Window", rl.config.Window.String())

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": rl.config.Window.Seconds(),
			})
			return
		}

		c.Next()
	}
}

// keyFor derives the rate limit key from the request context.
// It uses the API key if present, otherwise falls back to the client IP.
func (rl *RateLimiter) keyFor(c *gin.Context) string {
	// Prefer API key for rate limiting if available.
	if apiKey, exists := c.Get("api_key_id"); exists {
		return rl.config.KeyPrefix + fmt.Sprintf("key:%v", apiKey)
	}

	// Fall back to user ID if authenticated.
	if userID, exists := c.Get("user_id"); exists {
		return rl.config.KeyPrefix + fmt.Sprintf("user:%v", userID)
	}

	// Fall back to client IP.
	return rl.config.KeyPrefix + "ip:" + c.ClientIP()
}

// allow checks whether a request is allowed under the rate limit using a
// Redis sliding window log algorithm. It returns whether the request is
// allowed and how many requests remain in the window.
func (rl *RateLimiter) allow(ctx context.Context, key string) (bool, int, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-rl.config.Window)

	pipe := rl.client.Pipeline()

	// Remove expired entries outside the current window.
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count current entries in the window.
	countCmd := pipe.ZCard(ctx, key)

	// Add the current request.
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// Set expiration on the key to auto-clean.
	pipe.Expire(ctx, key, rl.config.Window+time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, rl.config.RequestsPerWindow, err
	}

	count := int(countCmd.Val())
	remaining := rl.config.RequestsPerWindow - count - 1
	if remaining < 0 {
		remaining = 0
	}

	allowed := count < rl.config.RequestsPerWindow
	return allowed, remaining, nil
}

// RateLimitByKey returns a middleware that rate limits by a custom key function.
func RateLimitByKey(rl *RateLimiter, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.config.KeyPrefix + keyFn(c)
		allowed, remaining, err := rl.allow(c.Request.Context(), key)
		if err != nil {
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerWindow))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": rl.config.Window.Seconds(),
			})
			return
		}

		c.Next()
	}
}
