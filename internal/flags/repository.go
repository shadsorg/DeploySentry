// Package flags implements feature flag management, evaluation, and targeting.
package flags

import (
	"context"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
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

// LifecyclePatch describes a partial update to the lifecycle-only columns
// on a feature flag. nil pointers mean "leave unchanged". The `Set*` flags
// distinguish "clear this to NULL" from "leave untouched" for nullable
// columns whose zero value is meaningful.
type LifecyclePatch struct {
	SmokeTestStatus    *models.LifecycleTestStatus
	UserTestStatus     *models.LifecycleTestStatus
	IterationExhausted *bool
	IterationIncrement int

	SetScheduledRemovalAt bool
	ScheduledRemovalAt    *time.Time

	SetSmokeNotes bool
	SmokeNotes    *string

	SetUserNotes bool
	UserNotes    *string
}

// FlagRepository defines the persistence interface for feature flag entities.
type FlagRepository interface {
	// CreateFlag persists a new feature flag.
	CreateFlag(ctx context.Context, flag *models.FeatureFlag) error

	// GetFlag retrieves a feature flag by its unique identifier.
	GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error)

	// GetFlagByKey retrieves a feature flag by its project, environment, and key.
	GetFlagByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*models.FeatureFlag, error)

	// GetFlagByProjectKey retrieves a feature flag by project and key without
	// an environment constraint. Used by lifecycle endpoints where the API key
	// scopes the caller to a single project.
	GetFlagByProjectKey(ctx context.Context, projectID uuid.UUID, key string) (*models.FeatureFlag, error)

	// UpdateFlagLifecycle applies a partial update to the lifecycle columns.
	// When disableEverywhere is true, the flag and all its per-environment
	// overrides are disabled in the same transaction.
	UpdateFlagLifecycle(ctx context.Context, id uuid.UUID, patch LifecyclePatch, disableEverywhere bool) error

	// ListFlagsDueForRemoval returns flags whose scheduled_removal_at has
	// passed and whose due webhook has not yet been fired.
	ListFlagsDueForRemoval(ctx context.Context, now time.Time) ([]*models.FeatureFlag, error)

	// MarkFlagRemovalFired records the time the scheduler emitted the 'due'
	// webhook so subsequent ticks don't re-fire.
	MarkFlagRemovalFired(ctx context.Context, id uuid.UUID, firedAt time.Time) error

	// ListFlags returns feature flags for a project, with optional filtering.
	ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error)

	// UpdateFlag persists changes to an existing feature flag.
	UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error

	// ArchiveFlag soft-archives a feature flag by setting archived_at = now().
	// Returns ErrNotFound if the flag is already archived (archived_at IS NOT NULL).
	ArchiveFlag(ctx context.Context, id uuid.UUID) error

	// UnarchiveFlag clears archived_at on a feature flag, restoring it to active.
	UnarchiveFlag(ctx context.Context, id uuid.UUID) error

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

	// GetFlagEnvState returns the per-environment state for a specific flag and environment.
	GetFlagEnvState(ctx context.Context, flagID, environmentID uuid.UUID) (*models.FlagEnvironmentState, error)

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
