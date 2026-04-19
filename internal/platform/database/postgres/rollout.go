package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StrategyRepo is a Postgres-backed rollout.StrategyRepository.
type StrategyRepo struct {
	db *pgxpool.Pool
}

// NewStrategyRepo returns a new StrategyRepo.
func NewStrategyRepo(db *pgxpool.Pool) *StrategyRepo {
	return &StrategyRepo{db: db}
}

var _ rollout.StrategyRepository = (*StrategyRepo)(nil)

// ErrVersionConflict is returned when Update is called with an expectedVersion
// that does not match the row's current version.
var ErrVersionConflict = errors.New("version conflict")

// ErrStrategyNotFound is returned when a strategy lookup fails.
var ErrStrategyNotFound = errors.New("strategy not found")

func (r *StrategyRepo) Create(ctx context.Context, s *models.Strategy) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	s.CreatedAt, s.UpdatedAt = now, now
	s.Version = 1
	stepsJSON, err := json.Marshal(s.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	_, err = r.db.Exec(ctx, `
        INSERT INTO strategies (
            id, scope_type, scope_id, name, description, target_type, steps,
            default_health_threshold, default_rollback_on_failure,
            version, is_system, created_by, updated_by, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		s.ID, s.ScopeType, s.ScopeID, s.Name, s.Description, s.TargetType, stepsJSON,
		s.DefaultHealthThreshold, s.DefaultRollbackOnFailure,
		s.Version, s.IsSystem, s.CreatedBy, s.UpdatedBy, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *StrategyRepo) Get(ctx context.Context, id uuid.UUID) (*models.Strategy, error) {
	return r.scanOne(ctx, `WHERE id=$1 AND deleted_at IS NULL`, id)
}

func (r *StrategyRepo) GetByName(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, name string) (*models.Strategy, error) {
	return r.scanOne(ctx, `WHERE scope_type=$1 AND scope_id=$2 AND name=$3 AND deleted_at IS NULL`, scopeType, scopeID, name)
}

func (r *StrategyRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.Strategy, error) {
	return r.scanMany(ctx, `WHERE scope_type=$1 AND scope_id=$2 AND deleted_at IS NULL ORDER BY name`, scopeType, scopeID)
}

func (r *StrategyRepo) ListByAnyScope(ctx context.Context, refs []rollout.ScopeRef) ([]*models.Strategy, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	types := make([]string, 0, len(refs))
	ids := make([]uuid.UUID, 0, len(refs))
	for _, ref := range refs {
		types = append(types, string(ref.Type))
		ids = append(ids, ref.ID)
	}
	return r.scanMany(ctx, `
        WHERE (scope_type, scope_id) IN (SELECT unnest($1::text[]), unnest($2::uuid[]))
          AND deleted_at IS NULL
        ORDER BY scope_type, name`, types, ids)
}

func (r *StrategyRepo) Update(ctx context.Context, s *models.Strategy, expected int) error {
	stepsJSON, err := json.Marshal(s.Steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	s.UpdatedAt = time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
        UPDATE strategies SET
            description=$1, target_type=$2, steps=$3,
            default_health_threshold=$4, default_rollback_on_failure=$5,
            updated_by=$6, updated_at=$7, version=version+1
        WHERE id=$8 AND version=$9 AND deleted_at IS NULL`,
		s.Description, s.TargetType, stepsJSON,
		s.DefaultHealthThreshold, s.DefaultRollbackOnFailure,
		s.UpdatedBy, s.UpdatedAt, s.ID, expected,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	s.Version = expected + 1
	return nil
}

func (r *StrategyRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE strategies SET deleted_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStrategyNotFound
	}
	return nil
}

func (r *StrategyRepo) IsReferenced(ctx context.Context, id uuid.UUID) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM strategy_defaults WHERE strategy_id=$1`, id).Scan(&n)
	return n > 0, err
}

func (r *StrategyRepo) scanOne(ctx context.Context, where string, args ...any) (*models.Strategy, error) {
	rows, err := r.db.Query(ctx, selectStrategyCols+" FROM strategies "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanStrategies(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrStrategyNotFound
	}
	return list[0], nil
}

func (r *StrategyRepo) scanMany(ctx context.Context, where string, args ...any) ([]*models.Strategy, error) {
	rows, err := r.db.Query(ctx, selectStrategyCols+" FROM strategies "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStrategies(rows)
}

const selectStrategyCols = `SELECT
    id, scope_type, scope_id, name, description, target_type, steps,
    default_health_threshold, default_rollback_on_failure,
    version, is_system, created_by, updated_by, created_at, updated_at`

func scanStrategies(rows pgx.Rows) ([]*models.Strategy, error) {
	var out []*models.Strategy
	for rows.Next() {
		var s models.Strategy
		var stepsJSON []byte
		var createdBy, updatedBy *uuid.UUID
		if err := rows.Scan(
			&s.ID, &s.ScopeType, &s.ScopeID, &s.Name, &s.Description, &s.TargetType, &stepsJSON,
			&s.DefaultHealthThreshold, &s.DefaultRollbackOnFailure,
			&s.Version, &s.IsSystem, &createdBy, &updatedBy, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(stepsJSON, &s.Steps); err != nil {
			return nil, fmt.Errorf("decode steps: %w", err)
		}
		s.CreatedBy = createdBy
		s.UpdatedBy = updatedBy
		out = append(out, &s)
	}
	return out, rows.Err()
}
