package flagcache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// mockRedis is an in-memory implementation of RedisClient for testing.
type mockRedis struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMockRedis() *mockRedis {
	return &mockRedis{data: make(map[string]string)}
}

func (m *mockRedis) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value.(string)
	return nil
}

func (m *mockRedis) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return "", errors.New("redis: nil")
	}
	return v, nil
}

func (m *mockRedis) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func newTestFlag() *models.FeatureFlag {
	return &models.FeatureFlag{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		EnvironmentID: uuid.New(),
		Key:           "test-flag",
		Name:          "Test Flag",
		FlagType:      models.FlagTypeBoolean,
		Category:      models.FlagCategoryFeature,
		Enabled:       true,
		DefaultValue:  "false",
	}
}

func TestSetFlagThenGetFlag(t *testing.T) {
	c := NewFlagCache(newMockRedis())
	ctx := context.Background()
	flag := newTestFlag()

	if err := c.SetFlag(ctx, flag, time.Minute); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}

	got, err := c.GetFlag(ctx, flag.ProjectID, flag.EnvironmentID, flag.Key)
	if err != nil {
		t.Fatalf("GetFlag: %v", err)
	}
	if got == nil {
		t.Fatal("GetFlag returned nil, expected flag")
	}
	if got.ID != flag.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, flag.ID)
	}
	if got.Key != flag.Key {
		t.Errorf("Key mismatch: got %s, want %s", got.Key, flag.Key)
	}
}

func TestGetFlagMissingKeyReturnsNilNil(t *testing.T) {
	c := NewFlagCache(newMockRedis())
	ctx := context.Background()

	got, err := c.GetFlag(ctx, uuid.New(), uuid.New(), "nonexistent")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil flag, got: %+v", got)
	}
}

func TestSetRulesThenGetRules(t *testing.T) {
	c := NewFlagCache(newMockRedis())
	ctx := context.Background()
	flagID := uuid.New()
	pct := 50
	rules := []*models.TargetingRule{
		{
			ID:         uuid.New(),
			FlagID:     flagID,
			RuleType:   models.RuleTypePercentage,
			Priority:   1,
			Value:      "true",
			Percentage: &pct,
			Enabled:    true,
		},
	}

	if err := c.SetRules(ctx, flagID, rules, time.Minute); err != nil {
		t.Fatalf("SetRules: %v", err)
	}

	got, err := c.GetRules(ctx, flagID)
	if err != nil {
		t.Fatalf("GetRules: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(got))
	}
	if got[0].ID != rules[0].ID {
		t.Errorf("Rule ID mismatch: got %s, want %s", got[0].ID, rules[0].ID)
	}
	if got[0].RuleType != models.RuleTypePercentage {
		t.Errorf("RuleType mismatch: got %s, want %s", got[0].RuleType, models.RuleTypePercentage)
	}
}

func TestGetRulesMissingKeyReturnsNilNil(t *testing.T) {
	c := NewFlagCache(newMockRedis())
	ctx := context.Background()

	got, err := c.GetRules(ctx, uuid.New())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil rules, got: %+v", got)
	}
}

func TestInvalidateRemovesFlagAndRules(t *testing.T) {
	c := NewFlagCache(newMockRedis())
	ctx := context.Background()
	flag := newTestFlag()
	pct := 25
	rules := []*models.TargetingRule{
		{
			ID:         uuid.New(),
			FlagID:     flag.ID,
			RuleType:   models.RuleTypePercentage,
			Priority:   1,
			Value:      "true",
			Percentage: &pct,
			Enabled:    true,
		},
	}

	if err := c.SetFlag(ctx, flag, time.Minute); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	if err := c.SetRules(ctx, flag.ID, rules, time.Minute); err != nil {
		t.Fatalf("SetRules: %v", err)
	}

	// Confirm they exist before invalidation.
	gotFlag, _ := c.GetFlag(ctx, flag.ProjectID, flag.EnvironmentID, flag.Key)
	if gotFlag == nil {
		t.Fatal("expected flag in cache before invalidation")
	}
	gotRules, _ := c.GetRules(ctx, flag.ID)
	if len(gotRules) == 0 {
		t.Fatal("expected rules in cache before invalidation")
	}

	if err := c.Invalidate(ctx, flag.ID); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	// Flag should be gone.
	gotFlag, err := c.GetFlag(ctx, flag.ProjectID, flag.EnvironmentID, flag.Key)
	if err != nil {
		t.Fatalf("GetFlag after invalidate: %v", err)
	}
	if gotFlag != nil {
		t.Error("expected nil flag after invalidation, got one")
	}

	// Rules should be gone.
	gotRules, err = c.GetRules(ctx, flag.ID)
	if err != nil {
		t.Fatalf("GetRules after invalidate: %v", err)
	}
	if gotRules != nil {
		t.Error("expected nil rules after invalidation, got some")
	}
}
