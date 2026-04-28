package applicator

import (
	"context"
	"errors"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/models"
)

// Router dispatches Applicator calls to an inner applicator chosen by
// rollout.TargetType. Used by cmd/api wiring so one engine can drive deploy
// and config rollouts from the same goroutine.
type Router struct {
	deploy Applicator
	config Applicator
}

// NewRouter builds a Router. Either inner may be nil at construction time, but
// calls with a matching TargetType on a nil inner will return ErrNoApplicator.
func NewRouter(deploy, config Applicator) *Router {
	return &Router{deploy: deploy, config: config}
}

var _ Applicator = (*Router)(nil)

// ErrNoApplicator is returned when no inner applicator is registered for the
// rollout's TargetType.
var ErrNoApplicator = errors.New("no applicator registered for target_type")

func (r *Router) pick(ro *models.Rollout) (Applicator, error) {
	switch ro.TargetType {
	case models.TargetTypeDeploy:
		if r.deploy == nil {
			return nil, fmt.Errorf("%w: deploy", ErrNoApplicator)
		}
		return r.deploy, nil
	case models.TargetTypeConfig:
		if r.config == nil {
			return nil, fmt.Errorf("%w: config", ErrNoApplicator)
		}
		return r.config, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrNoApplicator, string(ro.TargetType))
}

// Apply dispatches to the target-type applicator.
func (r *Router) Apply(ctx context.Context, ro *models.Rollout, step models.Step) error {
	a, err := r.pick(ro)
	if err != nil {
		return err
	}
	return a.Apply(ctx, ro, step)
}

// Revert dispatches to the target-type applicator.
func (r *Router) Revert(ctx context.Context, ro *models.Rollout) error {
	a, err := r.pick(ro)
	if err != nil {
		return err
	}
	return a.Revert(ctx, ro)
}

// CurrentSignal dispatches to the target-type applicator.
func (r *Router) CurrentSignal(ctx context.Context, ro *models.Rollout, override *models.SignalSource) (HealthScore, error) {
	a, err := r.pick(ro)
	if err != nil {
		return HealthScore{}, err
	}
	return a.CurrentSignal(ctx, ro, override)
}
