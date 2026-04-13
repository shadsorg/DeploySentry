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
	q    querier
	pool *pgxpool.Pool
}

// NewDeployRepository creates a new DeployRepository backed by the given pool.
func NewDeployRepository(pool *pgxpool.Pool) *DeployRepository {
	return &DeployRepository{q: pool, pool: pool}
}

// ---------------------------------------------------------------------------
// Column lists
// ---------------------------------------------------------------------------

const deploymentSelectCols = `
	id, application_id, environment_id, strategy, status,
	artifact, version, COALESCE(commit_sha, ''),
	traffic_percent, previous_deployment_id, created_by, started_at, completed_at,
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
		&d.PreviousDeploymentID,
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
			 traffic_percent, previous_deployment_id, created_by, started_at, completed_at,
			 created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, $8,
			 $9, $10, $11, $12, $13,
			 $14, $15)`

	_, err := r.q.Exec(ctx, q,
		d.ID,
		d.ApplicationID,
		d.EnvironmentID,
		d.Strategy,
		d.Status,
		d.Artifact,
		d.Version,
		d.CommitSHA,
		d.TrafficPercent,
		d.PreviousDeploymentID,
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
	d, err := scanDeployment(r.q.QueryRow(ctx, q, id))
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
	if opts.ExcludeTerminal {
		w.Add("status NOT IN ('completed', 'failed', 'rolled_back', 'cancelled')", nil)
	}

	whereClause, args := w.Build()
	pagClause, args := paginationClause(opts.Limit, opts.Offset, args)

	q := `SELECT` + deploymentSelectCols + ` FROM deployments` + whereClause + ` ORDER BY created_at DESC` + pagClause

	rows, err := r.q.Query(ctx, q, args...)
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

	tag, err := r.q.Exec(ctx, q,
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

	d, err := scanDeployment(r.q.QueryRow(ctx, q, applicationID, environmentID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetLatestDeployment: %w", err)
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Phase column list and scan helper
// ---------------------------------------------------------------------------

const phaseSelectCols = `
	id, deployment_id, name, status, traffic_percent,
	duration_seconds, sort_order, auto_promote,
	started_at, completed_at`

// scanPhase reads a single DeploymentPhase row from the given pgx.Row.
// The SELECT must include columns in the order defined by phaseSelectCols.
func scanPhase(row pgx.Row) (*models.DeploymentPhase, error) {
	var p models.DeploymentPhase
	err := row.Scan(
		&p.ID,
		&p.DeploymentID,
		&p.Name,
		&p.Status,
		&p.TrafficPercent,
		&p.Duration,
		&p.SortOrder,
		&p.AutoPromote,
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

// ---------------------------------------------------------------------------
// Phase methods
// ---------------------------------------------------------------------------

// CreatePhase inserts a new deployment phase record into the database.
func (r *DeployRepository) CreatePhase(ctx context.Context, phase *models.DeploymentPhase) error {
	if phase.ID == uuid.Nil {
		phase.ID = uuid.New()
	}

	const q = `
		INSERT INTO deployment_phases
			(id, deployment_id, name, status, traffic_percent,
			 duration_seconds, sort_order, auto_promote,
			 started_at, completed_at)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, $8,
			 $9, $10)`

	_, err := r.q.Exec(ctx, q,
		phase.ID,
		phase.DeploymentID,
		phase.Name,
		phase.Status,
		phase.TrafficPercent,
		phase.Duration,
		phase.SortOrder,
		phase.AutoPromote,
		phase.StartedAt,
		phase.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreatePhase: %w", err)
	}
	return nil
}

// ListPhases returns all phases for a deployment ordered by sort_order ascending.
func (r *DeployRepository) ListPhases(ctx context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error) {
	q := `SELECT` + phaseSelectCols + `
		FROM deployment_phases
		WHERE deployment_id = $1
		ORDER BY sort_order ASC`

	rows, err := r.q.Query(ctx, q, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListPhases: %w", err)
	}
	defer rows.Close()

	var result []*models.DeploymentPhase
	for rows.Next() {
		p, err := scanPhase(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListPhases: %w", err)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListPhases: %w", err)
	}
	return result, nil
}

// UpdatePhase persists status, started_at, and completed_at changes for a phase.
func (r *DeployRepository) UpdatePhase(ctx context.Context, phase *models.DeploymentPhase) error {
	const q = `
		UPDATE deployment_phases SET
			status       = $2,
			started_at   = $3,
			completed_at = $4
		WHERE id = $1`

	tag, err := r.q.Exec(ctx, q,
		phase.ID,
		phase.Status,
		phase.StartedAt,
		phase.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdatePhase: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetActivePhase returns the currently active phase for a deployment, or ErrNotFound if none.
func (r *DeployRepository) GetActivePhase(ctx context.Context, deploymentID uuid.UUID) (*models.DeploymentPhase, error) {
	q := `SELECT` + phaseSelectCols + `
		FROM deployment_phases
		WHERE deployment_id = $1 AND status = 'active'
		LIMIT 1`

	p, err := scanPhase(r.q.QueryRow(ctx, q, deploymentID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetActivePhase: %w", err)
	}
	return p, nil
}

// GetLatestCompletedDeployment returns the most recent completed deployment for
// an application and environment. Used to populate previous_deployment_id.
func (r *DeployRepository) GetLatestCompletedDeployment(ctx context.Context, applicationID, environmentID uuid.UUID) (*models.Deployment, error) {
	q := `SELECT` + deploymentSelectCols + `
		FROM deployments
		WHERE application_id = $1 AND environment_id = $2 AND status = 'completed'
		ORDER BY completed_at DESC
		LIMIT 1`

	d, err := scanDeployment(r.q.QueryRow(ctx, q, applicationID, environmentID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetLatestCompletedDeployment: %w", err)
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Rollback record column list and scan helper
// ---------------------------------------------------------------------------

const rollbackSelectCols = `
	id, deployment_id, target_deployment_id, reason,
	health_score, automatic, strategy,
	started_at, completed_at, created_at`

// scanRollbackRecord reads a single RollbackRecord row from the given pgx.Row.
func scanRollbackRecord(row pgx.Row) (*models.RollbackRecord, error) {
	var rec models.RollbackRecord
	err := row.Scan(
		&rec.ID,
		&rec.DeploymentID,
		&rec.TargetDeploymentID,
		&rec.Reason,
		&rec.HealthScore,
		&rec.Automatic,
		&rec.Strategy,
		&rec.StartedAt,
		&rec.CompletedAt,
		&rec.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// ---------------------------------------------------------------------------
// Rollback record methods
// ---------------------------------------------------------------------------

// CreateRollbackRecord inserts a new rollback history entry into the database.
func (r *DeployRepository) CreateRollbackRecord(ctx context.Context, record *models.RollbackRecord) error {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	now := time.Now().UTC()
	record.CreatedAt = now

	const q = `
		INSERT INTO rollback_history
			(id, deployment_id, target_deployment_id, reason,
			 health_score, automatic, strategy,
			 started_at, completed_at, created_at)
		VALUES
			($1, $2, $3, $4,
			 $5, $6, $7,
			 $8, $9, $10)`

	_, err := r.q.Exec(ctx, q,
		record.ID,
		record.DeploymentID,
		record.TargetDeploymentID,
		record.Reason,
		record.HealthScore,
		record.Automatic,
		record.Strategy,
		record.StartedAt,
		record.CompletedAt,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateRollbackRecord: %w", err)
	}
	return nil
}

// ListRollbackRecords returns rollback history for a deployment, ordered by created_at DESC.
func (r *DeployRepository) ListRollbackRecords(ctx context.Context, deploymentID uuid.UUID) ([]*models.RollbackRecord, error) {
	q := `SELECT` + rollbackSelectCols + `
		FROM rollback_history
		WHERE deployment_id = $1
		ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListRollbackRecords: %w", err)
	}
	defer rows.Close()

	var result []*models.RollbackRecord
	for rows.Next() {
		rec, err := scanRollbackRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListRollbackRecords: %w", err)
		}
		result = append(result, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListRollbackRecords: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Advisory lock methods
// ---------------------------------------------------------------------------

// advisoryLockKey derives a stable int64 key from the first 8 bytes of a UUID.
func advisoryLockKey(id uuid.UUID) int64 {
	b := id[:]
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}

// TryAdvisoryLock attempts to acquire a PostgreSQL session-level advisory lock
// for the given deployment. Returns true if the lock was acquired, false if it
// is already held by another session.
func (r *DeployRepository) TryAdvisoryLock(ctx context.Context, deploymentID uuid.UUID) (bool, error) {
	key := advisoryLockKey(deploymentID)
	var locked bool
	err := r.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("postgres.TryAdvisoryLock: %w", err)
	}
	return locked, nil
}

// AdvisoryUnlock releases a previously acquired session-level advisory lock.
func (r *DeployRepository) AdvisoryUnlock(ctx context.Context, deploymentID uuid.UUID) error {
	key := advisoryLockKey(deploymentID)
	_, err := r.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	if err != nil {
		return fmt.Errorf("postgres.AdvisoryUnlock: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ListNonTerminalDeployments
// ---------------------------------------------------------------------------

// ListNonTerminalDeployments returns all canary deployments in a non-terminal
// state (pending, running, paused, promoting), ordered by created_at ASC.
func (r *DeployRepository) ListNonTerminalDeployments(ctx context.Context) ([]*models.Deployment, error) {
	q := `SELECT` + deploymentSelectCols + `
		FROM deployments
		WHERE status IN ('pending', 'running', 'paused', 'promoting')
		  AND strategy = 'canary'
		ORDER BY created_at ASC`

	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListNonTerminalDeployments: %w", err)
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListNonTerminalDeployments: scan: %w", err)
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

// ---------------------------------------------------------------------------
// Transaction support
// ---------------------------------------------------------------------------

// WithTx executes fn inside a database transaction. If fn returns an error the
// transaction is rolled back; otherwise it is committed.
func (r *DeployRepository) WithTx(ctx context.Context, fn deploy.TxFunc) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.WithTx: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txRepo := &DeployRepository{q: tx, pool: r.pool}
	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres.WithTx: commit: %w", err)
	}
	return nil
}
