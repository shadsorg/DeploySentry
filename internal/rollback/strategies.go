package rollback

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RollbackStrategy defines the interface for a rollback execution strategy.
type RollbackStrategy interface {
	// Name returns the strategy identifier.
	Name() string

	// Execute performs the rollback for the given deployment.
	Execute(ctx context.Context, deploymentID uuid.UUID) error
}

// ImmediateRollbackStrategy performs an immediate, full rollback by cutting
// over all traffic to the previous version at once.
type ImmediateRollbackStrategy struct{}

// NewImmediateRollbackStrategy creates a new ImmediateRollbackStrategy.
func NewImmediateRollbackStrategy() *ImmediateRollbackStrategy {
	return &ImmediateRollbackStrategy{}
}

// Name returns the strategy identifier.
func (s *ImmediateRollbackStrategy) Name() string {
	return "immediate"
}

// Execute performs an immediate rollback. In production this would interact
// with the deployment infrastructure to switch all traffic back to the
// previous version.
func (s *ImmediateRollbackStrategy) Execute(ctx context.Context, deploymentID uuid.UUID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// In production: set traffic to 0% on new version, 100% on old version.
	// This is a stub that records the intent.
	return nil
}

// GradualRollbackConfig holds configuration for a gradual rollback strategy.
type GradualRollbackConfig struct {
	// Steps defines the traffic reduction steps (e.g., 50%, 25%, 0%).
	Steps []int `json:"steps"`
	// StepDelay is the delay between each step.
	StepDelay time.Duration `json:"step_delay"`
}

// DefaultGradualRollbackConfig returns a sensible default gradual rollback configuration.
func DefaultGradualRollbackConfig() GradualRollbackConfig {
	return GradualRollbackConfig{
		Steps:     []int{50, 25, 10, 0},
		StepDelay: 30 * time.Second,
	}
}

// GradualRollbackStrategy reduces traffic to the new version incrementally,
// allowing monitoring between steps to detect any issues with the rollback itself.
type GradualRollbackStrategy struct {
	config GradualRollbackConfig
}

// NewGradualRollbackStrategy creates a new GradualRollbackStrategy.
func NewGradualRollbackStrategy(config GradualRollbackConfig) *GradualRollbackStrategy {
	return &GradualRollbackStrategy{config: config}
}

// Name returns the strategy identifier.
func (s *GradualRollbackStrategy) Name() string {
	return "gradual"
}

// Execute performs a gradual rollback by reducing traffic to the new version
// through the configured steps with delays between each step.
func (s *GradualRollbackStrategy) Execute(ctx context.Context, deploymentID uuid.UUID) error {
	for i, targetPercent := range s.config.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// In production: adjust traffic split to targetPercent on new version.
		_ = fmt.Sprintf("rollback step %d: setting traffic to %d%% for deployment %s",
			i+1, targetPercent, deploymentID)

		// Wait between steps (skip wait for the last step).
		if i < len(s.config.Steps)-1 && s.config.StepDelay > 0 {
			timer := time.NewTimer(s.config.StepDelay)
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

// CanaryRollbackStrategy is specifically designed for rolling back canary
// deployments. It sets the canary traffic to 0% immediately.
type CanaryRollbackStrategy struct{}

// NewCanaryRollbackStrategy creates a new CanaryRollbackStrategy.
func NewCanaryRollbackStrategy() *CanaryRollbackStrategy {
	return &CanaryRollbackStrategy{}
}

// Name returns the strategy identifier.
func (s *CanaryRollbackStrategy) Name() string {
	return "canary_rollback"
}

// Execute removes all canary traffic, reverting to the stable version.
func (s *CanaryRollbackStrategy) Execute(ctx context.Context, deploymentID uuid.UUID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// In production: set canary traffic to 0%.
	return nil
}
