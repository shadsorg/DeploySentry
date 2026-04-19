// Package engine implements the phase-driven engine that advances canary
// deployments through their traffic ramp-up phases, performing health checks
// and triggering rollbacks as necessary.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/deploy/strategies"
	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// MessagePublisher publishes events to the messaging system.
type MessagePublisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// MessageSubscriber subscribes to messaging subjects.
type MessageSubscriber interface {
	Subscribe(subject string, handler func(msg []byte)) error
}

// Engine drives canary deployments through their phases.
type Engine struct {
	repo          deploy.DeployRepository
	publisher     MessagePublisher
	healthMonitor *health.HealthMonitor
	logger        *slog.Logger
}

// New creates a new Engine. healthMonitor may be nil; when nil, health checks
// are skipped.
func New(repo deploy.DeployRepository, publisher MessagePublisher, healthMonitor *health.HealthMonitor, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		repo:          repo,
		publisher:     publisher,
		healthMonitor: healthMonitor,
		logger:        logger,
	}
}

// Start subscribes to the deployment.created event and begins processing
// canary deployments. It blocks until ctx is cancelled.
func (e *Engine) Start(ctx context.Context, subscriber MessageSubscriber) error {
	if err := subscriber.Subscribe("deployments.deployment.created", func(msg []byte) {
		var payload struct {
			DeploymentID string `json:"deployment_id"`
		}
		if err := json.Unmarshal(msg, &payload); err != nil {
			e.logger.Error("engine: failed to unmarshal deployment.created", "error", err)
			return
		}
		id, err := uuid.Parse(payload.DeploymentID)
		if err != nil {
			e.logger.Error("engine: invalid deployment_id in event", "error", err)
			return
		}
		go func() {
			if err := e.driveDeployment(ctx, id); err != nil {
				e.logger.Error("engine: driveDeployment error", "deployment_id", id, "error", err)
			}
		}()
	}); err != nil {
		return fmt.Errorf("engine: subscribe failed: %w", err)
	}

	<-ctx.Done()
	return ctx.Err()
}

// Advance manually advances a paused deployment past its current gate.
// It marks the currently active (gated) phase as passed before resuming so
// that driveDeployment continues from the next phase rather than re-entering
// the same manual gate.
func (e *Engine) Advance(ctx context.Context, deploymentID uuid.UUID) error {
	d, err := e.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("engine.Advance: get deployment: %w", err)
	}
	if d.Status != models.DeployStatusPaused {
		return fmt.Errorf("engine.Advance: deployment is not paused (status=%s)", d.Status)
	}

	// Mark the active (gated) phase as passed so driveDeployment skips it.
	activePhase, err := e.repo.GetActivePhase(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("engine.Advance: get active phase: %w", err)
	}
	if activePhase != nil {
		now := time.Now().UTC()
		activePhase.Status = models.DeploymentPhaseStatusPassed
		activePhase.CompletedAt = &now
		if err := e.repo.UpdatePhase(ctx, activePhase); err != nil {
			return fmt.Errorf("engine.Advance: mark phase passed: %w", err)
		}
	}

	if err := d.TransitionTo(models.DeployStatusRunning); err != nil {
		return fmt.Errorf("engine.Advance: transition to running: %w", err)
	}
	if err := e.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("engine.Advance: update deployment: %w", err)
	}
	go func() {
		if err := e.driveDeployment(ctx, deploymentID); err != nil {
			e.logger.Error("engine: Advance driveDeployment error", "deployment_id", deploymentID, "error", err)
		}
	}()
	return nil
}

// BuildPhases creates DeploymentPhase records from a CanaryConfig. The phases
// are not persisted; callers are responsible for persistence.
func BuildPhases(deploymentID uuid.UUID, config strategies.CanaryConfig) []*models.DeploymentPhase {
	phases := make([]*models.DeploymentPhase, 0, len(config.Steps))
	for i, step := range config.Steps {
		autoPromote := step.IsAutoPromote(config.AutoPromote)
		phase := &models.DeploymentPhase{
			ID:             uuid.New(),
			DeploymentID:   deploymentID,
			Name:           fmt.Sprintf("canary-%d%%", step.TrafficPercent),
			Status:         models.DeploymentPhaseStatusPending,
			TrafficPercent: step.TrafficPercent,
			Duration:       int(step.Duration.Seconds()),
			SortOrder:      i,
			AutoPromote:    autoPromote,
		}
		phases = append(phases, phase)
	}
	return phases
}

// driveDeployment is the main loop that advances a single deployment through
// all of its canary phases.
func (e *Engine) driveDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	// 1. Get deployment.
	d, err := e.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("driveDeployment: get deployment: %w", err)
	}

	// 2. Only process canary deployments.
	if d.Strategy != models.DeployStrategyCanary {
		return nil
	}

	// 3. If pending, transition to running.
	if d.Status == models.DeployStatusPending {
		if err := d.TransitionTo(models.DeployStatusRunning); err != nil {
			return fmt.Errorf("driveDeployment: transition pending->running: %w", err)
		}
		if err := e.repo.UpdateDeployment(ctx, d); err != nil {
			return fmt.Errorf("driveDeployment: update deployment to running: %w", err)
		}
	}

	if d.Status != models.DeployStatusRunning {
		// Not runnable at this time (e.g. completed, failed, etc.).
		return nil
	}

	// 4. Load phases from repo, or create from DefaultCanaryConfig if none exist.
	phases, err := e.repo.ListPhases(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("driveDeployment: list phases: %w", err)
	}
	if len(phases) == 0 {
		config := strategies.DefaultCanaryConfig()
		phases = BuildPhases(deploymentID, config)
		for _, ph := range phases {
			if err := e.repo.CreatePhase(ctx, ph); err != nil {
				return fmt.Errorf("driveDeployment: create phase: %w", err)
			}
		}
	}

	// 5. Find start position: first non-passed phase.
	startIdx := len(phases) // default: all done
	for i, ph := range phases {
		if ph.Status != models.DeploymentPhaseStatusPassed {
			startIdx = i
			break
		}
	}

	// Get previous deployment for event payloads.
	var prevDeployment *models.Deployment
	if d.PreviousDeploymentID != nil {
		prevDeployment, _ = e.repo.GetDeployment(ctx, *d.PreviousDeploymentID)
	}

	config := strategies.DefaultCanaryConfig()

	// 6. Iterate from start.
	for i := startIdx; i < len(phases); i++ {
		ph := phases[i]

		// a. Set phase active.
		now := time.Now().UTC()
		ph.Status = models.DeploymentPhaseStatusActive
		ph.StartedAt = &now
		if err := e.repo.UpdatePhase(ctx, ph); err != nil {
			return fmt.Errorf("driveDeployment: update phase active: %w", err)
		}

		// b. Update deployment traffic percent.
		d.TrafficPercent = ph.TrafficPercent
		d.UpdatedAt = time.Now().UTC()
		if err := e.repo.UpdateDeployment(ctx, d); err != nil {
			return fmt.Errorf("driveDeployment: update deployment traffic: %w", err)
		}

		// c. Publish phase_changed event.
		if err := e.publishPhaseChanged(ctx, d, ph, prevDeployment); err != nil {
			e.logger.Warn("engine: failed to publish phase_changed", "error", err)
		}

		// d. Wait phase duration.
		if ph.Duration > 0 {
			dur := time.Duration(ph.Duration) * time.Second
			timer := time.NewTimer(dur)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		// e. Check health if monitor is available.
		if e.healthMonitor != nil {
			h, err := e.healthMonitor.GetHealth(deploymentID)
			if err == nil && !h.Healthy {
				// f. Unhealthy + RollbackOnFailure: rollback.
				if config.RollbackOnFailure {
					ph.Status = models.DeploymentPhaseStatusFailed
					completedAt := time.Now().UTC()
					ph.CompletedAt = &completedAt
					_ = e.repo.UpdatePhase(ctx, ph)
					return e.triggerRollback(ctx, d, h)
				}
			}
		}

		// g. If not auto-promote: pause and return.
		if !ph.AutoPromote {
			if err := d.TransitionTo(models.DeployStatusPaused); err != nil {
				return fmt.Errorf("driveDeployment: transition to paused: %w", err)
			}
			if err := e.repo.UpdateDeployment(ctx, d); err != nil {
				return fmt.Errorf("driveDeployment: update deployment paused: %w", err)
			}
			return nil
		}

		// h. Mark phase passed.
		ph.Status = models.DeploymentPhaseStatusPassed
		completedAt := time.Now().UTC()
		ph.CompletedAt = &completedAt
		if err := e.repo.UpdatePhase(ctx, ph); err != nil {
			return fmt.Errorf("driveDeployment: update phase passed: %w", err)
		}
	}

	// 7. All phases complete: set traffic=100, transition to completed.
	d.TrafficPercent = 100
	if err := d.TransitionTo(models.DeployStatusCompleted); err != nil {
		return fmt.Errorf("driveDeployment: transition to completed: %w", err)
	}
	if err := e.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("driveDeployment: update deployment completed: %w", err)
	}

	// Publish completed event.
	payload, _ := json.Marshal(map[string]interface{}{
		"deployment_id":   d.ID,
		"application_id":  d.ApplicationID,
		"environment_id":  d.EnvironmentID,
		"artifact":        d.Artifact,
		"version":         d.Version,
		"traffic_percent": 100,
		"status":          string(d.Status),
		"timestamp":       time.Now().UTC(),
	})
	if err := e.publisher.Publish(ctx, "deployments.deployment.completed", payload); err != nil {
		e.logger.Warn("engine: failed to publish deployment.completed", "error", err)
	}

	return nil
}

// triggerRollback transitions the deployment to rolled_back, creates a
// RollbackRecord, and publishes the relevant events.
func (e *Engine) triggerRollback(ctx context.Context, d *models.Deployment, h *health.DeploymentHealth) error {
	// 1. Transition to rolled_back.
	if err := d.TransitionTo(models.DeployStatusRolledBack); err != nil {
		return fmt.Errorf("triggerRollback: transition: %w", err)
	}
	d.TrafficPercent = 0
	if err := e.repo.UpdateDeployment(ctx, d); err != nil {
		return fmt.Errorf("triggerRollback: update deployment: %w", err)
	}

	// 2. Create RollbackRecord.
	var healthScore *float64
	if h != nil {
		score := h.Overall
		healthScore = &score
	}
	record := &models.RollbackRecord{
		ID:                 uuid.New(),
		DeploymentID:       d.ID,
		TargetDeploymentID: d.PreviousDeploymentID,
		Reason:             "health check failure",
		HealthScore:        healthScore,
		Automatic:          true,
		Strategy:           string(d.Strategy),
		StartedAt:          time.Now().UTC(),
		CreatedAt:          time.Now().UTC(),
	}
	if err := e.repo.CreateRollbackRecord(ctx, record); err != nil {
		e.logger.Warn("engine: failed to create rollback record", "error", err)
	}

	// 3. Publish phase_changed event (traffic 0).
	if err := e.publishPhaseChanged(ctx, d, nil, nil); err != nil {
		e.logger.Warn("engine: failed to publish phase_changed on rollback", "error", err)
	}

	// 4. Publish deployment.rolled_back event.
	payload, _ := json.Marshal(map[string]interface{}{
		"deployment_id":   d.ID,
		"application_id":  d.ApplicationID,
		"environment_id":  d.EnvironmentID,
		"artifact":        d.Artifact,
		"version":         d.Version,
		"traffic_percent": 0,
		"status":          string(d.Status),
		"timestamp":       time.Now().UTC(),
	})
	if err := e.publisher.Publish(ctx, "deployments.deployment.rolled_back", payload); err != nil {
		e.logger.Warn("engine: failed to publish deployment.rolled_back", "error", err)
	}

	return nil
}

// phaseChangedPayload is the JSON structure published to
// deployments.deployment.phase_changed.
type phaseChangedPayload struct {
	DeploymentID         string             `json:"deployment_id"`
	ApplicationID        string             `json:"application_id"`
	EnvironmentID        string             `json:"environment_id"`
	Artifact             string             `json:"artifact"`
	Version              string             `json:"version"`
	DesiredTrafficPercent int               `json:"desired_traffic_percent"`
	Status               string             `json:"status"`
	Timestamp            time.Time          `json:"timestamp"`
	CurrentPhase         *phaseInfo         `json:"current_phase,omitempty"`
	PreviousDeployment   *prevDeployInfo    `json:"previous_deployment,omitempty"`
}

type phaseInfo struct {
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	AutoPromote bool   `json:"auto_promote"`
}

type prevDeployInfo struct {
	DeploymentID string `json:"deployment_id"`
	Artifact     string `json:"artifact"`
	Version      string `json:"version"`
}

// publishPhaseChanged marshals and publishes the phase_changed event.
// phase may be nil (e.g. during rollback).
func (e *Engine) publishPhaseChanged(ctx context.Context, d *models.Deployment, phase *models.DeploymentPhase, prev *models.Deployment) error {
	p := phaseChangedPayload{
		DeploymentID:          d.ID.String(),
		ApplicationID:         d.ApplicationID.String(),
		EnvironmentID:         d.EnvironmentID.String(),
		Artifact:              d.Artifact,
		Version:               d.Version,
		DesiredTrafficPercent: d.TrafficPercent,
		Status:                string(d.Status),
		Timestamp:             time.Now().UTC(),
	}
	if phase != nil {
		p.CurrentPhase = &phaseInfo{
			Name:        phase.Name,
			SortOrder:   phase.SortOrder,
			AutoPromote: phase.AutoPromote,
		}
	}
	if prev != nil {
		p.PreviousDeployment = &prevDeployInfo{
			DeploymentID: prev.ID.String(),
			Artifact:     prev.Artifact,
			Version:      prev.Version,
		}
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("publishPhaseChanged: marshal: %w", err)
	}
	return e.publisher.Publish(ctx, "deployments.deployment.phase_changed", payload)
}
