package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shadsorg/deploysentry/internal/models"
)

// StagedChangesRepository implements staging.Repository against PostgreSQL.
type StagedChangesRepository struct {
	pool *pgxpool.Pool
}

// NewStagedChangesRepository builds a repository backed by pool.
func NewStagedChangesRepository(pool *pgxpool.Pool) *StagedChangesRepository {
	return &StagedChangesRepository{pool: pool}
}

// Upsert writes a staged row, collapsing repeats per the unique index in
// migration 061. The upsert key is (user_id, org_id, resource_type, COALESCE(
// resource_id, provisional_id, sentinel), COALESCE(field_path, '')).
func (r *StagedChangesRepository) Upsert(ctx context.Context, row *models.StagedChange) error {
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	now := time.Now().UTC()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now

	const q = `
		INSERT INTO staged_changes (
			id, user_id, org_id, resource_type, resource_id, provisional_id,
			action, field_path, old_value, new_value, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $12
		)
		ON CONFLICT (
			user_id, org_id, resource_type,
			COALESCE(resource_id, provisional_id, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(field_path, '')
		) DO UPDATE SET
			action     = EXCLUDED.action,
			old_value  = EXCLUDED.old_value,
			new_value  = EXCLUDED.new_value,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at`

	return r.pool.QueryRow(ctx, q,
		row.ID, row.UserID, row.OrgID, row.ResourceType,
		nullableUUID(row.ResourceID), nullableUUID(row.ProvisionalID),
		row.Action, nullableString(row.FieldPath),
		nullableJSON(row.OldValue), nullableJSON(row.NewValue),
		row.CreatedAt, row.UpdatedAt,
	).Scan(&row.ID, &row.CreatedAt, &row.UpdatedAt)
}

// ListForUser returns the user's staged changes within an org, newest first.
func (r *StagedChangesRepository) ListForUser(ctx context.Context, userID, orgID uuid.UUID) ([]*models.StagedChange, error) {
	const q = `
		SELECT id, user_id, org_id, resource_type, resource_id, provisional_id,
		       action, COALESCE(field_path, ''),
		       COALESCE(old_value::text, ''), COALESCE(new_value::text, ''),
		       created_at, updated_at
		FROM staged_changes
		WHERE user_id = $1 AND org_id = $2
		ORDER BY created_at DESC`
	return r.queryRows(ctx, q, userID, orgID)
}

// ListForResource returns staged rows for a single resource type the user has
// pending in the given org.
func (r *StagedChangesRepository) ListForResource(ctx context.Context, userID, orgID uuid.UUID, resourceType string) ([]*models.StagedChange, error) {
	const q = `
		SELECT id, user_id, org_id, resource_type, resource_id, provisional_id,
		       action, COALESCE(field_path, ''),
		       COALESCE(old_value::text, ''), COALESCE(new_value::text, ''),
		       created_at, updated_at
		FROM staged_changes
		WHERE user_id = $1 AND org_id = $2 AND resource_type = $3
		ORDER BY created_at ASC`
	return r.queryRows(ctx, q, userID, orgID, resourceType)
}

// GetByIDs is the commit-endpoint guard: only rows owned by (userID, orgID)
// are returned. A caller submitting another user's ids gets nothing back and
// the commit endpoint returns 404 / 403 accordingly.
func (r *StagedChangesRepository) GetByIDs(ctx context.Context, userID, orgID uuid.UUID, ids []uuid.UUID) ([]*models.StagedChange, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const q = `
		SELECT id, user_id, org_id, resource_type, resource_id, provisional_id,
		       action, COALESCE(field_path, ''),
		       COALESCE(old_value::text, ''), COALESCE(new_value::text, ''),
		       created_at, updated_at
		FROM staged_changes
		WHERE user_id = $1 AND org_id = $2 AND id = ANY($3)`
	return r.queryRows(ctx, q, userID, orgID, ids)
}

// DeleteByIDsTx removes rows owned by (userID, orgID) inside tx.
func (r *StagedChangesRepository) DeleteByIDsTx(ctx context.Context, tx pgx.Tx, userID, orgID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `DELETE FROM staged_changes WHERE user_id = $1 AND org_id = $2 AND id = ANY($3)`
	_, err := tx.Exec(ctx, q, userID, orgID, ids)
	if err != nil {
		return fmt.Errorf("postgres.DeleteByIDsTx: %w", err)
	}
	return nil
}

// DeleteAllForUser drops every staged row owned by the user in the given org.
func (r *StagedChangesRepository) DeleteAllForUser(ctx context.Context, userID, orgID uuid.UUID) (int64, error) {
	const q = `DELETE FROM staged_changes WHERE user_id = $1 AND org_id = $2`
	tag, err := r.pool.Exec(ctx, q, userID, orgID)
	if err != nil {
		return 0, fmt.Errorf("postgres.DeleteAllForUser: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteOlderThan implements the 30-day sweeper.
func (r *StagedChangesRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const q = `DELETE FROM staged_changes WHERE created_at < $1`
	tag, err := r.pool.Exec(ctx, q, cutoff)
	if err != nil {
		return 0, fmt.Errorf("postgres.DeleteOlderThan: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CountForUser returns the total pending row count for the (user, org) pair.
func (r *StagedChangesRepository) CountForUser(ctx context.Context, userID, orgID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM staged_changes WHERE user_id = $1 AND org_id = $2`
	var n int
	if err := r.pool.QueryRow(ctx, q, userID, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres.CountForUser: %w", err)
	}
	return n, nil
}

func (r *StagedChangesRepository) queryRows(ctx context.Context, q string, args ...any) ([]*models.StagedChange, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.queryRows: %w", err)
	}
	defer rows.Close()

	var out []*models.StagedChange
	for rows.Next() {
		var row models.StagedChange
		var oldText, newText string
		var resourceID, provisionalID *uuid.UUID
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.OrgID, &row.ResourceType,
			&resourceID, &provisionalID,
			&row.Action, &row.FieldPath, &oldText, &newText,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.queryRows scan: %w", err)
		}
		row.ResourceID = resourceID
		row.ProvisionalID = provisionalID
		if oldText != "" {
			row.OldValue = json.RawMessage(oldText)
		}
		if newText != "" {
			row.NewValue = json.RawMessage(newText)
		}
		out = append(out, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.queryRows: %w", err)
	}
	return out, nil
}

func nullableUUID(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableJSON(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// ErrNoRows is exposed for handler-side mapping to 404. It mirrors pgx's own
// error so callers don't need to import pgx just to test for "not found".
var ErrNoRows = errors.New("postgres: no rows")

// notFound wraps pgx.ErrNoRows so callers can check with errors.Is.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRows
	}
	return err
}

// GetProvisionalCreate returns the user's staged CREATE row whose
// provisional_id matches, or (nil, nil) when not found.
func (r *StagedChangesRepository) GetProvisionalCreate(ctx context.Context, userID, orgID uuid.UUID, resourceType string, provisionalID uuid.UUID) (*models.StagedChange, error) {
	const q = `
		SELECT id, user_id, org_id, resource_type, resource_id, provisional_id,
		       action, COALESCE(field_path, ''),
		       COALESCE(old_value::text, ''), COALESCE(new_value::text, ''),
		       created_at, updated_at
		  FROM staged_changes
		 WHERE user_id = $1 AND org_id = $2 AND resource_type = $3
		   AND provisional_id = $4 AND action = 'create'
		 LIMIT 1`
	var row models.StagedChange
	var oldText, newText string
	var resourceID, provID *uuid.UUID
	err := r.pool.QueryRow(ctx, q, userID, orgID, resourceType, provisionalID).Scan(
		&row.ID, &row.UserID, &row.OrgID, &row.ResourceType,
		&resourceID, &provID,
		&row.Action, &row.FieldPath, &oldText, &newText,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres.GetProvisionalCreate: %w", err)
	}
	row.ResourceID = resourceID
	row.ProvisionalID = provID
	if oldText != "" {
		row.OldValue = json.RawMessage(oldText)
	}
	if newText != "" {
		row.NewValue = json.RawMessage(newText)
	}
	return &row, nil
}

var _ = notFound // reserved for future Get-single helpers
