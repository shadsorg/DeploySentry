// Package cache provides a Redis client wrapper for caching and key-value storage.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/shadsorg/deploysentry/internal/platform/config"
)

// Redis wraps the go-redis client with convenience methods.
type Redis struct {
	Client *redis.Client
}

// New creates a new Redis client from the provided configuration.
// It validates the connection by pinging Redis before returning.
func New(ctx context.Context, cfg config.RedisConfig) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Verify connectivity.
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return &Redis{Client: client}, nil
}

// Close shuts down the Redis client connection.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// Health checks whether Redis is reachable. Returns an error if the
// ping fails or the context deadline is exceeded.
func (r *Redis) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return r.Client.Ping(ctx).Err()
}

// Set stores a key-value pair in Redis with an expiration duration.
// A zero expiration means the key has no expiration time.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.Client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves the value for a key from Redis.
// Returns redis.Nil error if the key does not exist.
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, key).Result()
}

// Delete removes one or more keys from Redis.
func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	return r.Client.Del(ctx, keys...).Err()
}

// Exists checks whether a key exists in Redis.
func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.Client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
