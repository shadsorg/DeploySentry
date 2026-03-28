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
	id, application_id, name,
	COALESCE(description, ''),
	session_sticky, COALESCE(sticky_header, ''),
	traffic_percent, status,
	created_by, started_at, completed_at,
	created_at, updated_at`

const releaseFlagChangeSelectCols = `
	id, release_id, flag_id, environment_id,
	previous_value, new_value,
	previous_enabled, new_enabled,
	applied_at, created_at`

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// scanRelease reads a single Release row from the given pgx.Row.
// The SELECT must include columns in the order defined by releaseSelectCols.
func scanRelease(row pgx.Row) (*models.Release, error) {
	var r models.Release
	err := row.Scan(
		&r.ID,
		&r.ApplicationID,
		&r.Name,
		&r.Description,
		&r.SessionSticky,
		&r.StickyHeader,
		&r.TrafficPercent,
		&r.Status,
		&r.CreatedBy,
		&r.StartedAt,
		&r.CompletedAt,
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

// scanReleaseFlagChange reads a single ReleaseFlagChange row from the given pgx.Row.
// The SELECT must include columns in the order defined by releaseFlagChangeSelectCols.
func scanReleaseFlagChange(row pgx.Row) (*models.ReleaseFlagChange, error) {
	var fc models.ReleaseFlagChange
	err := row.Scan(
		&fc.ID,
		&fc.ReleaseID,
		&fc.FlagID,
		&fc.EnvironmentID,
		&fc.PreviousValue,
		&fc.NewValue,
		&fc.PreviousEnabled,
		&fc.NewEnabled,
		&fc.AppliedAt,
		&fc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &fc, nil
}

// ---------------------------------------------------------------------------
// Release methods
// ---------------------------------------------------------------------------

// Create inserts a new release into the database.
func (r *ReleaseRepository) Create(ctx context.Context, release *models.Release) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}
	now := time.Now().UTC()
	release.CreatedAt = now
	release.UpdatedAt = now

	const q = `
		INSERT INTO releases
			(id, application_id, name, description,
			 session_sticky, sticky_header, traffic_percent, status,
			 created_by, started_at, completed_at,
			 created_at, updated_at)
		VALUES
			($1, $2, $3, $4,
			 $5, $6, $7, $8,
			 $9, $10, $11,
			 $12, $13)`

	_, err := r.pool.Exec(ctx, q,
		release.ID,
		release.ApplicationID,
		release.Name,
		release.Description,
		release.SessionSticky,
		release.StickyHeader,
		release.TrafficPercent,
		release.Status,
		release.CreatedBy,
		release.StartedAt,
		release.CompletedAt,
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

// GetByID retrieves a release by its unique identifier.
func (r *ReleaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Release, error) {
	q := `SELECT` + releaseSelectCols + ` FROM releases WHERE id = $1`
	release, err := scanRelease(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetReleaseByID: %w", err)
	}
	return release, nil
}

// ListByApplication returns releases for an application, ordered by creation time descending.
func (r *ReleaseRepository) ListByApplication(ctx context.Context, appID uuid.UUID) ([]models.Release, error) {
	q := `SELECT` + releaseSelectCols + ` FROM releases WHERE application_id = $1 ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListReleasesByApplication: %w", err)
	}
	defer rows.Close()

	var result []models.Release
	for rows.Next() {
		release, err := scanRelease(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListReleasesByApplication: %w", err)
		}
		result = append(result, *release)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListReleasesByApplication: %w", err)
	}
	return result, nil
}

// Update persists changes to an existing release.
func (r *ReleaseRepository) Update(ctx context.Context, release *models.Release) error {
	release.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE releases SET
			name             = $2,
			description      = $3,
			session_sticky   = $4,
			sticky_header    = $5,
			traffic_percent  = $6,
			status           = $7,
			created_by       = $8,
			started_at       = $9,
			completed_at     = $10,
			updated_at       = $11
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		release.ID,
		release.Name,
		release.Description,
		release.SessionSticky,
		release.StickyHeader,
		release.TrafficPercent,
		release.Status,
		release.CreatedBy,
		release.StartedAt,
		release.CompletedAt,
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

// Delete removes a release by its unique identifier.
func (r *ReleaseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM releases WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteRelease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// ReleaseFlagChange methods
// ---------------------------------------------------------------------------

// AddFlagChange persists a new flag change associated with a release.
func (r *ReleaseRepository) AddFlagChange(ctx context.Context, fc *models.ReleaseFlagChange) error {
	if fc.ID == uuid.Nil {
		fc.ID = uuid.New()
	}
	fc.CreatedAt = time.Now().UTC()

	const q = `
		INSERT INTO release_flag_changes
			(id, release_id, flag_id, environment_id,
			 previous_value, new_value,
			 previous_enabled, new_enabled,
			 applied_at, created_at)
		VALUES
			($1, $2, $3, $4,
			 $5, $6,
			 $7, $8,
			 $9, $10)`

	_, err := r.pool.Exec(ctx, q,
		fc.ID,
		fc.ReleaseID,
		fc.FlagID,
		fc.EnvironmentID,
		fc.PreviousValue,
		fc.NewValue,
		fc.PreviousEnabled,
		fc.NewEnabled,
		fc.AppliedAt,
		fc.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.AddFlagChange: %w", err)
	}
	return nil
}

// ListFlagChanges returns the flag changes associated with a release.
func (r *ReleaseRepository) ListFlagChanges(ctx context.Context, releaseID uuid.UUID) ([]models.ReleaseFlagChange, error) {
	q := `SELECT` + releaseFlagChangeSelectCols + ` FROM release_flag_changes WHERE release_id = $1 ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, releaseID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListFlagChanges: %w", err)
	}
	defer rows.Close()

	var result []models.ReleaseFlagChange
	for rows.Next() {
		fc, err := scanReleaseFlagChange(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListFlagChanges: %w", err)
		}
		result = append(result, *fc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListFlagChanges: %w", err)
	}
	return result, nil
}
