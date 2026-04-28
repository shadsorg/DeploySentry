package strategies

import (
	"context"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Helper: build a minimal valid Deployment for strategy tests.
// ---------------------------------------------------------------------------

func testDeployment() *models.Deployment {
	return &models.Deployment{
		ID:             uuid.New(),
		ApplicationID:  uuid.New(),
		EnvironmentID:  uuid.New(),
		Strategy:       models.DeployStrategyCanary,
		Artifact:       "app:latest",
		Version:        "v1.0.0",
		Status:         models.DeployStatusRunning,
		TrafficPercent: 0,
		CreatedBy:      uuid.New(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

// ===========================================================================
// Canary Strategy Tests
// ===========================================================================

func TestCanaryStrategy_Name(t *testing.T) {
	s := NewCanaryStrategy(DefaultCanaryConfig())
	assert.Equal(t, "canary", s.Name())
}

func TestCanaryStrategy_Execute_CancelledContext(t *testing.T) {
	s := NewCanaryStrategy(CanaryConfig{
		Steps: []CanaryStep{
			{TrafficPercent: 50, Duration: 10 * time.Minute},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	d := testDeployment()
	err := s.Execute(ctx, d)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCanaryStrategy_Execute_EmptySteps(t *testing.T) {
	s := NewCanaryStrategy(CanaryConfig{Steps: nil})

	d := testDeployment()
	err := s.Execute(context.Background(), d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one step")
}

func TestCanaryStrategy_Execute_ZeroDurationSteps(t *testing.T) {
	s := NewCanaryStrategy(CanaryConfig{
		Steps: []CanaryStep{
			{TrafficPercent: 25, Duration: 0},
			{TrafficPercent: 50, Duration: 0},
			{TrafficPercent: 100, Duration: 0},
		},
	})

	d := testDeployment()
	err := s.Execute(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, 100, d.TrafficPercent, "traffic should reach 100% after all steps")
}

func TestCanaryStrategy_Rollback(t *testing.T) {
	s := NewCanaryStrategy(DefaultCanaryConfig())

	d := testDeployment()
	d.TrafficPercent = 50 // simulate mid-rollout

	err := s.Rollback(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, 0, d.TrafficPercent, "traffic should be set to 0% on rollback")
}

func TestDefaultCanaryConfig(t *testing.T) {
	cfg := DefaultCanaryConfig()
	assert.NotEmpty(t, cfg.Steps, "default config should have steps")
	assert.Greater(t, len(cfg.Steps), 0)

	// Verify the last step reaches 100%.
	last := cfg.Steps[len(cfg.Steps)-1]
	assert.Equal(t, 100, last.TrafficPercent, "last canary step should be 100%")

	// Verify health threshold is set.
	assert.Greater(t, cfg.HealthThreshold, 0.0)
}

// ===========================================================================
// Blue-Green Strategy Tests
// ===========================================================================

func TestBlueGreenStrategy_Name(t *testing.T) {
	s := NewBlueGreenStrategy(DefaultBlueGreenConfig())
	assert.Equal(t, "blue_green", s.Name())
}

func TestBlueGreenStrategy_Execute_ZeroWarmup(t *testing.T) {
	s := NewBlueGreenStrategy(BlueGreenConfig{
		WarmupDuration: 0, // no wait
	})

	d := testDeployment()
	err := s.Execute(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, 100, d.TrafficPercent, "traffic should be 100% after blue-green switch")
}

func TestBlueGreenStrategy_Execute_CancelledContext(t *testing.T) {
	s := NewBlueGreenStrategy(BlueGreenConfig{
		WarmupDuration: 10 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	d := testDeployment()
	err := s.Execute(ctx, d)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBlueGreenStrategy_Rollback(t *testing.T) {
	s := NewBlueGreenStrategy(DefaultBlueGreenConfig())

	d := testDeployment()
	d.TrafficPercent = 100

	err := s.Rollback(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, 0, d.TrafficPercent, "traffic should be 0% after rollback")
}

func TestDefaultBlueGreenConfig(t *testing.T) {
	cfg := DefaultBlueGreenConfig()
	assert.Greater(t, cfg.WarmupDuration, time.Duration(0), "warmup should be positive")
	assert.Greater(t, cfg.HealthThreshold, 0.0, "health threshold should be set")
	assert.True(t, cfg.RollbackOnFailure, "rollback_on_failure should default to true")
}

// ===========================================================================
// Rolling Strategy Tests
// ===========================================================================

func TestRollingStrategy_Name(t *testing.T) {
	s := NewRollingStrategy(DefaultRollingConfig())
	assert.Equal(t, "rolling", s.Name())
}

func TestRollingStrategy_Execute_TotalInstancesZero(t *testing.T) {
	s := NewRollingStrategy(RollingConfig{
		TotalInstances: 0,
		BatchSize:      1,
	})

	d := testDeployment()
	err := s.Execute(context.Background(), d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total_instances must be greater than 0")
}

func TestRollingStrategy_Execute_BatchSizeZero(t *testing.T) {
	s := NewRollingStrategy(RollingConfig{
		TotalInstances: 3,
		BatchSize:      0,
	})

	d := testDeployment()
	err := s.Execute(context.Background(), d)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size must be greater than 0")
}

func TestRollingStrategy_Execute_CompletesWithBatchSize1(t *testing.T) {
	s := NewRollingStrategy(RollingConfig{
		TotalInstances: 3,
		BatchSize:      1,
		BatchDelay:     0, // no delay between batches
	})

	d := testDeployment()
	err := s.Execute(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, 100, d.TrafficPercent, "traffic should reach 100% when all instances updated")
}

func TestRollingStrategy_Execute_CancelledContext(t *testing.T) {
	s := NewRollingStrategy(RollingConfig{
		TotalInstances: 5,
		BatchSize:      1,
		BatchDelay:     10 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	d := testDeployment()
	err := s.Execute(ctx, d)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRollingStrategy_Rollback(t *testing.T) {
	s := NewRollingStrategy(DefaultRollingConfig())

	d := testDeployment()
	d.TrafficPercent = 66

	err := s.Rollback(context.Background(), d)
	assert.NoError(t, err)
	assert.Equal(t, 0, d.TrafficPercent, "traffic should be 0% after rollback")
}

func TestDefaultRollingConfig(t *testing.T) {
	cfg := DefaultRollingConfig()
	assert.Greater(t, cfg.TotalInstances, 0, "total_instances should be positive")
	assert.Greater(t, cfg.BatchSize, 0, "batch_size should be positive")
	assert.Greater(t, cfg.BatchDelay, time.Duration(0), "batch_delay should be positive")
	assert.Greater(t, cfg.HealthThreshold, 0.0, "health threshold should be set")
	assert.True(t, cfg.RollbackOnFailure, "rollback_on_failure should default to true")
}
