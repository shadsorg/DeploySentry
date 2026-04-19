package rollout

import (
	"context"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// SiblingActor is the subset of RolloutService the coordinator uses to act on
// siblings. Implemented by RolloutService; kept as an interface so the
// coordinator can be unit-tested without the full service surface.
type SiblingActor interface {
	Pause(ctx context.Context, rolloutID uuid.UUID, actor uuid.UUID, reason string) error
	Rollback(ctx context.Context, rolloutID uuid.UUID, actor uuid.UUID, reason string) error
}

// Coordinator applies a group's coordination_policy to sibling rollouts when
// any member rollout rolls back.
type Coordinator struct {
	groups *RolloutGroupService
	actor  SiblingActor
}

// NewCoordinator builds a Coordinator.
func NewCoordinator(gs *RolloutGroupService, a SiblingActor) *Coordinator {
	return &Coordinator{groups: gs, actor: a}
}

// OnRollback is invoked when a rollout transitions to rolled_back. Looks up
// the rollout's group (if any) and applies the coordination_policy to active siblings.
func (c *Coordinator) OnRollback(ctx context.Context, rolledBackID uuid.UUID) error {
	ro, err := c.groups.rollouts.Get(ctx, rolledBackID)
	if err != nil {
		return fmt.Errorf("lookup rolled-back rollout: %w", err)
	}
	if ro.ReleaseID == nil {
		return nil
	}
	policy, err := c.groups.GetPolicy(ctx, *ro.ReleaseID)
	if err != nil {
		return fmt.Errorf("lookup group policy: %w", err)
	}
	if policy == models.CoordinationIndependent {
		return nil
	}
	siblings, err := c.groups.ActiveSiblings(ctx, *ro.ReleaseID, rolledBackID)
	if err != nil {
		return fmt.Errorf("list siblings: %w", err)
	}

	systemActor := uuid.Nil
	reason := fmt.Sprintf("sibling_aborted:%s", rolledBackID)

	for _, s := range siblings {
		switch policy {
		case models.CoordinationPauseOnSiblingAbort:
			if s.Status == models.RolloutActive {
				if err := c.actor.Pause(ctx, s.ID, systemActor, reason); err != nil {
					return fmt.Errorf("pause sibling %s: %w", s.ID, err)
				}
			}
		case models.CoordinationCascadeAbort:
			if err := c.actor.Rollback(ctx, s.ID, systemActor, reason); err != nil {
				return fmt.Errorf("rollback sibling %s: %w", s.ID, err)
			}
		}
	}
	return nil
}
