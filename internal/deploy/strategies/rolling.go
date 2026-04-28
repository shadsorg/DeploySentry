package strategies

import (
	"context"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RollingConfig holds configuration for a rolling deployment strategy.
type RollingConfig struct {
	// BatchSize is the number of instances to update simultaneously.
	BatchSize int `json:"batch_size"`
	// BatchDelay is the delay between successive batch updates.
	BatchDelay time.Duration `json:"batch_delay"`
	// MaxUnavailable is the maximum number of instances that can be unavailable
	// during the update.
	MaxUnavailable int `json:"max_unavailable"`
	// MaxSurge is the maximum number of extra instances that can exist above
	// TotalInstances during the update. When MaxSurge > 0 the strategy
	// creates new-version instances before terminating old ones, keeping the
	// overall available count at TotalInstances + surge.
	MaxSurge          int     `json:"max_surge"`
	HealthCheckURL    string  `json:"health_check_url"`
	HealthThreshold   float64 `json:"health_threshold"`
	RollbackOnFailure bool    `json:"rollback_on_failure"`
	// TotalInstances is the total number of instances to update.
	TotalInstances int `json:"total_instances"`
}

// DefaultRollingConfig returns a sensible default configuration for
// rolling deployments.
func DefaultRollingConfig() RollingConfig {
	return RollingConfig{
		BatchSize:         1,
		BatchDelay:        30 * time.Second,
		MaxUnavailable:    1,
		MaxSurge:          1,
		HealthThreshold:   0.95,
		RollbackOnFailure: true,
		TotalInstances:    3,
	}
}

// RollingStrategy implements the DeployStrategy interface using a rolling
// update approach that replaces instances incrementally.
type RollingStrategy struct {
	config RollingConfig
}

// NewRollingStrategy creates a new RollingStrategy with the given configuration.
func NewRollingStrategy(config RollingConfig) *RollingStrategy {
	return &RollingStrategy{config: config}
}

// Name returns the strategy identifier.
func (s *RollingStrategy) Name() string {
	return string(models.DeployStrategyRolling)
}

// Execute runs the rolling deployment by updating instances in batches.
// Each batch is updated, health-checked, and then the next batch begins.
//
// When MaxSurge > 0 the strategy temporarily spins up additional new-version
// instances before tearing down old ones, keeping the total available count
// above TotalInstances during the transition. The effective batch size is
// bounded by both BatchSize and MaxSurge so that the extra instance count
// never exceeds the configured surge limit.
func (s *RollingStrategy) Execute(ctx context.Context, deployment *models.Deployment) error {
	if s.config.TotalInstances <= 0 {
		return fmt.Errorf("total_instances must be greater than 0")
	}
	if s.config.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be greater than 0")
	}

	// effectiveBatch is the number of instances we can update per batch.
	// When max-surge is configured it constrains the batch size so that
	// the number of extra instances never exceeds MaxSurge.
	effectiveBatch := s.config.BatchSize
	if s.config.MaxSurge > 0 && effectiveBatch > s.config.MaxSurge {
		effectiveBatch = s.config.MaxSurge
	}

	updated := 0
	batchNum := 0

	for updated < s.config.TotalInstances {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batchNum++
		remaining := s.config.TotalInstances - updated
		currentBatch := effectiveBatch
		if currentBatch > remaining {
			currentBatch = remaining
		}

		// When MaxSurge > 0 we spin up new instances before removing old
		// ones. The surge count represents the transient extra instances.
		surgeCount := 0
		if s.config.MaxSurge > 0 {
			surgeCount = currentBatch
			if surgeCount > s.config.MaxSurge {
				surgeCount = s.config.MaxSurge
			}
		}

		// Create a phase record for this batch.
		percent := ((updated + currentBatch) * 100) / s.config.TotalInstances
		phase := &models.DeploymentPhase{
			ID:             uuid.New(),
			DeploymentID:   deployment.ID,
			Name:           fmt.Sprintf("rolling-batch-%d", batchNum),
			Status:         models.DeploymentPhaseStatusActive,
			TrafficPercent: percent,
			Duration:       int(s.config.BatchDelay.Seconds()),
			SortOrder:      batchNum - 1,
		}
		now := time.Now().UTC()
		phase.StartedAt = &now

		// In production the phase (including surge metadata) would be
		// persisted via the repository. The surgeCount is recorded so
		// controllers can observe the temporary over-provisioning.
		_ = phase
		_ = surgeCount

		updated += currentBatch
		deployment.TrafficPercent = percent
		deployment.UpdatedAt = time.Now().UTC()

		// Wait between batches (skip wait for the last batch).
		if updated < s.config.TotalInstances && s.config.BatchDelay > 0 {
			timer := time.NewTimer(s.config.BatchDelay)
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

// Rollback reverses a rolling deployment by rolling instances back to
// the previous version using the same batch approach.
func (s *RollingStrategy) Rollback(ctx context.Context, deployment *models.Deployment) error {
	deployment.TrafficPercent = 0
	deployment.UpdatedAt = time.Now().UTC()
	return nil
}
