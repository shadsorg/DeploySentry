// Package flags implements feature flag management, evaluation, and targeting.
package flags

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

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
}

// ListOptions controls pagination and filtering for flag list queries.
type ListOptions struct {
	Limit         int        `json:"limit"`
	Offset        int        `json:"offset"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty"`
	Archived      *bool      `json:"archived,omitempty"`
	Tag           string     `json:"tag,omitempty"`
}
