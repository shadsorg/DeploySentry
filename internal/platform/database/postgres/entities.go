package postgres

import (
	"context"
	"encoding/json"
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
		INSERT INTO projects (id, org_id, name, slug, description, repo_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		project.ID,
		project.OrgID,
		project.Name,
		project.Slug,
		project.Description,
		project.RepoURL,
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

// GetProjectByID retrieves a project by its primary key.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_at, updated_at, deleted_at
		FROM projects WHERE id = $1`

	var p models.Project
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&p.ID,
		&p.OrgID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.RepoURL,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetProjectByID: %w", err)
	}
	return &p, nil
}

// GetProjectBySlug retrieves a project by org ID and slug.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_at, updated_at, deleted_at
		FROM projects WHERE org_id = $1 AND slug = $2`

	var p models.Project
	err := r.pool.QueryRow(ctx, q, orgID, slug).Scan(
		&p.ID,
		&p.OrgID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.RepoURL,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.DeletedAt,
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
// Visibility filtering: owners see all; other roles see only projects that have
// no grants or where the user has a direct or group-based grant.
func (r *EntityRepository) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error) {
	deletedFilter := " AND p.deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	q := `SELECT p.id, p.org_id, p.name, p.slug, COALESCE(p.description, ''), COALESCE(p.repo_url, ''), p.created_at, p.updated_at, p.deleted_at
		FROM projects p
		WHERE p.org_id = $1` + deletedFilter + `
		AND (
			$3 = 'owner'
			OR NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = p.id)
			OR EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = p.id AND rg.user_id = $2)
			OR EXISTS (
				SELECT 1 FROM resource_grants rg
				JOIN group_members gm ON gm.group_id = rg.group_id
				WHERE rg.project_id = p.id AND gm.user_id = $2
			)
		)
		ORDER BY p.name`

	rows, err := r.pool.Query(ctx, q, orgID, userID, orgRole)
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
			&p.Description,
			&p.RepoURL,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.DeletedAt,
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

// UpdateProject updates the name, description, repo_url, and updated_at fields of a project.
func (r *EntityRepository) UpdateProject(ctx context.Context, project *models.Project) error {
	const q = `
		UPDATE projects
		SET name = $1, description = $2, repo_url = $3, updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, project.Name, project.Description, project.RepoURL, project.UpdatedAt, project.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpdateProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteProject marks a project as deleted by setting deleted_at.
func (r *EntityRepository) SoftDeleteProject(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE projects SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.SoftDeleteProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteProject permanently removes a project that has been soft-deleted for at least 7 days.
func (r *EntityRepository) HardDeleteProject(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM projects WHERE id = $1 AND deleted_at <= now() - interval '7 days'`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.HardDeleteProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RestoreProject un-deletes a soft-deleted project by clearing deleted_at.
func (r *EntityRepository) RestoreProject(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE projects SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.RestoreProject: %w", err)
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
	linksJSON, err := json.Marshal(appMonitoringLinksOrEmpty(app.MonitoringLinks))
	if err != nil {
		return fmt.Errorf("postgres.CreateApp marshal links: %w", err)
	}
	const q = `
		INSERT INTO applications (id, project_id, name, slug, description, repo_url, monitoring_links, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err = r.pool.Exec(ctx, q,
		app.ID,
		app.ProjectID,
		app.Name,
		app.Slug,
		app.Description,
		app.RepoURL,
		linksJSON,
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

// GetAppByID retrieves an application by its primary key.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetAppByID(ctx context.Context, id uuid.UUID) (*models.Application, error) {
	const q = `
		SELECT id, project_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), monitoring_links, created_by, created_at, updated_at, deleted_at
		FROM applications WHERE id = $1`

	var a models.Application
	var linksJSON []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID,
		&a.ProjectID,
		&a.Name,
		&a.Slug,
		&a.Description,
		&a.RepoURL,
		&linksJSON,
		&a.CreatedBy,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetAppByID: %w", err)
	}
	a.MonitoringLinks = decodeMonitoringLinks(linksJSON)
	return &a, nil
}

// GetAppBySlug retrieves an application by project ID and slug.
// Returns nil, nil when no row is found.
func (r *EntityRepository) GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	const q = `
		SELECT id, project_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), monitoring_links, created_by, created_at, updated_at, deleted_at
		FROM applications WHERE project_id = $1 AND slug = $2`

	var a models.Application
	var linksJSON []byte
	err := r.pool.QueryRow(ctx, q, projectID, slug).Scan(
		&a.ID,
		&a.ProjectID,
		&a.Name,
		&a.Slug,
		&a.Description,
		&a.RepoURL,
		&linksJSON,
		&a.CreatedBy,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetAppBySlug: %w", err)
	}
	a.MonitoringLinks = decodeMonitoringLinks(linksJSON)
	return &a, nil
}

// ListAppsByProject returns all applications for a project, ordered by name.
// Visibility filtering: owners see all; other roles see apps based on cascade
// logic (app-level grants, then project-level grants, then open).
func (r *EntityRepository) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error) {
	deletedFilter := " AND a.deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	q := `SELECT a.id, a.project_id, a.name, a.slug, COALESCE(a.description, ''), COALESCE(a.repo_url, ''), a.monitoring_links, a.created_by, a.created_at, a.updated_at, a.deleted_at
		FROM applications a
		WHERE a.project_id = $1` + deletedFilter + `
		AND (
			$3 = 'owner'
			OR (
				NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id)
				AND NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = a.project_id)
			)
			OR (
				NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id)
				AND (
					EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = a.project_id AND rg.user_id = $2)
					OR EXISTS (
						SELECT 1 FROM resource_grants rg
						JOIN group_members gm ON gm.group_id = rg.group_id
						WHERE rg.project_id = a.project_id AND gm.user_id = $2
					)
				)
			)
			OR EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id AND rg.user_id = $2)
			OR EXISTS (
				SELECT 1 FROM resource_grants rg
				JOIN group_members gm ON gm.group_id = rg.group_id
				WHERE rg.application_id = a.id AND gm.user_id = $2
			)
		)
		ORDER BY a.name`

	rows, err := r.pool.Query(ctx, q, projectID, userID, orgRole)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Application, 0)
	for rows.Next() {
		var a models.Application
		var linksJSON []byte
		if err := rows.Scan(
			&a.ID,
			&a.ProjectID,
			&a.Name,
			&a.Slug,
			&a.Description,
			&a.RepoURL,
			&linksJSON,
			&a.CreatedBy,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
		}
		a.MonitoringLinks = decodeMonitoringLinks(linksJSON)
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

// UpdateAppMonitoringLinks replaces the monitoring_links JSONB column for
// the given application and bumps updated_at. The caller is responsible
// for validating the links before invoking this.
func (r *EntityRepository) UpdateAppMonitoringLinks(ctx context.Context, appID uuid.UUID, links []models.MonitoringLink) error {
	linksJSON, err := json.Marshal(appMonitoringLinksOrEmpty(links))
	if err != nil {
		return fmt.Errorf("postgres.UpdateAppMonitoringLinks marshal: %w", err)
	}
	const q = `UPDATE applications SET monitoring_links = $1, updated_at = now() WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, linksJSON, appID)
	if err != nil {
		return fmt.Errorf("postgres.UpdateAppMonitoringLinks: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// appMonitoringLinksOrEmpty normalizes a nil slice to [] so the JSON
// column always holds a JSON array (not null) on the wire.
func appMonitoringLinksOrEmpty(links []models.MonitoringLink) []models.MonitoringLink {
	if links == nil {
		return []models.MonitoringLink{}
	}
	return links
}

// decodeMonitoringLinks parses the stored JSONB into the typed slice.
// Empty / unparseable input produces an empty (non-nil) slice so callers
// can iterate without nil-checking.
func decodeMonitoringLinks(raw []byte) []models.MonitoringLink {
	if len(raw) == 0 {
		return []models.MonitoringLink{}
	}
	var out []models.MonitoringLink
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []models.MonitoringLink{}
	}
	return out
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

// ---------------------------------------------------------------------------
// Project delete / restore methods
// ---------------------------------------------------------------------------

// CountFlagsByProject returns the number of feature flags belonging to a project.
func (r *EntityRepository) CountFlagsByProject(ctx context.Context, projectID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM feature_flags WHERE project_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, q, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres.CountFlagsByProject: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Application delete / restore methods
// ---------------------------------------------------------------------------

// CountFlagsByApp returns the number of feature flags belonging to an application.
func (r *EntityRepository) CountFlagsByApp(ctx context.Context, applicationID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM feature_flags WHERE application_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, q, applicationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres.CountFlagsByApp: %w", err)
	}
	return count, nil
}

// SoftDeleteApp marks an application as deleted without removing it.
func (r *EntityRepository) SoftDeleteApp(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.SoftDeleteApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteApp permanently removes an application from the database.
func (r *EntityRepository) HardDeleteApp(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM applications WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.HardDeleteApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RestoreApp clears deleted_at on a soft-deleted application.
func (r *EntityRepository) RestoreApp(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET deleted_at = NULL, updated_at = now() WHERE id = $1 AND deleted_at IS NOT NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.RestoreApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// User lookup
// ---------------------------------------------------------------------------

// GetUserName returns the display name for a user by ID.
func (r *EntityRepository) GetUserName(ctx context.Context, id uuid.UUID) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(name, email) FROM users WHERE id = $1`, id).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

// ---------------------------------------------------------------------------
// Flag activity query
// ---------------------------------------------------------------------------

// HasRecentFlagActivity returns flags with evaluation activity since the given time.
func (r *EntityRepository) HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, applicationID *uuid.UUID, since time.Time) ([]models.FlagActivitySummary, error) {
	appFilter := ""
	args := []interface{}{projectID, since}
	if applicationID != nil {
		appFilter = " AND f.application_id = $3"
		args = append(args, *applicationID)
	}
	q := `SELECT f.key, f.name, MAX(l.evaluated_at) as last_evaluated
		FROM feature_flags f
		JOIN flag_evaluation_log l ON f.id = l.flag_id
		WHERE f.project_id = $1 AND l.evaluated_at >= $2` + appFilter + `
		GROUP BY f.key, f.name
		ORDER BY last_evaluated DESC`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.HasRecentFlagActivity: %w", err)
	}
	defer rows.Close()
	var result []models.FlagActivitySummary
	for rows.Next() {
		var s models.FlagActivitySummary
		if err := rows.Scan(&s.Key, &s.Name, &s.LastEvaluated); err != nil {
			return nil, fmt.Errorf("postgres.HasRecentFlagActivity: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
