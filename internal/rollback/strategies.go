package rollback

import (
	"context"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/health"
	"github.com/shadsorg/deploysentry/internal/models"
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

// BlueGreenRollbackStrategy handles rollback for blue/green deployments by
// shifting all traffic back to the blue (stable) environment.
type BlueGreenRollbackStrategy struct {
	// TrafficShifter is called to shift traffic between environments.
	// In production this would interact with a load balancer or service mesh.
	TrafficShifter TrafficShifter
}

// TrafficShifter defines the interface for shifting traffic between deployment
// targets (e.g., blue and green environments).
type TrafficShifter interface {
	// ShiftTraffic moves the given percentage of traffic to the specified target.
	// target identifies the environment (e.g., "blue" or "green").
	ShiftTraffic(ctx context.Context, deploymentID uuid.UUID, target string, percent int) error
}

// NewBlueGreenRollbackStrategy creates a new BlueGreenRollbackStrategy.
func NewBlueGreenRollbackStrategy(shifter TrafficShifter) *BlueGreenRollbackStrategy {
	return &BlueGreenRollbackStrategy{
		TrafficShifter: shifter,
	}
}

// Name returns the strategy identifier.
func (s *BlueGreenRollbackStrategy) Name() string {
	return "blue_green"
}

// Execute shifts all traffic back to the blue (stable) environment.
// In a blue/green deployment, the blue environment runs the previous stable
// version while the green environment runs the new version being rolled back.
func (s *BlueGreenRollbackStrategy) Execute(ctx context.Context, deploymentID uuid.UUID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if s.TrafficShifter != nil {
		// Shift 100% of traffic back to the blue (stable) environment.
		if err := s.TrafficShifter.ShiftTraffic(ctx, deploymentID, "blue", 100); err != nil {
			return fmt.Errorf("shifting traffic to blue: %w", err)
		}
		// Set green (new version) to 0%.
		if err := s.TrafficShifter.ShiftTraffic(ctx, deploymentID, "green", 0); err != nil {
			return fmt.Errorf("removing traffic from green: %w", err)
		}
	}

	return nil
}

// FlagToggler defines the interface for toggling feature flags. This is used
// by the FlagKillSwitchStrategy to disable flags as part of a rollback.
type FlagToggler interface {
	// DisableFlag disables the specified feature flag.
	DisableFlag(ctx context.Context, flagID uuid.UUID) error
}

// FlagKillSwitchStrategy rolls back by disabling a feature flag, effectively
// turning off the feature without redeploying. This is useful for feature-flag-
// gated releases where the deployment itself does not need to change.
type FlagKillSwitchStrategy struct {
	flagToggler FlagToggler
	flagID      uuid.UUID
}

// NewFlagKillSwitchStrategy creates a new FlagKillSwitchStrategy that disables
// the specified feature flag when executed.
func NewFlagKillSwitchStrategy(toggler FlagToggler, flagID uuid.UUID) *FlagKillSwitchStrategy {
	return &FlagKillSwitchStrategy{
		flagToggler: toggler,
		flagID:      flagID,
	}
}

// Name returns the strategy identifier.
func (s *FlagKillSwitchStrategy) Name() string {
	return "flag_kill_switch"
}

// Execute disables the configured feature flag. This immediately stops the
// feature from being served to users without requiring a full redeployment.
func (s *FlagKillSwitchStrategy) Execute(ctx context.Context, deploymentID uuid.UUID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if s.flagToggler == nil {
		return fmt.Errorf("flag toggler is not configured")
	}

	if err := s.flagToggler.DisableFlag(ctx, s.flagID); err != nil {
		return fmt.Errorf("disabling feature flag %s: %w", s.flagID, err)
	}

	return nil
}

// SelectStrategy returns the appropriate rollback strategy based on the
// deployment type. This provides a single entry point for strategy selection
// rather than requiring callers to manually map deployment types to strategies.
func SelectStrategy(deploymentType models.DeployStrategyType, opts ...StrategyOption) RollbackStrategy {
	cfg := &strategyConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	switch deploymentType {
	case models.DeployStrategyCanary:
		return NewCanaryRollbackStrategy()
	case models.DeployStrategyBlueGreen:
		return NewBlueGreenRollbackStrategy(cfg.trafficShifter)
	case models.DeployStrategyRolling:
		if cfg.gradualConfig != nil {
			return NewGradualRollbackStrategy(*cfg.gradualConfig)
		}
		return NewGradualRollbackStrategy(DefaultGradualRollbackConfig())
	default:
		return NewImmediateRollbackStrategy()
	}
}

// strategyConfig holds optional configuration for strategy selection.
type strategyConfig struct {
	trafficShifter TrafficShifter
	gradualConfig  *GradualRollbackConfig
	flagToggler    FlagToggler
	flagID         uuid.UUID
}

// StrategyOption configures optional parameters for SelectStrategy.
type StrategyOption func(*strategyConfig)

// WithTrafficShifter sets the traffic shifter for blue/green strategies.
func WithTrafficShifter(shifter TrafficShifter) StrategyOption {
	return func(cfg *strategyConfig) {
		cfg.trafficShifter = shifter
	}
}

// WithGradualConfig sets the configuration for gradual rollback strategies.
func WithGradualConfig(config GradualRollbackConfig) StrategyOption {
	return func(cfg *strategyConfig) {
		cfg.gradualConfig = &config
	}
}

// WithFlagToggler sets the feature flag toggler for kill switch strategies.
func WithFlagToggler(toggler FlagToggler, flagID uuid.UUID) StrategyOption {
	return func(cfg *strategyConfig) {
		cfg.flagToggler = toggler
		cfg.flagID = flagID
	}
}

// HealthChecker defines the interface for checking deployment health,
// used by VerifyRollback to confirm the rolled-back version is healthy.
type HealthChecker interface {
	// GetHealth returns the current health of a deployment.
	GetHealth(deploymentID uuid.UUID) (*health.DeploymentHealth, error)
}

// VerifyRollback checks that the rolled-back version is healthy by querying
// the health monitor. It polls the health status up to maxAttempts times with
// the given interval between checks. Returns nil if the deployment is healthy,
// or an error describing why verification failed.
func VerifyRollback(ctx context.Context, checker HealthChecker, deploymentID uuid.UUID, healthThreshold float64, maxAttempts int, interval time.Duration) error {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		dh, err := checker.GetHealth(deploymentID)
		if err != nil {
			if attempt == maxAttempts {
				return fmt.Errorf("rollback verification failed: unable to get health after %d attempts: %w", maxAttempts, err)
			}
			// Wait and retry.
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}

		if dh.Overall >= healthThreshold && dh.Healthy {
			return nil // Rollback verified successfully.
		}

		if attempt == maxAttempts {
			return fmt.Errorf("rollback verification failed: health score %.2f below threshold %.2f after %d attempts",
				dh.Overall, healthThreshold, maxAttempts)
		}

		// Wait before next attempt.
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("rollback verification: exhausted all attempts")
}
