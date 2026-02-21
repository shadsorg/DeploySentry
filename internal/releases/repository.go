// Package releases implements release versioning and promotion through
// deployment pipelines.
package releases

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ReleaseRepository defines the persistence interface for release entities.
type ReleaseRepository interface {
	// CreateRelease persists a new release record.
	CreateRelease(ctx context.Context, release *models.Release) error

	// GetRelease retrieves a release by its unique identifier.
	GetRelease(ctx context.Context, id uuid.UUID) (*models.Release, error)

	// ListReleases returns releases for a project, ordered by creation time descending.
	ListReleases(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Release, error)

	// UpdateRelease persists changes to an existing release.
	UpdateRelease(ctx context.Context, release *models.Release) error

	// CreateReleaseEnvironment persists a new release-environment association.
	CreateReleaseEnvironment(ctx context.Context, re *models.ReleaseEnvironment) error

	// ListReleaseEnvironments returns the environments associated with a release.
	ListReleaseEnvironments(ctx context.Context, releaseID uuid.UUID) ([]*models.ReleaseEnvironment, error)

	// UpdateReleaseEnvironment persists changes to a release-environment association.
	UpdateReleaseEnvironment(ctx context.Context, re *models.ReleaseEnvironment) error
}

// ListOptions controls pagination and filtering for release list queries.
type ListOptions struct {
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Status *models.ReleaseStatus  `json:"status,omitempty"`
}
