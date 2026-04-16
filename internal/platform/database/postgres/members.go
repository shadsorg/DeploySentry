// internal/platform/database/postgres/members.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/members"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemberRepository implements members.Repository using PostgreSQL.
type MemberRepository struct {
	pool *pgxpool.Pool
}

// NewMemberRepository creates a new MemberRepository.
func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Org members
// ---------------------------------------------------------------------------

func (r *MemberRepository) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]members.OrgMemberRow, error) {
	const q = `
		SELECT om.id, om.org_id, om.user_id, om.role,
			COALESCE(om.invited_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(om.joined_at, om.created_at),
			om.created_at, om.updated_at,
			u.name, u.email, COALESCE(u.avatar_url, '')
		FROM org_members om
		JOIN users u ON om.user_id = u.id
		WHERE om.org_id = $1
		ORDER BY om.joined_at`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListOrgMembers: %w", err)
	}
	defer rows.Close()

	var result []members.OrgMemberRow
	for rows.Next() {
		var row members.OrgMemberRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.UserID, &row.Role,
			&row.InvitedBy, &row.JoinedAt, &row.CreatedAt, &row.UpdatedAt,
			&row.Name, &row.Email, &row.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListOrgMembers scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *MemberRepository) GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	const q = `
		SELECT id, org_id, user_id, role,
			COALESCE(invited_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(joined_at, created_at),
			created_at, updated_at
		FROM org_members
		WHERE org_id = $1 AND user_id = $2`

	var m models.OrgMember
	err := r.pool.QueryRow(ctx, q, orgID, userID).Scan(
		&m.ID, &m.OrgID, &m.UserID, &m.Role,
		&m.InvitedBy, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetOrgMember: %w", err)
	}
	return &m, nil
}

func (r *MemberRepository) AddOrgMember(ctx context.Context, m *models.OrgMember) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.JoinedAt.IsZero() {
		m.JoinedAt = now
	}

	const q = `
		INSERT INTO org_members (id, org_id, user_id, role, invited_by, joined_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		m.ID, m.OrgID, m.UserID, m.Role, m.InvitedBy, m.JoinedAt, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.AddOrgMember: %w", err)
	}
	return nil
}

func (r *MemberRepository) UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error {
	const q = `UPDATE org_members SET role = $3, updated_at = $4 WHERE org_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, orgID, userID, role, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("postgres.UpdateOrgMemberRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MemberRepository) RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("postgres.RemoveOrgMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MemberRepository) CountOrgOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE org_id = $1 AND role = 'owner'`, orgID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres.CountOrgOwners: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// User lookup
// ---------------------------------------------------------------------------

func (r *MemberRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, name, COALESCE(avatar_url, ''),
			auth_provider, COALESCE(auth_provider_id, ''),
			COALESCE(password_hash, ''), email_verified,
			last_login_at, created_at, updated_at
		FROM users WHERE email = $1`

	var u models.User
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.AvatarURL,
		&u.AuthProvider, &u.ProviderID,
		&u.PasswordHash, &u.EmailVerified,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetUserByEmail: %w", err)
	}
	return &u, nil
}
