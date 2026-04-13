package engine

import (
	"fmt"

	"github.com/deploysentry/deploysentry/internal/deploy/strategies"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// BuildPhasesForStrategy creates DeploymentPhase records for any deployment strategy.
func BuildPhasesForStrategy(deploymentID uuid.UUID, strategy models.DeployStrategyType) []*models.DeploymentPhase {
	switch strategy {
	case models.DeployStrategyCanary:
		return BuildPhases(deploymentID, strategies.DefaultCanaryConfig())
	case models.DeployStrategyRolling:
		return BuildRollingPhases(deploymentID, strategies.DefaultRollingConfig())
	case models.DeployStrategyBlueGreen:
		return BuildBlueGreenPhases(deploymentID, strategies.DefaultBlueGreenConfig())
	default:
		return nil
	}
}

// BuildRollingPhases creates phases for a rolling deployment.
// Traffic is divided evenly across BatchCount steps.
func BuildRollingPhases(deploymentID uuid.UUID, config strategies.RollingConfig) []*models.DeploymentPhase {
	count := config.BatchCount
	if count <= 0 {
		count = 3
	}
	phases := make([]*models.DeploymentPhase, 0, count)
	for i := 1; i <= count; i++ {
		trafficPercent := (i * 100) / count
		phases = append(phases, &models.DeploymentPhase{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           fmt.Sprintf("rolling-batch-%d", i),
			Status:         models.PhaseStatusPending,
			TrafficPercent: trafficPercent,
			Duration:       int(config.BatchDelay.Seconds()),
			SortOrder:      i - 1,
			AutoPromote:    config.AutoPromote,
		})
	}
	return phases
}

// BuildBlueGreenPhases creates three fixed phases for a blue-green deployment:
// deploy-green (warmup), health-check (30s), switch-traffic (atomic cutover).
func BuildBlueGreenPhases(deploymentID uuid.UUID, config strategies.BlueGreenConfig) []*models.DeploymentPhase {
	return []*models.DeploymentPhase{
		{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           "deploy-green",
			Status:         models.PhaseStatusPending,
			TrafficPercent: 0,
			Duration:       int(config.WarmupDuration.Seconds()),
			SortOrder:      0,
			AutoPromote:    config.AutoPromote,
		},
		{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           "health-check",
			Status:         models.PhaseStatusPending,
			TrafficPercent: 0,
			Duration:       int(config.HealthCheckDuration.Seconds()),
			SortOrder:      1,
			AutoPromote:    config.AutoPromote,
		},
		{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           "switch-traffic",
			Status:         models.PhaseStatusPending,
			TrafficPercent: 100,
			Duration:       0,
			SortOrder:      2,
			AutoPromote:    config.AutoPromote,
		},
	}
}
