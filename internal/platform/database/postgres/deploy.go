package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeployRepository implements deploy.DeployRepository using a PostgreSQL connection pool.
type DeployRepository struct {
	pool *pgxpool.Pool
}

// NewDeployRepository creates a new DeployRepository backed by the given pool.
func NewDeployRepository(pool *pgxpool.Pool) *DeployRepository {
	return &DeployRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Column lists
// ---------------------------------------------------------------------------

const deploymentSelectCols = `
	id, application_id, environment_id, strategy, status,
	artifact, version, COALESCE(commit_sha, ''),
	traffic_percent, created_by, started_at, completed_at,
	created_at, updated_at`

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// scanDeployment reads a single Deployment row from the given pgx.Row.
// The SELECT must include columns in the order defined by deploymentSelectCols.
func scanDeployment(row pgx.Row) (*models.Deployment, error) {
	var d models.Deployment

	err := row.Scan(
		&d.ID,
		&d.ApplicationID,
		&d.EnvironmentID,
		&d.Strategy,
		&d.Status,
		&d.Artifact,
		&d.Version,
		&d.CommitSHA,
		&d.TrafficPercent,
		&d.CreatedBy,
		&d.StartedAt,
		&d.CompletedAt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &d, nil
}

// ---------------------------------------------------------------------------
// Deployment methods
// ---------------------------------------------------------------------------

// CreateDeployment inserts a new deployment record into the database.
func (r *DeployRepository) CreateDeployment(ctx context.Context, d *models.Deployment) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now

	const q = `
		INSERT INTO deployments
			(id, application_id, environment_id, strategy, status,
			 artifact, version, commit_sha,
			 traffic_percent, created_by, started_at, completed_at,
			 created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, $8,
			 $9, $10, $11, $12,
			 $13, $14)`

	_, err := r.pool.Exec(ctx, q,
		d.ID,
		d.ApplicationID,
		d.EnvironmentID,
		d.Strategy,
		d.Status,
		d.Artifact,
		d.Version,
		d.CommitSHA,
		d.TrafficPercent,
		d.CreatedBy,
		d.StartedAt,
		d.CompletedAt,
		d.CreatedAt,
		d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateDeployment: %w", err)
	}
	return nil
}

// GetDeployment retrieves a deployment by its unique identifier.
func (r *DeployRepository) GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	q := `SELECT` + deploymentSelectCols + ` FROM deployments WHERE id = $1`
	d, err := scanDeployment(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetDeployment: %w", err)
	}
	return d, nil
}

// ListDeployments returns deployments for an application, with optional environment and status filters.
func (r *DeployRepository) ListDeployments(ctx context.Context, applicationID uuid.UUID, opts deploy.ListOptions) ([]*models.Deployment, error) {
	var w whereBuilder
	w.Add("application_id = $%d", applicationID)

	if opts.EnvironmentID != nil {
		w.Add("environment_id = $%d", *opts.EnvironmentID)
	}
	if opts.Status != nil {
		w.Add("status = $%d", string(*opts.Status))
	}

	whereClause, args := w.Build()
	pagClause, args := paginationClause(opts.Limit, opts.Offset, args)

	q := `SELECT` + deploymentSelectCols + ` FROM deployments` + whereClause + ` ORDER BY created_at DESC` + pagClause

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListDeployments: %w", err)
	}
	defer rows.Close()

	var result []*models.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListDeployments: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListDeployments: %w", err)
	}
	return result, nil
}

// UpdateDeployment persists status, traffic_percent, started_at, and completed_at changes.
func (r *DeployRepository) UpdateDeployment(ctx context.Context, d *models.Deployment) error {
	d.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE deployments SET
			status          = $2,
			traffic_percent = $3,
			started_at      = $4,
			completed_at    = $5,
			updated_at      = $6
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		d.ID,
		d.Status,
		d.TrafficPercent,
		d.StartedAt,
		d.CompletedAt,
		d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateDeployment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetLatestDeployment returns the most recent deployment for an application and environment.
func (r *DeployRepository) GetLatestDeployment(ctx context.Context, applicationID, environmentID uuid.UUID) (*models.Deployment, error) {
	q := `SELECT` + deploymentSelectCols + `
		FROM deployments
		WHERE application_id = $1 AND environment_id = $2
		ORDER BY created_at DESC
		LIMIT 1`

	d, err := scanDeployment(r.pool.QueryRow(ctx, q, applicationID, environmentID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetLatestDeployment: %w", err)
	}
	return d, nil
}
