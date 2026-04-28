package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AppStatusRepository is the PostgreSQL implementation of appstatus.Repository.
type AppStatusRepository struct {
	pool *pgxpool.Pool
}

// NewAppStatusRepository constructs an AppStatusRepository.
func NewAppStatusRepository(pool *pgxpool.Pool) *AppStatusRepository {
	return &AppStatusRepository{pool: pool}
}

// UpsertStatus writes (or replaces) the latest status row for (app, env).
func (r *AppStatusRepository) UpsertStatus(ctx context.Context, s *models.AppStatus) error {
	tagsJSON, err := json.Marshal(s.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	const q = `
		INSERT INTO app_status (
			application_id, environment_id, version, commit_sha,
			health_state, health_score, health_reason, deploy_slot,
			tags, source, reported_at
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11)
		ON CONFLICT (application_id, environment_id) DO UPDATE SET
			version       = EXCLUDED.version,
			commit_sha    = EXCLUDED.commit_sha,
			health_state  = EXCLUDED.health_state,
			health_score  = EXCLUDED.health_score,
			health_reason = EXCLUDED.health_reason,
			deploy_slot   = EXCLUDED.deploy_slot,
			tags          = EXCLUDED.tags,
			source        = EXCLUDED.source,
			reported_at   = EXCLUDED.reported_at`

	_, err = r.pool.Exec(ctx, q,
		s.ApplicationID, s.EnvironmentID, s.Version, s.CommitSHA,
		s.HealthState, s.HealthScore, s.HealthReason, s.DeploySlot,
		tagsJSON, s.Source, s.ReportedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpsertStatus: %w", err)
	}
	return nil
}

// AppendHistory appends one retained sample to app_status_history.
func (r *AppStatusRepository) AppendHistory(ctx context.Context, sample *models.AppStatusSample) error {
	const q = `
		INSERT INTO app_status_history
			(application_id, environment_id, version, health_state, health_score, reported_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		sample.ApplicationID, sample.EnvironmentID, sample.Version,
		sample.HealthState, sample.HealthScore, sample.ReportedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.AppendHistory: %w", err)
	}
	return nil
}

// GetStatus returns the latest status row, or (nil, ErrNotFound) when empty.
func (r *AppStatusRepository) GetStatus(ctx context.Context, appID, envID uuid.UUID) (*models.AppStatus, error) {
	const q = `
		SELECT application_id, environment_id, version, COALESCE(commit_sha, ''),
		       health_state, health_score, COALESCE(health_reason, ''), COALESCE(deploy_slot, ''),
		       tags, source, reported_at
		FROM app_status
		WHERE application_id = $1 AND environment_id = $2`

	row := r.pool.QueryRow(ctx, q, appID, envID)
	var s models.AppStatus
	var tagsJSON []byte
	err := row.Scan(
		&s.ApplicationID, &s.EnvironmentID, &s.Version, &s.CommitSHA,
		&s.HealthState, &s.HealthScore, &s.HealthReason, &s.DeploySlot,
		&tagsJSON, &s.Source, &s.ReportedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetStatus: %w", err)
	}
	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &s.Tags); err != nil {
			return nil, fmt.Errorf("postgres.GetStatus tags: %w", err)
		}
	}
	if s.Tags == nil {
		s.Tags = map[string]string{}
	}
	return &s, nil
}

// HasDeploymentForVersion reports whether any deployment row exists for
// the given (app, env, version). Used to gate auto-record on status push.
func (r *AppStatusRepository) HasDeploymentForVersion(ctx context.Context, appID, envID uuid.UUID, version string) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1 FROM deployments
			WHERE application_id = $1 AND environment_id = $2 AND version = $3
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, q, appID, envID, version).Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres.HasDeploymentForVersion: %w", err)
	}
	return exists, nil
}
