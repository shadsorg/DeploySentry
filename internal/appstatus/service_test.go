package appstatus

import (
	"context"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockRepo struct {
	latest        *models.AppStatus
	history       []*models.AppStatusSample
	versionExists bool
	hasVersionErr error
}

func (m *mockRepo) UpsertStatus(_ context.Context, s *models.AppStatus) error {
	m.latest = s
	return nil
}

func (m *mockRepo) AppendHistory(_ context.Context, sample *models.AppStatusSample) error {
	m.history = append(m.history, sample)
	return nil
}

func (m *mockRepo) GetStatus(_ context.Context, _, _ uuid.UUID) (*models.AppStatus, error) {
	return m.latest, nil
}

func (m *mockRepo) HasDeploymentForVersion(_ context.Context, _, _ uuid.UUID, _ string) (bool, error) {
	if m.hasVersionErr != nil {
		return false, m.hasVersionErr
	}
	return m.versionExists, nil
}

type mockDeploy struct {
	created []*models.Deployment
}

func (m *mockDeploy) CreateDeployment(_ context.Context, d *models.Deployment) error {
	m.created = append(m.created, d)
	return nil
}

func validInput() ReportInput {
	return ReportInput{
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
		Payload: models.ReportStatusPayload{
			Version: "1.4.2",
			Health:  models.HealthStateHealthy,
		},
		Source:    "app-push",
		CreatedBy: uuid.New(),
	}
}

func TestReport_UpsertsAndHistory(t *testing.T) {
	repo := &mockRepo{versionExists: true} // no auto-deploy
	dep := &mockDeploy{}
	svc := NewService(repo, dep)

	status, err := svc.Report(context.Background(), validInput())
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.NotNil(t, repo.latest)
	assert.Equal(t, 1, len(repo.history))
	assert.Equal(t, 0, len(dep.created), "no deploy when version already exists")
}

func TestReport_AutoCreatesDeployOnNewVersion(t *testing.T) {
	repo := &mockRepo{versionExists: false}
	dep := &mockDeploy{}
	svc := NewService(repo, dep)

	in := validInput()
	in.Payload.CommitSHA = "abc123"
	status, err := svc.Report(context.Background(), in)
	assert.NoError(t, err)
	assert.NotNil(t, status)

	if assert.Equal(t, 1, len(dep.created), "should auto-create one deployment") {
		d := dep.created[0]
		assert.Equal(t, models.DeployModeRecord, d.Mode)
		assert.Equal(t, "1.4.2", d.Version)
		assert.Equal(t, "abc123", d.CommitSHA)
		if assert.NotNil(t, d.Source) {
			assert.Equal(t, "app-push", *d.Source)
		}
	}
}

func TestReport_DefaultSource(t *testing.T) {
	repo := &mockRepo{versionExists: false}
	dep := &mockDeploy{}
	svc := NewService(repo, dep)

	in := validInput()
	in.Source = "" // must fall back to default

	status, err := svc.Report(context.Background(), in)
	assert.NoError(t, err)
	assert.Equal(t, defaultSource, status.Source)
}

func TestReport_ValidatesPayload(t *testing.T) {
	repo := &mockRepo{}
	dep := &mockDeploy{}
	svc := NewService(repo, dep)

	in := validInput()
	in.Payload.Version = ""
	_, err := svc.Report(context.Background(), in)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version")

	in = validInput()
	in.Payload.Health = "nonsense"
	_, err = svc.Report(context.Background(), in)
	assert.Error(t, err)

	score := 1.5
	in = validInput()
	in.Payload.HealthScore = &score
	_, err = svc.Report(context.Background(), in)
	assert.Error(t, err)
}

func TestReport_RejectsNilIDs(t *testing.T) {
	svc := NewService(&mockRepo{}, &mockDeploy{})

	in := validInput()
	in.ApplicationID = uuid.Nil
	_, err := svc.Report(context.Background(), in)
	assert.Error(t, err)

	in = validInput()
	in.EnvironmentID = uuid.Nil
	_, err = svc.Report(context.Background(), in)
	assert.Error(t, err)
}
