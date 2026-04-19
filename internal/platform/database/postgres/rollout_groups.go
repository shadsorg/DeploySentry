package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RolloutGroupRepo is a Postgres-backed rollout.RolloutGroupRepository.
type RolloutGroupRepo struct{ db *pgxpool.Pool }

// NewRolloutGroupRepo returns a new RolloutGroupRepo.
func NewRolloutGroupRepo(db *pgxpool.Pool) *RolloutGroupRepo { return &RolloutGroupRepo{db: db} }

var _ rollout.RolloutGroupRepository = (*RolloutGroupRepo)(nil)

// ErrRolloutGroupNotFound is returned when a group lookup fails.
var ErrRolloutGroupNotFound = errors.New("rollout group not found")

func (r *RolloutGroupRepo) Create(ctx context.Context, g *models.RolloutGroup) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	now := time.Now().UTC()
	g.CreatedAt, g.UpdatedAt = now, now
	if g.CoordinationPolicy == "" {
		g.CoordinationPolicy = models.CoordinationIndependent
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO rollout_groups (id, scope_type, scope_id, name, description, coordination_policy, created_by, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		g.ID, g.ScopeType, g.ScopeID, g.Name, g.Description, g.CoordinationPolicy,
		g.CreatedBy, g.CreatedAt, g.UpdatedAt,
	)
	return err
}

func (r *RolloutGroupRepo) Get(ctx context.Context, id uuid.UUID) (*models.RolloutGroup, error) {
	row := r.db.QueryRow(ctx, selectGroupCols+` FROM rollout_groups WHERE id=$1`, id)
	return scanGroupRow(row)
}

func (r *RolloutGroupRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutGroup, error) {
	rows, err := r.db.Query(ctx, selectGroupCols+` FROM rollout_groups WHERE scope_type=$1 AND scope_id=$2 ORDER BY created_at DESC`, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RolloutGroup
	for rows.Next() {
		g, err := scanGroupRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *RolloutGroupRepo) Update(ctx context.Context, g *models.RolloutGroup) error {
	g.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
        UPDATE rollout_groups SET name=$1, description=$2, coordination_policy=$3, updated_at=$4 WHERE id=$5`,
		g.Name, g.Description, g.CoordinationPolicy, g.UpdatedAt, g.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRolloutGroupNotFound
	}
	return nil
}

func (r *RolloutGroupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM rollout_groups WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRolloutGroupNotFound
	}
	return nil
}

const selectGroupCols = `SELECT id, scope_type, scope_id, name, description, coordination_policy, created_by, created_at, updated_at`

type groupRowScanner interface {
	Scan(dest ...any) error
}

func scanGroupRow(r groupRowScanner) (*models.RolloutGroup, error) {
	var g models.RolloutGroup
	var createdBy *uuid.UUID
	if err := r.Scan(&g.ID, &g.ScopeType, &g.ScopeID, &g.Name, &g.Description, &g.CoordinationPolicy, &createdBy, &g.CreatedAt, &g.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRolloutGroupNotFound
		}
		return nil, err
	}
	g.CreatedBy = createdBy
	return &g, nil
}
