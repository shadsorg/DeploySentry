package deploy

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockDeployRepo implements DeployRepository with in-memory storage.
type mockDeployRepo struct {
	mu          sync.Mutex
	deployments map[uuid.UUID]*models.Deployment
	createErr   error
	updateErr   error
}

func newMockRepo() *mockDeployRepo {
	return &mockDeployRepo{
		deployments: make(map[uuid.UUID]*models.Deployment),
	}
}

func (m *mockDeployRepo) CreateDeployment(_ context.Context, d *models.Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	m.deployments[d.ID] = d
	return nil
}

func (m *mockDeployRepo) GetDeployment(_ context.Context, id uuid.UUID) (*models.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}
	return d, nil
}

func (m *mockDeployRepo) ListDeployments(_ context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*models.Deployment
	for _, d := range m.deployments {
		if d.ProjectID == projectID {
			result = append(result, d)
		}
	}
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}
	return result, nil
}

func (m *mockDeployRepo) UpdateDeployment(_ context.Context, d *models.Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	m.deployments[d.ID] = d
	return nil
}

func (m *mockDeployRepo) ListDeploymentPhases(_ context.Context, _ uuid.UUID) ([]*models.DeploymentPhase, error) {
	return nil, nil
}

func (m *mockDeployRepo) CreateDeploymentPhase(_ context.Context, _ *models.DeploymentPhase) error {
	return nil
}

func (m *mockDeployRepo) UpdateDeploymentPhase(_ context.Context, _ *models.DeploymentPhase) error {
	return nil
}

func (m *mockDeployRepo) GetPipeline(_ context.Context, _ uuid.UUID) (*models.DeployPipeline, error) {
	return nil, fmt.Errorf("pipeline not found")
}

func (m *mockDeployRepo) GetLatestDeployment(_ context.Context, _, _ uuid.UUID) (*models.Deployment, error) {
	return nil, fmt.Errorf("no deployments found")
}

// mockPublisher implements MessagePublisher and records every call.
type mockPublisher struct {
	mu        sync.Mutex
	published []struct {
		subject string
		payload []byte
	}
}

func (m *mockPublisher) Publish(_ context.Context, subject string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, struct {
		subject string
		payload []byte
	}{subject: subject, payload: payload})
	return nil
}

func (m *mockPublisher) events() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	subjects := make([]string, len(m.published))
	for i, e := range m.published {
		subjects[i] = e.subject
	}
	return subjects
}

// ---------------------------------------------------------------------------
// Helper: build a valid Deployment with required fields populated.
// ---------------------------------------------------------------------------

func validDeployment() *models.Deployment {
	return &models.Deployment{
		ProjectID:     uuid.New(),
		EnvironmentID: uuid.New(),
		Strategy:      models.DeployStrategyCanary,
		Artifact:      "myapp:v1.2.3",
		Version:       "v1.2.3",
		CreatedBy:     uuid.New(),
	}
}

// ---------------------------------------------------------------------------
// Tests: CreateDeployment
// ---------------------------------------------------------------------------

func TestCreateDeployment_Success(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	err := svc.CreateDeployment(context.Background(), d)

	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, d.ID, "should assign a non-nil ID")
	assert.Equal(t, models.DeployStatusPending, d.Status, "status should be pending")
	assert.False(t, d.CreatedAt.IsZero(), "created_at should be set")
	assert.False(t, d.UpdatedAt.IsZero(), "updated_at should be set")

	// Verify it was persisted.
	stored, getErr := repo.GetDeployment(context.Background(), d.ID)
	assert.NoError(t, getErr)
	assert.Equal(t, d.ID, stored.ID)
}

func TestCreateDeployment_AssignsID(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	d.ID = uuid.Nil // ensure it starts nil

	err := svc.CreateDeployment(context.Background(), d)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, d.ID, "service should assign an ID when nil")
}

func TestCreateDeployment_PreservesExistingID(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	presetID := uuid.New()
	d := validDeployment()
	d.ID = presetID

	err := svc.CreateDeployment(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, presetID, d.ID, "should keep the pre-existing ID")
}

func TestCreateDeployment_ValidationError(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := &models.Deployment{} // missing required fields

	err := svc.CreateDeployment(context.Background(), d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestCreateDeployment_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	err := svc.CreateDeployment(context.Background(), d)
	assert.NoError(t, err)

	events := pub.events()
	assert.Contains(t, events, "deployments.deployment.created")
}

// ---------------------------------------------------------------------------
// Tests: GetDeployment
// ---------------------------------------------------------------------------

func TestGetDeployment_Existing(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	err := svc.CreateDeployment(context.Background(), d)
	assert.NoError(t, err)

	got, err := svc.GetDeployment(context.Background(), d.ID)
	assert.NoError(t, err)
	assert.Equal(t, d.ID, got.ID)
	assert.Equal(t, d.Artifact, got.Artifact)
}

func TestGetDeployment_Missing(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	_, err := svc.GetDeployment(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment")
}

// ---------------------------------------------------------------------------
// Tests: ListDeployments
// ---------------------------------------------------------------------------

func TestListDeployments_DefaultLimit(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	projectID := uuid.New()
	// Create a few deployments under the same project.
	for i := 0; i < 3; i++ {
		d := validDeployment()
		d.ProjectID = projectID
		err := svc.CreateDeployment(context.Background(), d)
		assert.NoError(t, err)
	}

	list, err := svc.ListDeployments(context.Background(), projectID, ListOptions{Limit: 0})
	assert.NoError(t, err)
	assert.Len(t, list, 3, "should return all 3 deployments")
}

func TestListDeployments_CapsAt100(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	projectID := uuid.New()
	d := validDeployment()
	d.ProjectID = projectID
	_ = svc.CreateDeployment(context.Background(), d)

	// Request with limit > 100 should be capped. The service should still
	// return results (it caps the limit, doesn't error).
	list, err := svc.ListDeployments(context.Background(), projectID, ListOptions{Limit: 200})
	assert.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestListDeployments_ReturnsResults(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	projectID := uuid.New()
	for i := 0; i < 5; i++ {
		d := validDeployment()
		d.ProjectID = projectID
		_ = svc.CreateDeployment(context.Background(), d)
	}

	list, err := svc.ListDeployments(context.Background(), projectID, ListOptions{Limit: 10})
	assert.NoError(t, err)
	assert.Len(t, list, 5)
}

// ---------------------------------------------------------------------------
// Tests: PromoteDeployment
// ---------------------------------------------------------------------------

func TestPromoteDeployment_Success(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)

	// Transition to running first (pending -> running is valid).
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.PromoteDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	updated, _ := repo.GetDeployment(context.Background(), d.ID)
	assert.Equal(t, models.DeployStatusPromoting, updated.Status)
	assert.Equal(t, 100, updated.TrafficPercent)
}

func TestPromoteDeployment_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	// Deployment is in "pending" state -- promoting from pending is not valid.

	err := svc.PromoteDeployment(context.Background(), d.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestPromoteDeployment_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.PromoteDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	events := pub.events()
	assert.Contains(t, events, "deployments.deployment.promoted")
}

// ---------------------------------------------------------------------------
// Tests: RollbackDeployment
// ---------------------------------------------------------------------------

func TestRollbackDeployment_Success(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.RollbackDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	updated, _ := repo.GetDeployment(context.Background(), d.ID)
	assert.Equal(t, models.DeployStatusRolledBack, updated.Status)
}

func TestRollbackDeployment_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	// Force a terminal state.
	d.Status = models.DeployStatusCompleted
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.RollbackDeployment(context.Background(), d.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestRollbackDeployment_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.RollbackDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	events := pub.events()
	assert.Contains(t, events, "deployments.deployment.rolled_back")
}

// ---------------------------------------------------------------------------
// Tests: PauseDeployment
// ---------------------------------------------------------------------------

func TestPauseDeployment_Success(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.PauseDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	updated, _ := repo.GetDeployment(context.Background(), d.ID)
	assert.Equal(t, models.DeployStatusPaused, updated.Status)
}

func TestPauseDeployment_RejectsNonRunning(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	// Deployment is in "pending" state.

	err := svc.PauseDeployment(context.Background(), d.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only running deployments can be paused")
}

func TestPauseDeployment_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.PauseDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	events := pub.events()
	assert.Contains(t, events, "deployments.deployment.paused")
}

// ---------------------------------------------------------------------------
// Tests: ResumeDeployment
// ---------------------------------------------------------------------------

func TestResumeDeployment_Success(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusPaused
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.ResumeDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	updated, _ := repo.GetDeployment(context.Background(), d.ID)
	assert.Equal(t, models.DeployStatusRunning, updated.Status)
}

func TestResumeDeployment_RejectsNonPaused(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusRunning
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.ResumeDeployment(context.Background(), d.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only paused deployments can be resumed")
}

func TestResumeDeployment_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := NewDeployService(repo, pub)

	d := validDeployment()
	_ = svc.CreateDeployment(context.Background(), d)
	d.Status = models.DeployStatusPaused
	_ = repo.UpdateDeployment(context.Background(), d)

	err := svc.ResumeDeployment(context.Background(), d.ID)
	assert.NoError(t, err)

	events := pub.events()
	assert.Contains(t, events, "deployments.deployment.resumed")
}
