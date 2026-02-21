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

// HealthCheckCriteria defines the health thresholds that must be met
// before a canary phase can be promoted.
type HealthCheckCriteria struct {
	// MaxErrorRate is the maximum acceptable error rate (e.g. 0.01 = 1%).
	MaxErrorRate float64 `json:"max_error_rate"`
	// LatencyP99Ms is the maximum acceptable p99 latency in milliseconds.
	LatencyP99Ms int `json:"latency_p99_ms"`
	// CustomMetrics maps custom metric names to their maximum acceptable values.
	CustomMetrics map[string]float64 `json:"custom_metrics,omitempty"`
}

// CanaryStep defines a single step in a canary rollout, specifying
// the traffic percentage, observation duration, health criteria, and
// promotion behaviour.
type CanaryStep struct {
	// TrafficPercent is the percentage of traffic routed to the canary.
	TrafficPercent int `json:"traffic_percent"`
	// Duration is the observation / hold time before the next step.
	Duration time.Duration `json:"duration"`
	// HealthCriteria defines per-phase health thresholds. When nil the
	// strategy-level HealthThreshold is used instead.
	HealthCriteria *HealthCheckCriteria `json:"health_criteria,omitempty"`
	// AutoPromote controls whether this phase promotes automatically
	// after Duration elapses and health checks pass, or waits for a
	// manual gate. When nil the strategy-level AutoPromote value applies.
	AutoPromote *bool `json:"auto_promote,omitempty"`
}

// CanaryConfig holds configuration for a canary deployment strategy.
type CanaryConfig struct {
	Steps             []CanaryStep `json:"steps"`
	HealthCheckURL    string       `json:"health_check_url"`
	HealthThreshold   float64      `json:"health_threshold"`
	AutoPromote       bool         `json:"auto_promote"`
	RollbackOnFailure bool         `json:"rollback_on_failure"`
}

// DefaultCanaryConfig returns a sensible default canary configuration
// with five incremental traffic phases matching the deploy checklist:
//
//	Phase 1 — 1%   traffic, 5 min monitoring
//	Phase 2 — 5%   traffic, 5 min monitoring
//	Phase 3 — 25%  traffic, 10 min monitoring
//	Phase 4 — 50%  traffic, 10 min monitoring
//	Phase 5 — 100% traffic, full promotion
func DefaultCanaryConfig() CanaryConfig {
	return CanaryConfig{
		Steps: []CanaryStep{
			{TrafficPercent: 1, Duration: 5 * time.Minute},
			{TrafficPercent: 5, Duration: 5 * time.Minute},
			{TrafficPercent: 25, Duration: 10 * time.Minute},
			{TrafficPercent: 50, Duration: 10 * time.Minute},
			{TrafficPercent: 100, Duration: 0},
		},
		HealthThreshold:   0.95,
		AutoPromote:       false,
		RollbackOnFailure: true,
	}
}

// IsAutoPromote reports whether the step should auto-promote. It falls
// back to the strategy-level AutoPromote value when the per-step
// override is nil.
func (s *CanaryStep) IsAutoPromote(strategyDefault bool) bool {
	if s.AutoPromote != nil {
		return *s.AutoPromote
	}
	return strategyDefault
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
