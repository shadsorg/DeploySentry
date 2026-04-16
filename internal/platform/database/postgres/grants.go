package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/grants"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GrantRepository implements grants.Repository using PostgreSQL.
type GrantRepository struct {
	pool *pgxpool.Pool
}

// NewGrantRepository creates a new GrantRepository.
func NewGrantRepository(pool *pgxpool.Pool) *GrantRepository {
	return &GrantRepository{pool: pool}
}

func (r *GrantRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]grants.GrantRow, error) {
	const q = `
		SELECT rg.id, rg.org_id, rg.project_id, rg.application_id,
			rg.user_id, rg.group_id, rg.permission, rg.granted_by, rg.created_at,
			CASE WHEN rg.user_id IS NOT NULL THEN u.name WHEN rg.group_id IS NOT NULL THEN g.name END AS grantee_name,
			CASE WHEN rg.user_id IS NOT NULL THEN 'user' WHEN rg.group_id IS NOT NULL THEN 'group' END AS grantee_type
		FROM resource_grants rg
		LEFT JOIN users u ON rg.user_id = u.id
		LEFT JOIN groups g ON rg.group_id = g.id
		WHERE rg.project_id = $1
		ORDER BY grantee_type, grantee_name`

	return r.scanGrantRows(ctx, q, projectID)
}

func (r *GrantRepository) ListByApp(ctx context.Context, applicationID uuid.UUID) ([]grants.GrantRow, error) {
	const q = `
		SELECT rg.id, rg.org_id, rg.project_id, rg.application_id,
			rg.user_id, rg.group_id, rg.permission, rg.granted_by, rg.created_at,
			CASE WHEN rg.user_id IS NOT NULL THEN u.name WHEN rg.group_id IS NOT NULL THEN g.name END AS grantee_name,
			CASE WHEN rg.user_id IS NOT NULL THEN 'user' WHEN rg.group_id IS NOT NULL THEN 'group' END AS grantee_type
		FROM resource_grants rg
		LEFT JOIN users u ON rg.user_id = u.id
		LEFT JOIN groups g ON rg.group_id = g.id
		WHERE rg.application_id = $1
		ORDER BY grantee_type, grantee_name`

	return r.scanGrantRows(ctx, q, applicationID)
}

func (r *GrantRepository) scanGrantRows(ctx context.Context, query string, arg interface{}) ([]grants.GrantRow, error) {
	rows, err := r.pool.Query(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("postgres.GrantRepository.scanGrantRows: %w", err)
	}
	defer rows.Close()

	var result []grants.GrantRow
	for rows.Next() {
		var row grants.GrantRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.ProjectID, &row.ApplicationID,
			&row.UserID, &row.GroupID, &row.Permission, &row.GrantedBy, &row.CreatedAt,
			&row.GranteeName, &row.GranteeType,
		); err != nil {
			return nil, fmt.Errorf("postgres.GrantRepository.scanGrantRows scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GrantRepository) Create(ctx context.Context, g *models.ResourceGrant) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	g.CreatedAt = time.Now().UTC()

	const q = `
		INSERT INTO resource_grants (id, org_id, project_id, application_id, user_id, group_id, permission, granted_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		g.ID, g.OrgID, g.ProjectID, g.ApplicationID,
		g.UserID, g.GroupID, g.Permission, g.GrantedBy, g.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.GrantRepository.Create: %w", err)
	}
	return nil
}

func (r *GrantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM resource_grants WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.GrantRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GrantRepository) HasAnyGrants(ctx context.Context, projectID *uuid.UUID, applicationID *uuid.UUID) (bool, error) {
	var exists bool
	var err error

	if projectID != nil {
		err = r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM resource_grants WHERE project_id = $1)`,
			*projectID,
		).Scan(&exists)
	} else if applicationID != nil {
		err = r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM resource_grants WHERE application_id = $1)`,
			*applicationID,
		).Scan(&exists)
	} else {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("postgres.GrantRepository.HasAnyGrants: %w", err)
	}
	return exists, nil
}

func (r *GrantRepository) GetUserPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error) {
	var perm models.ResourcePermission
	var err error

	if projectID != nil {
		err = r.pool.QueryRow(ctx,
			`SELECT permission FROM resource_grants WHERE user_id = $1 AND project_id = $2`,
			userID, *projectID,
		).Scan(&perm)
	} else if applicationID != nil {
		err = r.pool.QueryRow(ctx,
			`SELECT permission FROM resource_grants WHERE user_id = $1 AND application_id = $2`,
			userID, *applicationID,
		).Scan(&perm)
	} else {
		return nil, nil
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GrantRepository.GetUserPermission: %w", err)
	}
	return &perm, nil
}

func (r *GrantRepository) GetUserGroupPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error) {
	var perm models.ResourcePermission
	var err error

	if projectID != nil {
		err = r.pool.QueryRow(ctx, `
			SELECT rg.permission FROM resource_grants rg
			JOIN group_members gm ON gm.group_id = rg.group_id
			WHERE rg.project_id = $1 AND gm.user_id = $2
			ORDER BY CASE WHEN rg.permission = 'write' THEN 0 ELSE 1 END
			LIMIT 1`,
			*projectID, userID,
		).Scan(&perm)
	} else if applicationID != nil {
		err = r.pool.QueryRow(ctx, `
			SELECT rg.permission FROM resource_grants rg
			JOIN group_members gm ON gm.group_id = rg.group_id
			WHERE rg.application_id = $1 AND gm.user_id = $2
			ORDER BY CASE WHEN rg.permission = 'write' THEN 0 ELSE 1 END
			LIMIT 1`,
			*applicationID, userID,
		).Scan(&perm)
	} else {
		return nil, nil
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GrantRepository.GetUserGroupPermission: %w", err)
	}
	return &perm, nil
}
