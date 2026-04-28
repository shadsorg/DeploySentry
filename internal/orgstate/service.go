// Package orgstate serves the org-wide status and deploy-history views.
//
// It layers on top of the per-app/env primitives shipped in the Agentless
// Deploy Reporting initiative — specifically `deployments` and `app_status`
// — to answer operator-level questions like "what's the state of every app
// × env in my org right now?" in a single HTTP round-trip.
package orgstate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/currentstate"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/platform/database/postgres"
	"github.com/google/uuid"
)

// ErrOrgNotFound is returned when the slug doesn't resolve to an org.
var ErrOrgNotFound = errors.New("organization not found")

// EntityLookup exposes the methods of entities.EntityService that this
// package needs. Narrowing the interface keeps test doubles small.
type EntityLookup interface {
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error)
}

// EnvironmentLookup matches the method on *entities.EnvironmentRepository
// used below; satisfied by the real repo.
type EnvironmentLookup interface {
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]entities.OrgEnvironment, error)
}

// OrgStatusBatcher is the subset of the org-status postgres repo used here.
type OrgStatusBatcher interface {
	ListLatestDeploymentsForApps(ctx context.Context, appIDs []uuid.UUID) (map[postgres.LatestDeploymentKey]*models.Deployment, error)
	ListAppStatusesForApps(ctx context.Context, appIDs []uuid.UUID) (map[postgres.LatestDeploymentKey]*models.AppStatus, error)
	ListDeploymentsByOrg(ctx context.Context, orgID uuid.UUID, f postgres.DeploymentsByOrgFilters, cursor string, limit int) (*postgres.DeploymentsByOrgPage, error)
	// ListLatestBuildsForApps returns the most recent deployment row per
	// (application, environment) where source starts with "github-actions".
	// Missing cells are omitted from the map.
	ListLatestBuildsForApps(ctx context.Context, appIDs []uuid.UUID) (map[postgres.LatestDeploymentKey]*models.Deployment, error)
}

// StatusService assembles /orgs/:slug/status responses.
type StatusService struct {
	entities EntityLookup
	envs     EnvironmentLookup
	batch    OrgStatusBatcher
	now      func() time.Time
}

func NewStatusService(e EntityLookup, envs EnvironmentLookup, batch OrgStatusBatcher) *StatusService {
	return &StatusService{entities: e, envs: envs, batch: batch, now: time.Now}
}

// Resolve returns the full org status grid for a viewer.
func (s *StatusService) Resolve(ctx context.Context, orgSlug string, userID uuid.UUID, orgRole string) (*models.OrgStatusResponse, error) {
	org, err := s.entities.GetOrgBySlug(ctx, orgSlug)
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}
	if org == nil {
		return nil, ErrOrgNotFound
	}

	projects, err := s.entities.ListProjectsByOrg(ctx, org.ID, false, userID, orgRole)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	envs, err := s.envs.ListByOrg(ctx, org.ID)
	if err != nil {
		return nil, fmt.Errorf("list envs: %w", err)
	}

	// Gather every visible app across every visible project.
	var allAppIDs []uuid.UUID
	appsByProject := make(map[uuid.UUID][]*models.Application, len(projects))
	for _, p := range projects {
		list, err := s.entities.ListAppsByProject(ctx, p.ID, false, userID, orgRole)
		if err != nil {
			return nil, fmt.Errorf("list apps for project %s: %w", p.ID, err)
		}
		appsByProject[p.ID] = list
		for _, a := range list {
			allAppIDs = append(allAppIDs, a.ID)
		}
	}

	latestDeploys, err := s.batch.ListLatestDeploymentsForApps(ctx, allAppIDs)
	if err != nil {
		return nil, fmt.Errorf("latest deploys: %w", err)
	}
	statuses, err := s.batch.ListAppStatusesForApps(ctx, allAppIDs)
	if err != nil {
		return nil, fmt.Errorf("app statuses: %w", err)
	}
	latestBuilds, err := s.batch.ListLatestBuildsForApps(ctx, allAppIDs)
	if err != nil {
		return nil, fmt.Errorf("latest builds: %w", err)
	}

	now := s.now()
	resp := &models.OrgStatusResponse{
		Org:         models.OrgSummary{ID: org.ID, Slug: org.Slug, Name: org.Name},
		GeneratedAt: now,
		Projects:    make([]models.OrgStatusProjectNode, 0, len(projects)),
	}

	for _, p := range projects {
		apps := appsByProject[p.ID]
		node := models.OrgStatusProjectNode{
			Project:      models.ProjectSummary{ID: p.ID, Slug: p.Slug, Name: p.Name},
			Applications: make([]models.OrgStatusApplicationNode, 0, len(apps)),
		}

		for _, a := range apps {
			appNode := models.OrgStatusApplicationNode{
				Application: models.ApplicationSummary{
					ID:              a.ID,
					Slug:            a.Slug,
					Name:            a.Name,
					MonitoringLinks: a.MonitoringLinks,
				},
				Envs: make([]models.OrgStatusEnvironmentCell, 0, len(envs)),
			}
			for _, e := range envs {
				cell := models.OrgStatusEnvironmentCell{
					Environment: models.EnvironmentSummary{
						ID:   e.ID,
						Slug: e.Slug,
						Name: e.Name,
					},
				}
				key := postgres.LatestDeploymentKey{ApplicationID: a.ID, EnvironmentID: e.ID}
				if d := latestDeploys[key]; d != nil {
					cell.CurrentDeployment = &models.OrgStatusDeploymentMini{
						ID:          d.ID,
						Version:     d.Version,
						CommitSHA:   d.CommitSHA,
						Status:      d.Status,
						Mode:        d.Mode,
						Source:      d.Source,
						CompletedAt: d.CompletedAt,
					}
				} else {
					cell.NeverDeployed = true
				}
				if build := latestBuilds[key]; build != nil {
					cell.LatestBuild = buildMiniFrom(build)
				}
				if status := statuses[key]; status != nil {
					reported := status.ReportedAt
					cell.Health = models.HealthBlock{
						State:          status.HealthState,
						Score:          status.HealthScore,
						Reason:         status.HealthReason,
						Source:         status.Source,
						LastReportedAt: &reported,
						Staleness:      classifyStaleness(now, reported),
					}
				} else {
					cell.Health = models.HealthBlock{
						State:     models.HealthStateUnknown,
						Source:    "unknown",
						Staleness: models.HealthStalenessMissing,
					}
				}
				appNode.Envs = append(appNode.Envs, cell)
			}
			node.Applications = append(node.Applications, appNode)
		}

		node.AggregateHealth = aggregateHealth(node.Applications)
		resp.Projects = append(resp.Projects, node)
	}

	return resp, nil
}

// buildMiniFrom unpacks a record-mode deployment whose source tag starts
// with "github-actions" into the slim `OrgStatusBuildMini` the UI renders
// as a build lane. The workflow name is stored after the "github-actions:"
// prefix (per internal/platform/database/postgres/deploy.go's
// UpsertBuildDeployment contract); when the prefix is missing (older rows
// or non-GH sources that happen to share the "github-actions" source),
// workflow_name is left empty and the UI falls back to a generic label.
// FlagTestKey is the column the ingester stashes html_url in for v1
// without a schema change.
func buildMiniFrom(d *models.Deployment) *models.OrgStatusBuildMini {
	out := &models.OrgStatusBuildMini{
		ID:          d.ID,
		Status:      d.Status,
		Version:     d.Version,
		CommitSHA:   d.CommitSHA,
		StartedAt:   d.StartedAt,
		CompletedAt: d.CompletedAt,
	}
	if d.Source != nil {
		s := *d.Source
		const prefix = "github-actions:"
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			out.WorkflowName = s[len(prefix):]
		}
	}
	if d.FlagTestKey != nil {
		out.HTMLURL = *d.FlagTestKey
	}
	return out
}

// classifyStaleness mirrors the rules used by internal/currentstate so the
// two surfaces always agree on fresh / stale / missing boundaries.
func classifyStaleness(now, reported time.Time) models.HealthStaleness {
	age := now.Sub(reported)
	switch {
	case age < currentstate.FreshnessWindow:
		return models.HealthStalenessFresh
	case age < currentstate.StalenessWindow:
		return models.HealthStalenessStale
	default:
		return models.HealthStalenessMissing
	}
}

// aggregateHealth rolls a project's app cells into a single worst-case
// indicator: unhealthy > degraded > unknown > healthy. An empty project
// (no apps) resolves to unknown.
func aggregateHealth(apps []models.OrgStatusApplicationNode) models.HealthState {
	if len(apps) == 0 {
		return models.HealthStateUnknown
	}
	worst := models.HealthStateHealthy
	severity := map[models.HealthState]int{
		models.HealthStateHealthy:   0,
		models.HealthStateUnknown:   1,
		models.HealthStateDegraded:  2,
		models.HealthStateUnhealthy: 3,
	}
	for _, a := range apps {
		for _, e := range a.Envs {
			if e.NeverDeployed {
				continue
			}
			if severity[e.Health.State] > severity[worst] {
				worst = e.Health.State
			}
		}
	}
	return worst
}
