package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgRoleLookup implements auth.OrgRoleLookup using PostgreSQL.
type OrgRoleLookup struct {
	pool *pgxpool.Pool
}

// NewOrgRoleLookup creates a new OrgRoleLookup.
func NewOrgRoleLookup(pool *pgxpool.Pool) *OrgRoleLookup {
	return &OrgRoleLookup{pool: pool}
}

// GetOrgIDBySlug returns the org ID for the given slug.
func (l *OrgRoleLookup) GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := l.pool.QueryRow(ctx, `SELECT id FROM organizations WHERE slug = $1`, slug).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("postgres.GetOrgIDBySlug: %w", err)
	}
	return id, nil
}

// GetOrgMemberRole returns the user's role in the given org.
func (l *OrgRoleLookup) GetOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var role string
	err := l.pool.QueryRow(ctx,
		`SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("postgres.GetOrgMemberRole: %w", err)
	}
	return role, nil
}

// GetUserDefaultOrgRole returns the user's highest org role across all orgs.
// Role hierarchy: owner > admin > member > viewer.
func (l *OrgRoleLookup) GetUserDefaultOrgRole(ctx context.Context, userID uuid.UUID) (string, error) {
	var role string
	err := l.pool.QueryRow(ctx, `
		SELECT role FROM org_members WHERE user_id = $1
		ORDER BY CASE role
			WHEN 'owner' THEN 1
			WHEN 'admin' THEN 2
			WHEN 'member' THEN 3
			WHEN 'viewer' THEN 4
			ELSE 5
		END
		LIMIT 1`, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("postgres.GetUserDefaultOrgRole: %w", err)
	}
	return role, nil
}
