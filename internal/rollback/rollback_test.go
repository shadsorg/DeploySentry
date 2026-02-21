package rollback

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockExecutor struct {
	executeErr error
	executed   []uuid.UUID
	mu         sync.Mutex
}

func (m *mockExecutor) Execute(ctx context.Context, deploymentID uuid.UUID, strategy RollbackStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = append(m.executed, deploymentID)
	return m.executeErr
}

type mockRollbackListener struct {
	decisions []*RollbackDecision
	mu        sync.Mutex
}

func (m *mockRollbackListener) OnRollbackStateChange(ctx context.Context, decision *RollbackDecision) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions = append(m.decisions, decision)
}

func (m *mockRollbackListener) getDecisions() []*RollbackDecision {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*RollbackDecision, len(m.decisions))
	copy(cp, m.decisions)
	return cp
}

type mockStrategy struct {
	name string
}

func (m *mockStrategy) Name() string { return m.name }
func (m *mockStrategy) Execute(ctx context.Context, deploymentID uuid.UUID) error {
	return nil
}

// ---------------------------------------------------------------------------
// Strategy Tests
// ---------------------------------------------------------------------------

// ---- ImmediateRollbackStrategy ----

func TestImmediateRollbackStrategy_Name(t *testing.T) {
	s := NewImmediateRollbackStrategy()
	assert.Equal(t, "immediate", s.Name())
}

func TestImmediateRollbackStrategy_Execute_ValidContext(t *testing.T) {
	s := NewImmediateRollbackStrategy()
	err := s.Execute(context.Background(), uuid.New())
	assert.NoError(t, err)
}

func TestImmediateRollbackStrategy_Execute_CancelledContext(t *testing.T) {
	s := NewImmediateRollbackStrategy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Execute(ctx, uuid.New())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// ---- GradualRollbackStrategy ----

func TestGradualRollbackStrategy_Name(t *testing.T) {
	s := NewGradualRollbackStrategy(DefaultGradualRollbackConfig())
	assert.Equal(t, "gradual", s.Name())
}

func TestGradualRollbackStrategy_Execute_CancelledContext(t *testing.T) {
	s := NewGradualRollbackStrategy(DefaultGradualRollbackConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Execute(ctx, uuid.New())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGradualRollbackStrategy_Execute_ZeroDelaySteps(t *testing.T) {
	config := GradualRollbackConfig{
		Steps:     []int{75, 50, 25, 0},
		StepDelay: 0,
	}
	s := NewGradualRollbackStrategy(config)
	err := s.Execute(context.Background(), uuid.New())
	assert.NoError(t, err)
}

func TestDefaultGradualRollbackConfig(t *testing.T) {
	config := DefaultGradualRollbackConfig()
	assert.Equal(t, []int{50, 25, 10, 0}, config.Steps)
	assert.Equal(t, 30*time.Second, config.StepDelay)
	assert.NotEmpty(t, config.Steps)
}

// ---- CanaryRollbackStrategy ----

func TestCanaryRollbackStrategy_Name(t *testing.T) {
	s := NewCanaryRollbackStrategy()
	assert.Equal(t, "canary_rollback", s.Name())
}

func TestCanaryRollbackStrategy_Execute_ValidContext(t *testing.T) {
	s := NewCanaryRollbackStrategy()
	err := s.Execute(context.Background(), uuid.New())
	assert.NoError(t, err)
}

func TestCanaryRollbackStrategy_Execute_CancelledContext(t *testing.T) {
	s := NewCanaryRollbackStrategy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Execute(ctx, uuid.New())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// ---------------------------------------------------------------------------
// RollbackController Tests
// ---------------------------------------------------------------------------

func newTestController(executor *mockExecutor, evalWindow time.Duration) *RollbackController {
	strategy := &mockStrategy{name: "test_strategy"}
	return NewRollbackController(executor, strategy, 0.5, evalWindow)
}

// ---- NewRollbackController ----

func TestNewRollbackController(t *testing.T) {
	executor := &mockExecutor{}
	strategy := &mockStrategy{name: "my_strategy"}
	threshold := 0.7
	evalWindow := 5 * time.Second

	rc := NewRollbackController(executor, strategy, threshold, evalWindow)

	assert.NotNil(t, rc)
	assert.Equal(t, threshold, rc.healthThreshold)
	assert.Equal(t, evalWindow, rc.evaluationWindow)
	assert.NotNil(t, rc.states)
	assert.Empty(t, rc.states)
	assert.Empty(t, rc.listeners)
}

// ---- Watch / Unwatch ----

func TestWatch_AddsDeployment(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id := uuid.New()

	rc.Watch(id)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateHealthy, state)
}

func TestUnwatch_RemovesDeployment(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id := uuid.New()

	rc.Watch(id)
	rc.Unwatch(id)

	_, err := rc.GetState(id)
	assert.Error(t, err)
}

func TestWatch_MultipleDeployments(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id1 := uuid.New()
	id2 := uuid.New()

	rc.Watch(id1)
	rc.Watch(id2)

	state1, err1 := rc.GetState(id1)
	state2, err2 := rc.GetState(id2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, RollbackStateHealthy, state1)
	assert.Equal(t, RollbackStateHealthy, state2)
}

// ---- GetState ----

func TestGetState_WatchedDeployment(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id := uuid.New()
	rc.Watch(id)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateHealthy, state)
}

func TestGetState_UnwatchedDeployment(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id := uuid.New()

	_, err := rc.GetState(id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not being monitored")
}

// ---- AddListener ----

func TestAddListener(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	listener := &mockRollbackListener{}

	rc.AddListener(listener)

	rc.mu.RLock()
	assert.Len(t, rc.listeners, 1)
	rc.mu.RUnlock()
}

func TestAddListener_Multiple(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	l1 := &mockRollbackListener{}
	l2 := &mockRollbackListener{}

	rc.AddListener(l1)
	rc.AddListener(l2)

	rc.mu.RLock()
	assert.Len(t, rc.listeners, 2)
	rc.mu.RUnlock()
}

// ---------------------------------------------------------------------------
// OnHealthUpdate Tests
// ---------------------------------------------------------------------------

func TestOnHealthUpdate_HealthyStaysHealthy(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id := uuid.New()
	rc.Watch(id)

	// Score above threshold (0.5) should keep deployment healthy.
	dh := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.9,
		Healthy:      true,
	}
	rc.OnHealthUpdate(context.Background(), dh)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateHealthy, state)
}

func TestOnHealthUpdate_HealthyToEvaluating(t *testing.T) {
	rc := newTestController(&mockExecutor{}, 10*time.Second)
	id := uuid.New()
	rc.Watch(id)
	listener := &mockRollbackListener{}
	rc.AddListener(listener)

	// Score below threshold should transition to evaluating.
	dh := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.3,
		Healthy:      false,
	}
	rc.OnHealthUpdate(context.Background(), dh)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateEvaluating, state)

	decisions := listener.getDecisions()
	assert.Len(t, decisions, 1)
	assert.Equal(t, RollbackStateEvaluating, decisions[0].State)
	assert.True(t, decisions[0].Automatic)
}

func TestOnHealthUpdate_EvaluatingToHealthy(t *testing.T) {
	rc := newTestController(&mockExecutor{}, 10*time.Second)
	id := uuid.New()
	rc.Watch(id)
	listener := &mockRollbackListener{}
	rc.AddListener(listener)

	// First: transition to evaluating.
	dhUnhealthy := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.3,
		Healthy:      false,
	}
	rc.OnHealthUpdate(context.Background(), dhUnhealthy)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateEvaluating, state)

	// Then: recover with good health score.
	dhHealthy := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.8,
		Healthy:      true,
	}
	rc.OnHealthUpdate(context.Background(), dhHealthy)

	state, err = rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateHealthy, state)

	decisions := listener.getDecisions()
	assert.Len(t, decisions, 2)
	assert.Equal(t, RollbackStateEvaluating, decisions[0].State)
	assert.Equal(t, RollbackStateHealthy, decisions[1].State)
	assert.Contains(t, decisions[1].Reason, "health recovered")
}

func TestOnHealthUpdate_EvaluatingToRollingBackToRolledBack(t *testing.T) {
	executor := &mockExecutor{}
	// Use a very short evaluation window so it expires immediately.
	rc := newTestController(executor, 1*time.Millisecond)
	id := uuid.New()
	rc.Watch(id)
	listener := &mockRollbackListener{}
	rc.AddListener(listener)

	// First: transition to evaluating.
	dhUnhealthy := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.2,
		Healthy:      false,
	}
	rc.OnHealthUpdate(context.Background(), dhUnhealthy)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateEvaluating, state)

	// Wait for the evaluation window to expire.
	time.Sleep(5 * time.Millisecond)

	// Second unhealthy update after evaluation window should trigger rollback.
	rc.OnHealthUpdate(context.Background(), dhUnhealthy)

	state, err = rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateRolledBack, state)

	// Executor should have been called.
	executor.mu.Lock()
	assert.Len(t, executor.executed, 1)
	assert.Equal(t, id, executor.executed[0])
	executor.mu.Unlock()

	// Listener should have received: EVALUATING, ROLLING_BACK, ROLLED_BACK.
	decisions := listener.getDecisions()
	assert.GreaterOrEqual(t, len(decisions), 3)
	assert.Equal(t, RollbackStateEvaluating, decisions[0].State)
	assert.Equal(t, RollbackStateRollingBack, decisions[1].State)
	assert.Equal(t, RollbackStateRolledBack, decisions[2].State)
	assert.True(t, decisions[1].Automatic)
}

func TestOnHealthUpdate_UnwatchedDeploymentIgnored(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	unwatchedID := uuid.New()

	dh := &health.DeploymentHealth{
		DeploymentID: unwatchedID,
		Overall:      0.1,
		Healthy:      false,
	}

	// Should not panic or error for unwatched deployment.
	rc.OnHealthUpdate(context.Background(), dh)

	_, err := rc.GetState(unwatchedID)
	assert.Error(t, err)
}

func TestOnHealthUpdate_NotifiesListenersOnStateChange(t *testing.T) {
	rc := newTestController(&mockExecutor{}, 10*time.Second)
	id := uuid.New()
	rc.Watch(id)

	l1 := &mockRollbackListener{}
	l2 := &mockRollbackListener{}
	rc.AddListener(l1)
	rc.AddListener(l2)

	dh := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.2,
		Healthy:      false,
	}
	rc.OnHealthUpdate(context.Background(), dh)

	// Both listeners should receive the state change.
	d1 := l1.getDecisions()
	d2 := l2.getDecisions()
	assert.Len(t, d1, 1)
	assert.Len(t, d2, 1)
	assert.Equal(t, RollbackStateEvaluating, d1[0].State)
	assert.Equal(t, RollbackStateEvaluating, d2[0].State)
}

// ---------------------------------------------------------------------------
// TriggerManualRollback Tests
// ---------------------------------------------------------------------------

func TestTriggerManualRollback_WatchedHealthyDeployment(t *testing.T) {
	executor := &mockExecutor{}
	rc := newTestController(executor, time.Second)
	id := uuid.New()
	rc.Watch(id)

	err := rc.TriggerManualRollback(context.Background(), id, "operator requested rollback")
	assert.NoError(t, err)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateRolledBack, state)

	executor.mu.Lock()
	assert.Len(t, executor.executed, 1)
	assert.Equal(t, id, executor.executed[0])
	executor.mu.Unlock()
}

func TestTriggerManualRollback_WatchedEvaluatingDeployment(t *testing.T) {
	executor := &mockExecutor{}
	rc := newTestController(executor, 10*time.Second)
	id := uuid.New()
	rc.Watch(id)

	// First transition to evaluating.
	dh := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.3,
		Healthy:      false,
	}
	rc.OnHealthUpdate(context.Background(), dh)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateEvaluating, state)

	// Manual rollback from evaluating state.
	err = rc.TriggerManualRollback(context.Background(), id, "manual override")
	assert.NoError(t, err)

	state, err = rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateRolledBack, state)

	executor.mu.Lock()
	assert.Len(t, executor.executed, 1)
	executor.mu.Unlock()
}

func TestTriggerManualRollback_UnwatchedDeployment(t *testing.T) {
	rc := newTestController(&mockExecutor{}, time.Second)
	id := uuid.New()

	err := rc.TriggerManualRollback(context.Background(), id, "rollback")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not being monitored")
}

func TestTriggerManualRollback_AlreadyRollingBack(t *testing.T) {
	executor := &mockExecutor{}
	rc := newTestController(executor, 1*time.Millisecond)
	id := uuid.New()
	rc.Watch(id)

	// Transition to evaluating.
	dh := &health.DeploymentHealth{
		DeploymentID: id,
		Overall:      0.2,
		Healthy:      false,
	}
	rc.OnHealthUpdate(context.Background(), dh)

	// Wait for evaluation window to expire.
	time.Sleep(5 * time.Millisecond)

	// Trigger automatic rollback.
	rc.OnHealthUpdate(context.Background(), dh)

	// Now the state should be ROLLED_BACK. To test ROLLING_BACK specifically,
	// we manually set it.
	rc.mu.Lock()
	rbCtx := rc.states[id]
	rbCtx.state = RollbackStateRollingBack
	rc.mu.Unlock()

	err := rc.TriggerManualRollback(context.Background(), id, "try again")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in state")
}

func TestTriggerManualRollback_AlreadyRolledBack(t *testing.T) {
	executor := &mockExecutor{}
	rc := newTestController(executor, time.Second)
	id := uuid.New()
	rc.Watch(id)

	// First manual rollback.
	err := rc.TriggerManualRollback(context.Background(), id, "first rollback")
	assert.NoError(t, err)

	state, err := rc.GetState(id)
	assert.NoError(t, err)
	assert.Equal(t, RollbackStateRolledBack, state)

	// Second manual rollback should fail.
	err = rc.TriggerManualRollback(context.Background(), id, "second rollback")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in state")
}

func TestTriggerManualRollback_NotifiesListeners(t *testing.T) {
	executor := &mockExecutor{}
	rc := newTestController(executor, time.Second)
	id := uuid.New()
	rc.Watch(id)
	listener := &mockRollbackListener{}
	rc.AddListener(listener)

	err := rc.TriggerManualRollback(context.Background(), id, "manual rollback reason")
	assert.NoError(t, err)

	decisions := listener.getDecisions()
	// Should receive ROLLING_BACK and ROLLED_BACK notifications.
	assert.Len(t, decisions, 2)
	assert.Equal(t, RollbackStateRollingBack, decisions[0].State)
	assert.False(t, decisions[0].Automatic)
	assert.Equal(t, "manual rollback reason", decisions[0].Reason)
	assert.Equal(t, RollbackStateRolledBack, decisions[1].State)
	assert.False(t, decisions[1].Automatic)
}

func TestTriggerManualRollback_CallsExecutor(t *testing.T) {
	executor := &mockExecutor{}
	rc := newTestController(executor, time.Second)
	id := uuid.New()
	rc.Watch(id)

	err := rc.TriggerManualRollback(context.Background(), id, "do it")
	assert.NoError(t, err)

	executor.mu.Lock()
	defer executor.mu.Unlock()
	assert.Len(t, executor.executed, 1)
	assert.Equal(t, id, executor.executed[0])
}

func TestTriggerManualRollback_ExecutorError(t *testing.T) {
	executor := &mockExecutor{executeErr: fmt.Errorf("infra failure")}
	rc := newTestController(executor, time.Second)
	id := uuid.New()
	rc.Watch(id)

	err := rc.TriggerManualRollback(context.Background(), id, "rollback")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executing rollback")
	assert.Contains(t, err.Error(), "infra failure")
}
