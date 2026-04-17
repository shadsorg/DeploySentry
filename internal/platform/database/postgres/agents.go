package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/agent/registry"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ registry.Repository = (*AgentRepository)(nil)

// AgentRepository implements registry.Repository using a PostgreSQL connection pool.
type AgentRepository struct {
	pool *pgxpool.Pool
}

// NewAgentRepository creates a new AgentRepository backed by the given pool.
func NewAgentRepository(pool *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Agent CRUD
// ---------------------------------------------------------------------------

func (r *AgentRepository) CreateAgent(ctx context.Context, a *models.Agent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agents (id, app_id, environment_id, status, version, upstream_config, last_seen_at, registered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.AppID, a.EnvironmentID, a.Status, a.Version, a.UpstreamConfig, a.LastSeenAt, a.RegisteredAt,
	)
	return err
}

func (r *AgentRepository) GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, app_id, environment_id, status, version, upstream_config, last_seen_at, registered_at
		FROM agents WHERE id = $1`, id)

	var a models.Agent
	err := row.Scan(&a.ID, &a.AppID, &a.EnvironmentID, &a.Status, &a.Version, &a.UpstreamConfig, &a.LastSeenAt, &a.RegisteredAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepository) ListAgentsByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, app_id, environment_id, status, version, upstream_config, last_seen_at, registered_at
		FROM agents WHERE app_id = $1 ORDER BY registered_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.AppID, &a.EnvironmentID, &a.Status, &a.Version, &a.UpstreamConfig, &a.LastSeenAt, &a.RegisteredAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (r *AgentRepository) UpdateAgentStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error {
	tag, err := r.pool.Exec(ctx, `UPDATE agents SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AgentRepository) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE agents SET last_seen_at = NOW(), status = 'connected' WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AgentRepository) DeleteAgent(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AgentRepository) MarkStaleAgents(ctx context.Context, staleDuration, disconnectDuration int) error {
	// Mark disconnected first (longer threshold), then stale (shorter threshold).
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE agents SET status = 'disconnected'
		WHERE status != 'disconnected'
		  AND last_seen_at < NOW() - INTERVAL '%d seconds'`, disconnectDuration))
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE agents SET status = 'stale'
		WHERE status NOT IN ('disconnected', 'stale')
		  AND last_seen_at < NOW() - INTERVAL '%d seconds'`, staleDuration))
	return err
}

// ---------------------------------------------------------------------------
// Heartbeats
// ---------------------------------------------------------------------------

func (r *AgentRepository) InsertHeartbeat(ctx context.Context, hb *models.AgentHeartbeat) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_heartbeats (id, agent_id, deployment_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		hb.ID, hb.AgentID, hb.DeploymentID, hb.Payload, hb.CreatedAt,
	)
	return err
}

func (r *AgentRepository) ListHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error) {
	var rows pgx.Rows
	var err error

	if deploymentID != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT id, agent_id, deployment_id, payload, created_at
			FROM agent_heartbeats
			WHERE agent_id = $1 AND deployment_id = $2
			ORDER BY created_at DESC LIMIT $3`, agentID, *deploymentID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, agent_id, deployment_id, payload, created_at
			FROM agent_heartbeats
			WHERE agent_id = $1
			ORDER BY created_at DESC LIMIT $2`, agentID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var heartbeats []models.AgentHeartbeat
	for rows.Next() {
		var hb models.AgentHeartbeat
		if err := rows.Scan(&hb.ID, &hb.AgentID, &hb.DeploymentID, &hb.Payload, &hb.CreatedAt); err != nil {
			return nil, err
		}
		heartbeats = append(heartbeats, hb)
	}
	return heartbeats, rows.Err()
}

func (r *AgentRepository) PruneHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, keep int) error {
	if deploymentID != nil {
		_, err := r.pool.Exec(ctx, `
			DELETE FROM agent_heartbeats
			WHERE agent_id = $1 AND deployment_id = $2
			  AND id NOT IN (
			    SELECT id FROM agent_heartbeats
			    WHERE agent_id = $1 AND deployment_id = $2
			    ORDER BY created_at DESC LIMIT $3
			  )`, agentID, *deploymentID, keep)
		return err
	}

	_, err := r.pool.Exec(ctx, `
		DELETE FROM agent_heartbeats
		WHERE agent_id = $1
		  AND id NOT IN (
		    SELECT id FROM agent_heartbeats
		    WHERE agent_id = $1
		    ORDER BY created_at DESC LIMIT $2
		  )`, agentID, keep)
	return err
}
