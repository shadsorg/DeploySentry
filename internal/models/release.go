package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReleaseStatus represents the lifecycle state of a release.
type ReleaseStatus string

const (
	// ReleaseStatusDraft indicates the release is being prepared.
	ReleaseStatusDraft ReleaseStatus = "draft"
	// ReleaseStatusActive indicates the release is actively being deployed.
	ReleaseStatusActive ReleaseStatus = "active"
	// ReleaseStatusCompleted indicates the release has been fully deployed.
	ReleaseStatusCompleted ReleaseStatus = "completed"
	// ReleaseStatusFailed indicates the release encountered a failure.
	ReleaseStatusFailed ReleaseStatus = "failed"
	// ReleaseStatusArchived indicates the release has been archived.
	ReleaseStatusArchived ReleaseStatus = "archived"
)

// ReleaseLifecycleStatus represents the fine-grained lifecycle state of a release
// through the build and deploy pipeline.
type ReleaseLifecycleStatus string

const (
	// ReleaseLifecycleBuilding indicates the release artifact is being built.
	ReleaseLifecycleBuilding ReleaseLifecycleStatus = "building"
	// ReleaseLifecycleBuilt indicates the release artifact has been built successfully.
	ReleaseLifecycleBuilt ReleaseLifecycleStatus = "built"
	// ReleaseLifecycleDeploying indicates the release is being deployed to an environment.
	ReleaseLifecycleDeploying ReleaseLifecycleStatus = "deploying"
	// ReleaseLifecycleDeployed indicates the release has been deployed to an environment.
	ReleaseLifecycleDeployed ReleaseLifecycleStatus = "deployed"
	// ReleaseLifecycleHealthy indicates the deployed release is healthy.
	ReleaseLifecycleHealthy ReleaseLifecycleStatus = "healthy"
	// ReleaseLifecycleDegraded indicates the deployed release is experiencing degraded health.
	ReleaseLifecycleDegraded ReleaseLifecycleStatus = "degraded"
	// ReleaseLifecycleRolledBack indicates the release has been rolled back.
	ReleaseLifecycleRolledBack ReleaseLifecycleStatus = "rolled_back"
)

// validLifecycleTransitions defines which lifecycle status transitions are allowed.
var validLifecycleTransitions = map[ReleaseLifecycleStatus][]ReleaseLifecycleStatus{
	ReleaseLifecycleBuilding:   {ReleaseLifecycleBuilt, ReleaseLifecycleRolledBack},
	ReleaseLifecycleBuilt:      {ReleaseLifecycleDeploying},
	ReleaseLifecycleDeploying:  {ReleaseLifecycleDeployed, ReleaseLifecycleRolledBack},
	ReleaseLifecycleDeployed:   {ReleaseLifecycleHealthy, ReleaseLifecycleDegraded, ReleaseLifecycleRolledBack},
	ReleaseLifecycleHealthy:    {ReleaseLifecycleDegraded, ReleaseLifecycleRolledBack},
	ReleaseLifecycleDegraded:   {ReleaseLifecycleHealthy, ReleaseLifecycleRolledBack},
	ReleaseLifecycleRolledBack: {},
}

// ValidateLifecycleTransition checks whether moving from the current lifecycle
// status to the target status is allowed.
func ValidateLifecycleTransition(current, target ReleaseLifecycleStatus) error {
	allowed, ok := validLifecycleTransitions[current]
	if !ok {
		return fmt.Errorf("unknown current lifecycle status %q", current)
	}
	for _, s := range allowed {
		if s == target {
			return nil
		}
	}
	return fmt.Errorf("invalid lifecycle transition from %q to %q", current, target)
}

// validReleaseTransitions defines which release status transitions are allowed.
var validReleaseTransitions = map[ReleaseStatus][]ReleaseStatus{
	ReleaseStatusDraft:     {ReleaseStatusActive, ReleaseStatusArchived},
	ReleaseStatusActive:    {ReleaseStatusCompleted, ReleaseStatusFailed, ReleaseStatusArchived},
	ReleaseStatusCompleted: {ReleaseStatusArchived},
	ReleaseStatusFailed:    {ReleaseStatusDraft, ReleaseStatusArchived},
	ReleaseStatusArchived:  {},
}

// Version represents a parsed semantic version (major.minor.patch).
type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

// String returns the string representation of the version (e.g., "1.2.3").
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseVersion parses a semantic version string in the format "major.minor.patch".
// An optional "v" prefix is allowed (e.g., "v1.2.3").
func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format %q: expected major.minor.patch", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("invalid major version %q", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return Version{}, fmt.Errorf("invalid minor version %q", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return Version{}, fmt.Errorf("invalid patch version %q", parts[2])
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// CompareVersions compares two semantic version strings.
// Returns -1 if a < b, 0 if a == b, and 1 if a > b.
// Returns an error if either version string is invalid.
func CompareVersions(a, b string) (int, error) {
	va, err := ParseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("parsing version a: %w", err)
	}
	vb, err := ParseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("parsing version b: %w", err)
	}

	if va.Major != vb.Major {
		if va.Major < vb.Major {
			return -1, nil
		}
		return 1, nil
	}
	if va.Minor != vb.Minor {
		if va.Minor < vb.Minor {
			return -1, nil
		}
		return 1, nil
	}
	if va.Patch != vb.Patch {
		if va.Patch < vb.Patch {
			return -1, nil
		}
		return 1, nil
	}
	return 0, nil
}

// Release represents a versioned release that can be deployed through a pipeline.
type Release struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	ProjectID       uuid.UUID              `json:"project_id" db:"project_id"`
	Version         string                 `json:"version" db:"version"`
	Title           string                 `json:"title" db:"title"`
	Description     string                 `json:"description,omitempty" db:"description"`
	CommitSHA       string                 `json:"commit_sha,omitempty" db:"commit_sha"`
	Artifact        string                 `json:"artifact" db:"artifact"`
	Status          ReleaseStatus          `json:"status" db:"status"`
	LifecycleStatus ReleaseLifecycleStatus `json:"lifecycle_status,omitempty" db:"lifecycle_status"`
	CreatedBy       uuid.UUID              `json:"created_by" db:"created_by"`
	ReleasedAt      *time.Time             `json:"released_at,omitempty" db:"released_at"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// ReleaseEnvironment tracks which environments a release has been deployed to
// and the status of each deployment.
type ReleaseEnvironment struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	ReleaseID       uuid.UUID              `json:"release_id" db:"release_id"`
	EnvironmentID   uuid.UUID              `json:"environment_id" db:"environment_id"`
	DeploymentID    *uuid.UUID             `json:"deployment_id,omitempty" db:"deployment_id"`
	Status          ReleaseStatus          `json:"status" db:"status"`
	LifecycleStatus ReleaseLifecycleStatus `json:"lifecycle_status,omitempty" db:"lifecycle_status"`
	HealthScore     float64                `json:"health_score" db:"health_score"`
	DeployedAt      *time.Time             `json:"deployed_at,omitempty" db:"deployed_at"`
	DeployedBy      *uuid.UUID             `json:"deployed_by,omitempty" db:"deployed_by"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}

// ReleaseTimeline represents a point in the release history for a project.
type ReleaseTimeline struct {
	ReleaseID     uuid.UUID              `json:"release_id"`
	Version       string                 `json:"version"`
	Title         string                 `json:"title"`
	EnvironmentID uuid.UUID              `json:"environment_id"`
	Status        ReleaseLifecycleStatus `json:"status"`
	DeployedAt    *time.Time             `json:"deployed_at,omitempty"`
	HealthScore   float64                `json:"health_score"`
}

// Validate checks that the Release has all required fields populated.
func (r *Release) Validate() error {
	if r.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if r.Version == "" {
		return errors.New("version is required")
	}
	if r.Title == "" {
		return errors.New("title is required")
	}
	if r.Artifact == "" {
		return errors.New("artifact is required")
	}
	if r.CreatedBy == uuid.Nil {
		return errors.New("created_by is required")
	}
	return nil
}

// ValidateTransition checks whether moving from the release's current status
// to the target status is allowed.
func (r *Release) ValidateTransition(target ReleaseStatus) error {
	allowed, ok := validReleaseTransitions[r.Status]
	if !ok {
		return fmt.Errorf("unknown current release status %q", r.Status)
	}
	for _, s := range allowed {
		if s == target {
			return nil
		}
	}
	return fmt.Errorf("invalid release status transition from %q to %q", r.Status, target)
}
