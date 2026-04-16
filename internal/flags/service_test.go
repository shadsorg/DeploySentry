package flags

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Mock implementations
// ===========================================================================

// mockFlagRepo is a test double for FlagRepository.
type mockFlagRepo struct {
	flags map[uuid.UUID]*models.FeatureFlag
	rules map[uuid.UUID][]*models.TargetingRule // keyed by flag ID

	// Optional hooks to inject custom behaviour.
	createFlagFn func(ctx context.Context, flag *models.FeatureFlag) error
	getFlagFn    func(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error)
	updateFlagFn func(ctx context.Context, flag *models.FeatureFlag) error
	deleteFlagFn func(ctx context.Context, id uuid.UUID) error
	createRuleFn func(ctx context.Context, rule *models.TargetingRule) error
	updateRuleFn func(ctx context.Context, rule *models.TargetingRule) error
	deleteRuleFn func(ctx context.Context, id uuid.UUID) error
}

func newMockFlagRepo() *mockFlagRepo {
	return &mockFlagRepo{
		flags: make(map[uuid.UUID]*models.FeatureFlag),
		rules: make(map[uuid.UUID][]*models.TargetingRule),
	}
}

func (m *mockFlagRepo) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if m.createFlagFn != nil {
		return m.createFlagFn(ctx, flag)
	}
	m.flags[flag.ID] = flag
	return nil
}

func (m *mockFlagRepo) GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
	if m.getFlagFn != nil {
		return m.getFlagFn(ctx, id)
	}
	f, ok := m.flags[id]
	if !ok {
		return nil, errors.New("flag not found")
	}
	return f, nil
}

func (m *mockFlagRepo) GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	for _, f := range m.flags {
		if f.ProjectID == projectID && f.EnvironmentID != nil && *f.EnvironmentID == environmentID && f.Key == key {
			return f, nil
		}
	}
	return nil, errors.New("flag not found")
}

func (m *mockFlagRepo) ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error) {
	var result []*models.FeatureFlag
	for _, f := range m.flags {
		if f.ProjectID == projectID {
			result = append(result, f)
		}
	}
	return result, nil
}

func (m *mockFlagRepo) UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if m.updateFlagFn != nil {
		return m.updateFlagFn(ctx, flag)
	}
	m.flags[flag.ID] = flag
	return nil
}

func (m *mockFlagRepo) DeleteFlag(ctx context.Context, id uuid.UUID) error {
	if m.deleteFlagFn != nil {
		return m.deleteFlagFn(ctx, id)
	}
	delete(m.flags, id)
	return nil
}

func (m *mockFlagRepo) CreateRule(ctx context.Context, rule *models.TargetingRule) error {
	if m.createRuleFn != nil {
		return m.createRuleFn(ctx, rule)
	}
	m.rules[rule.FlagID] = append(m.rules[rule.FlagID], rule)
	return nil
}

func (m *mockFlagRepo) GetRule(ctx context.Context, id uuid.UUID) (*models.TargetingRule, error) {
	for _, ruleSlice := range m.rules {
		for _, r := range ruleSlice {
			if r.ID == id {
				return r, nil
			}
		}
	}
	return nil, errors.New("rule not found")
}

func (m *mockFlagRepo) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	return m.rules[flagID], nil
}

func (m *mockFlagRepo) UpdateRule(ctx context.Context, rule *models.TargetingRule) error {
	if m.updateRuleFn != nil {
		return m.updateRuleFn(ctx, rule)
	}
	rules := m.rules[rule.FlagID]
	for i, r := range rules {
		if r.ID == rule.ID {
			rules[i] = rule
			return nil
		}
	}
	return errors.New("rule not found")
}

func (m *mockFlagRepo) DeleteRule(ctx context.Context, id uuid.UUID) error {
	if m.deleteRuleFn != nil {
		return m.deleteRuleFn(ctx, id)
	}
	for flagID, ruleSlice := range m.rules {
		for i, r := range ruleSlice {
			if r.ID == id {
				m.rules[flagID] = append(ruleSlice[:i], ruleSlice[i+1:]...)
				return nil
			}
		}
	}
	return errors.New("rule not found")
}

func (m *mockFlagRepo) WriteEvaluationLog(ctx context.Context, logs []EvaluationLog) error {
	return nil
}

func (m *mockFlagRepo) ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error) {
	return []*models.FlagEnvironmentState{}, nil
}

func (m *mockFlagRepo) UpsertFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error {
	return nil
}

func (m *mockFlagRepo) CreateSegment(ctx context.Context, segment *models.Segment) error {
	return nil
}

func (m *mockFlagRepo) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	return nil, nil
}

func (m *mockFlagRepo) GetSegmentByKey(ctx context.Context, projectID uuid.UUID, key string) (*models.Segment, error) {
	return nil, nil
}

func (m *mockFlagRepo) ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error) {
	return nil, nil
}

func (m *mockFlagRepo) UpdateSegment(ctx context.Context, segment *models.Segment) error {
	return nil
}

func (m *mockFlagRepo) DeleteSegment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockFlagRepo) SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	return &models.RuleEnvironmentState{RuleID: ruleID, EnvironmentID: environmentID, Enabled: enabled}, nil
}

func (m *mockFlagRepo) ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
	return nil, nil
}

func (m *mockFlagRepo) ListRuleEnvironmentStatesByEnv(ctx context.Context, flagID uuid.UUID, environmentID uuid.UUID) (map[uuid.UUID]bool, error) {
	rules, _ := m.ListRules(ctx, flagID)
	result := make(map[uuid.UUID]bool)
	for _, r := range rules {
		result[r.ID] = r.Enabled
	}
	return result, nil
}

// mockCache is a test double for Cache.
type mockCache struct {
	flags map[string]*models.FeatureFlag   // key: "projectID:envID:key"
	rules map[uuid.UUID][]*models.TargetingRule
}

func newMockCache() *mockCache {
	return &mockCache{
		flags: make(map[string]*models.FeatureFlag),
		rules: make(map[uuid.UUID][]*models.TargetingRule),
	}
}

func cacheKey(projectID, environmentID uuid.UUID, key string) string {
	return projectID.String() + ":" + environmentID.String() + ":" + key
}

func (c *mockCache) GetFlag(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	f, ok := c.flags[cacheKey(projectID, environmentID, key)]
	if !ok {
		return nil, nil
	}
	return f, nil
}

func (c *mockCache) SetFlag(ctx context.Context, flag *models.FeatureFlag, ttl time.Duration) error {
	envID := uuid.Nil
	if flag.EnvironmentID != nil {
		envID = *flag.EnvironmentID
	}
	c.flags[cacheKey(flag.ProjectID, envID, flag.Key)] = flag
	return nil
}

func (c *mockCache) GetRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	r, ok := c.rules[flagID]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (c *mockCache) SetRules(ctx context.Context, flagID uuid.UUID, rules []*models.TargetingRule, ttl time.Duration) error {
	c.rules[flagID] = rules
	return nil
}

func (c *mockCache) Invalidate(ctx context.Context, flagID uuid.UUID) error {
	delete(c.rules, flagID)
	return nil
}

func (c *mockCache) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	return nil, nil
}

func (c *mockCache) SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error {
	return nil
}

// emptyCacheThatMisses always returns nil (cache miss) for everything.
type emptyCacheThatMisses struct{}

func (e *emptyCacheThatMisses) GetFlag(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	return nil, nil
}
func (e *emptyCacheThatMisses) SetFlag(ctx context.Context, flag *models.FeatureFlag, ttl time.Duration) error {
	return nil
}
func (e *emptyCacheThatMisses) GetRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	return nil, nil
}
func (e *emptyCacheThatMisses) SetRules(ctx context.Context, flagID uuid.UUID, rules []*models.TargetingRule, ttl time.Duration) error {
	return nil
}
func (e *emptyCacheThatMisses) Invalidate(ctx context.Context, flagID uuid.UUID) error {
	return nil
}
func (e *emptyCacheThatMisses) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	return nil, nil
}
func (e *emptyCacheThatMisses) SetSegment(ctx context.Context, segment *models.Segment, ttl time.Duration) error {
	return nil
}

// ===========================================================================
// Helper to create a valid FeatureFlag for testing
// ===========================================================================

func uuidPtr(u uuid.UUID) *uuid.UUID { return &u }

func validFlag() *models.FeatureFlag {
	return &models.FeatureFlag{
		ProjectID:     uuid.New(),
		EnvironmentID: uuidPtr(uuid.New()),
		Key:           "test-flag",
		Name:          "Test Flag",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "false",
		CreatedBy:     uuid.New(),
	}
}

// ===========================================================================
// FlagService tests
// ===========================================================================

// ---------------------------------------------------------------------------
// CreateFlag
// ---------------------------------------------------------------------------

func TestCreateFlag_AssignsID(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	flag.ID = uuid.Nil

	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, flag.ID, "service should assign an ID")
	assert.False(t, flag.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, flag.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestCreateFlag_PreservesExistingID(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	existingID := uuid.New()
	flag := validFlag()
	flag.ID = existingID

	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)
	assert.Equal(t, existingID, flag.ID, "service should preserve existing ID")
}

func TestCreateFlag_ValidationError(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := &models.FeatureFlag{
		// Missing required fields: ProjectID, EnvironmentID, Key, Name
		FlagType: models.FlagTypeBoolean,
	}

	err := svc.CreateFlag(context.Background(), flag)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestCreateFlag_RepoError(t *testing.T) {
	repo := newMockFlagRepo()
	repo.createFlagFn = func(ctx context.Context, flag *models.FeatureFlag) error {
		return errors.New("database unavailable")
	}
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	err := svc.CreateFlag(context.Background(), flag)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating flag")
}

// ---------------------------------------------------------------------------
// GetFlag
// ---------------------------------------------------------------------------

func TestGetFlag_Exists(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	retrieved, err := svc.GetFlag(context.Background(), flag.ID)
	require.NoError(t, err)
	assert.Equal(t, flag.ID, retrieved.ID)
	assert.Equal(t, flag.Key, retrieved.Key)
}

func TestService_GetFlag_NotFound(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	_, err := svc.GetFlag(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting flag")
}

// ---------------------------------------------------------------------------
// ListFlags
// ---------------------------------------------------------------------------

func TestListFlags_DefaultLimit(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	projectID := uuid.New()
	for i := 0; i < 3; i++ {
		f := validFlag()
		f.ProjectID = projectID
		f.Key = "flag-" + uuid.New().String()[:8]
		err := svc.CreateFlag(context.Background(), f)
		require.NoError(t, err)
	}

	// Limit=0 should default to 20 (service normalizes it).
	flags, err := svc.ListFlags(context.Background(), projectID, ListOptions{Limit: 0})
	require.NoError(t, err)
	assert.Len(t, flags, 3)
}

func TestListFlags_CappedLimit(t *testing.T) {
	repo := newMockFlagRepo()
	// Track the opts actually passed to the repo.
	var capturedOpts ListOptions
	origListFlags := repo.ListFlags
	_ = origListFlags
	repo2 := &capturingMockFlagRepo{mockFlagRepo: repo, capturedOpts: &capturedOpts}

	svc := NewFlagService(repo2, newMockCache(), nil)

	_, err := svc.ListFlags(context.Background(), uuid.New(), ListOptions{Limit: 500})
	require.NoError(t, err)
	assert.Equal(t, 100, capturedOpts.Limit, "limit > 100 should be capped to 100")
}

func TestListFlags_ZeroLimitDefaultsTo20(t *testing.T) {
	repo := newMockFlagRepo()
	var capturedOpts ListOptions
	repo2 := &capturingMockFlagRepo{mockFlagRepo: repo, capturedOpts: &capturedOpts}

	svc := NewFlagService(repo2, newMockCache(), nil)

	_, err := svc.ListFlags(context.Background(), uuid.New(), ListOptions{Limit: 0})
	require.NoError(t, err)
	assert.Equal(t, 20, capturedOpts.Limit, "limit 0 should default to 20")
}

// capturingMockFlagRepo wraps mockFlagRepo and captures ListFlags opts.
type capturingMockFlagRepo struct {
	*mockFlagRepo
	capturedOpts *ListOptions
}

func (c *capturingMockFlagRepo) ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error) {
	*c.capturedOpts = opts
	return c.mockFlagRepo.ListFlags(ctx, projectID, opts)
}

// ---------------------------------------------------------------------------
// UpdateFlag
// ---------------------------------------------------------------------------

func TestUpdateFlag_Success(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	flag.Name = "Updated Name"
	err = svc.UpdateFlag(context.Background(), flag)
	require.NoError(t, err)

	retrieved, err := svc.GetFlag(context.Background(), flag.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
}

func TestUpdateFlag_ValidationError(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	flag.Key = "" // Make it invalid.
	err = svc.UpdateFlag(context.Background(), flag)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

// ---------------------------------------------------------------------------
// ArchiveFlag
// ---------------------------------------------------------------------------

func TestArchiveFlag_Success(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	flag.Enabled = true
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	err = svc.ArchiveFlag(context.Background(), flag.ID)
	require.NoError(t, err)

	retrieved, err := svc.GetFlag(context.Background(), flag.ID)
	require.NoError(t, err)
	assert.True(t, retrieved.Archived, "flag should be archived")
	assert.False(t, retrieved.Enabled, "archived flag should be disabled")
}

func TestArchiveFlag_GetError(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	err := svc.ArchiveFlag(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting flag for archive")
}

// ---------------------------------------------------------------------------
// ToggleFlag
// ---------------------------------------------------------------------------

func TestToggleFlag_On(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	flag.Enabled = false
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	err = svc.ToggleFlag(context.Background(), flag.ID, true)
	require.NoError(t, err)

	retrieved, err := svc.GetFlag(context.Background(), flag.ID)
	require.NoError(t, err)
	assert.True(t, retrieved.Enabled)
}

func TestToggleFlag_Off(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	flag.Enabled = true
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	err = svc.ToggleFlag(context.Background(), flag.ID, false)
	require.NoError(t, err)

	retrieved, err := svc.GetFlag(context.Background(), flag.ID)
	require.NoError(t, err)
	assert.False(t, retrieved.Enabled)
}

func TestToggleFlag_CannotToggleArchived(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flag := validFlag()
	err := svc.CreateFlag(context.Background(), flag)
	require.NoError(t, err)

	err = svc.ArchiveFlag(context.Background(), flag.ID)
	require.NoError(t, err)

	err = svc.ToggleFlag(context.Background(), flag.ID, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot toggle an archived flag")
}

// ---------------------------------------------------------------------------
// AddRule
// ---------------------------------------------------------------------------

func TestAddRule_AssignsID(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	pct := 50
	rule := &models.TargetingRule{
		FlagID:     uuid.New(),
		RuleType:   models.RuleTypePercentage,
		Percentage: &pct,
		Enabled:    true,
	}

	err := svc.AddRule(context.Background(), rule)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rule.ID)
	assert.False(t, rule.CreatedAt.IsZero())
	assert.False(t, rule.UpdatedAt.IsZero())
}

func TestAddRule_ValidationError(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	rule := &models.TargetingRule{
		// Missing FlagID.
		RuleType: models.RuleTypePercentage,
	}

	err := svc.AddRule(context.Background(), rule)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

// ---------------------------------------------------------------------------
// UpdateRule
// ---------------------------------------------------------------------------

func TestUpdateRule_Success(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flagID := uuid.New()
	pct := 50
	rule := &models.TargetingRule{
		FlagID:     flagID,
		RuleType:   models.RuleTypePercentage,
		Percentage: &pct,
		Enabled:    true,
	}
	err := svc.AddRule(context.Background(), rule)
	require.NoError(t, err)

	newPct := 75
	rule.Percentage = &newPct
	err = svc.UpdateRule(context.Background(), rule)
	require.NoError(t, err)
}

func TestUpdateRule_ValidationError(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	rule := &models.TargetingRule{
		ID:       uuid.New(),
		FlagID:   uuid.Nil, // Invalid: missing flag_id.
		RuleType: models.RuleTypePercentage,
	}

	err := svc.UpdateRule(context.Background(), rule)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

// ---------------------------------------------------------------------------
// DeleteRule
// ---------------------------------------------------------------------------

func TestDeleteRule_Success(t *testing.T) {
	repo := newMockFlagRepo()
	svc := NewFlagService(repo, newMockCache(), nil)

	flagID := uuid.New()
	pct := 50
	rule := &models.TargetingRule{
		FlagID:     flagID,
		RuleType:   models.RuleTypePercentage,
		Percentage: &pct,
		Enabled:    true,
	}
	err := svc.AddRule(context.Background(), rule)
	require.NoError(t, err)

	err = svc.DeleteRule(context.Background(), rule.ID)
	require.NoError(t, err)
}

func TestDeleteRule_ErrorPropagation(t *testing.T) {
	repo := newMockFlagRepo()
	repo.deleteRuleFn = func(ctx context.Context, id uuid.UUID) error {
		return errors.New("database connection lost")
	}
	svc := NewFlagService(repo, newMockCache(), nil)

	err := svc.DeleteRule(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting rule")
}

// ===========================================================================
// Evaluator tests
// ===========================================================================

// ---------------------------------------------------------------------------
// Flag not in cache, falls back to repo
// ---------------------------------------------------------------------------

func TestEvaluator_CacheMiss_FallsBackToRepo(t *testing.T) {
	repo := newMockFlagRepo()
	cache := &emptyCacheThatMisses{} // Always misses.

	projectID := uuid.New()
	envID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            uuid.New(),
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "my-flag",
		Name:          "My Flag",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "off",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "my-flag", models.EvaluationContext{})
	require.NoError(t, err)
	assert.Equal(t, "my-flag", result.FlagKey)
	assert.Equal(t, "off", result.Value)
	assert.Equal(t, "default", result.Reason)
}

// ---------------------------------------------------------------------------
// Flag disabled returns default value
// ---------------------------------------------------------------------------

func TestEvaluator_FlagDisabled_ReturnsDefault(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            uuid.New(),
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "disabled-flag",
		Name:          "Disabled",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "fallback",
		Enabled:       false,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "disabled-flag", models.EvaluationContext{UserID: "u1"})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "fallback", result.Value)
	assert.Equal(t, "flag_disabled", result.Reason)
}

// ---------------------------------------------------------------------------
// Flag archived returns default value
// ---------------------------------------------------------------------------

func TestEvaluator_FlagArchived_ReturnsDefault(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            uuid.New(),
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "archived-flag",
		Name:          "Archived",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "archived-default",
		Enabled:       true, // Even if enabled, archived takes precedence.
		Archived:      true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "archived-flag", models.EvaluationContext{UserID: "u1"})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "archived-default", result.Value)
	assert.Equal(t, "flag_disabled", result.Reason)
}

// ---------------------------------------------------------------------------
// Rules match in priority order
// ---------------------------------------------------------------------------

func TestEvaluator_RulesMatchInPriorityOrder(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "priority-flag",
		Name:          "Priority",
		FlagType:      models.FlagTypeString,
		DefaultValue:  "default",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	// Both rules target "alice" but with different priorities and values.
	// The evaluator iterates rules in the order returned by ListRules,
	// which should be by priority. We set up the slice to be in priority order.
	repo.rules[flagID] = []*models.TargetingRule{
		{
			ID:           uuid.New(),
			FlagID:       flagID,
			RuleType:     models.RuleTypeUserTarget,
			Priority:     1,
			Value:        "first-match",
			TargetValues: []string{"alice"},
			Enabled:      true,
		},
		{
			ID:           uuid.New(),
			FlagID:       flagID,
			RuleType:     models.RuleTypeUserTarget,
			Priority:     2,
			Value:        "second-match",
			TargetValues: []string{"alice"},
			Enabled:      true,
		},
	}

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "priority-flag", models.EvaluationContext{UserID: "alice"})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "first-match", result.Value, "first rule by priority should win")
	assert.Equal(t, "rule_match", result.Reason)
}

// ---------------------------------------------------------------------------
// Disabled rules are skipped
// ---------------------------------------------------------------------------

func TestEvaluator_DisabledRulesSkipped(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "skip-flag",
		Name:          "Skip",
		FlagType:      models.FlagTypeString,
		DefaultValue:  "default",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	repo.rules[flagID] = []*models.TargetingRule{
		{
			ID:           uuid.New(),
			FlagID:       flagID,
			RuleType:     models.RuleTypeUserTarget,
			Priority:     1,
			Value:        "disabled-value",
			TargetValues: []string{"alice"},
			Enabled:      false, // This rule is disabled.
		},
		{
			ID:           uuid.New(),
			FlagID:       flagID,
			RuleType:     models.RuleTypeUserTarget,
			Priority:     2,
			Value:        "enabled-value",
			TargetValues: []string{"alice"},
			Enabled:      true,
		},
	}

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "skip-flag", models.EvaluationContext{UserID: "alice"})
	require.NoError(t, err)
	assert.Equal(t, "enabled-value", result.Value, "should skip disabled rule and match enabled one")
}

// ---------------------------------------------------------------------------
// No rules match returns default
// ---------------------------------------------------------------------------

func TestEvaluator_NoRulesMatch_ReturnsDefault(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "no-match-flag",
		Name:          "No Match",
		FlagType:      models.FlagTypeString,
		DefaultValue:  "default-val",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	repo.rules[flagID] = []*models.TargetingRule{
		{
			ID:           uuid.New(),
			FlagID:       flagID,
			RuleType:     models.RuleTypeUserTarget,
			Priority:     1,
			Value:        "targeted-value",
			TargetValues: []string{"bob"},
			Enabled:      true,
		},
	}

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "no-match-flag", models.EvaluationContext{UserID: "charlie"})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "default-val", result.Value)
	assert.Equal(t, "default", result.Reason)
}

// ---------------------------------------------------------------------------
// Evaluate through FlagService (delegates to evaluator)
// ---------------------------------------------------------------------------

func TestFlagService_Evaluate_DelegatesToEvaluator(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()
	svc := NewFlagService(repo, cache, nil)

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "svc-eval-flag",
		Name:          "SvcEval",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "false",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	pct := 100
	repo.rules[flagID] = []*models.TargetingRule{
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

	result, err := svc.Evaluate(context.Background(), projectID, envID, "svc-eval-flag", models.EvaluationContext{UserID: "user-1"})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "true", result.Value)
	assert.Equal(t, "rule_match", result.Reason)
}

// ---------------------------------------------------------------------------
// Evaluator: flag from cache (cache hit path)
// ---------------------------------------------------------------------------

func TestEvaluator_CacheHit(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "cached-flag",
		Name:          "Cached",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "cached-default",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	// Put flag in cache only (not in repo). If repo is hit, GetFlagByKey
	// will return "not found", proving the cache was used.
	cache.flags[cacheKey(projectID, envID, "cached-flag")] = flag
	cache.rules[flagID] = []*models.TargetingRule{} // No rules in cache.

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "cached-flag", models.EvaluationContext{})
	require.NoError(t, err)
	assert.Equal(t, "cached-default", result.Value)
	assert.Equal(t, "default", result.Reason)
}

// ---------------------------------------------------------------------------
// Evaluator: flag not found anywhere returns error
// ---------------------------------------------------------------------------

func TestEvaluator_FlagNotFound(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	evaluator := NewEvaluator(repo, cache)
	_, err := evaluator.Evaluate(context.Background(), uuid.New(), uuid.New(), "nonexistent", models.EvaluationContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// Evaluator: attribute rule integration
// ---------------------------------------------------------------------------

func TestEvaluator_AttributeRuleMatches(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "attr-flag",
		Name:          "Attr",
		FlagType:      models.FlagTypeString,
		DefaultValue:  "default",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	repo.rules[flagID] = []*models.TargetingRule{
		{
			ID:        uuid.New(),
			FlagID:    flagID,
			RuleType:  models.RuleTypeAttribute,
			Priority:  1,
			Value:     "premium-value",
			Attribute: "plan",
			Operator:  "eq",
			Enabled:   true,
		},
	}
	// Set the rule's Value field (the attribute value to match against).
	repo.rules[flagID][0].Value = "premium-value"

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "attr-flag", models.EvaluationContext{
		Attributes: map[string]string{"plan": "premium-value"},
	})
	require.NoError(t, err)
	// The rule matches because attrValue("premium-value") == ruleValue("premium-value")
	// when using "eq" operator on the "plan" attribute.
	assert.Equal(t, "premium-value", result.Value)
	assert.Equal(t, "rule_match", result.Reason)
}

// ---------------------------------------------------------------------------
// Evaluator: schedule rule integration
// ---------------------------------------------------------------------------

func TestEvaluator_ScheduleRuleWithinWindow(t *testing.T) {
	repo := newMockFlagRepo()
	cache := newMockCache()

	projectID := uuid.New()
	envID := uuid.New()
	flagID := uuid.New()

	flag := &models.FeatureFlag{
		ID:            flagID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "schedule-flag",
		Name:          "Schedule",
		FlagType:      models.FlagTypeString,
		DefaultValue:  "default",
		Enabled:       true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	repo.flags[flag.ID] = flag

	start := time.Now().UTC().Add(-1 * time.Hour)
	end := time.Now().UTC().Add(1 * time.Hour)

	repo.rules[flagID] = []*models.TargetingRule{
		{
			ID:        uuid.New(),
			FlagID:    flagID,
			RuleType:  models.RuleTypeSchedule,
			Priority:  1,
			Value:     "scheduled-value",
			StartTime: &start,
			EndTime:   &end,
			Enabled:   true,
		},
	}

	evaluator := NewEvaluator(repo, cache)
	result, err := evaluator.Evaluate(context.Background(), projectID, envID, "schedule-flag", models.EvaluationContext{})
	require.NoError(t, err)
	assert.Equal(t, "scheduled-value", result.Value)
	assert.Equal(t, "rule_match", result.Reason)
}

// ---------------------------------------------------------------------------
// Task 12: Concurrent batch evaluation — error field
// ---------------------------------------------------------------------------

// errorOnKeyRepo returns an error for a specific flag key and otherwise
// delegates to the embedded mockFlagRepo.
type errorOnKeyRepo struct {
	mockFlagRepo
	badKey string
}

func (r *errorOnKeyRepo) GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error) {
	if key == r.badKey {
		return nil, errors.New("flag not found: " + key)
	}
	return r.mockFlagRepo.GetFlagByKey(ctx, projectID, environmentID, key)
}

func TestBatchEvaluate_ErrorField(t *testing.T) {
	projectID := uuid.New()
	envID := uuid.New()

	goodID := uuid.New()
	goodFlag := &models.FeatureFlag{
		ID:            goodID,
		ProjectID:     projectID,
		EnvironmentID: &envID,
		Key:           "good-flag",
		Name:          "Good Flag",
		FlagType:      models.FlagTypeBoolean,
		DefaultValue:  "false",
		Enabled:       true,
		CreatedBy:     uuid.New(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	repo := &errorOnKeyRepo{
		mockFlagRepo: mockFlagRepo{
			flags: map[uuid.UUID]*models.FeatureFlag{goodID: goodFlag},
			rules: make(map[uuid.UUID][]*models.TargetingRule),
		},
		badKey: "bad-flag",
	}

	svc := NewFlagService(repo, newMockCache(), nil)

	results, err := svc.BatchEvaluate(context.Background(), projectID, envID,
		[]string{"good-flag", "bad-flag"},
		models.EvaluationContext{},
	)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// good-flag should succeed with no error
	assert.Equal(t, "good-flag", results[0].FlagKey)
	assert.Empty(t, results[0].Error, "good flag should have no error")

	// bad-flag should have an Error field populated and be disabled
	assert.Equal(t, "bad-flag", results[1].FlagKey)
	assert.NotEmpty(t, results[1].Error, "bad flag should have error populated")
	assert.False(t, results[1].Enabled, "failed flag should be disabled")
	assert.Equal(t, "error", results[1].Reason)
}
