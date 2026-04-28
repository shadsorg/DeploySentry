package orgstate

import (
	"context"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/platform/database/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Fakes
// -----------------------------------------------------------------------------

type fakeEntities struct {
	org      *models.Organization
	projects []*models.Project
	apps     map[uuid.UUID][]*models.Application
}

func (f *fakeEntities) GetOrgBySlug(_ context.Context, _ string) (*models.Organization, error) {
	return f.org, nil
}

func (f *fakeEntities) ListProjectsByOrg(_ context.Context, _ uuid.UUID, _ bool, _ uuid.UUID, _ string) ([]*models.Project, error) {
	return f.projects, nil
}

func (f *fakeEntities) ListAppsByProject(_ context.Context, projectID uuid.UUID, _ bool, _ uuid.UUID, _ string) ([]*models.Application, error) {
	return f.apps[projectID], nil
}

type fakeEnvs struct {
	envs []entities.OrgEnvironment
}

func (f *fakeEnvs) ListByOrg(_ context.Context, _ uuid.UUID) ([]entities.OrgEnvironment, error) {
	return f.envs, nil
}

type fakeBatch struct {
	deploys  map[postgres.LatestDeploymentKey]*models.Deployment
	statuses map[postgres.LatestDeploymentKey]*models.AppStatus
	builds   map[postgres.LatestDeploymentKey]*models.Deployment
	page     *postgres.DeploymentsByOrgPage
}

func (f *fakeBatch) ListLatestDeploymentsForApps(_ context.Context, _ []uuid.UUID) (map[postgres.LatestDeploymentKey]*models.Deployment, error) {
	return f.deploys, nil
}

func (f *fakeBatch) ListAppStatusesForApps(_ context.Context, _ []uuid.UUID) (map[postgres.LatestDeploymentKey]*models.AppStatus, error) {
	return f.statuses, nil
}

func (f *fakeBatch) ListLatestBuildsForApps(_ context.Context, _ []uuid.UUID) (map[postgres.LatestDeploymentKey]*models.Deployment, error) {
	return f.builds, nil
}

func (f *fakeBatch) ListDeploymentsByOrg(_ context.Context, _ uuid.UUID, _ postgres.DeploymentsByOrgFilters, _ string, _ int) (*postgres.DeploymentsByOrgPage, error) {
	return f.page, nil
}

// -----------------------------------------------------------------------------
// StatusService tests
// -----------------------------------------------------------------------------

func TestResolve_BuildsCompleteGrid(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	appID := uuid.New()
	envProdID := uuid.New()
	envStageID := uuid.New()

	now := time.Now().UTC()
	completed := now.Add(-30 * time.Second)

	deploys := map[postgres.LatestDeploymentKey]*models.Deployment{
		{ApplicationID: appID, EnvironmentID: envProdID}: {
			ID: uuid.New(), ApplicationID: appID, EnvironmentID: envProdID,
			Version: "1.4.2", Status: models.DeployStatusCompleted,
			Mode:        models.DeployModeRecord,
			CompletedAt: &completed,
		},
	}
	statuses := map[postgres.LatestDeploymentKey]*models.AppStatus{
		{ApplicationID: appID, EnvironmentID: envProdID}: {
			ApplicationID: appID, EnvironmentID: envProdID,
			HealthState: models.HealthStateHealthy, Source: "app-push",
			ReportedAt: now.Add(-10 * time.Second),
		},
	}

	svc := NewStatusService(
		&fakeEntities{
			org:      &models.Organization{ID: orgID, Slug: "acme", Name: "Acme"},
			projects: []*models.Project{{ID: projectID, Slug: "core", Name: "Core"}},
			apps: map[uuid.UUID][]*models.Application{
				projectID: {{ID: appID, Slug: "api", Name: "API", ProjectID: projectID}},
			},
		},
		&fakeEnvs{envs: []entities.OrgEnvironment{
			{ID: envProdID, Slug: "prod", Name: "Production"},
			{ID: envStageID, Slug: "stg", Name: "Staging"},
		}},
		&fakeBatch{deploys: deploys, statuses: statuses},
	)
	svc.now = func() time.Time { return now }

	resp, err := svc.Resolve(context.Background(), "acme", uuid.Nil, "owner")
	assert.NoError(t, err)
	assert.Equal(t, "acme", resp.Org.Slug)
	if assert.Len(t, resp.Projects, 1) {
		p := resp.Projects[0]
		assert.Equal(t, "core", p.Project.Slug)
		assert.Equal(t, models.HealthStateHealthy, p.AggregateHealth)
		if assert.Len(t, p.Applications, 1) {
			app := p.Applications[0]
			assert.Equal(t, "api", app.Application.Slug)
			if assert.Len(t, app.Envs, 2) {
				prodCell, stageCell := app.Envs[0], app.Envs[1]
				// prod has deploy + fresh health
				assert.False(t, prodCell.NeverDeployed)
				assert.NotNil(t, prodCell.CurrentDeployment)
				assert.Equal(t, "1.4.2", prodCell.CurrentDeployment.Version)
				assert.Equal(t, models.HealthStalenessFresh, prodCell.Health.Staleness)
				assert.Equal(t, models.HealthStateHealthy, prodCell.Health.State)
				// staging never deployed
				assert.True(t, stageCell.NeverDeployed)
				assert.Nil(t, stageCell.CurrentDeployment)
				assert.Equal(t, models.HealthStateUnknown, stageCell.Health.State)
				assert.Equal(t, models.HealthStalenessMissing, stageCell.Health.Staleness)
			}
		}
	}
}

func TestResolve_AggregateHealthRollsUpWorst(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	appID := uuid.New()
	envID := uuid.New()

	now := time.Now().UTC()
	svc := NewStatusService(
		&fakeEntities{
			org:      &models.Organization{ID: orgID, Slug: "x", Name: "X"},
			projects: []*models.Project{{ID: projectID, Slug: "p", Name: "P"}},
			apps:     map[uuid.UUID][]*models.Application{projectID: {{ID: appID, Slug: "a", Name: "A", ProjectID: projectID}}},
		},
		&fakeEnvs{envs: []entities.OrgEnvironment{{ID: envID, Slug: "prod", Name: "Production"}}},
		&fakeBatch{
			deploys: map[postgres.LatestDeploymentKey]*models.Deployment{
				{ApplicationID: appID, EnvironmentID: envID}: {ID: uuid.New(), Version: "1"},
			},
			statuses: map[postgres.LatestDeploymentKey]*models.AppStatus{
				{ApplicationID: appID, EnvironmentID: envID}: {
					HealthState: models.HealthStateDegraded, Source: "app-push",
					ReportedAt: now.Add(-10 * time.Second),
				},
			},
		},
	)
	svc.now = func() time.Time { return now }

	resp, err := svc.Resolve(context.Background(), "x", uuid.Nil, "owner")
	assert.NoError(t, err)
	assert.Equal(t, models.HealthStateDegraded, resp.Projects[0].AggregateHealth)
}

func TestResolve_OrgNotFound(t *testing.T) {
	svc := NewStatusService(
		&fakeEntities{org: nil},
		&fakeEnvs{},
		&fakeBatch{},
	)
	_, err := svc.Resolve(context.Background(), "missing", uuid.Nil, "owner")
	assert.ErrorIs(t, err, ErrOrgNotFound)
}

// -----------------------------------------------------------------------------
// DeploymentsService tests
// -----------------------------------------------------------------------------

func TestListDeployments_FiltersOutNonVisibleProjects(t *testing.T) {
	orgID := uuid.New()
	visibleProject := uuid.New()
	hiddenProject := uuid.New()
	appID := uuid.New()
	envID := uuid.New()

	page := &postgres.DeploymentsByOrgPage{
		Rows: []postgres.DeploymentsByOrgRow{
			{
				Deployment: &models.Deployment{ID: uuid.New(), ApplicationID: appID, EnvironmentID: envID, Version: "v1"},
				ProjectID:  visibleProject, ProjectSlug: "vis", ProjectName: "Visible",
				ApplicationSlug: "a", ApplicationName: "A",
				EnvironmentSlug: "prod", EnvironmentName: "Production",
			},
			{
				Deployment: &models.Deployment{ID: uuid.New(), ApplicationID: appID, EnvironmentID: envID, Version: "v2"},
				ProjectID:  hiddenProject, ProjectSlug: "hid", ProjectName: "Hidden",
				ApplicationSlug: "a", ApplicationName: "A",
				EnvironmentSlug: "prod", EnvironmentName: "Production",
			},
		},
	}
	svc := NewDeploymentsService(
		&fakeEntities{
			org:      &models.Organization{ID: orgID, Slug: "acme"},
			projects: []*models.Project{{ID: visibleProject, Slug: "vis"}},
		},
		&fakeBatch{page: page},
	)
	resp, err := svc.List(context.Background(), "acme", uuid.Nil, "member", ListParams{})
	assert.NoError(t, err)
	assert.Len(t, resp.Deployments, 1)
	assert.Equal(t, "v1", resp.Deployments[0].Deployment.Version)
}

func TestListDeployments_PassesThroughCursor(t *testing.T) {
	orgID := uuid.New()
	page := &postgres.DeploymentsByOrgPage{NextCursor: "cur-xyz"}
	svc := NewDeploymentsService(
		&fakeEntities{org: &models.Organization{ID: orgID, Slug: "x"}},
		&fakeBatch{page: page},
	)
	resp, err := svc.List(context.Background(), "x", uuid.Nil, "owner", ListParams{Cursor: "prior"})
	assert.NoError(t, err)
	assert.Equal(t, "cur-xyz", resp.NextCursor)
}
