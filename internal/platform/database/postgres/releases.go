package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/releases"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReleaseRepository implements releases.ReleaseRepository using a PostgreSQL connection pool.
type ReleaseRepository struct {
	pool *pgxpool.Pool
}

// NewReleaseRepository creates a new ReleaseRepository backed by the given pool.
func NewReleaseRepository(pool *pgxpool.Pool) *ReleaseRepository {
	return &ReleaseRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Column lists
// ---------------------------------------------------------------------------

const releaseSelectCols = `
	id, project_id, version,
	COALESCE(title, ''),
	COALESCE(description, ''),
	COALESCE(commit_sha, ''),
	COALESCE(artifact, COALESCE(artifact_url, '')),
	status,
	COALESCE(lifecycle_status, ''),
	created_by,
	released_at,
	created_at, updated_at`

const releaseEnvSelectCols = `
	id, release_id,
	COALESCE(environment_id, '00000000-0000-0000-0000-000000000000'::uuid),
	deployment_id,
	status,
	COALESCE(lifecycle_status, ''),
	COALESCE(health_score, 0),
	deployed_at,
	deployed_by,
	created_at, updated_at`

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// scanRelease reads a single Release row from the given pgx.Row.
// The SELECT must include columns in the order defined by releaseSelectCols.
func scanRelease(row pgx.Row) (*models.Release, error) {
	var r models.Release
	err := row.Scan(
		&r.ID,
		&r.ProjectID,
		&r.Version,
		&r.Title,
		&r.Description,
		&r.CommitSHA,
		&r.Artifact,
		&r.Status,
		&r.LifecycleStatus,
		&r.CreatedBy,
		&r.ReleasedAt,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

// scanReleaseEnvironment reads a single ReleaseEnvironment row from the given pgx.Row.
// The SELECT must include columns in the order defined by releaseEnvSelectCols.
func scanReleaseEnvironment(row pgx.Row) (*models.ReleaseEnvironment, error) {
	var re models.ReleaseEnvironment
	err := row.Scan(
		&re.ID,
		&re.ReleaseID,
		&re.EnvironmentID,
		&re.DeploymentID,
		&re.Status,
		&re.LifecycleStatus,
		&re.HealthScore,
		&re.DeployedAt,
		&re.DeployedBy,
		&re.CreatedAt,
		&re.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &re, nil
}

// ---------------------------------------------------------------------------
// Release methods
// ---------------------------------------------------------------------------

// CreateRelease inserts a new release into the database.
func (r *ReleaseRepository) CreateRelease(ctx context.Context, release *models.Release) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}
	now := time.Now().UTC()
	release.CreatedAt = now
	release.UpdatedAt = now

	const q = `
		INSERT INTO releases
			(id, project_id, version, title, description, commit_sha, artifact,
			 status, lifecycle_status, created_by, released_at, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7,
			 $8, $9, $10, $11, $12, $13)`

	_, err := r.pool.Exec(ctx, q,
		release.ID,
		release.ProjectID,
		release.Version,
		release.Title,
		release.Description,
		release.CommitSHA,
		release.Artifact,
		release.Status,
		release.LifecycleStatus,
		release.CreatedBy,
		release.ReleasedAt,
		release.CreatedAt,
		release.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateRelease: %w", err)
	}
	return nil
}

// GetRelease retrieves a release by its unique identifier.
func (r *ReleaseRepository) GetRelease(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	q := `SELECT` + releaseSelectCols + ` FROM releases WHERE id = $1`
	release, err := scanRelease(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetRelease: %w", err)
	}
	return release, nil
}

// ListReleases returns releases for a project, ordered by creation time descending.
func (r *ReleaseRepository) ListReleases(ctx context.Context, projectID uuid.UUID, opts releases.ListOptions) ([]*models.Release, error) {
	var w whereBuilder
	w.Add("project_id = $%d", projectID)

	if opts.Status != nil {
		w.Add("status = $%d", *opts.Status)
	}

	whereClause, args := w.Build()
	pagClause, args := paginationClause(opts.Limit, opts.Offset, args)
	q := `SELECT` + releaseSelectCols + ` FROM releases` + whereClause + ` ORDER BY created_at DESC` + pagClause

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListReleases: %w", err)
	}
	defer rows.Close()

	var result []*models.Release
	for rows.Next() {
		release, err := scanRelease(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListReleases: %w", err)
		}
		result = append(result, release)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListReleases: %w", err)
	}
	return result, nil
}

// UpdateRelease persists changes to an existing release.
func (r *ReleaseRepository) UpdateRelease(ctx context.Context, release *models.Release) error {
	release.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE releases SET
			status           = $2,
			lifecycle_status = $3,
			released_at      = $4,
			updated_at       = $5
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		release.ID,
		release.Status,
		release.LifecycleStatus,
		release.ReleasedAt,
		release.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateRelease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// ReleaseEnvironment methods
// ---------------------------------------------------------------------------

// CreateReleaseEnvironment inserts a new release-environment association.
func (r *ReleaseRepository) CreateReleaseEnvironment(ctx context.Context, re *models.ReleaseEnvironment) error {
	if re.ID == uuid.Nil {
		re.ID = uuid.New()
	}
	now := time.Now().UTC()
	re.CreatedAt = now
	re.UpdatedAt = now

	const q = `
		INSERT INTO release_environments
			(id, release_id, environment_id, deployment_id,
			 status, lifecycle_status, health_score,
			 deployed_at, deployed_by, created_at, updated_at)
		VALUES
			($1, $2, $3, $4,
			 $5, $6, $7,
			 $8, $9, $10, $11)`

	_, err := r.pool.Exec(ctx, q,
		re.ID,
		re.ReleaseID,
		re.EnvironmentID,
		re.DeploymentID,
		re.Status,
		re.LifecycleStatus,
		re.HealthScore,
		re.DeployedAt,
		re.DeployedBy,
		re.CreatedAt,
		re.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateReleaseEnvironment: %w", err)
	}
	return nil
}

// ListReleaseEnvironments returns the environments associated with a release.
func (r *ReleaseRepository) ListReleaseEnvironments(ctx context.Context, releaseID uuid.UUID) ([]*models.ReleaseEnvironment, error) {
	q := `SELECT` + releaseEnvSelectCols + ` FROM release_environments WHERE release_id = $1 ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, releaseID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListReleaseEnvironments: %w", err)
	}
	defer rows.Close()

	var result []*models.ReleaseEnvironment
	for rows.Next() {
		re, err := scanReleaseEnvironment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListReleaseEnvironments: %w", err)
		}
		result = append(result, re)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListReleaseEnvironments: %w", err)
	}
	return result, nil
}

// UpdateReleaseEnvironment persists changes to a release-environment association.
func (r *ReleaseRepository) UpdateReleaseEnvironment(ctx context.Context, re *models.ReleaseEnvironment) error {
	re.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE release_environments SET
			status           = $2,
			lifecycle_status = $3,
			health_score     = $4,
			deployed_at      = $5,
			deployed_by      = $6,
			updated_at       = $7
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		re.ID,
		re.Status,
		re.LifecycleStatus,
		re.HealthScore,
		re.DeployedAt,
		re.DeployedBy,
		re.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateReleaseEnvironment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Query methods
// ---------------------------------------------------------------------------

// GetLatestRelease returns the most recent release for a project and environment
// combination, ordered by creation time descending.
func (r *ReleaseRepository) GetLatestRelease(ctx context.Context, projectID, environmentID uuid.UUID) (*models.Release, error) {
	q := `
		SELECT` + releaseSelectCols + `
		FROM releases rel
		JOIN release_environments re ON re.release_id = rel.id
		WHERE rel.project_id = $1
		  AND re.environment_id = $2
		ORDER BY rel.created_at DESC
		LIMIT 1`

	release, err := scanRelease(r.pool.QueryRow(ctx, q, projectID, environmentID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetLatestRelease: %w", err)
	}
	return release, nil
}

// GetReleaseTimeline returns a chronological timeline of release deployments
// across all environments for a project.
func (r *ReleaseRepository) GetReleaseTimeline(ctx context.Context, projectID uuid.UUID) ([]*models.ReleaseTimeline, error) {
	const q = `
		SELECT
			rel.id,
			rel.version,
			COALESCE(rel.title, ''),
			COALESCE(re.environment_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(re.lifecycle_status, ''),
			re.deployed_at,
			COALESCE(re.health_score, 0)
		FROM releases rel
		JOIN release_environments re ON re.release_id = rel.id
		WHERE rel.project_id = $1
		ORDER BY rel.created_at DESC`

	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetReleaseTimeline: %w", err)
	}
	defer rows.Close()

	var result []*models.ReleaseTimeline
	for rows.Next() {
		var t models.ReleaseTimeline
		if err := rows.Scan(
			&t.ReleaseID,
			&t.Version,
			&t.Title,
			&t.EnvironmentID,
			&t.Status,
			&t.DeployedAt,
			&t.HealthScore,
		); err != nil {
			return nil, fmt.Errorf("postgres.GetReleaseTimeline: %w", err)
		}
		result = append(result, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.GetReleaseTimeline: %w", err)
	}
	return result, nil
}
