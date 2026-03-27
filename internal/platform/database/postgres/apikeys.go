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

// APIKeyRepository implements auth.APIKeyRepository using a PostgreSQL connection pool.
type APIKeyRepository struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepository creates a new APIKeyRepository backed by the given pool.
func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// apiKeySelectCols lists the columns returned for an API key row.
// Scopes are cast to text[] for scanning into a []string.
const apiKeySelectCols = `
	id, org_id,
	COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid),
	name, key_prefix, key_hash,
	scopes::text[],
	expires_at, last_used_at, created_by, created_at, revoked_at`

// scanAPIKey reads a single APIKey row. Because project_id is nullable we
// scan it into a plain uuid.UUID and convert back to *uuid.UUID afterwards.
func scanAPIKey(row pgx.Row) (*models.APIKey, error) {
	var k models.APIKey
	var projectID uuid.UUID
	var scopeStrings []string

	err := row.Scan(
		&k.ID,
		&k.OrgID,
		&projectID,
		&k.Name,
		&k.KeyPrefix,
		&k.KeyHash,
		&scopeStrings,
		&k.ExpiresAt,
		&k.LastUsedAt,
		&k.CreatedBy,
		&k.CreatedAt,
		&k.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Convert sentinel nil-UUID back to a nil pointer.
	if projectID != uuid.Nil {
		pid := projectID
		k.ProjectID = &pid
	}

	// Convert []string → []models.APIKeyScope.
	k.Scopes = make([]models.APIKeyScope, len(scopeStrings))
	for i, s := range scopeStrings {
		k.Scopes[i] = models.APIKeyScope(s)
	}

	return &k, nil
}

// ---------------------------------------------------------------------------
// APIKeyRepository methods
// ---------------------------------------------------------------------------

// CreateAPIKey inserts a new API key record.
func (r *APIKeyRepository) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now().UTC()
	}

	// Convert []models.APIKeyScope → []string for the TEXT[] column.
	scopeStrings := make([]string, len(key.Scopes))
	for i, s := range key.Scopes {
		scopeStrings[i] = string(s)
	}

	const q = `
		INSERT INTO api_keys
			(id, org_id, project_id, name, key_prefix, key_hash, scopes,
			 expires_at, last_used_at, created_by, created_at, revoked_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.pool.Exec(ctx, q,
		key.ID,
		key.OrgID,
		key.ProjectID,
		key.Name,
		key.KeyPrefix,
		key.KeyHash,
		scopeStrings,
		key.ExpiresAt,
		key.LastUsedAt,
		key.CreatedBy,
		key.CreatedAt,
		key.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateAPIKey: %w", err)
	}
	return nil
}

// GetAPIKey retrieves an API key by its ID.
func (r *APIKeyRepository) GetAPIKey(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	q := `SELECT` + apiKeySelectCols + ` FROM api_keys WHERE id = $1`
	key, err := scanAPIKey(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetAPIKey: %w", err)
	}
	return key, nil
}

// GetAPIKeyByPrefix retrieves a non-revoked API key by its prefix.
func (r *APIKeyRepository) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	q := `SELECT` + apiKeySelectCols + ` FROM api_keys WHERE key_prefix = $1 AND revoked_at IS NULL`
	key, err := scanAPIKey(r.pool.QueryRow(ctx, q, prefix))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetAPIKeyByPrefix: %w", err)
	}
	return key, nil
}

// ListAPIKeys returns API keys for an organization, optionally filtered by project.
func (r *APIKeyRepository) ListAPIKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.APIKey, error) {
	var wb whereBuilder
	wb.Add("org_id = $%d", orgID)
	if projectID != nil {
		wb.Add("project_id = $%d", *projectID)
	}
	where, args := wb.Build()

	pagClause, args := paginationClause(limit, offset, args)
	q := `SELECT` + apiKeySelectCols + ` FROM api_keys` + where + ` ORDER BY created_at DESC` + pagClause

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAPIKeys: %w", err)
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListAPIKeys: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListAPIKeys: %w", err)
	}
	return keys, nil
}

// UpdateAPIKey persists changes to an API key's name, scopes, expires_at, and revoked_at.
func (r *APIKeyRepository) UpdateAPIKey(ctx context.Context, key *models.APIKey) error {
	// Convert []models.APIKeyScope → []string for the TEXT[] column.
	scopeStrings := make([]string, len(key.Scopes))
	for i, s := range key.Scopes {
		scopeStrings[i] = string(s)
	}

	const q = `
		UPDATE api_keys SET
			name       = $2,
			scopes     = $3,
			expires_at = $4,
			revoked_at = $5
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		key.ID,
		key.Name,
		scopeStrings,
		key.ExpiresAt,
		key.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateAPIKey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAPIKey soft-deletes an API key by setting its revoked_at timestamp.
func (r *APIKeyRepository) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	const q = `UPDATE api_keys SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id, now)
	if err != nil {
		return fmt.Errorf("postgres.DeleteAPIKey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp for an API key.
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	const q = `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, usedAt)
	if err != nil {
		return fmt.Errorf("postgres.UpdateLastUsed: %w", err)
	}
	return nil
}
