package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// EventPublisher defines the interface for publishing flag change events.
// Implementations may use NATS JetStream, Kafka, or any message broker.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// FlagChangeEvent represents a flag change event payload published to the
// message broker after create, update, toggle, or archive operations.
type FlagChangeEvent struct {
	EventType string    `json:"event_type"`
	FlagID    uuid.UUID `json:"flag_id"`
	FlagKey   string    `json:"flag_key"`
	ProjectID uuid.UUID `json:"project_id"`
	Timestamp time.Time `json:"timestamp"`
}

// FlagService defines the interface for managing feature flags.
type FlagService interface {
	// CreateFlag creates a new feature flag.
	CreateFlag(ctx context.Context, flag *models.FeatureFlag) error

	// GetFlag retrieves a feature flag by ID.
	GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error)

	// ListFlags returns feature flags for a project.
	ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error)

	// UpdateFlag updates an existing feature flag.
	UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error

	// ArchiveFlag marks a flag as archived, disabling it.
	ArchiveFlag(ctx context.Context, id uuid.UUID) error

	// UnarchiveFlag restores an archived flag to active status.
	UnarchiveFlag(ctx context.Context, id uuid.UUID) error

	// ToggleFlag toggles the enabled state of a flag.
	ToggleFlag(ctx context.Context, id uuid.UUID, enabled bool) error

	// Evaluate evaluates a feature flag for the given context.
	Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error)

	// BatchEvaluate evaluates multiple feature flags for the given context in a
	// single call. It returns results for each requested flag key.
	BatchEvaluate(ctx context.Context, projectID, environmentID uuid.UUID, keys []string, evalCtx models.EvaluationContext) ([]*models.FlagEvaluationResult, error)

	// BulkToggle toggles multiple flags at once.
	BulkToggle(ctx context.Context, flagIDs []uuid.UUID, enabled bool) error

	// AddRule adds a targeting rule to a flag.
	AddRule(ctx context.Context, rule *models.TargetingRule) error

	// UpdateRule updates an existing targeting rule.
	UpdateRule(ctx context.Context, rule *models.TargetingRule) error

	// GetRule retrieves a targeting rule by ID.
	GetRule(ctx context.Context, ruleID uuid.UUID) (*models.TargetingRule, error)

	// DeleteRule removes a targeting rule.
	DeleteRule(ctx context.Context, ruleID uuid.UUID) error

	// ListRules returns all targeting rules for a flag.
	ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)

	// ListFlagEnvStates returns all per-environment states for a given flag.
	ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error)

	// SetFlagEnvState creates or updates a per-environment flag state.
	SetFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error

	// SetRuleEnvironmentState creates or updates a per-environment rule state.
	SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error)

	// ListRuleEnvironmentStates returns all per-environment states for a flag's rules.
	ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error)

	// DetectStaleFlags returns flags that have not been evaluated within the
	// given threshold duration for the specified project.
	DetectStaleFlags(ctx context.Context, projectID uuid.UUID, threshold time.Duration) ([]*models.FeatureFlag, error)

	// WarmCache pre-loads active flags into the evaluation cache.
	WarmCache(ctx context.Context, projectID uuid.UUID) error

	// Segment operations

	// CreateSegment creates a new reusable segment.
	CreateSegment(ctx context.Context, segment *models.Segment) error

	// GetSegment retrieves a segment by ID.
	GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error)

	// ListSegments returns all segments for a project.
	ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error)

	// UpdateSegment persists changes to an existing segment.
	UpdateSegment(ctx context.Context, segment *models.Segment) error

	// DeleteSegment removes a segment by ID.
	DeleteSegment(ctx context.Context, id uuid.UUID) error

	// ExportFlags builds a complete snapshot of all flags, their per-environment
	// states, targeting rules, and rule-environment states for the given project.
	ExportFlags(ctx context.Context, projectID uuid.UUID, envs []YAMLEnvironment) (*YAMLExport, error)

	// ---- Feature lifecycle layer ----

	// GetFlagByProjectKey resolves a flag key against a project scope (used by
	// the lifecycle endpoints where the API key defines the project).
	GetFlagByProjectKey(ctx context.Context, projectID uuid.UUID, key string) (*models.FeatureFlag, error)

	// RecordSmokeTestResult persists a smoke-test outcome reported by the
	// feature-agent. On fail: increments iteration_count and disables the flag
	// across all environments. Returns the updated flag.
	RecordSmokeTestResult(ctx context.Context, flagID uuid.UUID, status models.LifecycleTestStatus, notes, testRunURL string) (*models.FeatureFlag, error)

	// RecordUserTestResult persists a user-test outcome. On fail: increments
	// iteration_count and disables the flag across all environments.
	RecordUserTestResult(ctx context.Context, flagID uuid.UUID, status models.LifecycleTestStatus, notes string) (*models.FeatureFlag, error)

	// ScheduleRemoval sets scheduled_removal_at to now + days.
	ScheduleRemoval(ctx context.Context, flagID uuid.UUID, days int) (*models.FeatureFlag, error)

	// CancelScheduledRemoval clears scheduled_removal_at.
	CancelScheduledRemoval(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error)

	// MarkIterationExhausted sets iteration_exhausted = true.
	MarkIterationExhausted(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error)

	// ListFlagsDueForRemoval / MarkFlagRemovalFired are used by the scheduler.
	ListFlagsDueForRemoval(ctx context.Context, now time.Time) ([]*models.FeatureFlag, error)
	MarkFlagRemovalFired(ctx context.Context, flagID uuid.UUID, firedAt time.Time) error

	// QueueDeletion marks an archived flag for permanent removal at
	// archived_at + retention. The retention sweep job tombstones the flag
	// when delete_after elapses.
	QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error

	// HardDeleteFlag tombstones the flag (sets deleted_at = now()) provided
	// retention has elapsed. Returns an error if the flag is missing,
	// not archived, or retention has not yet elapsed.
	HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error

	// RestoreFlag clears archived_at, delete_after, and deleted_at on a
	// flag, returning it to active state. Idempotent on already-active flags.
	RestoreFlag(ctx context.Context, id uuid.UUID) error
}

// flagService is the concrete implementation of FlagService.
type flagService struct {
	repo      FlagRepository
	evaluator *Evaluator
	publisher EventPublisher
	cache     Cache
}

// NewFlagService creates a new FlagService backed by the given repository and cache.
// The publisher parameter is optional; pass nil to disable event emission.
func NewFlagService(repo FlagRepository, cache Cache, publisher EventPublisher) FlagService {
	return &flagService{
		repo:      repo,
		evaluator: NewEvaluator(repo, cache),
		publisher: publisher,
		cache:     cache,
	}
}

// publishEvent marshals and publishes a flag change event. If no publisher is
// configured, the call is a no-op. Publish errors are non-fatal and logged
// silently to avoid disrupting the primary operation.
func (s *flagService) publishEvent(ctx context.Context, eventType string, flag *models.FeatureFlag) {
	if s.publisher == nil {
		return
	}
	event := FlagChangeEvent{
		EventType: eventType,
		FlagID:    flag.ID,
		FlagKey:   flag.Key,
		ProjectID: flag.ProjectID,
		Timestamp: time.Now().UTC(),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	// Best-effort publish; do not block the caller on failures.
	_ = s.publisher.Publish(ctx, "flags."+eventType, data)
}

// CreateFlag validates and persists a new feature flag.
func (s *flagService) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	now := time.Now().UTC()
	flag.CreatedAt = now
	flag.UpdatedAt = now

	if err := flag.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.CreateFlag(ctx, flag); err != nil {
		return fmt.Errorf("creating flag: %w", err)
	}

	s.publishEvent(ctx, "created", flag)
	return nil
}

// GetFlag retrieves a feature flag by its unique identifier.
func (s *flagService) GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
	flag, err := s.repo.GetFlag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting flag: %w", err)
	}
	return flag, nil
}

// ListFlags returns a paginated list of feature flags for a project.
func (s *flagService) ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	flags, err := s.repo.ListFlags(ctx, projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("listing flags: %w", err)
	}
	return flags, nil
}

// UpdateFlag validates and persists changes to an existing feature flag.
func (s *flagService) UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	flag.UpdatedAt = time.Now().UTC()

	if err := flag.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.UpdateFlag(ctx, flag); err != nil {
		return fmt.Errorf("updating flag: %w", err)
	}

	// Invalidate cached flag data so subsequent evaluations pick up the change.
	_ = s.cache.Invalidate(ctx, flag.ID)

	s.publishEvent(ctx, "updated", flag)
	return nil
}

// ArchiveFlag marks a flag as archived (sets archived_at = now() in the DB).
// Idempotent: already-archived flags return nil instead of an error.
func (s *flagService) ArchiveFlag(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.ArchiveFlag(ctx, id); err != nil {
		// The repo returns a "not found" error when the flag is missing OR already
		// archived (the WHERE clause includes `archived_at IS NULL`). We can't
		// distinguish them at the repo layer without an extra query.
		// For idempotency, treat "not found" as success — the post-condition
		// (flag is archived) holds. If callers need a 404 for "missing",
		// they should GetFlag first.
		// Note: we use strings.Contains rather than importing the postgres package
		// directly because postgres/flags.go imports internal/flags (cycle).
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("archiving flag: %w", err)
	}

	// Reload for event payload + cache invalidation. Failures here are non-fatal.
	flag, err := s.repo.GetFlag(ctx, id)
	if err != nil {
		_ = s.cache.Invalidate(ctx, id)
		return nil
	}
	_ = s.cache.Invalidate(ctx, flag.ID)
	s.publishEvent(ctx, "archived", flag)
	return nil
}

// UnarchiveFlag restores an archived flag to active status by clearing archived_at.
func (s *flagService) UnarchiveFlag(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.UnarchiveFlag(ctx, id); err != nil {
		return fmt.Errorf("unarchiving flag: %w", err)
	}

	// Reload the flag so the event payload is accurate.
	flag, err := s.repo.GetFlag(ctx, id)
	if err != nil {
		return fmt.Errorf("getting flag after unarchive: %w", err)
	}

	_ = s.cache.Invalidate(ctx, id)
	s.publishEvent(ctx, "unarchived", flag)
	return nil
}

// ToggleFlag sets the enabled state of a feature flag.
func (s *flagService) ToggleFlag(ctx context.Context, id uuid.UUID, enabled bool) error {
	flag, err := s.repo.GetFlag(ctx, id)
	if err != nil {
		return fmt.Errorf("getting flag for toggle: %w", err)
	}

	if flag.Archived {
		return fmt.Errorf("cannot toggle an archived flag")
	}

	flag.Enabled = enabled
	flag.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateFlag(ctx, flag); err != nil {
		return fmt.Errorf("toggling flag: %w", err)
	}

	// Invalidate cached flag data after toggle.
	_ = s.cache.Invalidate(ctx, flag.ID)

	s.publishEvent(ctx, "toggled", flag)
	return nil
}

// BulkToggle toggles the enabled state of multiple flags in a single operation.
// It skips archived flags and continues on individual failures, returning the
// first error encountered.
func (s *flagService) BulkToggle(ctx context.Context, flagIDs []uuid.UUID, enabled bool) error {
	var firstErr error
	for _, id := range flagIDs {
		if err := s.ToggleFlag(ctx, id, enabled); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("bulk toggle flag %s: %w", id, err)
			}
		}
	}
	return firstErr
}

// Evaluate evaluates a feature flag for the given context using the
// flag evaluation engine.
func (s *flagService) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error) {
	return s.evaluator.Evaluate(ctx, projectID, environmentID, key, evalCtx)
}

// BatchEvaluate evaluates multiple feature flags concurrently for the given
// context. Individual flag failures are captured in the Error field of each
// result rather than aborting the entire batch. Concurrency is bounded to 10.
func (s *flagService) BatchEvaluate(ctx context.Context, projectID, environmentID uuid.UUID, keys []string, evalCtx models.EvaluationContext) ([]*models.FlagEvaluationResult, error) {
	results := make([]*models.FlagEvaluationResult, len(keys))
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, key := range keys {
		i, key := i, key
		g.Go(func() error {
			result, err := s.evaluator.Evaluate(gCtx, projectID, environmentID, key, evalCtx)
			if err != nil {
				results[i] = &models.FlagEvaluationResult{
					FlagKey: key,
					Enabled: false,
					Value:   "",
					Reason:  "error",
					Error:   err.Error(),
				}
				return nil
			}
			results[i] = result
			return nil
		})
	}
	_ = g.Wait()
	return results, nil
}

// AddRule validates and persists a new targeting rule.
func (s *flagService) AddRule(ctx context.Context, rule *models.TargetingRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	now := time.Now().UTC()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	if err := rule.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return fmt.Errorf("creating rule: %w", err)
	}
	return nil
}

// UpdateRule validates and persists changes to an existing targeting rule.
func (s *flagService) UpdateRule(ctx context.Context, rule *models.TargetingRule) error {
	rule.UpdatedAt = time.Now().UTC()

	if err := rule.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return fmt.Errorf("updating rule: %w", err)
	}
	return nil
}

// DeleteRule removes a targeting rule by its ID.
// GetRule retrieves a targeting rule by its unique identifier.
func (s *flagService) GetRule(ctx context.Context, ruleID uuid.UUID) (*models.TargetingRule, error) {
	rule, err := s.repo.GetRule(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("getting rule: %w", err)
	}
	return rule, nil
}

func (s *flagService) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	if err := s.repo.DeleteRule(ctx, ruleID); err != nil {
		return fmt.Errorf("deleting rule: %w", err)
	}
	return nil
}

// ListRules returns all targeting rules for a given flag.
func (s *flagService) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	rules, err := s.repo.ListRules(ctx, flagID)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}
	return rules, nil
}

// ListFlagEnvStates returns all per-environment states for a given flag.
func (s *flagService) ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error) {
	states, err := s.repo.ListFlagEnvStates(ctx, flagID)
	if err != nil {
		return nil, fmt.Errorf("listing flag env states: %w", err)
	}
	return states, nil
}

// SetFlagEnvState validates and persists a per-environment flag state.
func (s *flagService) SetFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error {
	if err := state.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	if err := s.repo.UpsertFlagEnvState(ctx, state); err != nil {
		return fmt.Errorf("setting flag env state: %w", err)
	}
	return nil
}

// SetRuleEnvironmentState creates or updates a per-environment rule state.
func (s *flagService) SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	state, err := s.repo.SetRuleEnvironmentState(ctx, ruleID, environmentID, enabled)
	if err != nil {
		return nil, fmt.Errorf("setting rule environment state: %w", err)
	}
	return state, nil
}

// ListRuleEnvironmentStates returns all per-environment states for a flag's rules.
func (s *flagService) ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
	states, err := s.repo.ListRuleEnvironmentStates(ctx, flagID)
	if err != nil {
		return nil, fmt.Errorf("listing rule environment states: %w", err)
	}
	return states, nil
}

// DetectStaleFlags returns feature flags that have not been evaluated within
// the given threshold duration. It queries all active (non-archived) flags for
// the project and returns those whose UpdatedAt timestamp is older than
// time.Now() - threshold.
func (s *flagService) DetectStaleFlags(ctx context.Context, projectID uuid.UUID, threshold time.Duration) ([]*models.FeatureFlag, error) {
	notArchived := false
	flags, err := s.repo.ListFlags(ctx, projectID, ListOptions{
		Limit:    1000,
		Offset:   0,
		Archived: &notArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("listing flags for stale detection: %w", err)
	}

	cutoff := time.Now().UTC().Add(-threshold)
	var stale []*models.FeatureFlag
	for _, f := range flags {
		if f.UpdatedAt.Before(cutoff) {
			stale = append(stale, f)
		}
	}
	return stale, nil
}

// CreateSegment persists a new segment, generating an ID if not set.
func (s *flagService) CreateSegment(ctx context.Context, segment *models.Segment) error {
	return s.repo.CreateSegment(ctx, segment)
}

// GetSegment retrieves a segment by its unique identifier.
func (s *flagService) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	return s.repo.GetSegment(ctx, id)
}

// ListSegments returns all segments for a project.
func (s *flagService) ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error) {
	return s.repo.ListSegments(ctx, projectID)
}

// UpdateSegment persists changes to an existing segment.
func (s *flagService) UpdateSegment(ctx context.Context, segment *models.Segment) error {
	return s.repo.UpdateSegment(ctx, segment)
}

// DeleteSegment removes a segment by its ID.
func (s *flagService) DeleteSegment(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSegment(ctx, id)
}

// ExportFlags builds a YAMLExport snapshot of all flags for the given project.
func (s *flagService) ExportFlags(ctx context.Context, projectID uuid.UUID, envs []YAMLEnvironment) (*YAMLExport, error) {
	flags, err := s.repo.ListFlags(ctx, projectID, ListOptions{Limit: 10000})
	if err != nil {
		return nil, fmt.Errorf("listing flags for export: %w", err)
	}

	export := &YAMLExport{
		Version:      1,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		Environments: envs,
		Flags:        make([]YAMLFlag, 0, len(flags)),
	}

	for _, f := range flags {
		yf := YAMLFlag{
			Key:          f.Key,
			Name:         f.Name,
			FlagType:     string(f.FlagType),
			Category:     string(f.Category),
			DefaultValue: f.DefaultValue,
			IsPermanent:  f.IsPermanent,
			Environments: make(map[string]YAMLFlagEnv),
		}
		if f.ExpiresAt != nil {
			yf.ExpiresAt = f.ExpiresAt.Format(time.RFC3339)
		}

		// Per-environment flag states.
		envStates, err := s.repo.ListFlagEnvStates(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("listing env states for flag %s: %w", f.Key, err)
		}
		for _, es := range envStates {
			val := ""
			if es.Value != nil {
				val = string(*es.Value)
			}
			yf.Environments[es.EnvironmentID.String()] = YAMLFlagEnv{
				Enabled: es.Enabled,
				Value:   val,
			}
		}

		// Targeting rules.
		rules, err := s.repo.ListRules(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("listing rules for flag %s: %w", f.Key, err)
		}

		// Rule environment states (all at once for the flag).
		ruleEnvStates, err := s.repo.ListRuleEnvironmentStates(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("listing rule env states for flag %s: %w", f.Key, err)
		}
		// Build a lookup: ruleID -> envID -> enabled.
		ruleEnvMap := make(map[uuid.UUID]map[string]bool)
		for _, res := range ruleEnvStates {
			if ruleEnvMap[res.RuleID] == nil {
				ruleEnvMap[res.RuleID] = make(map[string]bool)
			}
			ruleEnvMap[res.RuleID][res.EnvironmentID.String()] = res.Enabled
		}

		for _, r := range rules {
			yr := YAMLRule{
				Attribute:    r.Attribute,
				Operator:     r.Operator,
				TargetValues: r.TargetValues,
				Value:        r.Value,
				Priority:     r.Priority,
				Environments: ruleEnvMap[r.ID],
			}
			yf.Rules = append(yf.Rules, yr)
		}

		export.Flags = append(export.Flags, yf)
	}

	return export, nil
}

// ---------------------------------------------------------------------------
// Feature lifecycle layer
// ---------------------------------------------------------------------------

// GetFlagByProjectKey resolves a flag by project and key (lifecycle endpoints).
func (s *flagService) GetFlagByProjectKey(ctx context.Context, projectID uuid.UUID, key string) (*models.FeatureFlag, error) {
	flag, err := s.repo.GetFlagByProjectKey(ctx, projectID, key)
	if err != nil {
		return nil, fmt.Errorf("getting flag by project/key: %w", err)
	}
	return flag, nil
}

// RecordSmokeTestResult records a smoke-test outcome. On fail the flag is
// disabled across every environment and iteration_count is bumped.
func (s *flagService) RecordSmokeTestResult(ctx context.Context, flagID uuid.UUID, status models.LifecycleTestStatus, notes, _ string) (*models.FeatureFlag, error) {
	if err := validateLifecycleStatus(status); err != nil {
		return nil, err
	}
	patch := LifecyclePatch{
		SmokeTestStatus: &status,
		SetSmokeNotes:   true,
		SmokeNotes:      stringPtrOrNil(notes),
	}
	if status == models.LifecycleTestFail {
		patch.IterationIncrement = 1
	}
	if err := s.repo.UpdateFlagLifecycle(ctx, flagID, patch, status == models.LifecycleTestFail); err != nil {
		return nil, fmt.Errorf("updating lifecycle (smoke): %w", err)
	}
	_ = s.cache.Invalidate(ctx, flagID)
	return s.repo.GetFlag(ctx, flagID)
}

// RecordUserTestResult records a user sign-off outcome. On fail the flag is
// disabled across every environment and iteration_count is bumped.
func (s *flagService) RecordUserTestResult(ctx context.Context, flagID uuid.UUID, status models.LifecycleTestStatus, notes string) (*models.FeatureFlag, error) {
	if err := validateLifecycleStatus(status); err != nil {
		return nil, err
	}
	if status == models.LifecycleTestFail && notes == "" {
		return nil, fmt.Errorf("notes are required for a failing user-test result")
	}
	patch := LifecyclePatch{
		UserTestStatus: &status,
		SetUserNotes:   true,
		UserNotes:      stringPtrOrNil(notes),
	}
	if status == models.LifecycleTestFail {
		patch.IterationIncrement = 1
	}
	if err := s.repo.UpdateFlagLifecycle(ctx, flagID, patch, status == models.LifecycleTestFail); err != nil {
		return nil, fmt.Errorf("updating lifecycle (user): %w", err)
	}
	_ = s.cache.Invalidate(ctx, flagID)
	return s.repo.GetFlag(ctx, flagID)
}

// ScheduleRemoval queues a flag for removal in `days` days.
func (s *flagService) ScheduleRemoval(ctx context.Context, flagID uuid.UUID, days int) (*models.FeatureFlag, error) {
	if days <= 0 {
		return nil, fmt.Errorf("days must be a positive integer")
	}
	t := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	patch := LifecyclePatch{SetScheduledRemovalAt: true, ScheduledRemovalAt: &t}
	if err := s.repo.UpdateFlagLifecycle(ctx, flagID, patch, false); err != nil {
		return nil, fmt.Errorf("scheduling removal: %w", err)
	}
	return s.repo.GetFlag(ctx, flagID)
}

// CancelScheduledRemoval clears the scheduled removal timestamp.
func (s *flagService) CancelScheduledRemoval(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error) {
	patch := LifecyclePatch{SetScheduledRemovalAt: true, ScheduledRemovalAt: nil}
	if err := s.repo.UpdateFlagLifecycle(ctx, flagID, patch, false); err != nil {
		return nil, fmt.Errorf("cancelling scheduled removal: %w", err)
	}
	return s.repo.GetFlag(ctx, flagID)
}

// MarkIterationExhausted flips the exhausted flag on.
func (s *flagService) MarkIterationExhausted(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error) {
	exhausted := true
	patch := LifecyclePatch{IterationExhausted: &exhausted}
	if err := s.repo.UpdateFlagLifecycle(ctx, flagID, patch, false); err != nil {
		return nil, fmt.Errorf("marking exhausted: %w", err)
	}
	return s.repo.GetFlag(ctx, flagID)
}

// ListFlagsDueForRemoval is a thin pass-through used by the scheduler.
func (s *flagService) ListFlagsDueForRemoval(ctx context.Context, now time.Time) ([]*models.FeatureFlag, error) {
	return s.repo.ListFlagsDueForRemoval(ctx, now)
}

// MarkFlagRemovalFired is a thin pass-through used by the scheduler.
func (s *flagService) MarkFlagRemovalFired(ctx context.Context, flagID uuid.UUID, firedAt time.Time) error {
	return s.repo.MarkFlagRemovalFired(ctx, flagID, firedAt)
}

// QueueDeletion delegates to repo.QueueDeletion, invalidates the flag's
// cache entry, and publishes a "queued_for_deletion" event.
func (s *flagService) QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error {
	if err := s.repo.QueueDeletion(ctx, id, retention); err != nil {
		return fmt.Errorf("queue deletion: %w", err)
	}
	_ = s.cache.Invalidate(ctx, id)
	flag, err := s.repo.GetFlag(ctx, id)
	if err == nil {
		s.publishEvent(ctx, "queued_for_deletion", flag)
	}
	return nil
}

// HardDeleteFlag delegates to repo.HardDeleteFlag, invalidates the
// flag's cache entry, and publishes a "hard_deleted" event.
func (s *flagService) HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error {
	if err := s.repo.HardDeleteFlag(ctx, id, retention); err != nil {
		return fmt.Errorf("hard delete: %w", err)
	}
	_ = s.cache.Invalidate(ctx, id)
	// Tombstoned: GetFlag will still return the row but with deleted_at set.
	s.publishEvent(ctx, "hard_deleted", &models.FeatureFlag{ID: id})
	return nil
}

// RestoreFlag delegates to repo.RestoreFlag, invalidates the flag's
// cache entry, and publishes a "restored" event. The repo's
// RestoreFlag returns ErrNotFound when no row matches; we treat
// missing flags as a real error (not idempotent).
func (s *flagService) RestoreFlag(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.RestoreFlag(ctx, id); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	_ = s.cache.Invalidate(ctx, id)
	flag, err := s.repo.GetFlag(ctx, id)
	if err == nil {
		s.publishEvent(ctx, "restored", flag)
	}
	return nil
}

func validateLifecycleStatus(s models.LifecycleTestStatus) error {
	switch s {
	case models.LifecycleTestPending, models.LifecycleTestPass, models.LifecycleTestFail:
		return nil
	default:
		return fmt.Errorf("status must be one of pending, pass, fail")
	}
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// WarmCache pre-loads all active flags for a project into the evaluation cache.
// This should be called during service startup to reduce cache miss latency for
// the first evaluations.
func (s *flagService) WarmCache(ctx context.Context, projectID uuid.UUID) error {
	notArchived := false
	flags, err := s.repo.ListFlags(ctx, projectID, ListOptions{
		Limit:    1000,
		Offset:   0,
		Archived: &notArchived,
	})
	if err != nil {
		return fmt.Errorf("loading flags for cache warm-up: %w", err)
	}

	for _, flag := range flags {
		if !flag.Enabled {
			continue
		}
		_ = s.cache.SetFlag(ctx, flag, 30*time.Second)

		rules, err := s.repo.ListRules(ctx, flag.ID)
		if err != nil {
			continue
		}
		_ = s.cache.SetRules(ctx, flag.ID, rules, 30*time.Second)
	}
	return nil
}
