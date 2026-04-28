// Package releases implements release versioning and promotion through
// deployment pipelines.
package releases

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ReleaseRepository defines the persistence interface for release entities.
type ReleaseRepository interface {
	// Create persists a new release record.
	Create(ctx context.Context, release *models.Release) error

	// GetByID retrieves a release by its unique identifier.
	GetByID(ctx context.Context, id uuid.UUID) (*models.Release, error)

	// ListByApplication returns releases for an application, ordered by creation time descending.
	ListByApplication(ctx context.Context, appID uuid.UUID) ([]models.Release, error)

	// Update persists changes to an existing release.
	Update(ctx context.Context, release *models.Release) error

	// Delete removes a release by its unique identifier.
	Delete(ctx context.Context, id uuid.UUID) error

	// AddFlagChange persists a new flag change associated with a release.
	AddFlagChange(ctx context.Context, fc *models.ReleaseFlagChange) error

	// ListFlagChanges returns the flag changes associated with a release.
	ListFlagChanges(ctx context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error)
}
