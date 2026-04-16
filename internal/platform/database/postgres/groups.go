package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/groups"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GroupRepository implements groups.Repository using PostgreSQL.
type GroupRepository struct {
	pool *pgxpool.Pool
}

// NewGroupRepository creates a new GroupRepository.
func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Groups
// ---------------------------------------------------------------------------

func (r *GroupRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]groups.GroupRow, error) {
	const q = `
		SELECT g.id, g.org_id, g.name, g.slug, COALESCE(g.description, ''),
			g.created_by, g.created_at, g.updated_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id)::int AS member_count
		FROM groups g
		WHERE g.org_id = $1
		ORDER BY g.name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GroupRepository.ListByOrg: %w", err)
	}
	defer rows.Close()

	var result []groups.GroupRow
	for rows.Next() {
		var row groups.GroupRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.Name, &row.Slug, &row.Description,
			&row.CreatedBy, &row.CreatedAt, &row.UpdatedAt,
			&row.MemberCount,
		); err != nil {
			return nil, fmt.Errorf("postgres.GroupRepository.ListByOrg scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GroupRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error) {
	const q = `
		SELECT id, org_id, name, slug, COALESCE(description, ''),
			created_by, created_at, updated_at
		FROM groups
		WHERE org_id = $1 AND slug = $2`

	var g models.Group
	err := r.pool.QueryRow(ctx, q, orgID, slug).Scan(
		&g.ID, &g.OrgID, &g.Name, &g.Slug, &g.Description,
		&g.CreatedBy, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GroupRepository.GetBySlug: %w", err)
	}
	return &g, nil
}

func (r *GroupRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	const q = `
		SELECT id, org_id, name, slug, COALESCE(description, ''),
			created_by, created_at, updated_at
		FROM groups
		WHERE id = $1`

	var g models.Group
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&g.ID, &g.OrgID, &g.Name, &g.Slug, &g.Description,
		&g.CreatedBy, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GroupRepository.GetByID: %w", err)
	}
	return &g, nil
}

func (r *GroupRepository) Create(ctx context.Context, g *models.Group) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	now := time.Now().UTC()
	g.CreatedAt = now
	g.UpdatedAt = now

	const q = `
		INSERT INTO groups (id, org_id, name, slug, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		g.ID, g.OrgID, g.Name, g.Slug, g.Description, g.CreatedBy, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.GroupRepository.Create: %w", err)
	}
	return nil
}

func (r *GroupRepository) Update(ctx context.Context, g *models.Group) error {
	g.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE groups
		SET name = $2, slug = $3, description = $4, updated_at = $5
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, g.ID, g.Name, g.Slug, g.Description, g.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.GroupRepository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Group members
// ---------------------------------------------------------------------------

func (r *GroupRepository) ListMembers(ctx context.Context, groupID uuid.UUID) ([]groups.GroupMemberRow, error) {
	const q = `
		SELECT gm.group_id, gm.user_id, gm.created_at,
			u.name, u.email, COALESCE(u.avatar_url, '')
		FROM group_members gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY u.name`

	rows, err := r.pool.Query(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GroupRepository.ListMembers: %w", err)
	}
	defer rows.Close()

	var result []groups.GroupMemberRow
	for rows.Next() {
		var row groups.GroupMemberRow
		if err := rows.Scan(
			&row.GroupID, &row.UserID, &row.CreatedAt,
			&row.Name, &row.Email, &row.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("postgres.GroupRepository.ListMembers scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GroupRepository) AddMember(ctx context.Context, groupID, userID uuid.UUID) error {
	const q = `
		INSERT INTO group_members (group_id, user_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`

	_, err := r.pool.Exec(ctx, q, groupID, userID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.AddMember: %w", err)
	}
	return nil
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.RemoveMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GroupRepository) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres.GroupRepository.IsMember: %w", err)
	}
	return exists, nil
}
