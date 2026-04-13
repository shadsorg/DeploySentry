package engine

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/deploy/strategies"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRollingPhases(t *testing.T) {
	depID := uuid.New()
	config := strategies.RollingConfig{
		BatchCount: 3, BatchDelay: 30 * time.Second,
		HealthThreshold: 0.95, RollbackOnFailure: true, AutoPromote: true,
	}
	phases := BuildRollingPhases(depID, config)
	require.Len(t, phases, 3)
	assert.Equal(t, "rolling-batch-1", phases[0].Name)
	assert.Equal(t, 33, phases[0].TrafficPercent)
	assert.Equal(t, 30, phases[0].Duration)
	assert.Equal(t, 0, phases[0].SortOrder)
	assert.True(t, phases[0].AutoPromote)
	assert.Equal(t, models.PhaseStatusPending, phases[0].Status)
	assert.Equal(t, "rolling-batch-2", phases[1].Name)
	assert.Equal(t, 66, phases[1].TrafficPercent)
	assert.Equal(t, "rolling-batch-3", phases[2].Name)
	assert.Equal(t, 100, phases[2].TrafficPercent)
}

func TestBuildRollingPhases_SingleBatch(t *testing.T) {
	depID := uuid.New()
	config := strategies.RollingConfig{BatchCount: 1, BatchDelay: 0, AutoPromote: true}
	phases := BuildRollingPhases(depID, config)
	require.Len(t, phases, 1)
	assert.Equal(t, 100, phases[0].TrafficPercent)
}

func TestBuildBlueGreenPhases(t *testing.T) {
	depID := uuid.New()
	config := strategies.BlueGreenConfig{
		WarmupDuration: 2 * time.Minute, HealthThreshold: 0.95,
		RollbackOnFailure: true, AutoPromote: true,
	}
	phases := BuildBlueGreenPhases(depID, config)
	require.Len(t, phases, 3)
	assert.Equal(t, "deploy-green", phases[0].Name)
	assert.Equal(t, 0, phases[0].TrafficPercent)
	assert.Equal(t, 120, phases[0].Duration)
	assert.Equal(t, "health-check", phases[1].Name)
	assert.Equal(t, 0, phases[1].TrafficPercent)
	assert.Equal(t, 30, phases[1].Duration)
	assert.Equal(t, "switch-traffic", phases[2].Name)
	assert.Equal(t, 100, phases[2].TrafficPercent)
	assert.Equal(t, 0, phases[2].Duration)
}

func TestBuildPhasesForStrategy_Dispatches(t *testing.T) {
	depID := uuid.New()
	canary := BuildPhasesForStrategy(depID, models.DeployStrategyCanary)
	assert.Greater(t, len(canary), 0)
	assert.Contains(t, canary[0].Name, "canary-")
	rolling := BuildPhasesForStrategy(depID, models.DeployStrategyRolling)
	assert.Greater(t, len(rolling), 0)
	assert.Contains(t, rolling[0].Name, "rolling-")
	bg := BuildPhasesForStrategy(depID, models.DeployStrategyBlueGreen)
	assert.Len(t, bg, 3)
	assert.Equal(t, "deploy-green", bg[0].Name)
}
