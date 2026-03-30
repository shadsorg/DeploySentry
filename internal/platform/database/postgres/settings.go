package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const settingCols = `id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at`

// SettingRepository implements settings.SettingRepository using PostgreSQL.
type SettingRepository struct {
	pool *pgxpool.Pool
}

// NewSettingRepository creates a new SettingRepository backed by the given pool.
func NewSettingRepository(pool *pgxpool.Pool) *SettingRepository {
	return &SettingRepository{pool: pool}
}

func scanSetting(row pgx.Row) (*models.Setting, error) {
	var s models.Setting
	err := row.Scan(
		&s.ID, &s.OrgID, &s.ProjectID, &s.ApplicationID, &s.EnvironmentID,
		&s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func scanSettings(rows pgx.Rows) ([]*models.Setting, error) {
	result := make([]*models.Setting, 0)
	for rows.Next() {
		var s models.Setting
		if err := rows.Scan(
			&s.ID, &s.OrgID, &s.ProjectID, &s.ApplicationID, &s.EnvironmentID,
			&s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// Create inserts a new setting. Returns ErrConflict on unique-index violation.
func (r *SettingRepository) Create(ctx context.Context, setting *models.Setting) error {
	now := time.Now().UTC()
	setting.UpdatedAt = now
	if setting.ID == uuid.Nil {
		setting.ID = uuid.New()
	}

	q := `INSERT INTO settings (` + settingCols + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		setting.ID, setting.OrgID, setting.ProjectID, setting.ApplicationID, setting.EnvironmentID,
		setting.Key, setting.Value, setting.UpdatedBy, setting.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateSetting: %w", err)
	}
	return nil
}

// GetByID retrieves a single setting by primary key.
func (r *SettingRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Setting, error) {
	q := `SELECT ` + settingCols + ` FROM settings WHERE id = $1`
	s, err := scanSetting(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("postgres.GetSettingByID: %w", err)
	}
	return s, nil
}

// ListByScope returns all settings for a given scope and target ID.
func (r *SettingRepository) ListByScope(ctx context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error) {
	var col string
	switch scope {
	case "org":
		col = "org_id"
	case "project":
		col = "project_id"
	case "application":
		col = "application_id"
	case "environment":
		col = "environment_id"
	default:
		return nil, fmt.Errorf("postgres.ListSettingsByScope: invalid scope %q", scope)
	}

	q := fmt.Sprintf(`SELECT %s FROM settings WHERE %s = $1 ORDER BY key`, settingCols, col)
	rows, err := r.pool.Query(ctx, q, targetID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListSettingsByScope: %w", err)
	}
	defer rows.Close()

	settings, err := scanSettings(rows)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListSettingsByScope: %w", err)
	}
	return settings, nil
}

// Resolve finds the most specific setting for a key using hierarchical resolution
// (environment > application > project > org).
func (r *SettingRepository) Resolve(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error) {
	if envID != nil {
		q := `SELECT ` + settingCols + ` FROM settings WHERE key = $1 AND environment_id = $2`
		s, err := scanSetting(r.pool.QueryRow(ctx, q, key, envID))
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("postgres.ResolveSetting: %w", err)
		}
	}
	if appID != nil {
		q := `SELECT ` + settingCols + ` FROM settings WHERE key = $1 AND application_id = $2`
		s, err := scanSetting(r.pool.QueryRow(ctx, q, key, appID))
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("postgres.ResolveSetting: %w", err)
		}
	}
	if projectID != nil {
		q := `SELECT ` + settingCols + ` FROM settings WHERE key = $1 AND project_id = $2`
		s, err := scanSetting(r.pool.QueryRow(ctx, q, key, projectID))
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("postgres.ResolveSetting: %w", err)
		}
	}
	if orgID != nil {
		q := `SELECT ` + settingCols + ` FROM settings WHERE key = $1 AND org_id = $2`
		s, err := scanSetting(r.pool.QueryRow(ctx, q, key, orgID))
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("postgres.ResolveSetting: %w", err)
		}
	}
	return nil, ErrNotFound
}

// Upsert inserts or updates a setting based on its scope and key.
func (r *SettingRepository) Upsert(ctx context.Context, setting *models.Setting) error {
	now := time.Now().UTC()
	setting.UpdatedAt = now
	if setting.ID == uuid.Nil {
		setting.ID = uuid.New()
	}

	var conflictCol string
	switch setting.ScopeLevel() {
	case "org":
		conflictCol = "org_id"
	case "project":
		conflictCol = "project_id"
	case "application":
		conflictCol = "application_id"
	case "environment":
		conflictCol = "environment_id"
	default:
		return fmt.Errorf("postgres.UpsertSetting: no scope set")
	}

	q := fmt.Sprintf(`
		INSERT INTO settings (%s)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (%s, key) WHERE %s IS NOT NULL
		DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
		RETURNING id`, settingCols, conflictCol, conflictCol)

	err := r.pool.QueryRow(ctx, q,
		setting.ID, setting.OrgID, setting.ProjectID, setting.ApplicationID, setting.EnvironmentID,
		setting.Key, setting.Value, setting.UpdatedBy, setting.UpdatedAt,
	).Scan(&setting.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpsertSetting: %w", err)
	}
	return nil
}

// Delete removes a setting by ID. Returns ErrNotFound if no rows were affected.
func (r *SettingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM settings WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteSetting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
