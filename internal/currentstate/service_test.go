package currentstate

import (
	"context"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/deploy"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeDeploys struct {
	active []*models.Deployment
	recent []*models.Deployment
}

func (f *fakeDeploys) GetActiveDeployments(_ context.Context, _ uuid.UUID) ([]*models.Deployment, error) {
	return f.active, nil
}

func (f *fakeDeploys) ListDeployments(_ context.Context, _ uuid.UUID, opts deploy.ListOptions) ([]*models.Deployment, error) {
	if opts.Limit > 0 && len(f.recent) > opts.Limit {
		return f.recent[:opts.Limit], nil
	}
	return f.recent, nil
}

type fakeStatus struct {
	status *models.AppStatus
	err    error
}

func (f *fakeStatus) GetStatus(_ context.Context, _, _ uuid.UUID) (*models.AppStatus, error) {
	return f.status, f.err
}

type fakeEnvs struct {
	env *entities.OrgEnvironment
}

func (f *fakeEnvs) GetByID(_ context.Context, _ uuid.UUID) (*entities.OrgEnvironment, error) {
	return f.env, nil
}

func sampleEnv() *entities.OrgEnvironment {
	return &entities.OrgEnvironment{ID: uuid.New(), Slug: "production", Name: "Production"}
}

func sampleDeploy(t time.Time) *models.Deployment {
	completed := t
	return &models.Deployment{
		ID:             uuid.New(),
		Version:        "1.4.2",
		CommitSHA:      "abc123",
		Status:         models.DeployStatusCompleted,
		Mode:           models.DeployModeRecord,
		TrafficPercent: 100,
		StartedAt:      &completed,
		CompletedAt:    &completed,
		CreatedAt:      t,
		EnvironmentID:  uuid.New(),
	}
}

func TestResolve_HappyPath(t *testing.T) {
	env := sampleEnv()
	now := time.Now().UTC()
	dep := sampleDeploy(now.Add(-2 * time.Minute))
	dep.EnvironmentID = env.ID

	appID := uuid.New()
	svc := NewService(
		&fakeDeploys{recent: []*models.Deployment{dep}},
		&fakeStatus{status: &models.AppStatus{
			ApplicationID: appID,
			EnvironmentID: env.ID,
			Version:       "1.4.2",
			HealthState:   models.HealthStateHealthy,
			Source:        "app-push",
			ReportedAt:    now.Add(-20 * time.Second),
		}},
		&fakeEnvs{env: env},
	)
	svc.now = func() time.Time { return now }

	resp, err := svc.Resolve(context.Background(), appID, env.ID, 10)
	assert.NoError(t, err)
	assert.Equal(t, env.ID, resp.Environment.ID)
	assert.Equal(t, "production", resp.Environment.Slug)
	if assert.NotNil(t, resp.CurrentDeployment) {
		assert.Equal(t, "1.4.2", resp.CurrentDeployment.Version)
	}
	assert.Equal(t, models.HealthStateHealthy, resp.Health.State)
	assert.Equal(t, models.HealthStalenessFresh, resp.Health.Staleness)
	assert.Equal(t, "app-push", resp.Health.Source)
	assert.Len(t, resp.RecentDeployments, 1)
}

func TestResolve_NoStatus_UnknownMissing(t *testing.T) {
	env := sampleEnv()
	appID := uuid.New()
	svc := NewService(&fakeDeploys{}, &fakeStatus{status: nil}, &fakeEnvs{env: env})

	resp, err := svc.Resolve(context.Background(), appID, env.ID, 10)
	assert.NoError(t, err)
	assert.Equal(t, models.HealthStateUnknown, resp.Health.State)
	assert.Equal(t, models.HealthStalenessMissing, resp.Health.Staleness)
	assert.Equal(t, "unknown", resp.Health.Source)
	assert.Nil(t, resp.CurrentDeployment)
	assert.Empty(t, resp.RecentDeployments)
}

func TestResolve_StaleStatus(t *testing.T) {
	env := sampleEnv()
	now := time.Now().UTC()
	appID := uuid.New()

	svc := NewService(
		&fakeDeploys{},
		&fakeStatus{status: &models.AppStatus{
			ApplicationID: appID, EnvironmentID: env.ID,
			HealthState: models.HealthStateHealthy, Source: "app-push",
			ReportedAt: now.Add(-3 * time.Minute),
		}},
		&fakeEnvs{env: env},
	)
	svc.now = func() time.Time { return now }

	resp, err := svc.Resolve(context.Background(), appID, env.ID, 10)
	assert.NoError(t, err)
	assert.Equal(t, models.HealthStalenessStale, resp.Health.Staleness)
}

func TestResolve_OldSampleIsMissing(t *testing.T) {
	env := sampleEnv()
	now := time.Now().UTC()
	appID := uuid.New()
	svc := NewService(
		&fakeDeploys{},
		&fakeStatus{status: &models.AppStatus{
			ApplicationID: appID, EnvironmentID: env.ID,
			HealthState: models.HealthStateHealthy, Source: "app-push",
			ReportedAt: now.Add(-10 * time.Minute),
		}},
		&fakeEnvs{env: env},
	)
	svc.now = func() time.Time { return now }

	resp, err := svc.Resolve(context.Background(), appID, env.ID, 10)
	assert.NoError(t, err)
	assert.Equal(t, models.HealthStalenessMissing, resp.Health.Staleness)
}

func TestResolve_EnvNotFound(t *testing.T) {
	appID := uuid.New()
	svc := NewService(&fakeDeploys{}, &fakeStatus{}, &fakeEnvs{env: nil})

	_, err := svc.Resolve(context.Background(), appID, uuid.New(), 10)
	assert.ErrorIs(t, err, ErrEnvNotFound)
}

func TestResolve_LimitClampsAndFallsBack(t *testing.T) {
	env := sampleEnv()
	appID := uuid.New()

	svc := NewService(
		&fakeDeploys{recent: []*models.Deployment{sampleDeploy(time.Now())}},
		&fakeStatus{},
		&fakeEnvs{env: env},
	)
	resp, err := svc.Resolve(context.Background(), appID, env.ID, 0) // 0 -> default 10
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// CurrentDeployment falls back to the most recent when no active match.
	assert.NotNil(t, resp.CurrentDeployment)
}
