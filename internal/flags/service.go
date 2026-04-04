package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
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

	// DeleteRule removes a targeting rule.
	DeleteRule(ctx context.Context, ruleID uuid.UUID) error

	// ListRules returns all targeting rules for a flag.
	ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)

	// ListFlagEnvStates returns all per-environment states for a given flag.
	ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error)

	// SetFlagEnvState creates or updates a per-environment flag state.
	SetFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error

	// DetectStaleFlags returns flags that have not been evaluated within the
	// given threshold duration for the specified project.
	DetectStaleFlags(ctx context.Context, projectID uuid.UUID, threshold time.Duration) ([]*models.FeatureFlag, error)

	// WarmCache pre-loads active flags into the evaluation cache.
	WarmCache(ctx context.Context, projectID uuid.UUID) error
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

// ArchiveFlag marks a flag as archived and disabled.
func (s *flagService) ArchiveFlag(ctx context.Context, id uuid.UUID) error {
	flag, err := s.repo.GetFlag(ctx, id)
	if err != nil {
		return fmt.Errorf("getting flag for archive: %w", err)
	}

	flag.Archived = true
	flag.Enabled = false
	flag.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateFlag(ctx, flag); err != nil {
		return fmt.Errorf("archiving flag: %w", err)
	}

	// Invalidate cached flag data after archival.
	_ = s.cache.Invalidate(ctx, flag.ID)

	s.publishEvent(ctx, "archived", flag)
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

// BatchEvaluate evaluates multiple feature flags for the given context in a
// single call. Each flag is evaluated independently; evaluation errors for
// individual flags result in a default disabled result rather than failing the
// entire batch.
func (s *flagService) BatchEvaluate(ctx context.Context, projectID, environmentID uuid.UUID, keys []string, evalCtx models.EvaluationContext) ([]*models.FlagEvaluationResult, error) {
	results := make([]*models.FlagEvaluationResult, 0, len(keys))
	for _, key := range keys {
		result, err := s.evaluator.Evaluate(ctx, projectID, environmentID, key, evalCtx)
		if err != nil {
			// Return a default disabled result for flags that fail evaluation.
			results = append(results, &models.FlagEvaluationResult{
				FlagKey: key,
				Enabled: false,
				Value:   "",
				Reason:  "error",
			})
			continue
		}
		results = append(results, result)
	}
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
