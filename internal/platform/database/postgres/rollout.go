package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/rollout"
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

// StrategyDefaultsRepo is a Postgres-backed rollout.StrategyDefaultRepository.
type StrategyDefaultsRepo struct {
	db *pgxpool.Pool
}

// NewStrategyDefaultsRepo returns a new StrategyDefaultsRepo.
func NewStrategyDefaultsRepo(db *pgxpool.Pool) *StrategyDefaultsRepo {
	return &StrategyDefaultsRepo{db: db}
}

var _ rollout.StrategyDefaultRepository = (*StrategyDefaultsRepo)(nil)

func (r *StrategyDefaultsRepo) Upsert(ctx context.Context, d *models.StrategyDefault) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	d.UpdatedAt = now
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	envStr := ""
	if d.Environment != nil {
		envStr = *d.Environment
	}
	ttStr := ""
	if d.TargetType != nil {
		ttStr = string(*d.TargetType)
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO strategy_defaults (id, scope_type, scope_id, environment, target_type, strategy_id, created_by, updated_by, created_at, updated_at)
        VALUES ($1,$2,$3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, $9, $10)
        ON CONFLICT (scope_type, scope_id, COALESCE(environment,''), COALESCE(target_type,''))
        DO UPDATE SET strategy_id=EXCLUDED.strategy_id, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		d.ID, d.ScopeType, d.ScopeID, envStr, ttStr, d.StrategyID, d.CreatedBy, d.UpdatedBy, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *StrategyDefaultsRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.StrategyDefault, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, scope_type, scope_id, environment, target_type, strategy_id, created_by, updated_by, created_at, updated_at
        FROM strategy_defaults WHERE scope_type=$1 AND scope_id=$2 ORDER BY COALESCE(environment,''), COALESCE(target_type,'')`,
		scopeType, scopeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.StrategyDefault
	for rows.Next() {
		var d models.StrategyDefault
		var env *string
		var tt *models.TargetType
		var createdBy, updatedBy *uuid.UUID
		if err := rows.Scan(&d.ID, &d.ScopeType, &d.ScopeID, &env, &tt, &d.StrategyID, &createdBy, &updatedBy, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Environment = env
		d.TargetType = tt
		d.CreatedBy = createdBy
		d.UpdatedBy = updatedBy
		out = append(out, &d)
	}
	return out, rows.Err()
}

func (r *StrategyDefaultsRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM strategy_defaults WHERE id=$1`, id)
	return err
}

func (r *StrategyDefaultsRepo) DeleteByKey(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID, env *string, target *models.TargetType) error {
	envStr, ttStr := "", ""
	if env != nil {
		envStr = *env
	}
	if target != nil {
		ttStr = string(*target)
	}
	_, err := r.db.Exec(ctx, `
        DELETE FROM strategy_defaults
        WHERE scope_type=$1 AND scope_id=$2 AND COALESCE(environment,'')=$3 AND COALESCE(target_type,'')=$4`,
		scopeType, scopeID, envStr, ttStr,
	)
	return err
}

// RolloutPolicyRepo is a Postgres-backed rollout.RolloutPolicyRepository.
type RolloutPolicyRepo struct {
	db *pgxpool.Pool
}

// NewRolloutPolicyRepo returns a new RolloutPolicyRepo.
func NewRolloutPolicyRepo(db *pgxpool.Pool) *RolloutPolicyRepo {
	return &RolloutPolicyRepo{db: db}
}

var _ rollout.RolloutPolicyRepository = (*RolloutPolicyRepo)(nil)

func (r *RolloutPolicyRepo) Upsert(ctx context.Context, p *models.RolloutPolicy) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.UpdatedAt = now
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	envStr := ""
	if p.Environment != nil {
		envStr = *p.Environment
	}
	ttStr := ""
	if p.TargetType != nil {
		ttStr = string(*p.TargetType)
	}
	_, err := r.db.Exec(ctx, `
        INSERT INTO rollout_policies (id, scope_type, scope_id, environment, target_type, enabled, policy, created_by, updated_by, created_at, updated_at)
        VALUES ($1,$2,$3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, $9, $10, $11)
        ON CONFLICT (scope_type, scope_id, COALESCE(environment,''), COALESCE(target_type,''))
        DO UPDATE SET enabled=EXCLUDED.enabled, policy=EXCLUDED.policy, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		p.ID, p.ScopeType, p.ScopeID, envStr, ttStr, p.Enabled, p.Policy, p.CreatedBy, p.UpdatedBy, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *RolloutPolicyRepo) ListByScope(ctx context.Context, scopeType models.ScopeType, scopeID uuid.UUID) ([]*models.RolloutPolicy, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, scope_type, scope_id, environment, target_type, enabled, policy, created_by, updated_by, created_at, updated_at
        FROM rollout_policies WHERE scope_type=$1 AND scope_id=$2 ORDER BY COALESCE(environment,''), COALESCE(target_type,'')`,
		scopeType, scopeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RolloutPolicy
	for rows.Next() {
		var p models.RolloutPolicy
		var env *string
		var tt *models.TargetType
		var createdBy, updatedBy *uuid.UUID
		if err := rows.Scan(&p.ID, &p.ScopeType, &p.ScopeID, &env, &tt, &p.Enabled, &p.Policy, &createdBy, &updatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Environment = env
		p.TargetType = tt
		p.CreatedBy = createdBy
		p.UpdatedBy = updatedBy
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *RolloutPolicyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM rollout_policies WHERE id=$1`, id)
	return err
}
