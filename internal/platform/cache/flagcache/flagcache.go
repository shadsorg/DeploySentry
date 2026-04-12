package flagcache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RedisClient is the subset of cache.Redis methods that FlagCache needs.
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, keys ...string) error
}

// FlagCache implements the flags.Cache interface using Redis.
type FlagCache struct {
	redis RedisClient
}

func NewFlagCache(redis RedisClient) *FlagCache {
	return &FlagCache{redis: redis}
}

func flagKey(projectID, environmentID uuid.UUID, key string) string {
	return fmt.Sprintf("flag:%s:%s:%s", projectID, environmentID, key)
}

func flagIDKey(flagID uuid.UUID) string {
	return fmt.Sprintf("flag:id:%s", flagID)
}

func rulesKey(flagID uuid.UUID) string {
	return fmt.Sprintf("rules:%s", flagID)
}

func (c *FlagCache) GetFlag(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	data, err := c.redis.Get(ctx, flagKey(projectID, environmentID, key))
	if err != nil {
		return nil, nil // cache miss
	}
	var flag models.FeatureFlag
	if err := json.Unmarshal([]byte(data), &flag); err != nil {
		return nil, fmt.Errorf("flagcache.GetFlag unmarshal: %w", err)
	}
	return &flag, nil
}

func (c *FlagCache) SetFlag(ctx context.Context, flag *models.FeatureFlag, ttl time.Duration) error {
	data, err := json.Marshal(flag)
	if err != nil {
		return fmt.Errorf("flagcache.SetFlag marshal: %w", err)
	}
	s := string(data)
	if err := c.redis.Set(ctx, flagKey(flag.ProjectID, flag.EnvironmentID, flag.Key), s, ttl); err != nil {
		return fmt.Errorf("flagcache.SetFlag: %w", err)
	}
	meta := fmt.Sprintf("%s:%s:%s", flag.ProjectID, flag.EnvironmentID, flag.Key)
	if err := c.redis.Set(ctx, flagIDKey(flag.ID), meta, ttl); err != nil {
		return fmt.Errorf("flagcache.SetFlag id mapping: %w", err)
	}
	return nil
}

func (c *FlagCache) GetRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	data, err := c.redis.Get(ctx, rulesKey(flagID))
	if err != nil {
		return nil, nil // cache miss
	}
	var rules []*models.TargetingRule
	if err := json.Unmarshal([]byte(data), &rules); err != nil {
		return nil, fmt.Errorf("flagcache.GetRules unmarshal: %w", err)
	}
	return rules, nil
}

func (c *FlagCache) SetRules(ctx context.Context, flagID uuid.UUID, rules []*models.TargetingRule, ttl time.Duration) error {
	data, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("flagcache.SetRules marshal: %w", err)
	}
	if err := c.redis.Set(ctx, rulesKey(flagID), string(data), ttl); err != nil {
		return fmt.Errorf("flagcache.SetRules: %w", err)
	}
	return nil
}

func (c *FlagCache) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	key := fmt.Sprintf("segment:%s", id)
	data, err := c.redis.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var segment models.Segment
	if err := json.Unmarshal([]byte(data), &segment); err != nil {
		return nil, err
	}
	return &segment, nil
}

func (c *FlagCache) SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error {
	key := fmt.Sprintf("segment:%s", segment.ID)
	data, err := json.Marshal(segment)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, key, string(data), ttl)
}

func (c *FlagCache) Invalidate(ctx context.Context, flagID uuid.UUID) error {
	meta, err := c.redis.Get(ctx, flagIDKey(flagID))
	if err == nil && meta != "" {
		_ = c.redis.Delete(ctx, "flag:"+meta)
	}
	_ = c.redis.Delete(ctx, flagIDKey(flagID), rulesKey(flagID))
	return nil
}
