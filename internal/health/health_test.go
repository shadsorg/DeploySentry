package health

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockHealthCheck struct {
	name   string
	result *CheckResult
	err    error
}

func (m *mockHealthCheck) Name() string { return m.name }
func (m *mockHealthCheck) Check(_ context.Context, _ uuid.UUID) (*CheckResult, error) {
	return m.result, m.err
}

type mockHealthListener struct {
	updates []*DeploymentHealth
	mu      sync.Mutex
}

func (m *mockHealthListener) OnHealthUpdate(_ context.Context, health *DeploymentHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, health)
}

func (m *mockHealthListener) getUpdates() []*DeploymentHealth {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*DeploymentHealth, len(m.updates))
	copy(out, m.updates)
	return out
}

// ---------------------------------------------------------------------------
// HealthScorer – NewHealthScorer
// ---------------------------------------------------------------------------

func TestNewHealthScorer_CreatesWithWeights(t *testing.T) {
	weights := []Weight{
		{CheckName: "cpu", Weight: 2.0},
		{CheckName: "memory", Weight: 3.0},
	}
	scorer := NewHealthScorer(weights, 1.0)

	assert.NotNil(t, scorer)
	assert.Equal(t, 2.0, scorer.weights["cpu"])
	assert.Equal(t, 3.0, scorer.weights["memory"])
	assert.Equal(t, 1.0, scorer.defaultWeight)
}

func TestNewHealthScorer_DefaultWeightUsedWhenZeroOrNegative(t *testing.T) {
	tests := []struct {
		name          string
		defaultWeight float64
	}{
		{"zero default weight", 0.0},
		{"negative default weight", -5.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scorer := NewHealthScorer(nil, tc.defaultWeight)
			assert.Equal(t, 1.0, scorer.defaultWeight, "default weight should be set to 1.0 when provided value is <= 0")
		})
	}
}

// ---------------------------------------------------------------------------
// HealthScorer – ComputeScore
// ---------------------------------------------------------------------------

func TestComputeScore_EmptyResults_ReturnsOne(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	score := scorer.ComputeScore(nil)
	assert.Equal(t, 1.0, score)

	score = scorer.ComputeScore([]*CheckResult{})
	assert.Equal(t, 1.0, score)
}

func TestComputeScore_SingleResult_ReturnsItsScore(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	results := []*CheckResult{
		{Name: "cpu", Score: 0.75},
	}
	score := scorer.ComputeScore(results)
	assert.InDelta(t, 0.75, score, 1e-9)
}

func TestComputeScore_MultipleResultsEqualWeights_ReturnsAverage(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0) // all weights default to 1.0
	results := []*CheckResult{
		{Name: "cpu", Score: 0.8},
		{Name: "memory", Score: 0.6},
		{Name: "disk", Score: 1.0},
	}
	// average = (0.8 + 0.6 + 1.0) / 3 = 0.8
	score := scorer.ComputeScore(results)
	assert.InDelta(t, 0.8, score, 1e-9)
}

func TestComputeScore_WeightedResults_ReturnsWeightedAverage(t *testing.T) {
	weights := []Weight{
		{CheckName: "cpu", Weight: 2.0},
		{CheckName: "memory", Weight: 1.0},
	}
	scorer := NewHealthScorer(weights, 1.0)
	results := []*CheckResult{
		{Name: "cpu", Score: 0.9},
		{Name: "memory", Score: 0.3},
	}
	// weighted avg = (0.9*2 + 0.3*1) / (2+1) = 2.1/3 = 0.7
	score := scorer.ComputeScore(results)
	assert.InDelta(t, 0.7, score, 1e-9)
}

func TestComputeScore_AllZeroScores_ReturnsZero(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	results := []*CheckResult{
		{Name: "cpu", Score: 0.0},
		{Name: "memory", Score: 0.0},
	}
	score := scorer.ComputeScore(results)
	assert.Equal(t, 0.0, score)
}

func TestComputeScore_ClampedToRange(t *testing.T) {
	// Craft a scenario where raw weighted average exceeds 1.0.
	// If a CheckResult has a Score > 1.0, the raw average can exceed 1.0.
	scorer := NewHealthScorer(nil, 1.0)
	results := []*CheckResult{
		{Name: "over", Score: 1.5},
	}
	score := scorer.ComputeScore(results)
	assert.Equal(t, 1.0, score, "score should be clamped to 1.0")

	// Scenario where raw average would be negative.
	results = []*CheckResult{
		{Name: "under", Score: -0.5},
	}
	score = scorer.ComputeScore(results)
	assert.Equal(t, 0.0, score, "score should be clamped to 0.0")
}

func TestComputeScore_UsesDefaultWeightForUnknownCheck(t *testing.T) {
	weights := []Weight{
		{CheckName: "cpu", Weight: 3.0},
	}
	scorer := NewHealthScorer(weights, 1.0)
	results := []*CheckResult{
		{Name: "cpu", Score: 0.9},
		{Name: "unknown", Score: 0.3}, // should use default weight 1.0
	}
	// weighted avg = (0.9*3 + 0.3*1) / (3+1) = 3.0/4 = 0.75
	score := scorer.ComputeScore(results)
	assert.InDelta(t, 0.75, score, 1e-9)
}

// ---------------------------------------------------------------------------
// clamp
// ---------------------------------------------------------------------------

func TestClamp_ValueWithinRange(t *testing.T) {
	assert.Equal(t, 0.5, clamp(0.5, 0.0, 1.0))
}

func TestClamp_ValueBelowMin(t *testing.T) {
	assert.Equal(t, 0.0, clamp(-0.3, 0.0, 1.0))
}

func TestClamp_ValueAboveMax(t *testing.T) {
	assert.Equal(t, 1.0, clamp(1.7, 0.0, 1.0))
}

func TestClamp_ValueAtBoundaries(t *testing.T) {
	assert.Equal(t, 0.0, clamp(0.0, 0.0, 1.0))
	assert.Equal(t, 1.0, clamp(1.0, 0.0, 1.0))
}

// ---------------------------------------------------------------------------
// HealthMonitor – NewHealthMonitor
// ---------------------------------------------------------------------------

func TestNewHealthMonitor_CreatesWithDefaults(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 5*time.Second, 0.7)

	assert.NotNil(t, monitor)
	assert.Equal(t, scorer, monitor.scorer)
	assert.Equal(t, 5*time.Second, monitor.interval)
	assert.Equal(t, 0.7, monitor.threshold)
	assert.NotNil(t, monitor.deployments)
	assert.Empty(t, monitor.checks)
	assert.Empty(t, monitor.listeners)
}

// ---------------------------------------------------------------------------
// HealthMonitor – AddCheck
// ---------------------------------------------------------------------------

func TestAddCheck_RegistersChecks(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	check1 := &mockHealthCheck{name: "cpu"}
	check2 := &mockHealthCheck{name: "memory"}

	monitor.AddCheck(check1)
	monitor.AddCheck(check2)

	assert.Len(t, monitor.checks, 2)
	assert.Equal(t, "cpu", monitor.checks[0].Name())
	assert.Equal(t, "memory", monitor.checks[1].Name())
}

// ---------------------------------------------------------------------------
// HealthMonitor – AddListener
// ---------------------------------------------------------------------------

func TestAddListener_RegistersListeners(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	listener1 := &mockHealthListener{}
	listener2 := &mockHealthListener{}

	monitor.AddListener(listener1)
	monitor.AddListener(listener2)

	assert.Len(t, monitor.listeners, 2)
}

// ---------------------------------------------------------------------------
// HealthMonitor – Watch / Unwatch
// ---------------------------------------------------------------------------

func TestWatch_AddsDeployment(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	id := uuid.New()
	monitor.Watch(id)

	health, err := monitor.GetHealth(id)
	require.NoError(t, err)
	assert.Equal(t, id, health.DeploymentID)
	assert.True(t, health.Healthy)
	assert.Equal(t, 1.0, health.Overall)
}

func TestUnwatch_RemovesDeployment(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	id := uuid.New()
	monitor.Watch(id)
	monitor.Unwatch(id)

	_, err := monitor.GetHealth(id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not being monitored")
}

func TestWatch_MultipleDeployments(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	id1 := uuid.New()
	id2 := uuid.New()
	monitor.Watch(id1)
	monitor.Watch(id2)

	h1, err := monitor.GetHealth(id1)
	require.NoError(t, err)
	assert.Equal(t, id1, h1.DeploymentID)

	h2, err := monitor.GetHealth(id2)
	require.NoError(t, err)
	assert.Equal(t, id2, h2.DeploymentID)
}

// ---------------------------------------------------------------------------
// HealthMonitor – GetHealth
// ---------------------------------------------------------------------------

func TestGetHealth_ReturnsHealthForWatchedDeployment(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	id := uuid.New()
	monitor.Watch(id)

	health, err := monitor.GetHealth(id)
	require.NoError(t, err)
	assert.Equal(t, id, health.DeploymentID)
}

func TestGetHealth_ErrorForUnwatchedDeployment(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, time.Second, 0.7)

	id := uuid.New()
	_, err := monitor.GetHealth(id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not being monitored")
}

// ---------------------------------------------------------------------------
// HealthMonitor – evaluate (tested indirectly via Start)
// ---------------------------------------------------------------------------

func TestEvaluate_HealthyChecks_HighScore(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	check := &mockHealthCheck{
		name: "cpu",
		result: &CheckResult{
			Name:      "cpu",
			Healthy:   true,
			Score:     0.95,
			CheckedAt: time.Now().UTC(),
		},
	}
	monitor.AddCheck(check)

	listener := &mockHealthListener{}
	monitor.AddListener(listener)

	depID := uuid.New()
	monitor.Watch(depID)

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	// Wait for at least one evaluation cycle.
	require.Eventually(t, func() bool {
		return len(listener.getUpdates()) > 0
	}, 2*time.Second, 10*time.Millisecond, "expected at least one listener update")

	cancel()

	updates := listener.getUpdates()
	require.NotEmpty(t, updates)

	latest := updates[len(updates)-1]
	assert.Equal(t, depID, latest.DeploymentID)
	assert.True(t, latest.Healthy)
	assert.InDelta(t, 0.95, latest.Overall, 1e-9)
	assert.Len(t, latest.Checks, 1)
	assert.Equal(t, "cpu", latest.Checks[0].Name)
}

func TestEvaluate_FailingChecks_LowScore(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	check := &mockHealthCheck{
		name: "cpu",
		result: &CheckResult{
			Name:      "cpu",
			Healthy:   false,
			Score:     0.2,
			Message:   "CPU overloaded",
			CheckedAt: time.Now().UTC(),
		},
	}
	monitor.AddCheck(check)

	listener := &mockHealthListener{}
	monitor.AddListener(listener)

	depID := uuid.New()
	monitor.Watch(depID)

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	require.Eventually(t, func() bool {
		return len(listener.getUpdates()) > 0
	}, 2*time.Second, 10*time.Millisecond)

	cancel()

	updates := listener.getUpdates()
	latest := updates[len(updates)-1]
	assert.Equal(t, depID, latest.DeploymentID)
	assert.False(t, latest.Healthy, "should be unhealthy when score < threshold")
	assert.InDelta(t, 0.2, latest.Overall, 1e-9)
}

func TestEvaluate_CheckReturnsError_CreatesUnhealthyResult(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	check := &mockHealthCheck{
		name: "network",
		err:  errors.New("connection refused"),
	}
	monitor.AddCheck(check)

	listener := &mockHealthListener{}
	monitor.AddListener(listener)

	depID := uuid.New()
	monitor.Watch(depID)

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	require.Eventually(t, func() bool {
		return len(listener.getUpdates()) > 0
	}, 2*time.Second, 10*time.Millisecond)

	cancel()

	updates := listener.getUpdates()
	latest := updates[len(updates)-1]

	assert.False(t, latest.Healthy)
	assert.Equal(t, 0.0, latest.Overall)
	require.Len(t, latest.Checks, 1)
	assert.Equal(t, "network", latest.Checks[0].Name)
	assert.False(t, latest.Checks[0].Healthy)
	assert.Equal(t, float64(0), latest.Checks[0].Score)
	assert.Equal(t, "connection refused", latest.Checks[0].Message)
}

func TestEvaluate_NotifiesListeners(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	check := &mockHealthCheck{
		name: "disk",
		result: &CheckResult{
			Name:      "disk",
			Healthy:   true,
			Score:     0.85,
			CheckedAt: time.Now().UTC(),
		},
	}
	monitor.AddCheck(check)

	listener1 := &mockHealthListener{}
	listener2 := &mockHealthListener{}
	monitor.AddListener(listener1)
	monitor.AddListener(listener2)

	depID := uuid.New()
	monitor.Watch(depID)

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	require.Eventually(t, func() bool {
		return len(listener1.getUpdates()) > 0 && len(listener2.getUpdates()) > 0
	}, 2*time.Second, 10*time.Millisecond, "both listeners should receive updates")

	cancel()

	assert.NotEmpty(t, listener1.getUpdates(), "listener1 should have received updates")
	assert.NotEmpty(t, listener2.getUpdates(), "listener2 should have received updates")
}

func TestEvaluate_MultipleChecks_AggregatesCorrectly(t *testing.T) {
	weights := []Weight{
		{CheckName: "cpu", Weight: 2.0},
		{CheckName: "memory", Weight: 1.0},
	}
	scorer := NewHealthScorer(weights, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	cpuCheck := &mockHealthCheck{
		name: "cpu",
		result: &CheckResult{
			Name:      "cpu",
			Healthy:   true,
			Score:     0.9,
			CheckedAt: time.Now().UTC(),
		},
	}
	memCheck := &mockHealthCheck{
		name: "memory",
		result: &CheckResult{
			Name:      "memory",
			Healthy:   true,
			Score:     0.6,
			CheckedAt: time.Now().UTC(),
		},
	}
	monitor.AddCheck(cpuCheck)
	monitor.AddCheck(memCheck)

	listener := &mockHealthListener{}
	monitor.AddListener(listener)

	depID := uuid.New()
	monitor.Watch(depID)

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	require.Eventually(t, func() bool {
		return len(listener.getUpdates()) > 0
	}, 2*time.Second, 10*time.Millisecond)

	cancel()

	updates := listener.getUpdates()
	latest := updates[len(updates)-1]

	// weighted avg = (0.9*2 + 0.6*1) / (2+1) = 2.4/3 = 0.8
	assert.InDelta(t, 0.8, latest.Overall, 1e-9)
	assert.True(t, latest.Healthy, "0.8 >= 0.7 threshold means healthy")
	assert.Len(t, latest.Checks, 2)
}

func TestEvaluate_UpdatesStoredHealth(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	check := &mockHealthCheck{
		name: "cpu",
		result: &CheckResult{
			Name:      "cpu",
			Healthy:   true,
			Score:     0.85,
			CheckedAt: time.Now().UTC(),
		},
	}
	monitor.AddCheck(check)

	depID := uuid.New()
	monitor.Watch(depID)

	listener := &mockHealthListener{}
	monitor.AddListener(listener)

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	require.Eventually(t, func() bool {
		return len(listener.getUpdates()) > 0
	}, 2*time.Second, 10*time.Millisecond)

	cancel()

	// Verify that GetHealth returns the updated data after evaluation.
	health, err := monitor.GetHealth(depID)
	require.NoError(t, err)
	assert.InDelta(t, 0.85, health.Overall, 1e-9)
	assert.True(t, health.Healthy)
	assert.NotEmpty(t, health.Checks)
	assert.False(t, health.EvaluatedAt.IsZero(), "EvaluatedAt should be set after evaluation")
}

// ---------------------------------------------------------------------------
// HealthMonitor – Start / Stop
// ---------------------------------------------------------------------------

func TestStart_ContextCancellation(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Start returned — success.
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestStop_HaltsMonitor(t *testing.T) {
	scorer := NewHealthScorer(nil, 1.0)
	monitor := NewHealthMonitor(scorer, 50*time.Millisecond, 0.7)

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	monitor.Stop()

	select {
	case <-done:
		// Start returned — success.
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}
