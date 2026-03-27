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
	id, project_id, environment, pipeline_id, release_id,
	strategy, status, artifact, version, COALESCE(commit_sha, ''),
	traffic_percent, initiated_by, started_at, completed_at,
	created_at, updated_at`

const phaseSelectCols = `
	id, deployment_id, COALESCE(name, ''), status, traffic_pct,
	COALESCE(duration_secs, 0), COALESCE(sort_order, 0), started_at, completed_at`

const pipelineSelectCols = `
	id, project_id, COALESCE(name, ''), created_at, updated_at`

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// scanDeployment reads a single Deployment row from the given pgx.Row.
// The SELECT must include columns in the order defined by deploymentSelectCols.
func scanDeployment(row pgx.Row) (*models.Deployment, error) {
	var d models.Deployment
	var envStr string

	err := row.Scan(
		&d.ID,
		&d.ProjectID,
		&envStr,
		&d.PipelineID,
		&d.ReleaseID,
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

	// environment column stores the UUID as a string; parse it back.
	if parsed, parseErr := uuid.Parse(envStr); parseErr == nil {
		d.EnvironmentID = parsed
	}

	return &d, nil
}

// scanDeploymentPhase reads a single DeploymentPhase row from the given pgx.Row.
// The SELECT must include columns in the order defined by phaseSelectCols.
func scanDeploymentPhase(row pgx.Row) (*models.DeploymentPhase, error) {
	var p models.DeploymentPhase

	err := row.Scan(
		&p.ID,
		&p.DeploymentID,
		&p.Name,
		&p.Status,
		&p.TrafficPercent,
		&p.Duration,
		&p.SortOrder,
		&p.StartedAt,
		&p.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &p, nil
}

// scanPipeline reads a single DeployPipeline row from the given pgx.Row.
// The SELECT must include columns in the order defined by pipelineSelectCols.
func scanPipeline(row pgx.Row) (*models.DeployPipeline, error) {
	var p models.DeployPipeline

	err := row.Scan(
		&p.ID,
		&p.ProjectID,
		&p.Name,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &p, nil
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
			(id, project_id, environment, pipeline_id, release_id,
			 strategy, status, artifact, version, commit_sha,
			 traffic_percent, initiated_by, started_at, completed_at,
			 created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, $8, $9, $10,
			 $11, $12, $13, $14,
			 $15, $16)`

	_, err := r.pool.Exec(ctx, q,
		d.ID,
		d.ProjectID,
		d.EnvironmentID.String(),
		d.PipelineID,
		d.ReleaseID,
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

// ListDeployments returns deployments for a project, with optional environment and status filters.
func (r *DeployRepository) ListDeployments(ctx context.Context, projectID uuid.UUID, opts deploy.ListOptions) ([]*models.Deployment, error) {
	var w whereBuilder
	w.Add("project_id = $%d", projectID)

	if opts.EnvironmentID != nil {
		w.Add("environment = $%d", opts.EnvironmentID.String())
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

// GetLatestDeployment returns the most recent deployment for a project and environment.
func (r *DeployRepository) GetLatestDeployment(ctx context.Context, projectID, environmentID uuid.UUID) (*models.Deployment, error) {
	q := `SELECT` + deploymentSelectCols + `
		FROM deployments
		WHERE project_id = $1 AND environment = $2
		ORDER BY created_at DESC
		LIMIT 1`

	d, err := scanDeployment(r.pool.QueryRow(ctx, q, projectID, environmentID.String()))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetLatestDeployment: %w", err)
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// DeploymentPhase methods
// ---------------------------------------------------------------------------

// ListDeploymentPhases returns the ordered phases for a deployment.
func (r *DeployRepository) ListDeploymentPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error) {
	q := `SELECT` + phaseSelectCols + ` FROM deployment_phases WHERE deployment_id = $1 ORDER BY sort_order ASC`

	rows, err := r.pool.Query(ctx, q, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListDeploymentPhases: %w", err)
	}
	defer rows.Close()

	var result []*models.DeploymentPhase
	for rows.Next() {
		p, err := scanDeploymentPhase(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListDeploymentPhases: %w", err)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListDeploymentPhases: %w", err)
	}
	return result, nil
}

// CreateDeploymentPhase inserts a new deployment phase record.
func (r *DeployRepository) CreateDeploymentPhase(ctx context.Context, phase *models.DeploymentPhase) error {
	if phase.ID == uuid.Nil {
		phase.ID = uuid.New()
	}

	const q = `
		INSERT INTO deployment_phases
			(id, deployment_id, name, status, traffic_pct, duration_secs, sort_order,
			 started_at, completed_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		phase.ID,
		phase.DeploymentID,
		phase.Name,
		phase.Status,
		phase.TrafficPercent,
		phase.Duration,
		phase.SortOrder,
		phase.StartedAt,
		phase.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateDeploymentPhase: %w", err)
	}
	return nil
}

// UpdateDeploymentPhase persists status, started_at, and completed_at changes to a phase.
func (r *DeployRepository) UpdateDeploymentPhase(ctx context.Context, phase *models.DeploymentPhase) error {
	const q = `
		UPDATE deployment_phases SET
			status       = $2,
			started_at   = $3,
			completed_at = $4
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		phase.ID,
		phase.Status,
		phase.StartedAt,
		phase.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateDeploymentPhase: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Pipeline methods
// ---------------------------------------------------------------------------

// GetPipeline retrieves a deploy pipeline by ID.
func (r *DeployRepository) GetPipeline(ctx context.Context, id uuid.UUID) (*models.DeployPipeline, error) {
	q := `SELECT` + pipelineSelectCols + ` FROM deploy_pipelines WHERE id = $1`
	p, err := scanPipeline(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetPipeline: %w", err)
	}
	return p, nil
}
