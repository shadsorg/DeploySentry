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

// UserRepository implements auth.UserRepository using a PostgreSQL connection pool.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository backed by the given pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

// scanUser reads a single User row from the given pgx.Row.
func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.AvatarURL,
		&u.AuthProvider,
		&u.ProviderID,
		&u.PasswordHash,
		&u.EmailVerified,
		&u.LastLoginAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// scanOrgMember reads a single OrgMember row from the given pgx.Row.
func scanOrgMember(row pgx.Row) (*models.OrgMember, error) {
	var m models.OrgMember
	err := row.Scan(
		&m.ID,
		&m.OrgID,
		&m.UserID,
		&m.Role,
		&m.InvitedBy,
		&m.JoinedAt,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// scanProjectMember reads a single ProjectMember row from the given pgx.Row.
func scanProjectMember(row pgx.Row) (*models.ProjectMember, error) {
	var m models.ProjectMember
	err := row.Scan(
		&m.ID,
		&m.ProjectID,
		&m.UserID,
		&m.Role,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// ---------------------------------------------------------------------------
// User methods
// ---------------------------------------------------------------------------

const userSelectCols = `
	id, email, name,
	COALESCE(avatar_url, ''),
	auth_provider,
	COALESCE(auth_provider_id, ''),
	COALESCE(password_hash, ''),
	email_verified,
	last_login_at,
	created_at,
	updated_at`

// CreateUser inserts a new user record into the database.
func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	const q = `
		INSERT INTO users
			(id, email, name, avatar_url, auth_provider, auth_provider_id, password_hash, email_verified, last_login_at, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.pool.Exec(ctx, q,
		user.ID,
		user.Email,
		user.Name,
		user.AvatarURL,
		user.AuthProvider,
		user.ProviderID,
		user.PasswordHash,
		user.EmailVerified,
		user.LastLoginAt,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateUser: %w", err)
	}
	return nil
}

// GetUser retrieves a user by their UUID.
func (r *UserRepository) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	q := `SELECT` + userSelectCols + ` FROM users WHERE id = $1`
	user, err := scanUser(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetUser: %w", err)
	}
	return user, nil
}

// GetUserByEmail retrieves a user by their email address.
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	q := `SELECT` + userSelectCols + ` FROM users WHERE email = $1`
	user, err := scanUser(r.pool.QueryRow(ctx, q, email))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetUserByEmail: %w", err)
	}
	return user, nil
}

// UpdateUser persists changes to an existing user record.
func (r *UserRepository) UpdateUser(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE users SET
			email         = $2,
			name          = $3,
			avatar_url    = $4,
			auth_provider = $5,
			auth_provider_id = $6,
			password_hash = $7,
			email_verified = $8,
			last_login_at = $9,
			updated_at    = $10
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		user.ID,
		user.Email,
		user.Name,
		user.AvatarURL,
		user.AuthProvider,
		user.ProviderID,
		user.PasswordHash,
		user.EmailVerified,
		user.LastLoginAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.UpdateUser: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes a user record from the database.
func (r *UserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteUser: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// OrgMember methods
// ---------------------------------------------------------------------------

const orgMemberSelectCols = `
	id, org_id, user_id, role,
	COALESCE(invited_by, '00000000-0000-0000-0000-000000000000'::uuid),
	COALESCE(joined_at, created_at),
	created_at,
	updated_at`

// ListOrgMembers returns a paginated list of members for the given org.
func (r *UserRepository) ListOrgMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.OrgMember, error) {
	args := []any{orgID}
	pagClause, args := paginationClause(limit, offset, args)
	q := `SELECT` + orgMemberSelectCols + ` FROM org_members WHERE org_id = $1 ORDER BY created_at ASC` + pagClause

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListOrgMembers: %w", err)
	}
	defer rows.Close()

	var members []*models.OrgMember
	for rows.Next() {
		m, err := scanOrgMember(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListOrgMembers: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListOrgMembers: %w", err)
	}
	return members, nil
}

// GetOrgMember retrieves a single org membership record.
func (r *UserRepository) GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	q := `SELECT` + orgMemberSelectCols + ` FROM org_members WHERE org_id = $1 AND user_id = $2`
	m, err := scanOrgMember(r.pool.QueryRow(ctx, q, orgID, userID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetOrgMember: %w", err)
	}
	return m, nil
}

// CreateOrgMember inserts a new org membership record.
func (r *UserRepository) CreateOrgMember(ctx context.Context, member *models.OrgMember) error {
	if member.ID == uuid.Nil {
		member.ID = uuid.New()
	}
	now := time.Now().UTC()
	member.CreatedAt = now
	member.UpdatedAt = now
	if member.JoinedAt.IsZero() {
		member.JoinedAt = now
	}

	const q = `
		INSERT INTO org_members
			(id, org_id, user_id, role, invited_by, joined_at, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		member.ID,
		member.OrgID,
		member.UserID,
		member.Role,
		member.InvitedBy,
		member.JoinedAt,
		member.CreatedAt,
		member.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateOrgMember: %w", err)
	}
	return nil
}

// UpdateOrgMember updates an existing org membership record.
func (r *UserRepository) UpdateOrgMember(ctx context.Context, member *models.OrgMember) error {
	member.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE org_members SET
			role       = $3,
			updated_at = $4
		WHERE org_id = $1 AND user_id = $2`

	tag, err := r.pool.Exec(ctx, q, member.OrgID, member.UserID, member.Role, member.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres.UpdateOrgMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteOrgMember removes a user from an organization.
func (r *UserRepository) DeleteOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("postgres.DeleteOrgMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// ProjectMember methods
// ---------------------------------------------------------------------------

const projectMemberSelectCols = `
	id, project_id, user_id, role, created_at, updated_at`

// ListProjectMembers returns a paginated list of members for the given project.
func (r *UserRepository) ListProjectMembers(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*models.ProjectMember, error) {
	args := []any{projectID}
	pagClause, args := paginationClause(limit, offset, args)
	q := `SELECT` + projectMemberSelectCols + ` FROM project_members WHERE project_id = $1 ORDER BY created_at ASC` + pagClause

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListProjectMembers: %w", err)
	}
	defer rows.Close()

	var members []*models.ProjectMember
	for rows.Next() {
		m, err := scanProjectMember(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListProjectMembers: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListProjectMembers: %w", err)
	}
	return members, nil
}

// GetProjectMember retrieves a single project membership record.
func (r *UserRepository) GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error) {
	q := `SELECT` + projectMemberSelectCols + ` FROM project_members WHERE project_id = $1 AND user_id = $2`
	m, err := scanProjectMember(r.pool.QueryRow(ctx, q, projectID, userID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetProjectMember: %w", err)
	}
	return m, nil
}

// CreateProjectMember inserts a new project membership record.
func (r *UserRepository) CreateProjectMember(ctx context.Context, member *models.ProjectMember) error {
	if member.ID == uuid.Nil {
		member.ID = uuid.New()
	}
	now := time.Now().UTC()
	member.CreatedAt = now
	member.UpdatedAt = now

	const q = `
		INSERT INTO project_members
			(id, project_id, user_id, role, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		member.ID,
		member.ProjectID,
		member.UserID,
		member.Role,
		member.CreatedAt,
		member.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.CreateProjectMember: %w", err)
	}
	return nil
}

// UpdateProjectMember updates an existing project membership record.
func (r *UserRepository) UpdateProjectMember(ctx context.Context, member *models.ProjectMember) error {
	member.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE project_members SET
			role       = $3,
			updated_at = $4
		WHERE project_id = $1 AND user_id = $2`

	tag, err := r.pool.Exec(ctx, q, member.ProjectID, member.UserID, member.Role, member.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres.UpdateProjectMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProjectMember removes a user from a project.
func (r *UserRepository) DeleteProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err != nil {
		return fmt.Errorf("postgres.DeleteProjectMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
