package rollout

import (
	"context"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RolloutGroupService provides CRUD + attach semantics for RolloutGroup bundles.
type RolloutGroupService struct {
	groups   RolloutGroupRepository
	rollouts RolloutRepository
}

// NewRolloutGroupService builds a RolloutGroupService.
func NewRolloutGroupService(g RolloutGroupRepository, ro RolloutRepository) *RolloutGroupService {
	return &RolloutGroupService{groups: g, rollouts: ro}
}

// Create persists a group. Defaults coordination_policy to independent.
func (s *RolloutGroupService) Create(ctx context.Context, g *models.RolloutGroup) error {
	if g.CoordinationPolicy == "" {
		g.CoordinationPolicy = models.CoordinationIndependent
	}
	return s.groups.Create(ctx, g)
}

// Get returns a group by ID.
func (s *RolloutGroupService) Get(ctx context.Context, id uuid.UUID) (*models.RolloutGroup, error) {
	return s.groups.Get(ctx, id)
}

// List returns groups defined directly on the scope.
func (s *RolloutGroupService) List(ctx context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutGroup, error) {
	return s.groups.ListByScope(ctx, st, sid)
}

// Update persists changes (name, description, coordination_policy).
func (s *RolloutGroupService) Update(ctx context.Context, g *models.RolloutGroup) error {
	return s.groups.Update(ctx, g)
}

// Delete removes a group row. Rollouts keep their release_id (dangling);
// app reads tolerate missing groups.
func (s *RolloutGroupService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.groups.Delete(ctx, id)
}

// Attach sets the rollout's release_id to this group. Both must exist.
func (s *RolloutGroupService) Attach(ctx context.Context, groupID uuid.UUID, rolloutID uuid.UUID) error {
	if _, err := s.groups.Get(ctx, groupID); err != nil {
		return fmt.Errorf("group not found: %w", err)
	}
	id := groupID
	return s.rollouts.SetReleaseID(ctx, rolloutID, &id)
}

// Members returns all rollouts attached to a group.
func (s *RolloutGroupService) Members(ctx context.Context, groupID uuid.UUID) ([]*models.Rollout, error) {
	return s.rollouts.ListByRelease(ctx, groupID)
}

// ActiveSiblings returns rollouts in the group with status in
// (pending, active, paused, awaiting_approval), excluding the given origin id.
func (s *RolloutGroupService) ActiveSiblings(ctx context.Context, groupID uuid.UUID, excludeID uuid.UUID) ([]*models.Rollout, error) {
	all, err := s.rollouts.ListByRelease(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []*models.Rollout
	for _, r := range all {
		if r.ID == excludeID {
			continue
		}
		switch r.Status {
		case models.RolloutPending, models.RolloutActive, models.RolloutPaused, models.RolloutAwaitingApproval:
			out = append(out, r)
		}
	}
	return out, nil
}

// GetPolicy is a convenience lookup for the coordinator.
func (s *RolloutGroupService) GetPolicy(ctx context.Context, groupID uuid.UUID) (models.CoordinationPolicy, error) {
	g, err := s.groups.Get(ctx, groupID)
	if err != nil {
		return "", err
	}
	return g.CoordinationPolicy, nil
}
