// Package applicator defines the contract a rollout target must satisfy so
// the rollout engine can advance it through phases.
package applicator

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
)

// HealthScore is a normalized health reading in [0, 1].
type HealthScore struct {
	Score        float64 // 0..1; 1 = healthy
	ErrorRate    float64 // as fraction (e.g., 0.01 = 1%)
	LatencyP99Ms float64
	LatencyP50Ms float64
	RequestRate  float64
}

// Applicator applies a rollout step to its target and reads back signals.
type Applicator interface {
	// Apply sets the target to the percent declared in step. Called once per phase entry.
	Apply(ctx context.Context, rollout *models.Rollout, step models.Step) error

	// Revert resets the target to its previous (safe) value. Called on rollback.
	Revert(ctx context.Context, rollout *models.Rollout) error

	// CurrentSignal reads the current health score for this rollout, honoring
	// a per-step signal override when provided (may be nil).
	CurrentSignal(ctx context.Context, rollout *models.Rollout, override *models.SignalSource) (HealthScore, error)
}
