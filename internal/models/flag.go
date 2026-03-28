package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// FlagType defines the data type of a feature flag's value.
type FlagType string

const (
	// FlagTypeBoolean represents a simple on/off toggle.
	FlagTypeBoolean FlagType = "boolean"
	// FlagTypeString represents a string-valued flag.
	FlagTypeString FlagType = "string"
	// FlagTypeInteger represents an integer-valued flag.
	FlagTypeInteger FlagType = "integer"
	// FlagTypeJSON represents a flag whose value is arbitrary JSON.
	FlagTypeJSON FlagType = "json"
)

// FlagCategory classifies a flag's intent so teams can filter and manage flags
// by their lifecycle pattern.
type FlagCategory string

const (
	// FlagCategoryRelease gates code that ships with a release and is removed
	// once the release is fully rolled out.
	FlagCategoryRelease FlagCategory = "release"
	// FlagCategoryFeature controls long-lived product features that may remain
	// flagged permanently (e.g. plan-gated functionality).
	FlagCategoryFeature FlagCategory = "feature"
	// FlagCategoryExperiment is used for A/B tests and experiments with a
	// defined end date.
	FlagCategoryExperiment FlagCategory = "experiment"
	// FlagCategoryOps controls operational behaviors like maintenance mode,
	// rate limits, or circuit breakers.
	FlagCategoryOps FlagCategory = "ops"
	// FlagCategoryPermission gates access to functionality based on
	// entitlements or roles.
	FlagCategoryPermission FlagCategory = "permission"
)

// RuleType identifies the kind of targeting rule applied to a feature flag.
type RuleType string

const (
	// RuleTypePercentage routes a percentage of traffic based on a hash.
	RuleTypePercentage RuleType = "percentage"
	// RuleTypeUserTarget matches specific user IDs.
	RuleTypeUserTarget RuleType = "user_target"
	// RuleTypeAttribute matches evaluation context attributes.
	RuleTypeAttribute RuleType = "attribute"
	// RuleTypeSegment matches pre-defined user segments.
	RuleTypeSegment RuleType = "segment"
	// RuleTypeSchedule activates based on a time window.
	RuleTypeSchedule RuleType = "schedule"
)

// FeatureFlag represents a feature flag configuration.
type FeatureFlag struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	ProjectID     uuid.UUID    `json:"project_id" db:"project_id"`
	ApplicationID *uuid.UUID   `json:"application_id,omitempty" db:"application_id"`
	EnvironmentID uuid.UUID    `json:"environment_id" db:"environment_id"`
	Key           string       `json:"key" db:"key"`
	Name          string       `json:"name" db:"name"`
	Description   string       `json:"description,omitempty" db:"description"`
	FlagType      FlagType     `json:"flag_type" db:"flag_type"`
	Category      FlagCategory `json:"category" db:"category"`
	Purpose       string       `json:"purpose,omitempty" db:"purpose"`
	Owners        []string     `json:"owners,omitempty" db:"-"`
	IsPermanent   bool         `json:"is_permanent" db:"is_permanent"`
	ExpiresAt     *time.Time   `json:"expires_at,omitempty" db:"expires_at"`
	Enabled       bool         `json:"enabled" db:"enabled"`
	DefaultValue  string       `json:"default_value" db:"default_value"`
	Archived      bool         `json:"archived" db:"archived"`
	Tags          []string     `json:"tags,omitempty" db:"-"`
	CreatedBy     uuid.UUID    `json:"created_by" db:"created_by"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// TargetingRule defines a single rule that influences how a feature flag is
// evaluated for a given context.
type TargetingRule struct {
	ID         uuid.UUID `json:"id" db:"id"`
	FlagID     uuid.UUID `json:"flag_id" db:"flag_id"`
	RuleType   RuleType  `json:"rule_type" db:"rule_type"`
	Priority   int       `json:"priority" db:"priority"`
	Value      string    `json:"value" db:"value"`
	Percentage *int      `json:"percentage,omitempty" db:"percentage"`
	Attribute  string    `json:"attribute,omitempty" db:"attribute"`
	Operator   string    `json:"operator,omitempty" db:"operator"`
	// TargetValues holds the list of values to match against (user IDs, segments, etc.).
	TargetValues []string   `json:"target_values,omitempty" db:"-"`
	SegmentID    *uuid.UUID `json:"segment_id,omitempty" db:"segment_id"`
	StartTime    *time.Time `json:"start_time,omitempty" db:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty" db:"end_time"`
	Enabled      bool       `json:"enabled" db:"enabled"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// EvaluationContext provides the runtime context used to evaluate a feature flag.
type EvaluationContext struct {
	UserID     string            `json:"user_id,omitempty"`
	OrgID      string            `json:"org_id,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// FlagEvaluationResult contains the outcome of evaluating a feature flag
// against a given context.
type FlagEvaluationResult struct {
	FlagKey     string        `json:"flag_key"`
	Enabled     bool          `json:"enabled"`
	Value       string        `json:"value"`
	Reason      string        `json:"reason"`
	RuleID      string        `json:"rule_id,omitempty"`
	VariationID string        `json:"variation_id,omitempty"`
	Metadata    *FlagMetadata `json:"metadata,omitempty"`
}

// FlagMetadata contains descriptive information about a flag returned alongside
// evaluation results. SDKs use this to surface flag intent in logs and tooling.
type FlagMetadata struct {
	Category    FlagCategory `json:"category"`
	Purpose     string       `json:"purpose,omitempty"`
	Owners      []string     `json:"owners,omitempty"`
	IsPermanent bool         `json:"is_permanent"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
}

// Validate checks that the FeatureFlag has all required fields populated.
func (f *FeatureFlag) Validate() error {
	if f.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if f.Key == "" {
		return errors.New("flag key is required")
	}
	if len(f.Key) > 200 {
		return errors.New("flag key must be 200 characters or fewer")
	}
	if f.Name == "" {
		return errors.New("flag name is required")
	}
	switch f.FlagType {
	case FlagTypeBoolean, FlagTypeString, FlagTypeInteger, FlagTypeJSON:
		// valid
	default:
		return errors.New("unsupported flag type")
	}
	switch f.Category {
	case FlagCategoryRelease, FlagCategoryFeature, FlagCategoryExperiment, FlagCategoryOps, FlagCategoryPermission, "":
		// valid (empty defaults to "feature" at persistence layer)
	default:
		return errors.New("unsupported flag category: must be release, feature, experiment, ops, or permission")
	}
	if f.Category == FlagCategoryRelease && f.ApplicationID == nil {
		return errors.New("application_id is required for release flags")
	}
	if !f.IsPermanent && f.ExpiresAt == nil && f.Category == FlagCategoryRelease {
		return errors.New("release flags must have an expiration date or be marked permanent")
	}
	return nil
}

// Validate checks that the TargetingRule has all required fields populated.
func (r *TargetingRule) Validate() error {
	if r.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	switch r.RuleType {
	case RuleTypePercentage:
		if r.Percentage == nil {
			return errors.New("percentage is required for percentage rules")
		}
		if *r.Percentage < 0 || *r.Percentage > 100 {
			return errors.New("percentage must be between 0 and 100")
		}
	case RuleTypeUserTarget:
		if len(r.TargetValues) == 0 {
			return errors.New("target_values required for user_target rules")
		}
	case RuleTypeAttribute:
		if r.Attribute == "" {
			return errors.New("attribute is required for attribute rules")
		}
		if r.Operator == "" {
			return errors.New("operator is required for attribute rules")
		}
	case RuleTypeSegment:
		if r.SegmentID == nil {
			return errors.New("segment_id is required for segment rules")
		}
	case RuleTypeSchedule:
		if r.StartTime == nil || r.EndTime == nil {
			return errors.New("start_time and end_time are required for schedule rules")
		}
	default:
		return errors.New("unsupported rule type")
	}
	return nil
}
