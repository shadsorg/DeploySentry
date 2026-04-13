package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/deploy/engine"
	"github.com/deploysentry/deploysentry/internal/deploy/strategies"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Mock publisher
// ---------------------------------------------------------------------------

type mockPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
}

type publishedMsg struct {
	Subject string
	Payload []byte
}

func (m *mockPublisher) Publish(_ context.Context, subject string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, publishedMsg{Subject: subject, Payload: payload})
	return nil
}

func (m *mockPublisher) subjects() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.messages))
	for i, msg := range m.messages {
		out[i] = msg.Subject
	}
	return out
}

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type mockEngineRepo struct {
	mu          sync.RWMutex
	deployments map[uuid.UUID]*models.Deployment
	phases      map[uuid.UUID][]*models.DeploymentPhase // keyed by deploymentID
	rollbacks   []*models.RollbackRecord
}

func newMockEngineRepo() *mockEngineRepo {
	return &mockEngineRepo{
		deployments: make(map[uuid.UUID]*models.Deployment),
		phases:      make(map[uuid.UUID][]*models.DeploymentPhase),
	}
}

func (r *mockEngineRepo) CreateDeployment(_ context.Context, d *models.Deployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *d
	r.deployments[d.ID] = &copy
	return nil
}

func (r *mockEngineRepo) GetDeployment(_ context.Context, id uuid.UUID) (*models.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.deployments[id]
	if !ok {
		return nil, errors.New("deployment not found")
	}
	copy := *d
	return &copy, nil
}

func (r *mockEngineRepo) ListDeployments(_ context.Context, _ uuid.UUID, _ deploy.ListOptions) ([]*models.Deployment, error) {
	return nil, nil
}

func (r *mockEngineRepo) UpdateDeployment(_ context.Context, d *models.Deployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *d
	r.deployments[d.ID] = &copy
	return nil
}

func (r *mockEngineRepo) GetLatestDeployment(_ context.Context, _, _ uuid.UUID) (*models.Deployment, error) {
	return nil, nil
}

func (r *mockEngineRepo) GetLatestCompletedDeployment(_ context.Context, _, _ uuid.UUID) (*models.Deployment, error) {
	return nil, nil
}

func (r *mockEngineRepo) CreatePhase(_ context.Context, phase *models.DeploymentPhase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *phase
	r.phases[phase.DeploymentID] = append(r.phases[phase.DeploymentID], &copy)
	return nil
}

func (r *mockEngineRepo) ListPhases(_ context.Context, deploymentID uuid.UUID) ([]*models.DeploymentPhase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.phases[deploymentID]
	out := make([]*models.DeploymentPhase, len(list))
	for i, p := range list {
		copy := *p
		out[i] = &copy
	}
	return out, nil
}

func (r *mockEngineRepo) UpdatePhase(_ context.Context, phase *models.DeploymentPhase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.phases[phase.DeploymentID]
	for i, p := range list {
		if p.ID == phase.ID {
			copy := *phase
			list[i] = &copy
			return nil
		}
	}
	// Not found — append (shouldn't normally happen).
	copy := *phase
	r.phases[phase.DeploymentID] = append(list, &copy)
	return nil
}

func (r *mockEngineRepo) GetActivePhase(_ context.Context, deploymentID uuid.UUID) (*models.DeploymentPhase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.phases[deploymentID] {
		if p.Status == models.PhaseStatusActive {
			copy := *p
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *mockEngineRepo) CreateRollbackRecord(_ context.Context, record *models.RollbackRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *record
	r.rollbacks = append(r.rollbacks, &copy)
	return nil
}

func (r *mockEngineRepo) ListRollbackRecords(_ context.Context, _ uuid.UUID) ([]*models.RollbackRecord, error) {
	return nil, nil
}

func (r *mockEngineRepo) WithTx(_ context.Context, fn deploy.TxFunc) error {
	return fn(r)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestBuildPhases verifies that phases are created from a CanaryConfig with
// the correct field values.
func TestBuildPhases(t *testing.T) {
	deploymentID := uuid.New()
	config := strategies.CanaryConfig{
		Steps: []strategies.CanaryStep{
			{TrafficPercent: 10, Duration: 2 * time.Second},
			{TrafficPercent: 50, Duration: 5 * time.Second},
			{TrafficPercent: 100, Duration: 0},
		},
		AutoPromote:       true,
		RollbackOnFailure: true,
	}

	phases := engine.BuildPhases(deploymentID, config)

	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(phases))
	}

	cases := []struct {
		traffic     int
		duration    int
		autoPromote bool
		name        string
		sortOrder   int
	}{
		{10, 2, true, "canary-10%", 0},
		{50, 5, true, "canary-50%", 1},
		{100, 0, true, "canary-100%", 2},
	}

	for i, c := range cases {
		ph := phases[i]
		if ph.DeploymentID != deploymentID {
			t.Errorf("phase[%d] wrong deployment_id", i)
		}
		if ph.TrafficPercent != c.traffic {
			t.Errorf("phase[%d] traffic: want %d, got %d", i, c.traffic, ph.TrafficPercent)
		}
		if ph.Duration != c.duration {
			t.Errorf("phase[%d] duration: want %d, got %d", i, c.duration, ph.Duration)
		}
		if ph.AutoPromote != c.autoPromote {
			t.Errorf("phase[%d] auto_promote: want %v, got %v", i, c.autoPromote, ph.AutoPromote)
		}
		if ph.Name != c.name {
			t.Errorf("phase[%d] name: want %q, got %q", i, c.name, ph.Name)
		}
		if ph.SortOrder != c.sortOrder {
			t.Errorf("phase[%d] sort_order: want %d, got %d", i, c.sortOrder, ph.SortOrder)
		}
		if ph.Status != models.PhaseStatusPending {
			t.Errorf("phase[%d] status: want pending, got %s", i, ph.Status)
		}
		if ph.ID == uuid.Nil {
			t.Errorf("phase[%d] ID should not be nil", i)
		}
	}
}

// TestDriveDeployment_CanaryHappyPath verifies the full lifecycle:
// pending -> running -> phases -> completed.
func TestDriveDeployment_CanaryHappyPath(t *testing.T) {
	// Use zero-duration phases so the test is fast.
	strategies.SetDefaultCanaryConfigForTest(strategies.CanaryConfig{
		Steps: []strategies.CanaryStep{
			{TrafficPercent: 10, Duration: 0},
			{TrafficPercent: 50, Duration: 0},
			{TrafficPercent: 100, Duration: 0},
		},
		AutoPromote:       true,
		RollbackOnFailure: true,
	})
	t.Cleanup(func() {
		strategies.SetDefaultCanaryConfigForTest(strategies.CanaryConfig{
			Steps: []strategies.CanaryStep{
				{TrafficPercent: 1, Duration: 5 * time.Minute},
				{TrafficPercent: 5, Duration: 5 * time.Minute},
				{TrafficPercent: 25, Duration: 10 * time.Minute},
				{TrafficPercent: 50, Duration: 10 * time.Minute},
				{TrafficPercent: 100, Duration: 0},
			},
			HealthThreshold:   0.95,
			AutoPromote:       true,
			RollbackOnFailure: true,
		})
	})

	repo := newMockEngineRepo()
	pub := &mockPublisher{}
	eng := engine.New(repo, pub, nil, nil)

	// Create a pending canary deployment.
	appID := uuid.New()
	envID := uuid.New()
	createdBy := uuid.New()
	d := &models.Deployment{
		ID:            uuid.New(),
		ApplicationID: appID,
		EnvironmentID: envID,
		Strategy:      models.DeployStrategyCanary,
		Status:        models.DeployStatusPending,
		Artifact:      "registry.example.com/myapp",
		Version:       "v1.2.3",
		CreatedBy:     createdBy,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := repo.CreateDeployment(context.Background(), d); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	// Simulate what Start() would trigger when it receives the created event.
	ctx := context.Background()

	// Call driveDeployment indirectly via Advance is not possible yet since we
	// need it running.  We expose driveDeployment through a test shim by
	// constructing and calling it via the published event path. For simplicity,
	// use the exported BuildPhases + repo setup and drive directly.
	// Since driveDeployment is unexported we test via the exported Start path
	// using a captured subscriber.

	var capturedHandler func([]byte)
	type subRecord struct {
		subject string
		handler func([]byte)
	}
	var capturedSub subRecord

	mockSub := &mockSubscriber{
		onSubscribe: func(subject string, handler func([]byte)) error {
			capturedSub = subRecord{subject, handler}
			capturedHandler = handler
			return nil
		},
	}

	// Run Start in background, cancel after we've driven the deployment.
	startCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- eng.Start(startCtx, mockSub)
	}()

	// Wait for subscription to be registered.
	for capturedHandler == nil {
		time.Sleep(1 * time.Millisecond)
	}

	if capturedSub.subject != "deployments.deployment.created" {
		t.Fatalf("expected subscription to deployments.deployment.created, got %q", capturedSub.subject)
	}

	// Fire the event.
	eventPayload, _ := json.Marshal(map[string]string{"deployment_id": d.ID.String()})
	capturedHandler(eventPayload)

	// Give the goroutine time to finish processing.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := repo.GetDeployment(ctx, d.ID)
		if got.Status == models.DeployStatusCompleted {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	<-done

	// Verify final deployment state.
	final, err := repo.GetDeployment(ctx, d.ID)
	if err != nil {
		t.Fatalf("get final deployment: %v", err)
	}
	if final.Status != models.DeployStatusCompleted {
		t.Errorf("expected status completed, got %s", final.Status)
	}
	if final.TrafficPercent != 100 {
		t.Errorf("expected traffic_percent=100, got %d", final.TrafficPercent)
	}

	// Verify phases were created and all passed.
	phases, _ := repo.ListPhases(ctx, d.ID)
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(phases))
	}
	for i, ph := range phases {
		if ph.Status != models.PhaseStatusPassed {
			t.Errorf("phase[%d] expected passed, got %s", i, ph.Status)
		}
		if ph.StartedAt == nil {
			t.Errorf("phase[%d] started_at should not be nil", i)
		}
		if ph.CompletedAt == nil {
			t.Errorf("phase[%d] completed_at should not be nil", i)
		}
	}

	// Verify events were published.
	subjects := pub.subjects()
	phaseChangedCount := 0
	completedCount := 0
	for _, s := range subjects {
		switch s {
		case "deployments.deployment.phase_changed":
			phaseChangedCount++
		case "deployments.deployment.completed":
			completedCount++
		}
	}
	if phaseChangedCount < 3 {
		t.Errorf("expected at least 3 phase_changed events, got %d", phaseChangedCount)
	}
	if completedCount != 1 {
		t.Errorf("expected 1 completed event, got %d", completedCount)
	}
}

// mockSubscriber is used to capture the subscription in tests.
type mockSubscriber struct {
	onSubscribe func(subject string, handler func([]byte)) error
}

func (m *mockSubscriber) Subscribe(subject string, handler func(msg []byte)) error {
	return m.onSubscribe(subject, handler)
}
