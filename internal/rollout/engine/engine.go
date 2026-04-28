// Package engine drives rollouts through their strategy phases.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// RepoSet is the subset of rollout repositories the engine needs.
// Implemented by composing RolloutRepo + RolloutPhaseRepo + RolloutEventRepo
// at the wiring layer.
type RepoSet interface {
	GetRollout(ctx context.Context, id uuid.UUID) (*models.Rollout, error)
	UpdateRolloutStatus(ctx context.Context, id uuid.UUID, status models.RolloutStatus, reason *string) error
	UpdateRolloutPhasePointer(ctx context.Context, id uuid.UUID, phaseIdx int, startedAt, lastHealthy *time.Time) error
	MarkRolloutCompleted(ctx context.Context, id uuid.UUID) error

	BulkInsertPhases(ctx context.Context, phases []*models.RolloutPhase) error
	ListPhases(ctx context.Context, rolloutID uuid.UUID) ([]*models.RolloutPhase, error)
	UpdatePhaseStatus(ctx context.Context, id uuid.UUID, status models.PhaseStatus, enteredAt, exitedAt *time.Time, appliedPct, healthScore *float64, notes string) error

	InsertEvent(ctx context.Context, e *models.RolloutEvent) error
}

// Publisher publishes events to the message bus.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// EngineOptions tunes the polling cadence and time source (for tests).
type EngineOptions struct {
	PollInterval time.Duration // default 2s
	Logger       *slog.Logger
	Now          func() time.Time // default time.Now
}

// Engine drives rollouts through their strategy phases.
type Engine struct {
	repo RepoSet
	app  applicator.Applicator
	pub  Publisher
	opts EngineOptions
}

// New builds a new Engine.
func New(repo RepoSet, app applicator.Applicator, pub Publisher, opts EngineOptions) *Engine {
	if opts.PollInterval == 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Engine{repo: repo, app: app, pub: pub, opts: opts}
}

// ErrRollbackTriggered is returned by DriveRollout when the rollout ended in
// the RolledBack state.
var ErrRollbackTriggered = errors.New("rollout rolled back")

// DriveRollout walks a Rollout through its phases. It blocks until the rollout
// reaches a terminal state or the context is cancelled.
func (e *Engine) DriveRollout(ctx context.Context, rolloutID uuid.UUID) error {
	ro, err := e.repo.GetRollout(ctx, rolloutID)
	if err != nil {
		return fmt.Errorf("get rollout: %w", err)
	}
	if ro.IsTerminal() {
		return nil
	}
	if err := e.ensurePhasesExist(ctx, ro); err != nil {
		return err
	}
	_ = e.repo.UpdateRolloutStatus(ctx, ro.ID, models.RolloutActive, nil)
	ro.Status = models.RolloutActive

	phases, err := e.repo.ListPhases(ctx, ro.ID)
	if err != nil {
		return err
	}

	startIdx := ro.CurrentPhaseIndex
	for idx := startIdx; idx < len(phases); idx++ {
		phase := phases[idx]
		if phase.Status == models.PhasePassed {
			continue
		}
		if err := e.runPhase(ctx, ro, phase); err != nil {
			if errors.Is(err, errRolledBack) {
				return ErrRollbackTriggered
			}
			return err
		}
	}

	_ = e.repo.MarkRolloutCompleted(ctx, ro.ID)
	ro.Status = models.RolloutSucceeded
	now := e.opts.Now()
	ro.CompletedAt = &now
	e.emitEvent(ctx, ro.ID, models.EventCompleted, nil, nil)
	e.publish(ctx, "rollouts.rollout.completed", ro.ID)
	return nil
}

var errRolledBack = errors.New("phase triggered rollback")

func (e *Engine) ensurePhasesExist(ctx context.Context, ro *models.Rollout) error {
	existing, err := e.repo.ListPhases(ctx, ro.ID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	phases := make([]*models.RolloutPhase, len(ro.StrategySnapshot.Steps))
	for i, step := range ro.StrategySnapshot.Steps {
		phases[i] = &models.RolloutPhase{
			RolloutID:    ro.ID,
			PhaseIndex:   i,
			StepSnapshot: step,
			Status:       models.PhasePending,
		}
	}
	return e.repo.BulkInsertPhases(ctx, phases)
}

// runPhase executes a single phase: apply, poll for promote/abort, exit.
func (e *Engine) runPhase(ctx context.Context, ro *models.Rollout, phase *models.RolloutPhase) error {
	now := e.opts.Now()
	if err := e.app.Apply(ctx, ro, phase.StepSnapshot); err != nil {
		return fmt.Errorf("apply phase %d: %w", phase.PhaseIndex, err)
	}
	enteredAt := now
	_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseActive, &enteredAt, nil, &phase.StepSnapshot.Percent, nil, "")
	_ = e.repo.UpdateRolloutPhasePointer(ctx, ro.ID, phase.PhaseIndex, &enteredAt, nil)
	e.emitEvent(ctx, ro.ID, models.EventPhaseEntered, nil, map[string]any{"phase_index": phase.PhaseIndex, "percent": phase.StepSnapshot.Percent})
	e.publish(ctx, "rollouts.rollout.phase_entered", ro.ID)

	threshold := EffectiveHealthThreshold(&ro.StrategySnapshot, phase.StepSnapshot)
	// Approval is tracked inline: a phase enters awaiting_approval, the loop
	// returns, and a later external Approve call resets status back to Active
	// which re-enters this driveRollout via DriveRollout (caller-driven).
	approvalGranted := ro.Status != models.RolloutAwaitingApproval

	var lastHealthySince *time.Time
	ticker := time.NewTicker(e.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		now := e.opts.Now()
		elapsed := now.Sub(enteredAt)

		signal, err := e.app.CurrentSignal(ctx, ro, phase.StepSnapshot.SignalOverride)
		if err != nil {
			// Transient signal failure — keep polling. Log-worthy.
			continue
		}

		// Abort-conditions are checked even before min_duration.
		if trip := EvaluateAbortConditions(phase.StepSnapshot, signal); trip != nil {
			exit := now
			_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseRolledBack, nil, &exit, nil, &signal.Score, fmt.Sprintf("abort: %s %s %g", trip.Metric, trip.Operator, trip.Threshold))
			e.emitEvent(ctx, ro.ID, models.EventAbortConditionTripped, nil, map[string]any{"metric": trip.Metric})
			e.rollback(ctx, ro, fmt.Sprintf("abort condition tripped: %s %s %g", trip.Metric, trip.Operator, trip.Threshold))
			return errRolledBack
		}

		// Approval gate check.
		if phase.StepSnapshot.Approval != nil && !approvalGranted {
			_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseAwaitingApproval, nil, nil, nil, nil, "awaiting approval")
			_ = e.repo.UpdateRolloutStatus(ctx, ro.ID, models.RolloutAwaitingApproval, nil)
			e.publish(ctx, "rollouts.rollout.awaiting_approval", ro.ID)
			// Return without advancing; external signal (approve call) re-drives the rollout.
			return fmt.Errorf("awaiting approval")
		}

		// Track last-healthy-since.
		if signal.Score >= threshold {
			if lastHealthySince == nil {
				t := now
				lastHealthySince = &t
				_ = e.repo.UpdateRolloutPhasePointer(ctx, ro.ID, phase.PhaseIndex, &enteredAt, lastHealthySince)
			}
		} else {
			lastHealthySince = nil
			_ = e.repo.UpdateRolloutPhasePointer(ctx, ro.ID, phase.PhaseIndex, &enteredAt, nil)
		}

		// Promote if min_duration elapsed AND we've been healthy for bake_time.
		if elapsed >= phase.StepSnapshot.MinDuration {
			if lastHealthySince != nil && now.Sub(*lastHealthySince) >= phase.StepSnapshot.BakeTimeHealthy {
				exit := now
				_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhasePassed, nil, &exit, nil, &signal.Score, "")
				e.emitEvent(ctx, ro.ID, models.EventPhaseExited, nil, map[string]any{"phase_index": phase.PhaseIndex, "score": signal.Score})
				e.publish(ctx, "rollouts.rollout.phase_exited", ro.ID)
				return nil
			}
		}

		// Max-duration timeout → rollback if unhealthy.
		if phase.StepSnapshot.MaxDuration > 0 && elapsed >= phase.StepSnapshot.MaxDuration {
			if lastHealthySince == nil || now.Sub(*lastHealthySince) < phase.StepSnapshot.BakeTimeHealthy {
				exit := now
				_ = e.repo.UpdatePhaseStatus(ctx, phase.ID, models.PhaseRolledBack, nil, &exit, nil, &signal.Score, "unhealthy at max_duration")
				e.rollback(ctx, ro, fmt.Sprintf("unhealthy at max_duration in phase %d", phase.PhaseIndex))
				return errRolledBack
			}
		}
	}
}

func (e *Engine) rollback(ctx context.Context, ro *models.Rollout, reason string) {
	r := reason
	_ = e.repo.UpdateRolloutStatus(ctx, ro.ID, models.RolloutRolledBack, &r)
	_ = e.app.Revert(ctx, ro)
	e.emitEvent(ctx, ro.ID, models.EventRollbackTriggered, &r, nil)
	e.publish(ctx, "rollouts.rollout.rolled_back", ro.ID)
}

func (e *Engine) emitEvent(ctx context.Context, rolloutID uuid.UUID, evt models.EventType, reason *string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	_ = e.repo.InsertEvent(ctx, &models.RolloutEvent{
		RolloutID: rolloutID,
		EventType: evt,
		ActorType: "system",
		Reason:    reason,
		Payload:   payload,
	})
}

func (e *Engine) publish(ctx context.Context, subject string, rolloutID uuid.UUID) {
	payload, _ := json.Marshal(map[string]any{"rollout_id": rolloutID.String()})
	_ = e.pub.Publish(ctx, subject, payload)
}
