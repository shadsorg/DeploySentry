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

// EntityRepository implements entities.EntityRepository using a PostgreSQL connection pool.
type EntityRepository struct {
	pool *pgxpool.Pool
}

// NewEntityRepository creates a new EntityRepository backed by the given pool.
func NewEntityRepository(pool *pgxpool.Pool) *EntityRepository {
	return &EntityRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Organization methods
// ---------------------------------------------------------------------------

// CreateOrg inserts a new organization into the database.
func (r *EntityRepository) CreateOrg(ctx context.Context, org *models.Organization) error {
	const q = `
		INSERT INTO organizations (id, name, slug, owner_id, plan, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, q,
		org.ID,
		org.Name,
		org.Slug,
		org.OwnerID,
		org.Plan,
		org.CreatedAt,
		org.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateOrg: %w", err)
	}
	return nil
}

// GetOrgBySlug retrieves an organization by its slug.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	const q = `
		SELECT id, name, slug, owner_id, plan, created_at, updated_at
		FROM organizations WHERE slug = $1`

	var org models.Organization
	err := r.pool.QueryRow(ctx, q, slug).Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.OwnerID,
		&org.Plan,
		&org.CreatedAt,
		&org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetOrgBySlug: %w", err)
	}
	return &org, nil
}

// ListOrgsByUser returns all organizations the user belongs to, ordered by name.
func (r *EntityRepository) ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error) {
	const q = `
		SELECT o.id, o.name, o.slug, o.owner_id, o.plan, o.created_at, o.updated_at
		FROM organizations o
		JOIN org_members om ON o.id = om.org_id
		WHERE om.user_id = $1
		ORDER BY o.name`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListOrgsByUser: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Organization, 0)
	for rows.Next() {
		var org models.Organization
		if err := rows.Scan(
			&org.ID,
			&org.Name,
			&org.Slug,
			&org.OwnerID,
			&org.Plan,
			&org.CreatedAt,
			&org.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListOrgsByUser: %w", err)
		}
		result = append(result, &org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListOrgsByUser: %w", err)
	}
	return result, nil
}

// UpdateOrg updates the name and updated_at fields of an organization.
func (r *EntityRepository) UpdateOrg(ctx context.Context, org *models.Organization) error {
	const q = `UPDATE organizations SET name = $1, updated_at = $2 WHERE id = $3`

	tag, err := r.pool.Exec(ctx, q, org.Name, org.UpdatedAt, org.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpdateOrg: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Project methods
// ---------------------------------------------------------------------------

// CreateProject inserts a new project into the database.
func (r *EntityRepository) CreateProject(ctx context.Context, project *models.Project) error {
	const q = `
		INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		project.ID,
		project.OrgID,
		project.Name,
		project.Slug,
		project.CreatedAt,
		project.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateProject: %w", err)
	}
	return nil
}

// GetProjectBySlug retrieves a project by org ID and slug.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, created_at, updated_at
		FROM projects WHERE org_id = $1 AND slug = $2`

	var p models.Project
	err := r.pool.QueryRow(ctx, q, orgID, slug).Scan(
		&p.ID,
		&p.OrgID,
		&p.Name,
		&p.Slug,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetProjectBySlug: %w", err)
	}
	return &p, nil
}

// ListProjectsByOrg returns all projects for an organization, ordered by name.
func (r *EntityRepository) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, created_at, updated_at
		FROM projects WHERE org_id = $1 ORDER BY name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListProjectsByOrg: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Project, 0)
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(
			&p.ID,
			&p.OrgID,
			&p.Name,
			&p.Slug,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListProjectsByOrg: %w", err)
		}
		result = append(result, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListProjectsByOrg: %w", err)
	}
	return result, nil
}

// UpdateProject updates the name and updated_at fields of a project.
func (r *EntityRepository) UpdateProject(ctx context.Context, project *models.Project) error {
	const q = `UPDATE projects SET name = $1, updated_at = $2 WHERE id = $3`

	tag, err := r.pool.Exec(ctx, q, project.Name, project.UpdatedAt, project.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpdateProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Application methods
// ---------------------------------------------------------------------------

// CreateApp inserts a new application into the database.
func (r *EntityRepository) CreateApp(ctx context.Context, app *models.Application) error {
	const q = `
		INSERT INTO applications (id, project_id, name, slug, description, repo_url, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		app.ID,
		app.ProjectID,
		app.Name,
		app.Slug,
		app.Description,
		app.RepoURL,
		app.CreatedBy,
		app.CreatedAt,
		app.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateApp: %w", err)
	}
	return nil
}

// GetAppBySlug retrieves an application by project ID and slug.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	const q = `
		SELECT id, project_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_by, created_at, updated_at
		FROM applications WHERE project_id = $1 AND slug = $2`

	var a models.Application
	err := r.pool.QueryRow(ctx, q, projectID, slug).Scan(
		&a.ID,
		&a.ProjectID,
		&a.Name,
		&a.Slug,
		&a.Description,
		&a.RepoURL,
		&a.CreatedBy,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetAppBySlug: %w", err)
	}
	return &a, nil
}

// ListAppsByProject returns all applications for a project, ordered by name.
func (r *EntityRepository) ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error) {
	const q = `
		SELECT id, project_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_by, created_at, updated_at
		FROM applications WHERE project_id = $1 ORDER BY name`

	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Application, 0)
	for rows.Next() {
		var a models.Application
		if err := rows.Scan(
			&a.ID,
			&a.ProjectID,
			&a.Name,
			&a.Slug,
			&a.Description,
			&a.RepoURL,
			&a.CreatedBy,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
		}
		result = append(result, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
	}
	return result, nil
}

// UpdateApp updates the name, description, repo_url, and updated_at fields of an application.
func (r *EntityRepository) UpdateApp(ctx context.Context, app *models.Application) error {
	const q = `
		UPDATE applications
		SET name = $1, description = $2, repo_url = $3, updated_at = $4
		WHERE id = $5`

	tag, err := r.pool.Exec(ctx, q, app.Name, app.Description, app.RepoURL, app.UpdatedAt, app.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpdateApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Environment methods
// ---------------------------------------------------------------------------

// ListEnvironmentsByApp returns all environments for an application, ordered by sort_order.
func (r *EntityRepository) ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error) {
	const q = `
		SELECT id, application_id, name, slug, is_production, sort_order, created_at
		FROM environments WHERE application_id = $1 ORDER BY sort_order`

	rows, err := r.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListEnvironmentsByApp: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Environment, 0)
	for rows.Next() {
		var e models.Environment
		if err := rows.Scan(
			&e.ID,
			&e.ApplicationID,
			&e.Name,
			&e.Slug,
			&e.IsProduction,
			&e.SortOrder,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListEnvironmentsByApp: %w", err)
		}
		result = append(result, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListEnvironmentsByApp: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// OrgMember methods
// ---------------------------------------------------------------------------

// AddOrgMember inserts a new member record into org_members.
func (r *EntityRepository) AddOrgMember(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	now := time.Now().UTC()
	const q = `
		INSERT INTO org_members (id, org_id, user_id, role, joined_at, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		orgID,
		userID,
		role,
		now,
		now,
		now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.AddOrgMember: %w", err)
	}
	return nil
}
