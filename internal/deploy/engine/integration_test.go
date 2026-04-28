//go:build integration

package engine_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/deploy/engine"
	"github.com/shadsorg/deploysentry/internal/deploy/strategies"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// TestIntegration_FullCanaryFlow exercises the full canary deployment lifecycle
// using mocks — no real DB or NATS required.
//
// Flow:
//  1. Create a mock deployment in pending state
//  2. Create a mock subscriber that delivers the deployment.created event
//  3. Start the engine
//  4. Wait for completion
//  5. Assert deployment is completed with 100% traffic
//  6. Assert phase_changed events were published for each phase
//  7. Assert deployment.completed event was published
func TestIntegration_FullCanaryFlow(t *testing.T) {
	// Override DefaultCanaryConfig with zero-duration phases for speed.
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

	// 1. Create a mock deployment in pending state.
	d := &models.Deployment{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
		Strategy:      models.DeployStrategyCanary,
		Status:        models.DeployStatusPending,
		Artifact:      "registry.example.com/app",
		Version:       "v2.0.0",
		CreatedBy:     uuid.New(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := repo.CreateDeployment(context.Background(), d); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	// 2. Create a mock subscriber that delivers the deployment.created event.
	var capturedHandler func([]byte)
	mockSub := &mockSubscriber{
		onSubscribe: func(subject string, handler func([]byte)) error {
			capturedHandler = handler
			return nil
		},
	}

	// 3. Start the engine.
	startCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- eng.Start(startCtx, mockSub)
	}()

	// Wait for subscription to be registered.
	deadline := time.Now().Add(2 * time.Second)
	for capturedHandler == nil && time.Now().Before(deadline) {
		time.Sleep(1 * time.Millisecond)
	}
	if capturedHandler == nil {
		t.Fatal("engine did not register subscription in time")
	}

	// Fire the deployment.created event.
	eventPayload, _ := json.Marshal(map[string]string{"deployment_id": d.ID.String()})
	capturedHandler(eventPayload)

	// 4. Wait for completion (up to 5 seconds).
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := repo.GetDeployment(context.Background(), d.ID)
		if got != nil && got.Status == models.DeployStatusCompleted {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	<-done

	// 5. Assert deployment is completed with 100% traffic.
	final, err := repo.GetDeployment(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("get final deployment: %v", err)
	}
	if final.Status != models.DeployStatusCompleted {
		t.Errorf("expected status completed, got %s", final.Status)
	}
	if final.TrafficPercent != 100 {
		t.Errorf("expected traffic_percent=100, got %d", final.TrafficPercent)
	}

	// Assert all phases passed.
	phases, _ := repo.ListPhases(context.Background(), d.ID)
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(phases))
	}
	for i, ph := range phases {
		if ph.Status != models.DeploymentPhaseStatusPassed {
			t.Errorf("phase[%d] expected passed, got %s", i, ph.Status)
		}
	}

	// 6. Assert phase_changed events were published for each phase.
	// 7. Assert deployment.completed event was published.
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

// TestIntegration_ManualGate tests a deployment that pauses at a manual-gate
// phase and only completes after engine.Advance() is called.
//
// Flow:
//  1. Set up config with AutoPromote=false on phase 2
//  2. Start engine
//  3. Verify deployment pauses at phase 2
//  4. Call engine.Advance()
//  5. Verify deployment completes
func TestIntegration_ManualGate(t *testing.T) {
	autoPromoteFalse := false
	autoPromoteTrue := true

	strategies.SetDefaultCanaryConfigForTest(strategies.CanaryConfig{
		Steps: []strategies.CanaryStep{
			{TrafficPercent: 10, Duration: 0, AutoPromote: &autoPromoteTrue},
			{TrafficPercent: 50, Duration: 0, AutoPromote: &autoPromoteFalse}, // manual gate
			{TrafficPercent: 100, Duration: 0, AutoPromote: &autoPromoteTrue},
		},
		AutoPromote:       true,
		RollbackOnFailure: false,
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

	// 1. Create a pending deployment.
	d := &models.Deployment{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
		Strategy:      models.DeployStrategyCanary,
		Status:        models.DeployStatusPending,
		Artifact:      "registry.example.com/app",
		Version:       "v3.0.0",
		CreatedBy:     uuid.New(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := repo.CreateDeployment(context.Background(), d); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	// 2. Start engine with mock subscriber.
	var capturedHandler func([]byte)
	mockSub := &mockSubscriber{
		onSubscribe: func(subject string, handler func([]byte)) error {
			capturedHandler = handler
			return nil
		},
	}

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- eng.Start(startCtx, mockSub)
	}()

	// Wait for subscription to be registered.
	deadline := time.Now().Add(2 * time.Second)
	for capturedHandler == nil && time.Now().Before(deadline) {
		time.Sleep(1 * time.Millisecond)
	}
	if capturedHandler == nil {
		t.Fatal("engine did not register subscription in time")
	}

	// Fire the deployment.created event.
	eventPayload, _ := json.Marshal(map[string]string{"deployment_id": d.ID.String()})
	capturedHandler(eventPayload)

	// 3. Verify deployment pauses at phase 2 (the manual gate).
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := repo.GetDeployment(context.Background(), d.ID)
		if got != nil && got.Status == models.DeployStatusPaused {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	paused, err := repo.GetDeployment(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("get paused deployment: %v", err)
	}
	if paused.Status != models.DeployStatusPaused {
		t.Fatalf("expected deployment to be paused, got %s", paused.Status)
	}

	// 4. Call engine.Advance() to unblock the manual gate.
	if err := eng.Advance(context.Background(), d.ID); err != nil {
		t.Fatalf("engine.Advance: %v", err)
	}

	// 5. Verify deployment completes.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := repo.GetDeployment(context.Background(), d.ID)
		if got != nil && got.Status == models.DeployStatusCompleted {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	final, err := repo.GetDeployment(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("get final deployment: %v", err)
	}
	if final.Status != models.DeployStatusCompleted {
		t.Errorf("expected deployment completed after Advance, got %s", final.Status)
	}
	if final.TrafficPercent != 100 {
		t.Errorf("expected traffic_percent=100, got %d", final.TrafficPercent)
	}
}
