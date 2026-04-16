// Package flags implements feature flag management, evaluation, and targeting.
package flags

import (
	"context"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EvaluationLog represents a single recorded flag evaluation event, written
// in batches to the persistence layer for analytics and debugging.
type EvaluationLog struct {
	ID        uuid.UUID                `json:"id"`
	FlagKey   string                   `json:"flag_key"`
	Value     string                   `json:"value"`
	Reason    string                   `json:"reason"`
	RuleID    string                   `json:"rule_id,omitempty"`
	EvalCtx   models.EvaluationContext `json:"eval_context"`
	Timestamp time.Time                `json:"timestamp"`
}

// FlagRepository defines the persistence interface for feature flag entities.
type FlagRepository interface {
	// CreateFlag persists a new feature flag.
	CreateFlag(ctx context.Context, flag *models.FeatureFlag) error

	// GetFlag retrieves a feature flag by its unique identifier.
	GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error)

	// GetFlagByKey retrieves a feature flag by its project, environment, and key.
	GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error)

	// ListFlags returns feature flags for a project, with optional filtering.
	ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error)

	// UpdateFlag persists changes to an existing feature flag.
	UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error

	// DeleteFlag removes a feature flag. Typically flags are archived, not deleted.
	DeleteFlag(ctx context.Context, id uuid.UUID) error

	// CreateRule persists a new targeting rule.
	CreateRule(ctx context.Context, rule *models.TargetingRule) error

	// GetRule retrieves a targeting rule by ID.
	GetRule(ctx context.Context, id uuid.UUID) (*models.TargetingRule, error)

	// ListRules returns all targeting rules for a flag, ordered by priority.
	ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)

	// UpdateRule persists changes to an existing targeting rule.
	UpdateRule(ctx context.Context, rule *models.TargetingRule) error

	// DeleteRule removes a targeting rule.
	DeleteRule(ctx context.Context, id uuid.UUID) error

	// WriteEvaluationLog persists a batch of flag evaluation log entries. This
	// method is designed for batched, sampled writes to avoid overwhelming the
	// database with high-volume evaluation data.
	WriteEvaluationLog(ctx context.Context, logs []EvaluationLog) error

	// ListFlagEnvStates returns all per-environment states for a given flag.
	ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error)

	// UpsertFlagEnvState creates or updates a per-environment flag state.
	UpsertFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error

	// Rule environment state operations
	SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error)
	ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error)
	ListRuleEnvironmentStatesByEnv(ctx context.Context, flagID uuid.UUID, environmentID uuid.UUID) (map[uuid.UUID]bool, error)

	// Segment operations
	CreateSegment(ctx context.Context, segment *models.Segment) error
	GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error)
	GetSegmentByKey(ctx context.Context, projectID uuid.UUID, key string) (*models.Segment, error)
	ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error)
	UpdateSegment(ctx context.Context, segment *models.Segment) error
	DeleteSegment(ctx context.Context, id uuid.UUID) error
}

// ListOptions controls pagination and filtering for flag list queries.
type ListOptions struct {
	Limit         int        `json:"limit"`
	Offset        int        `json:"offset"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty"`
	ApplicationID *uuid.UUID `json:"application_id,omitempty"`
	Archived      *bool      `json:"archived,omitempty"`
	Tag           string     `json:"tag,omitempty"`
}
