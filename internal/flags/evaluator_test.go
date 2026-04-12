package flags

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Task 11: Singleflight cache stampede protection
// ---------------------------------------------------------------------------

// slowMissCache always returns a cache miss for flags (nil, nil) and never
// stores anything. It implements the full Cache interface.
type slowMissCache struct{}

func (c *slowMissCache) GetFlag(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	return nil, nil
}
func (c *slowMissCache) SetFlag(ctx context.Context, flag *models.FeatureFlag, ttl time.Duration) error {
	return nil
}
func (c *slowMissCache) GetRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	return nil, nil
}
func (c *slowMissCache) SetRules(ctx context.Context, flagID uuid.UUID, rules []*models.TargetingRule, ttl time.Duration) error {
	return nil
}
func (c *slowMissCache) Invalidate(ctx context.Context, flagID uuid.UUID) error {
	return nil
}
func (c *slowMissCache) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	return nil, nil
}
func (c *slowMissCache) SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error {
	return nil
}

// countingSlowRepo counts calls to GetFlagByKey and sleeps to allow concurrent
// goroutines to pile up before the first call returns.
type countingSlowRepo struct {
	mockFlagRepo
	callCount atomic.Int64
}

func (r *countingSlowRepo) GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	r.callCount.Add(1)
	time.Sleep(50 * time.Millisecond)
	for _, f := range r.flags {
		if f.ProjectID == projectID && f.EnvironmentID == environmentID && f.Key == key {
			return f, nil
		}
	}
	return r.mockFlagRepo.GetFlagByKey(ctx, projectID, environmentID, key)
}

// ListRules delegates to embedded mockFlagRepo (returns empty slice).
func (r *countingSlowRepo) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	return r.mockFlagRepo.ListRules(ctx, flagID)
}

func TestEvaluator_SingleflightCoalescesConcurrentDBCalls(t *testing.T) {
	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: envID,
		Key:           "sf-test-flag",
		Name:          "Singleflight Test Flag",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "false",
		Enabled:       true,
		CreatedBy:     uuid.New(),
	}

	repo := &countingSlowRepo{
		mockFlagRepo: mockFlagRepo{
			flags: map[uuid.UUID]*models.FeatureFlag{flagID: flag},
			rules: make(map[uuid.UUID][]*models.TargetingRule),
		},
	}
	cache := &slowMissCache{}
	evaluator := NewEvaluator(repo, cache)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = evaluator.Evaluate(context.Background(), projectID, envID, "sf-test-flag", models.EvaluationContext{})
		}()
	}
	wg.Wait()

	calls := repo.callCount.Load()
	assert.Equal(t, int64(1), calls, "singleflight should coalesce 10 concurrent cache misses into 1 DB call, got %d", calls)
}

// ---------------------------------------------------------------------------
// Helpers — segment-aware mock repo and cache
// ---------------------------------------------------------------------------

// segmentMockRepo extends mockFlagRepo with a controllable GetSegment response.
type segmentMockRepo struct {
	mockFlagRepo
	segments map[uuid.UUID]*models.Segment
}

func newSegmentMockRepo() *segmentMockRepo {
	return &segmentMockRepo{
		mockFlagRepo: mockFlagRepo{
			flags: make(map[uuid.UUID]*models.FeatureFlag),
			rules: make(map[uuid.UUID][]*models.TargetingRule),
		},
		segments: make(map[uuid.UUID]*models.Segment),
	}
}

func (r *segmentMockRepo) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	seg, ok := r.segments[id]
	if !ok {
		return nil, nil
	}
	return seg, nil
}

// ---------------------------------------------------------------------------
// Task 7: Segment rule evaluation
// ---------------------------------------------------------------------------

func TestEvaluateRule_Segment_ANDAllMatch(t *testing.T) {
	segID := uuid.New()
	seg := &models.Segment{
		ID:        segID,
		CombineOp: "AND",
		Conditions: []models.SegmentCondition{
			{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			{Attribute: "region", Operator: "eq", Value: "US"},
		},
	}

	repo := newSegmentMockRepo()
	repo.segments[segID] = seg

	e := NewEvaluator(repo, newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeSegment,
		SegmentID: &segID,
		Enabled:   true,
	}

	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{
			"plan":   "enterprise",
			"region": "US",
		},
	}

	match, err := e.evaluateRule(context.Background(), rule, evalCtx, "test-flag")
	require.NoError(t, err)
	assert.True(t, match, "all AND conditions match so rule should match")
}

func TestEvaluateRule_Segment_ANDOneFails(t *testing.T) {
	segID := uuid.New()
	seg := &models.Segment{
		ID:        segID,
		CombineOp: "AND",
		Conditions: []models.SegmentCondition{
			{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			{Attribute: "region", Operator: "eq", Value: "EU"},
		},
	}

	repo := newSegmentMockRepo()
	repo.segments[segID] = seg

	e := NewEvaluator(repo, newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeSegment,
		SegmentID: &segID,
		Enabled:   true,
	}

	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{
			"plan":   "enterprise",
			"region": "US", // does not match EU
		},
	}

	match, err := e.evaluateRule(context.Background(), rule, evalCtx, "test-flag")
	require.NoError(t, err)
	assert.False(t, match, "one AND condition fails so rule should not match")
}

func TestEvaluateRule_Segment_NilSegmentID(t *testing.T) {
	repo := newSegmentMockRepo()
	e := NewEvaluator(repo, newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeSegment,
		SegmentID: nil,
		Enabled:   true,
	}

	_, err := e.evaluateRule(context.Background(), rule, models.EvaluationContext{}, "test-flag")
	assert.Error(t, err, "nil segment_id should return an error")
}

// ---------------------------------------------------------------------------
// Task 8: Compound rule evaluation
// ---------------------------------------------------------------------------

func TestEvaluateRule_Compound_ANDAllMatch(t *testing.T) {
	e := NewEvaluator(newMockFlagRepo(), newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeCompound,
		CombineOp: "AND",
		Conditions: []models.CompoundCondition{
			{Attribute: "plan", Operator: "eq", Value: "pro"},
			{Attribute: "country", Operator: "eq", Value: "CA"},
		},
		Enabled: true,
	}

	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{
			"plan":    "pro",
			"country": "CA",
		},
	}

	match, err := e.evaluateRule(context.Background(), rule, evalCtx, "test-flag")
	require.NoError(t, err)
	assert.True(t, match, "all AND conditions match so compound rule should match")
}

func TestEvaluateRule_Compound_ANDOneFails(t *testing.T) {
	e := NewEvaluator(newMockFlagRepo(), newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeCompound,
		CombineOp: "AND",
		Conditions: []models.CompoundCondition{
			{Attribute: "plan", Operator: "eq", Value: "pro"},
			{Attribute: "country", Operator: "eq", Value: "UK"},
		},
		Enabled: true,
	}

	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{
			"plan":    "pro",
			"country": "CA", // does not match UK
		},
	}

	match, err := e.evaluateRule(context.Background(), rule, evalCtx, "test-flag")
	require.NoError(t, err)
	assert.False(t, match, "one AND condition fails so compound rule should not match")
}

func TestEvaluateRule_Compound_OROneMatches(t *testing.T) {
	e := NewEvaluator(newMockFlagRepo(), newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeCompound,
		CombineOp: "OR",
		Conditions: []models.CompoundCondition{
			{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			{Attribute: "plan", Operator: "eq", Value: "pro"},
		},
		Enabled: true,
	}

	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "pro"},
	}

	match, err := e.evaluateRule(context.Background(), rule, evalCtx, "test-flag")
	require.NoError(t, err)
	assert.True(t, match, "one OR condition matches so compound rule should match")
}

func TestEvaluateRule_Compound_ORNoneMatch(t *testing.T) {
	e := NewEvaluator(newMockFlagRepo(), newMockCache())

	rule := &models.TargetingRule{
		RuleType:  models.RuleTypeCompound,
		CombineOp: "OR",
		Conditions: []models.CompoundCondition{
			{Attribute: "plan", Operator: "eq", Value: "enterprise"},
			{Attribute: "plan", Operator: "eq", Value: "pro"},
		},
		Enabled: true,
	}

	evalCtx := models.EvaluationContext{
		Attributes: map[string]string{"plan": "free"},
	}

	match, err := e.evaluateRule(context.Background(), rule, evalCtx, "test-flag")
	require.NoError(t, err)
	assert.False(t, match, "no OR conditions match so compound rule should not match")
}
