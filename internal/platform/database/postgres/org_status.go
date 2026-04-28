package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgStatusRepository exposes the org-wide read queries backing the
// /orgs/:slug/status and /orgs/:slug/deployments endpoints.
type OrgStatusRepository struct {
	pool *pgxpool.Pool
}

func NewOrgStatusRepository(pool *pgxpool.Pool) *OrgStatusRepository {
	return &OrgStatusRepository{pool: pool}
}

// LatestDeploymentKey identifies an (application, environment) cell in the
// status grid.
type LatestDeploymentKey struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
}

// ListLatestDeploymentsForApps returns the most recent deployment for every
// (application, environment) pair in the supplied app set, one row per cell.
// Absent cells (no deploys ever for that pair) simply do not appear in the
// result — callers treat missing entries as "never deployed."
func (r *OrgStatusRepository) ListLatestDeploymentsForApps(ctx context.Context, appIDs []uuid.UUID) (map[LatestDeploymentKey]*models.Deployment, error) {
	if len(appIDs) == 0 {
		return map[LatestDeploymentKey]*models.Deployment{}, nil
	}
	const q = `
		SELECT DISTINCT ON (application_id, environment_id)
			id, application_id, environment_id, strategy, status,
			artifact, version, COALESCE(commit_sha, ''),
			traffic_percent, previous_deployment_id, flag_test_key, created_by,
			mode, source,
			started_at, completed_at,
			created_at, updated_at
		FROM deployments
		WHERE application_id = ANY($1)
		ORDER BY application_id, environment_id, created_at DESC`

	rows, err := r.pool.Query(ctx, q, appIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListLatestDeploymentsForApps: %w", err)
	}
	defer rows.Close()

	out := make(map[LatestDeploymentKey]*models.Deployment)
	for rows.Next() {
		var d models.Deployment
		if err := rows.Scan(
			&d.ID, &d.ApplicationID, &d.EnvironmentID, &d.Strategy, &d.Status,
			&d.Artifact, &d.Version, &d.CommitSHA,
			&d.TrafficPercent, &d.PreviousDeploymentID, &d.FlagTestKey, &d.CreatedBy,
			&d.Mode, &d.Source,
			&d.StartedAt, &d.CompletedAt,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListLatestDeploymentsForApps scan: %w", err)
		}
		out[LatestDeploymentKey{ApplicationID: d.ApplicationID, EnvironmentID: d.EnvironmentID}] = &d
	}
	return out, rows.Err()
}

// ListLatestBuildsForApps returns the most recent deployment row per
// (application, environment) whose `source` begins with "github-actions".
// Cells with no matching row are omitted. Used by the Org Status service to
// populate the per-cell "build in progress / build failed" pill without a
// second round-trip.
func (r *OrgStatusRepository) ListLatestBuildsForApps(ctx context.Context, appIDs []uuid.UUID) (map[LatestDeploymentKey]*models.Deployment, error) {
	if len(appIDs) == 0 {
		return map[LatestDeploymentKey]*models.Deployment{}, nil
	}
	const q = `
		SELECT DISTINCT ON (application_id, environment_id)
			id, application_id, environment_id, strategy, status,
			artifact, version, COALESCE(commit_sha, ''),
			traffic_percent, previous_deployment_id, flag_test_key, created_by,
			mode, source,
			started_at, completed_at,
			created_at, updated_at
		FROM deployments
		WHERE application_id = ANY($1)
		  AND source IS NOT NULL
		  AND source LIKE 'github-actions%'
		ORDER BY application_id, environment_id, created_at DESC`
	rows, err := r.pool.Query(ctx, q, appIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListLatestBuildsForApps: %w", err)
	}
	defer rows.Close()
	out := make(map[LatestDeploymentKey]*models.Deployment)
	for rows.Next() {
		var d models.Deployment
		if err := rows.Scan(
			&d.ID, &d.ApplicationID, &d.EnvironmentID, &d.Strategy, &d.Status,
			&d.Artifact, &d.Version, &d.CommitSHA,
			&d.TrafficPercent, &d.PreviousDeploymentID, &d.FlagTestKey, &d.CreatedBy,
			&d.Mode, &d.Source,
			&d.StartedAt, &d.CompletedAt,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListLatestBuildsForApps scan: %w", err)
		}
		out[LatestDeploymentKey{ApplicationID: d.ApplicationID, EnvironmentID: d.EnvironmentID}] = &d
	}
	return out, rows.Err()
}

// ListAppStatusesForApps returns every app_status row for the supplied apps,
// keyed on (application_id, environment_id).
func (r *OrgStatusRepository) ListAppStatusesForApps(ctx context.Context, appIDs []uuid.UUID) (map[LatestDeploymentKey]*models.AppStatus, error) {
	if len(appIDs) == 0 {
		return map[LatestDeploymentKey]*models.AppStatus{}, nil
	}
	const q = `
		SELECT application_id, environment_id, version, COALESCE(commit_sha, ''),
		       health_state, health_score, COALESCE(health_reason, ''), COALESCE(deploy_slot, ''),
		       tags, source, reported_at
		FROM app_status
		WHERE application_id = ANY($1)`
	rows, err := r.pool.Query(ctx, q, appIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAppStatusesForApps: %w", err)
	}
	defer rows.Close()

	out := make(map[LatestDeploymentKey]*models.AppStatus)
	for rows.Next() {
		var s models.AppStatus
		var tagsJSON []byte
		if err := rows.Scan(
			&s.ApplicationID, &s.EnvironmentID, &s.Version, &s.CommitSHA,
			&s.HealthState, &s.HealthScore, &s.HealthReason, &s.DeploySlot,
			&tagsJSON, &s.Source, &s.ReportedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListAppStatusesForApps scan: %w", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &s.Tags)
		}
		if s.Tags == nil {
			s.Tags = map[string]string{}
		}
		out[LatestDeploymentKey{ApplicationID: s.ApplicationID, EnvironmentID: s.EnvironmentID}] = &s
	}
	return out, rows.Err()
}

// DeploymentsByOrgFilters narrows the org-wide deployment history.
type DeploymentsByOrgFilters struct {
	ProjectID     *uuid.UUID
	ApplicationID *uuid.UUID
	EnvironmentID *uuid.UUID
	Status        *models.DeployStatus
	Mode          *models.DeployMode
	From          *time.Time
	To            *time.Time
}

// DeploymentsByOrgPage is one slice of the cursor-paginated org deploy
// history list.
type DeploymentsByOrgPage struct {
	Rows       []DeploymentsByOrgRow
	NextCursor string
}

// DeploymentsByOrgRow carries the joined data the UI needs to render a
// single history row without a second round-trip.
type DeploymentsByOrgRow struct {
	Deployment      *models.Deployment
	ProjectID       uuid.UUID
	ProjectSlug     string
	ProjectName     string
	ApplicationSlug string
	ApplicationName string
	EnvironmentSlug string
	EnvironmentName string
}

// ListDeploymentsByOrg returns org-scoped deployments ordered by
// created_at DESC, id DESC. The cursor encodes the last row's
// (created_at, id) pair.
func (r *OrgStatusRepository) ListDeploymentsByOrg(ctx context.Context, orgID uuid.UUID, f DeploymentsByOrgFilters, cursor string, limit int) (*DeploymentsByOrgPage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args := []any{orgID}
	var clauses []string
	clauses = append(clauses, "p.org_id = $1")

	addArg := func(v any) int {
		args = append(args, v)
		return len(args)
	}

	if f.ProjectID != nil {
		clauses = append(clauses, fmt.Sprintf("p.id = $%d", addArg(*f.ProjectID)))
	}
	if f.ApplicationID != nil {
		clauses = append(clauses, fmt.Sprintf("a.id = $%d", addArg(*f.ApplicationID)))
	}
	if f.EnvironmentID != nil {
		clauses = append(clauses, fmt.Sprintf("e.id = $%d", addArg(*f.EnvironmentID)))
	}
	if f.Status != nil {
		clauses = append(clauses, fmt.Sprintf("d.status = $%d", addArg(string(*f.Status))))
	}
	if f.Mode != nil {
		clauses = append(clauses, fmt.Sprintf("d.mode = $%d", addArg(string(*f.Mode))))
	}
	if f.From != nil {
		clauses = append(clauses, fmt.Sprintf("d.created_at >= $%d", addArg(*f.From)))
	}
	if f.To != nil {
		clauses = append(clauses, fmt.Sprintf("d.created_at <= $%d", addArg(*f.To)))
	}

	if cursor != "" {
		ts, id, err := decodeDeploymentsCursor(cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		clauses = append(clauses,
			fmt.Sprintf("(d.created_at, d.id) < ($%d, $%d)", addArg(ts), addArg(id)))
	}

	args = append(args, limit+1) // fetch one extra to detect more-pages

	q := `
		SELECT
			d.id, d.application_id, d.environment_id, d.strategy, d.status,
			d.artifact, d.version, COALESCE(d.commit_sha, ''),
			d.traffic_percent, d.previous_deployment_id, d.flag_test_key, d.created_by,
			d.mode, d.source,
			d.started_at, d.completed_at,
			d.created_at, d.updated_at,
			p.id, p.slug, p.name,
			a.slug, a.name,
			e.slug, e.name
		FROM deployments d
		JOIN applications a ON a.id = d.application_id
		JOIN projects p     ON p.id = a.project_id
		JOIN environments e ON e.id = d.environment_id
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY d.created_at DESC, d.id DESC
		LIMIT $` + fmt.Sprintf("%d", len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListDeploymentsByOrg: %w", err)
	}
	defer rows.Close()

	results := make([]DeploymentsByOrgRow, 0, limit)
	for rows.Next() {
		var d models.Deployment
		var row DeploymentsByOrgRow
		if err := rows.Scan(
			&d.ID, &d.ApplicationID, &d.EnvironmentID, &d.Strategy, &d.Status,
			&d.Artifact, &d.Version, &d.CommitSHA,
			&d.TrafficPercent, &d.PreviousDeploymentID, &d.FlagTestKey, &d.CreatedBy,
			&d.Mode, &d.Source,
			&d.StartedAt, &d.CompletedAt,
			&d.CreatedAt, &d.UpdatedAt,
			&row.ProjectID, &row.ProjectSlug, &row.ProjectName,
			&row.ApplicationSlug, &row.ApplicationName,
			&row.EnvironmentSlug, &row.EnvironmentName,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListDeploymentsByOrg scan: %w", err)
		}
		row.Deployment = &d
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	page := &DeploymentsByOrgPage{}
	if len(results) > limit {
		last := results[limit-1]
		page.NextCursor = encodeDeploymentsCursor(last.Deployment.CreatedAt, last.Deployment.ID)
		page.Rows = results[:limit]
	} else {
		page.Rows = results
	}
	return page, nil
}
