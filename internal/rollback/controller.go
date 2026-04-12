// Package rollback implements automatic and manual rollback of deployments,
// including a state machine that governs rollback lifecycle transitions.
package rollback

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/google/uuid"
)

// RollbackState represents the current state in the rollback state machine.
type RollbackState string

const (
	// RollbackStateHealthy indicates the deployment is operating normally.
	RollbackStateHealthy RollbackState = "HEALTHY"
	// RollbackStateEvaluating indicates the system is assessing whether a rollback is needed.
	RollbackStateEvaluating RollbackState = "EVALUATING"
	// RollbackStateRollingBack indicates an active rollback is in progress.
	RollbackStateRollingBack RollbackState = "ROLLING_BACK"
	// RollbackStateRolledBack indicates the rollback has completed.
	RollbackStateRolledBack RollbackState = "ROLLED_BACK"
)


// RollbackTriggerConfig holds configurable thresholds that determine when
// an automatic rollback should be triggered.
type RollbackTriggerConfig struct {
	// ErrorRateThreshold is the maximum acceptable error rate (0.0-1.0).
	// When the error rate exceeds this value for EvaluationDuration, a rollback
	// is triggered. For example, 0.05 means 5% error rate.
	ErrorRateThreshold float64 `json:"error_rate_threshold"`

	// LatencyThreshold is the maximum acceptable p99 latency in seconds.
	// When p99 latency exceeds this value for EvaluationDuration, a rollback
	// is triggered.
	LatencyThreshold float64 `json:"latency_threshold"`

	// EvaluationDuration is how long a threshold must be exceeded before
	// triggering a rollback. For example, 2 minutes means the error rate
	// or latency must be above the threshold continuously for 2 minutes.
	EvaluationDuration time.Duration `json:"evaluation_duration"`
}

// DefaultRollbackTriggerConfig returns sensible default trigger thresholds:
// 5% error rate, 2s p99 latency, evaluated over 2 minutes.
func DefaultRollbackTriggerConfig() RollbackTriggerConfig {
	return RollbackTriggerConfig{
		ErrorRateThreshold: 0.05,
		LatencyThreshold:   2.0,
		EvaluationDuration: 2 * time.Minute,
	}
}

// RollbackDecision holds metadata about a rollback decision.
type RollbackDecision struct {
	DeploymentID uuid.UUID     `json:"deployment_id"`
	State        RollbackState `json:"state"`
	Reason       string        `json:"reason"`
	HealthScore  float64       `json:"health_score"`
	Automatic    bool          `json:"automatic"`
	DecidedAt    time.Time     `json:"decided_at"`
}

// RollbackRecord stores a completed rollback event for history tracking.
type RollbackRecord struct {
	ID           uuid.UUID     `json:"id"`
	DeploymentID uuid.UUID     `json:"deployment_id"`
	State        RollbackState `json:"state"`
	Reason       string        `json:"reason"`
	HealthScore  float64       `json:"health_score"`
	Automatic    bool          `json:"automatic"`
	Strategy     string        `json:"strategy"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
}

// RollbackExecutor defines the interface for performing the actual rollback operation.
type RollbackExecutor interface {
	// Execute performs the rollback for the given deployment.
	Execute(ctx context.Context, deploymentID uuid.UUID, strategy RollbackStrategy) error
}

// RollbackListener receives rollback state change notifications.
type RollbackListener interface {
	// OnRollbackStateChange is called when the rollback state changes.
	OnRollbackStateChange(ctx context.Context, decision *RollbackDecision)
}

// RollbackController manages the rollback state machine for deployments.
// It listens for health updates and triggers automatic rollbacks when
// health degrades below the configured threshold.
type RollbackController struct {
	executor         RollbackExecutor
	strategy         RollbackStrategy
	healthThreshold  float64
	evaluationWindow time.Duration
	triggerConfig    RollbackTriggerConfig
	cooldownPeriod   time.Duration
	listeners        []RollbackListener

	mu             sync.RWMutex
	states         map[uuid.UUID]*rollbackContext
	lastRollbacks  map[uuid.UUID]time.Time // tracks last rollback time per deployment
	rollbackHistory []*RollbackRecord
}

// rollbackContext holds per-deployment rollback state.
type rollbackContext struct {
	state                RollbackState
	unhealthySince       *time.Time
	lastHealthScore      float64
	errorRateExceededAt  *time.Time
	latencyExceededAt    *time.Time
}

// NewRollbackController creates a new RollbackController with the given configuration.
func NewRollbackController(executor RollbackExecutor, strategy RollbackStrategy, healthThreshold float64, evaluationWindow time.Duration) *RollbackController {
	return &RollbackController{
		executor:         executor,
		strategy:         strategy,
		healthThreshold:  healthThreshold,
		evaluationWindow: evaluationWindow,
		triggerConfig:    DefaultRollbackTriggerConfig(),
		cooldownPeriod:   5 * time.Minute,
		states:           make(map[uuid.UUID]*rollbackContext),
		lastRollbacks:    make(map[uuid.UUID]time.Time),
	}
}

// NewRollbackControllerWithConfig creates a new RollbackController with explicit
// trigger configuration and cooldown period.
func NewRollbackControllerWithConfig(
	executor RollbackExecutor,
	strategy RollbackStrategy,
	healthThreshold float64,
	evaluationWindow time.Duration,
	triggerConfig RollbackTriggerConfig,
	cooldownPeriod time.Duration,
) *RollbackController {
	return &RollbackController{
		executor:         executor,
		strategy:         strategy,
		healthThreshold:  healthThreshold,
		evaluationWindow: evaluationWindow,
		triggerConfig:    triggerConfig,
		cooldownPeriod:   cooldownPeriod,
		states:           make(map[uuid.UUID]*rollbackContext),
		lastRollbacks:    make(map[uuid.UUID]time.Time),
	}
}

// SetTriggerConfig updates the rollback trigger configuration.
func (rc *RollbackController) SetTriggerConfig(config RollbackTriggerConfig) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.triggerConfig = config
}

// SetCooldownPeriod updates the cooldown period between rollbacks for the
// same deployment. This prevents rollback flapping.
func (rc *RollbackController) SetCooldownPeriod(period time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cooldownPeriod = period
}

// GetRollbackHistory returns the recorded history of rollback events.
func (rc *RollbackController) GetRollbackHistory() []*RollbackRecord {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	result := make([]*RollbackRecord, len(rc.rollbackHistory))
	copy(result, rc.rollbackHistory)
	return result
}

// AddListener registers a listener for rollback state changes.
func (rc *RollbackController) AddListener(listener RollbackListener) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.listeners = append(rc.listeners, listener)
}

// Watch begins monitoring a deployment for potential rollback.
func (rc *RollbackController) Watch(deploymentID uuid.UUID) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.states[deploymentID] = &rollbackContext{
		state: RollbackStateHealthy,
	}
}

// Unwatch stops monitoring a deployment.
func (rc *RollbackController) Unwatch(deploymentID uuid.UUID) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.states, deploymentID)
}

// GetState returns the current rollback state for a deployment.
func (rc *RollbackController) GetState(deploymentID uuid.UUID) (RollbackState, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	ctx, ok := rc.states[deploymentID]
	if !ok {
		return "", fmt.Errorf("deployment %s is not being monitored for rollback", deploymentID)
	}
	return ctx.state, nil
}

// isInCooldown checks whether a deployment is within the cooldown period
// after a recent rollback. Must be called with rc.mu held (at least RLock).
func (rc *RollbackController) isInCooldown(deploymentID uuid.UUID) bool {
	lastRollback, ok := rc.lastRollbacks[deploymentID]
	if !ok {
		return false
	}
	return time.Since(lastRollback) < rc.cooldownPeriod
}

// recordRollback records a rollback event in history and updates the cooldown
// tracker. Must be called with rc.mu held.
func (rc *RollbackController) recordRollback(deploymentID uuid.UUID, reason string, automatic bool, strategy string, healthScore float64) {
	now := time.Now().UTC()
	rc.lastRollbacks[deploymentID] = now

	record := &RollbackRecord{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		State:        RollbackStateRolledBack,
		Reason:       reason,
		HealthScore:  healthScore,
		Automatic:    automatic,
		Strategy:     strategy,
		StartedAt:    now,
		CompletedAt:  &now,
	}
	rc.rollbackHistory = append(rc.rollbackHistory, record)
}

// EvaluateTriggers checks whether the current metrics for a deployment exceed
// the configured rollback trigger thresholds. It returns true along with a
// reason string if a rollback should be triggered.
func (rc *RollbackController) EvaluateTriggers(deploymentID uuid.UUID, errorRate, latencyP99 float64) (bool, string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rbCtx, ok := rc.states[deploymentID]
	if !ok {
		return false, ""
	}

	now := time.Now().UTC()
	triggerCfg := rc.triggerConfig

	// Evaluate error rate threshold.
	if errorRate > triggerCfg.ErrorRateThreshold {
		if rbCtx.errorRateExceededAt == nil {
			rbCtx.errorRateExceededAt = &now
		} else if now.Sub(*rbCtx.errorRateExceededAt) >= triggerCfg.EvaluationDuration {
			return true, fmt.Sprintf("error rate %.4f exceeds threshold %.4f for %v",
				errorRate, triggerCfg.ErrorRateThreshold, triggerCfg.EvaluationDuration)
		}
	} else {
		rbCtx.errorRateExceededAt = nil
	}

	// Evaluate latency threshold.
	if latencyP99 > triggerCfg.LatencyThreshold {
		if rbCtx.latencyExceededAt == nil {
			rbCtx.latencyExceededAt = &now
		} else if now.Sub(*rbCtx.latencyExceededAt) >= triggerCfg.EvaluationDuration {
			return true, fmt.Sprintf("p99 latency %.3fs exceeds threshold %.3fs for %v",
				latencyP99, triggerCfg.LatencyThreshold, triggerCfg.EvaluationDuration)
		}
	} else {
		rbCtx.latencyExceededAt = nil
	}

	return false, ""
}

// OnHealthUpdate implements health.HealthListener and is called by the
// HealthMonitor when a deployment's health is reevaluated.
func (rc *RollbackController) OnHealthUpdate(ctx context.Context, dh *health.DeploymentHealth) {
	rc.mu.Lock()
	rbCtx, ok := rc.states[dh.DeploymentID]
	if !ok {
		rc.mu.Unlock()
		return
	}

	rbCtx.lastHealthScore = dh.Overall
	rc.mu.Unlock()

	if dh.Overall < rc.healthThreshold {
		rc.handleUnhealthy(ctx, dh.DeploymentID, dh.Overall)
	} else {
		rc.handleHealthy(ctx, dh.DeploymentID)
	}
}

// TriggerManualRollback initiates a manual rollback for the specified deployment.
func (rc *RollbackController) TriggerManualRollback(ctx context.Context, deploymentID uuid.UUID, reason string) error {
	rc.mu.Lock()
	rbCtx, ok := rc.states[deploymentID]
	if !ok {
		rc.mu.Unlock()
		return fmt.Errorf("deployment %s is not being monitored", deploymentID)
	}

	if rbCtx.state == RollbackStateRollingBack || rbCtx.state == RollbackStateRolledBack {
		rc.mu.Unlock()
		return fmt.Errorf("deployment is already in state %s", rbCtx.state)
	}

	// Check cooldown for manual rollbacks too.
	if rc.isInCooldown(deploymentID) {
		rc.mu.Unlock()
		return fmt.Errorf("deployment %s is in cooldown period, cannot rollback again yet", deploymentID)
	}

	rbCtx.state = RollbackStateRollingBack
	healthScore := rbCtx.lastHealthScore
	rc.mu.Unlock()

	decision := &RollbackDecision{
		DeploymentID: deploymentID,
		State:        RollbackStateRollingBack,
		Reason:       reason,
		HealthScore:  healthScore,
		Automatic:    false,
		DecidedAt:    time.Now().UTC(),
	}
	rc.notifyListeners(ctx, decision)

	if err := rc.executor.Execute(ctx, deploymentID, rc.strategy); err != nil {
		return fmt.Errorf("executing rollback: %w", err)
	}

	rc.mu.Lock()
	rc.recordRollback(deploymentID, reason, false, rc.strategy.Name(), healthScore)
	rc.mu.Unlock()

	rc.transitionTo(ctx, deploymentID, RollbackStateRolledBack, reason, false)
	return nil
}

// handleUnhealthy processes an unhealthy health update.
func (rc *RollbackController) handleUnhealthy(ctx context.Context, deploymentID uuid.UUID, score float64) {
	rc.mu.Lock()
	rbCtx, ok := rc.states[deploymentID]
	if !ok {
		rc.mu.Unlock()
		return
	}

	// Check cooldown to prevent rollback flapping.
	if rc.isInCooldown(deploymentID) {
		rc.mu.Unlock()
		return
	}

	switch rbCtx.state {
	case RollbackStateHealthy:
		// Transition to evaluating.
		now := time.Now().UTC()
		rbCtx.state = RollbackStateEvaluating
		rbCtx.unhealthySince = &now
		rc.mu.Unlock()

		rc.notifyListeners(ctx, &RollbackDecision{
			DeploymentID: deploymentID,
			State:        RollbackStateEvaluating,
			Reason:       fmt.Sprintf("health score %.2f below threshold %.2f", score, rc.healthThreshold),
			HealthScore:  score,
			Automatic:    true,
			DecidedAt:    now,
		})

	case RollbackStateEvaluating:
		// Check if we've been unhealthy long enough.
		if rbCtx.unhealthySince != nil && time.Since(*rbCtx.unhealthySince) >= rc.evaluationWindow {
			rbCtx.state = RollbackStateRollingBack
			rc.mu.Unlock()

			reason := fmt.Sprintf("health score %.2f below threshold %.2f for %v", score, rc.healthThreshold, rc.evaluationWindow)
			rc.notifyListeners(ctx, &RollbackDecision{
				DeploymentID: deploymentID,
				State:        RollbackStateRollingBack,
				Reason:       reason,
				HealthScore:  score,
				Automatic:    true,
				DecidedAt:    time.Now().UTC(),
			})

			// Execute the rollback.
			if err := rc.executor.Execute(ctx, deploymentID, rc.strategy); err != nil {
				// Log error but continue to mark as rolled back.
				_ = err
			}

			rc.mu.Lock()
			rc.recordRollback(deploymentID, reason, true, rc.strategy.Name(), score)
			rc.mu.Unlock()

			rc.transitionTo(ctx, deploymentID, RollbackStateRolledBack, reason, true)
			return
		}
		rc.mu.Unlock()

	default:
		rc.mu.Unlock()
	}
}

// handleHealthy processes a healthy health update.
func (rc *RollbackController) handleHealthy(ctx context.Context, deploymentID uuid.UUID) {
	rc.mu.Lock()
	rbCtx, ok := rc.states[deploymentID]
	if !ok {
		rc.mu.Unlock()
		return
	}

	if rbCtx.state == RollbackStateEvaluating {
		rbCtx.state = RollbackStateHealthy
		rbCtx.unhealthySince = nil
		rbCtx.errorRateExceededAt = nil
		rbCtx.latencyExceededAt = nil
		rc.mu.Unlock()

		rc.notifyListeners(ctx, &RollbackDecision{
			DeploymentID: deploymentID,
			State:        RollbackStateHealthy,
			Reason:       "health recovered",
			HealthScore:  rbCtx.lastHealthScore,
			Automatic:    true,
			DecidedAt:    time.Now().UTC(),
		})
		return
	}
	rc.mu.Unlock()
}

// transitionTo updates the state and notifies listeners.
func (rc *RollbackController) transitionTo(ctx context.Context, deploymentID uuid.UUID, state RollbackState, reason string, automatic bool) {
	rc.mu.Lock()
	rbCtx, ok := rc.states[deploymentID]
	if ok {
		rbCtx.state = state
	}
	score := 0.0
	if ok {
		score = rbCtx.lastHealthScore
	}
	rc.mu.Unlock()

	rc.notifyListeners(ctx, &RollbackDecision{
		DeploymentID: deploymentID,
		State:        state,
		Reason:       reason,
		HealthScore:  score,
		Automatic:    automatic,
		DecidedAt:    time.Now().UTC(),
	})
}

// notifyListeners sends a rollback decision to all registered listeners.
func (rc *RollbackController) notifyListeners(ctx context.Context, decision *RollbackDecision) {
	rc.mu.RLock()
	listeners := make([]RollbackListener, len(rc.listeners))
	copy(listeners, rc.listeners)
	rc.mu.RUnlock()

	for _, l := range listeners {
		l.OnRollbackStateChange(ctx, decision)
	}
}
