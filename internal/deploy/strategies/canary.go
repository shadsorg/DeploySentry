// Package strategies implements deployment rollout strategies such as canary,
// blue-green, and rolling deployments.
package strategies

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// DeployStrategy defines the interface for executing a deployment rollout.
type DeployStrategy interface {
	// Execute performs the deployment rollout according to the strategy.
	Execute(ctx context.Context, deployment *models.Deployment) error

	// Rollback reverses the deployment, restoring the previous version.
	Rollback(ctx context.Context, deployment *models.Deployment) error

	// Name returns the strategy identifier.
	Name() string
}

// CanaryStep defines a single step in a canary rollout, specifying
// the traffic percentage and observation duration.
type CanaryStep struct {
	TrafficPercent int           `json:"traffic_percent"`
	Duration       time.Duration `json:"duration"`
}

// CanaryConfig holds configuration for a canary deployment strategy.
type CanaryConfig struct {
	Steps              []CanaryStep `json:"steps"`
	HealthCheckURL     string       `json:"health_check_url"`
	HealthThreshold    float64      `json:"health_threshold"`
	AutoPromote        bool         `json:"auto_promote"`
	RollbackOnFailure  bool         `json:"rollback_on_failure"`
}

// DefaultCanaryConfig returns a sensible default canary configuration
// with incremental traffic steps.
func DefaultCanaryConfig() CanaryConfig {
	return CanaryConfig{
		Steps: []CanaryStep{
			{TrafficPercent: 5, Duration: 5 * time.Minute},
			{TrafficPercent: 10, Duration: 5 * time.Minute},
			{TrafficPercent: 25, Duration: 10 * time.Minute},
			{TrafficPercent: 50, Duration: 10 * time.Minute},
			{TrafficPercent: 100, Duration: 0},
		},
		HealthThreshold:   0.95,
		AutoPromote:       false,
		RollbackOnFailure: true,
	}
}

// CanaryStrategy implements the DeployStrategy interface using a canary
// rollout approach that gradually shifts traffic to the new version.
type CanaryStrategy struct {
	config CanaryConfig
}

// NewCanaryStrategy creates a new CanaryStrategy with the given configuration.
func NewCanaryStrategy(config CanaryConfig) *CanaryStrategy {
	return &CanaryStrategy{config: config}
}

// Name returns the strategy identifier.
func (s *CanaryStrategy) Name() string {
	return string(models.DeployStrategyCanary)
}

// Execute runs the canary rollout by iterating through configured traffic
// steps. Each step increases the traffic percentage sent to the new version
// and waits for the configured observation duration before proceeding.
func (s *CanaryStrategy) Execute(ctx context.Context, deployment *models.Deployment) error {
	if len(s.config.Steps) == 0 {
		return fmt.Errorf("canary strategy requires at least one step")
	}

	for i, step := range s.config.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		deployment.TrafficPercent = step.TrafficPercent
		deployment.UpdatedAt = time.Now().UTC()

		// Create a phase record for this step.
		phase := &models.DeploymentPhase{
			ID:             uuid.New(),
			DeploymentID:   deployment.ID,
			Name:           fmt.Sprintf("canary-step-%d", i+1),
			Status:         models.DeployStatusRunning,
			TrafficPercent: step.TrafficPercent,
			Duration:       int(step.Duration.Seconds()),
			SortOrder:      i,
		}
		now := time.Now().UTC()
		phase.StartedAt = &now

		// In production, this would persist the phase and wait for the duration
		// while checking health metrics. For now, we record the intent.
		_ = phase // Phase would be persisted via repository.

		if step.Duration > 0 {
			timer := time.NewTimer(step.Duration)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return nil
}

// Rollback reverts the canary deployment by setting traffic back to 0%
// on the new version.
func (s *CanaryStrategy) Rollback(ctx context.Context, deployment *models.Deployment) error {
	deployment.TrafficPercent = 0
	deployment.UpdatedAt = time.Now().UTC()
	return nil
}
