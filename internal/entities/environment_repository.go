package entities

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgEnvironment represents an org-level environment.
type OrgEnvironment struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	IsProduction bool      `json:"is_production"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AppEnvironmentOverride represents an app-level environment override.
type AppEnvironmentOverride struct {
	ID            uuid.UUID              `json:"id"`
	AppID         uuid.UUID              `json:"app_id"`
	EnvironmentID uuid.UUID              `json:"environment_id"`
	Config        map[string]interface{} `json:"config"`
	CreatedAt     time.Time              `json:"created_at"`
}

// EnvironmentRepository handles environment persistence.
type EnvironmentRepository struct {
	pool *pgxpool.Pool
}

// NewEnvironmentRepository creates a new EnvironmentRepository.
func NewEnvironmentRepository(pool *pgxpool.Pool) *EnvironmentRepository {
	return &EnvironmentRepository{pool: pool}
}

func (r *EnvironmentRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]OrgEnvironment, error) {
	const q = `
		SELECT id, org_id, name, slug, is_production, sort_order, created_at, updated_at
		FROM environments WHERE org_id = $1 ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("EnvironmentRepository.ListByOrg: %w", err)
	}
	defer rows.Close()

	result := make([]OrgEnvironment, 0)
	for rows.Next() {
		var env OrgEnvironment
		if err := rows.Scan(
			&env.ID,
			&env.OrgID,
			&env.Name,
			&env.Slug,
			&env.IsProduction,
			&env.SortOrder,
			&env.CreatedAt,
			&env.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("EnvironmentRepository.ListByOrg: %w", err)
		}
		result = append(result, env)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("EnvironmentRepository.ListByOrg: %w", err)
	}
	return result, nil
}

// GetByID looks up an environment by UUID. Returns (nil, nil) when not found.
func (r *EnvironmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*OrgEnvironment, error) {
	const q = `
		SELECT id, org_id, name, slug, is_production, sort_order, created_at, updated_at
		FROM environments WHERE id = $1`

	var env OrgEnvironment
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&env.ID,
		&env.OrgID,
		&env.Name,
		&env.Slug,
		&env.IsProduction,
		&env.SortOrder,
		&env.CreatedAt,
		&env.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("EnvironmentRepository.GetByID: %w", err)
	}
	return &env, nil
}

func (r *EnvironmentRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*OrgEnvironment, error) {
	const q = `
		SELECT id, org_id, name, slug, is_production, sort_order, created_at, updated_at
		FROM environments WHERE org_id = $1 AND slug = $2`

	var env OrgEnvironment
	err := r.pool.QueryRow(ctx, q, orgID, slug).Scan(
		&env.ID,
		&env.OrgID,
		&env.Name,
		&env.Slug,
		&env.IsProduction,
		&env.SortOrder,
		&env.CreatedAt,
		&env.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("EnvironmentRepository.GetBySlug: %w", err)
	}
	return &env, nil
}

// ResolveEnvironmentSlug looks up an environment by org and slug, returning its UUID.
func (r *EnvironmentRepository) ResolveEnvironmentSlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error) {
	env, err := r.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return uuid.Nil, err
	}
	if env == nil {
		return uuid.Nil, fmt.Errorf("environment not found: %s", slug)
	}
	return env.ID, nil
}

func (r *EnvironmentRepository) Create(ctx context.Context, env *OrgEnvironment) error {
	env.ID = uuid.New()
	now := time.Now().UTC()
	env.CreatedAt = now
	env.UpdatedAt = now

	const q = `
		INSERT INTO environments (id, org_id, name, slug, is_production, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		env.ID, env.OrgID, env.Name, env.Slug, env.IsProduction, env.SortOrder, env.CreatedAt, env.UpdatedAt)
	if err != nil {
		return fmt.Errorf("EnvironmentRepository.Create: %w", err)
	}
	return nil
}

func (r *EnvironmentRepository) Update(ctx context.Context, env *OrgEnvironment) error {
	env.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE environments SET name = $1, slug = $2, is_production = $3, sort_order = $4, updated_at = $5
		WHERE id = $6`

	tag, err := r.pool.Exec(ctx, q,
		env.Name, env.Slug, env.IsProduction, env.SortOrder, env.UpdatedAt, env.ID)
	if err != nil {
		return fmt.Errorf("EnvironmentRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("EnvironmentRepository.Update: no rows affected")
	}
	return nil
}

func (r *EnvironmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM environments WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("EnvironmentRepository.Delete: %w", err)
	}
	return nil
}
