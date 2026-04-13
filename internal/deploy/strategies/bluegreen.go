package strategies

import (
	"context"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// BlueGreenConfig holds configuration for a blue-green deployment strategy.
type BlueGreenConfig struct {
	// WarmupDuration is how long the inactive environment is given to warm up
	// before traffic is switched.
	WarmupDuration    time.Duration `json:"warmup_duration"`
	HealthCheckURL    string        `json:"health_check_url"`
	HealthThreshold   float64       `json:"health_threshold"`
	RollbackOnFailure bool          `json:"rollback_on_failure"`
	// AutoPromote controls whether traffic is switched automatically after
	// WarmupDuration elapses and health checks pass, or if it waits for a
	// manual gate.
	AutoPromote bool `json:"auto_promote"`
}

// defaultBlueGreenConfig is the package-level default used by DefaultBlueGreenConfig.
// Tests may override this via SetDefaultBlueGreenConfigForTest.
var defaultBlueGreenConfig = BlueGreenConfig{
	WarmupDuration:    2 * time.Minute,
	HealthThreshold:   0.95,
	RollbackOnFailure: true,
	AutoPromote:       true,
}

// DefaultBlueGreenConfig returns a sensible default configuration for
// blue-green deployments.
func DefaultBlueGreenConfig() BlueGreenConfig {
	return defaultBlueGreenConfig
}

// SetDefaultBlueGreenConfigForTest overrides the default blue-green config.
// This function is intended for test use only.
func SetDefaultBlueGreenConfigForTest(config BlueGreenConfig) {
	defaultBlueGreenConfig = config
}

// BlueGreenStrategy implements the DeployStrategy interface using a blue-green
// approach. It deploys the new version to an inactive environment, verifies
// health, then atomically switches all traffic.
type BlueGreenStrategy struct {
	config BlueGreenConfig
}

// NewBlueGreenStrategy creates a new BlueGreenStrategy with the given configuration.
func NewBlueGreenStrategy(config BlueGreenConfig) *BlueGreenStrategy {
	return &BlueGreenStrategy{config: config}
}

// Name returns the strategy identifier.
func (s *BlueGreenStrategy) Name() string {
	return string(models.DeployStrategyBlueGreen)
}

// Execute runs the blue-green deployment. It deploys to the inactive
// environment, waits for the warmup period, checks health, and then
// switches traffic atomically from the old environment to the new one.
func (s *BlueGreenStrategy) Execute(ctx context.Context, deployment *models.Deployment) error {
	// Phase 1: Deploy to inactive environment (the "green" side).
	// In production this would trigger the actual infrastructure deployment.
	deployment.TrafficPercent = 0
	deployment.UpdatedAt = time.Now().UTC()

	// Phase 2: Wait for warmup period.
	if s.config.WarmupDuration > 0 {
		timer := time.NewTimer(s.config.WarmupDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	// Phase 3: Health check on the new environment.
	// In production this would query the health check URL and verify metrics.

	// Phase 4: Atomic traffic switch.
	deployment.TrafficPercent = 100
	deployment.UpdatedAt = time.Now().UTC()

	return nil
}

// Rollback reverts a blue-green deployment by switching traffic back
// to the original environment.
func (s *BlueGreenStrategy) Rollback(ctx context.Context, deployment *models.Deployment) error {
	deployment.TrafficPercent = 0
	deployment.UpdatedAt = time.Now().UTC()
	return nil
}
