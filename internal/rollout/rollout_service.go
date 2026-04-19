package rollout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Publisher is the subset of NATS publishing used by the service.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// RolloutService owns rollout creation and the 6 runtime controls.
type RolloutService struct {
	rollouts RolloutRepository
	phases   RolloutPhaseRepository
	events   RolloutEventRepository
	pub      Publisher
}

// NewRolloutService builds a RolloutService.
func NewRolloutService(r RolloutRepository, p RolloutPhaseRepository, e RolloutEventRepository, pub Publisher) *RolloutService {
	return &RolloutService{rollouts: r, phases: p, events: e, pub: pub}
}

// Sentinel errors.
var (
	ErrReasonRequired        = errors.New("reason is required for this action")
	ErrInvalidStateForOp     = errors.New("rollout is not in a valid state for this operation")
	ErrAlreadyActiveOnTarget = errors.New("an active rollout already exists for this target")
)

// AttachDeploy creates a pending Rollout for a deployment, using the provided
// snapshot. It enforces one-active-per-deployment by returning
// ErrAlreadyActiveOnTarget.
func (s *RolloutService) AttachDeploy(ctx context.Context, depID uuid.UUID, snapshot *models.Strategy, releaseID *uuid.UUID, createdBy *uuid.UUID) (*models.Rollout, error) {
	if existing, _ := s.rollouts.GetActiveByDeployment(ctx, depID); existing != nil {
		return existing, ErrAlreadyActiveOnTarget
	}
	ref := depID.String()
	ro := &models.Rollout{
		ReleaseID:        releaseID,
		TargetType:       models.TargetTypeDeploy,
		TargetRef:        models.RolloutTargetRef{DeploymentID: &ref},
		StrategySnapshot: *snapshot,
		SignalSource:     models.SignalSource{Kind: "app_env"},
		Status:           models.RolloutPending,
		CreatedBy:        createdBy,
	}
	if err := s.rollouts.Create(ctx, ro); err != nil {
		return nil, fmt.Errorf("create rollout: %w", err)
	}
	s.emit(ctx, ro.ID, models.EventAttached, createdBy, nil, nil)
	s.publishRolloutSubject(ctx, "rollouts.rollout.created", ro.ID)
	return ro, nil
}

// AttachConfig creates a pending Rollout for a targeting rule. It captures the
// current (pre-rollout) percentage so Revert can restore it. Returns
// ErrAlreadyActiveOnTarget if a rollout already owns this rule.
func (s *RolloutService) AttachConfig(ctx context.Context, ruleID uuid.UUID, previousPct int, snapshot *models.Strategy, releaseID *uuid.UUID, createdBy *uuid.UUID) (*models.Rollout, error) {
	if existing, _ := s.rollouts.GetActiveByRule(ctx, ruleID); existing != nil {
		return existing, ErrAlreadyActiveOnTarget
	}
	ref := ruleID.String()
	prev := previousPct
	ro := &models.Rollout{
		ReleaseID:        releaseID,
		TargetType:       models.TargetTypeConfig,
		TargetRef:        models.RolloutTargetRef{RuleID: &ref, PreviousPercentage: &prev},
		StrategySnapshot: *snapshot,
		SignalSource:     models.SignalSource{Kind: "app_env"},
		Status:           models.RolloutPending,
		CreatedBy:        createdBy,
	}
	if err := s.rollouts.Create(ctx, ro); err != nil {
		return nil, fmt.Errorf("create rollout: %w", err)
	}
	s.emit(ctx, ro.ID, models.EventAttached, createdBy, nil, nil)
	s.publishRolloutSubject(ctx, "rollouts.rollout.created", ro.ID)
	return ro, nil
}

// GetActiveByRule is a pass-through for handler 409 checks on config rollouts.
func (s *RolloutService) GetActiveByRule(ctx context.Context, ruleID uuid.UUID) (*models.Rollout, error) {
	return s.rollouts.GetActiveByRule(ctx, ruleID)
}

// Pause freezes an active rollout. The engine's next tick observes the status
// change; callers assume eventual consistency.
func (s *RolloutService) Pause(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventPaused, models.RolloutPaused, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive
	}, false)
}

// Resume unfreezes a paused rollout.
func (s *RolloutService) Resume(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventResumed, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutPaused
	}, false)
}

// Promote signals the engine to skip remaining dwell on the current phase.
// The engine must still verify health before advancing.
func (s *RolloutService) Promote(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventPromoted, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive || cur == models.RolloutPaused
	}, false)
}

// ForcePromote advances even if unhealthy. Requires a reason.
func (s *RolloutService) ForcePromote(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	return s.transition(ctx, id, actorID, reason, models.EventForcePromoted, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive || cur == models.RolloutPaused || cur == models.RolloutAwaitingApproval
	}, false)
}

// Approve grants approval on an awaiting-approval phase.
func (s *RolloutService) Approve(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	return s.transition(ctx, id, actorID, reason, models.EventApproved, models.RolloutActive, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutAwaitingApproval
	}, false)
}

// Rollback aborts the rollout and reverts. Requires a reason.
func (s *RolloutService) Rollback(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	return s.transition(ctx, id, actorID, reason, models.EventRollbackTriggered, models.RolloutRolledBack, func(cur models.RolloutStatus) bool {
		return cur == models.RolloutActive || cur == models.RolloutPaused || cur == models.RolloutAwaitingApproval
	}, true)
}

// transition is the single state-machine helper for all six runtime controls.
func (s *RolloutService) transition(ctx context.Context, id uuid.UUID, actorID uuid.UUID, reason string, evt models.EventType, target models.RolloutStatus, allow func(models.RolloutStatus) bool, withReason bool) error {
	ro, err := s.rollouts.Get(ctx, id)
	if err != nil {
		return err
	}
	if !allow(ro.Status) {
		return fmt.Errorf("%w: current=%s", ErrInvalidStateForOp, ro.Status)
	}
	var reasonPtr *string
	if withReason || reason != "" {
		r := reason
		reasonPtr = &r
	}
	if err := s.rollouts.UpdateStatus(ctx, id, target, reasonPtr); err != nil {
		return err
	}
	ro.Status = target
	s.emit(ctx, id, evt, &actorID, reasonPtr, nil)
	s.publishRolloutSubject(ctx, fmt.Sprintf("rollouts.rollout.%s", evt), id)
	return nil
}

func (s *RolloutService) emit(ctx context.Context, rolloutID uuid.UUID, evt models.EventType, actor *uuid.UUID, reason *string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	actorType := "system"
	if actor != nil {
		actorType = "user"
	}
	_ = s.events.Insert(ctx, &models.RolloutEvent{
		RolloutID: rolloutID, EventType: evt, ActorType: actorType, ActorID: actor,
		Reason: reason, Payload: payload,
	})
}

func (s *RolloutService) publishRolloutSubject(ctx context.Context, subject string, id uuid.UUID) {
	payload, _ := json.Marshal(map[string]string{"rollout_id": id.String()})
	_ = s.pub.Publish(ctx, subject, payload)
}

// Get returns a rollout by ID.
func (s *RolloutService) Get(ctx context.Context, id uuid.UUID) (*models.Rollout, error) {
	return s.rollouts.Get(ctx, id)
}

// GetActiveByDeployment is a pass-through for handler 409 checks.
func (s *RolloutService) GetActiveByDeployment(ctx context.Context, depID uuid.UUID) (*models.Rollout, error) {
	return s.rollouts.GetActiveByDeployment(ctx, depID)
}

// List returns rollouts per filter.
func (s *RolloutService) List(ctx context.Context, opts RolloutListOptions) ([]*models.Rollout, error) {
	return s.rollouts.List(ctx, opts)
}

// Events returns the audit stream for a rollout.
func (s *RolloutService) Events(ctx context.Context, id uuid.UUID, limit int) ([]*models.RolloutEvent, error) {
	return s.events.ListByRollout(ctx, id, limit)
}
