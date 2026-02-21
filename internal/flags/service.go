package flags

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

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

	// AddRule adds a targeting rule to a flag.
	AddRule(ctx context.Context, rule *models.TargetingRule) error

	// UpdateRule updates an existing targeting rule.
	UpdateRule(ctx context.Context, rule *models.TargetingRule) error

	// DeleteRule removes a targeting rule.
	DeleteRule(ctx context.Context, ruleID uuid.UUID) error
}

// flagService is the concrete implementation of FlagService.
type flagService struct {
	repo      FlagRepository
	evaluator *Evaluator
}

// NewFlagService creates a new FlagService backed by the given repository and cache.
func NewFlagService(repo FlagRepository, cache Cache) FlagService {
	return &flagService{
		repo:      repo,
		evaluator: NewEvaluator(repo, cache),
	}
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
	return nil
}

// Evaluate evaluates a feature flag for the given context using the
// flag evaluation engine.
func (s *flagService) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error) {
	return s.evaluator.Evaluate(ctx, projectID, environmentID, key, evalCtx)
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
