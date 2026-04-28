// Package appstatus persists and serves self-reported application status
// (version + health) from apps running on PaaS platforms where the
// DeploySentry sidecar agent is unavailable.
package appstatus

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Repository defines persistence operations for app_status and app_status_history.
type Repository interface {
	// UpsertStatus replaces the latest row for (application_id, environment_id).
	UpsertStatus(ctx context.Context, s *models.AppStatus) error

	// AppendHistory writes a single historical sample.
	AppendHistory(ctx context.Context, sample *models.AppStatusSample) error

	// GetStatus returns the latest status for the given (app, env), or nil
	// when no sample has been reported yet.
	GetStatus(ctx context.Context, appID, envID uuid.UUID) (*models.AppStatus, error)

	// HasDeploymentForVersion reports whether any deployment row exists
	// for the given (app, env, version). Used to decide whether a new
	// status report should auto-create a mode=record deployment.
	HasDeploymentForVersion(ctx context.Context, appID, envID uuid.UUID, version string) (bool, error)
}
