// Package health implements deployment health monitoring, including periodic
// checks, health scoring, and integration with external observability platforms.
package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CheckResult holds the outcome of a single health check execution.
type CheckResult struct {
	Name      string    `json:"name"`
	Healthy   bool      `json:"healthy"`
	Score     float64   `json:"score"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	// Metrics holds raw metric values reported by the check (e.g., "error_rate": 0.02,
	// "latency_p99_ms": 120). Used by rollout abort conditions which operate on
	// raw metric thresholds, not the normalized Score.
	Metrics map[string]float64 `json:"metrics,omitempty"`
}

// HealthCheck defines the interface for a single health check probe.
type HealthCheck interface {
	// Name returns a human-readable identifier for this check.
	Name() string

	// Check executes the health check and returns the result.
	Check(ctx context.Context, deploymentID uuid.UUID) (*CheckResult, error)
}

// DeploymentHealth aggregates health information for a single deployment.
type DeploymentHealth struct {
	DeploymentID uuid.UUID          `json:"deployment_id"`
	Overall      float64            `json:"overall_score"`
	Healthy      bool               `json:"healthy"`
	Checks       []*CheckResult     `json:"checks"`
	Metrics      map[string]float64 `json:"metrics,omitempty"` // aggregated from Checks[].Metrics
	EvaluatedAt  time.Time          `json:"evaluated_at"`
}

// HealthListener receives health evaluation results.
type HealthListener interface {
	// OnHealthUpdate is called when a deployment's health status changes.
	OnHealthUpdate(ctx context.Context, health *DeploymentHealth)
}

// HealthMonitor runs periodic health checks against active deployments
// and publishes aggregated health scores.
type HealthMonitor struct {
	checks    []HealthCheck
	scorer    *HealthScorer
	listeners []HealthListener
	interval  time.Duration
	threshold float64

	mu          sync.RWMutex
	deployments map[uuid.UUID]*DeploymentHealth
	stopCh      chan struct{}
}

// NewHealthMonitor creates a new HealthMonitor with the given checks and interval.
func NewHealthMonitor(scorer *HealthScorer, interval time.Duration, threshold float64) *HealthMonitor {
	return &HealthMonitor{
		scorer:      scorer,
		interval:    interval,
		threshold:   threshold,
		deployments: make(map[uuid.UUID]*DeploymentHealth),
		stopCh:      make(chan struct{}),
	}
}

// AddCheck registers a health check with the monitor.
func (m *HealthMonitor) AddCheck(check HealthCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks = append(m.checks, check)
}

// AddListener registers a listener that will be notified of health updates.
func (m *HealthMonitor) AddListener(listener HealthListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// Watch begins monitoring a deployment. The monitor will periodically execute
// all registered health checks against this deployment.
func (m *HealthMonitor) Watch(deploymentID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deployments[deploymentID] = &DeploymentHealth{
		DeploymentID: deploymentID,
		Healthy:      true,
		Overall:      1.0,
	}
}

// Unwatch stops monitoring a deployment.
func (m *HealthMonitor) Unwatch(deploymentID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.deployments, deploymentID)
}

// GetHealth returns the latest aggregated health for a deployment.
func (m *HealthMonitor) GetHealth(deploymentID uuid.UUID) (*DeploymentHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	health, ok := m.deployments[deploymentID]
	if !ok {
		return nil, fmt.Errorf("deployment %s is not being monitored", deploymentID)
	}
	return health, nil
}

// Start begins the periodic health check loop. It runs until Stop is called
// or the context is cancelled.
func (m *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluate(ctx)
		}
	}
}

// Stop signals the monitor to cease periodic health checks.
func (m *HealthMonitor) Stop() {
	close(m.stopCh)
}

// evaluate runs all health checks against all watched deployments.
func (m *HealthMonitor) evaluate(ctx context.Context) {
	m.mu.RLock()
	deploymentIDs := make([]uuid.UUID, 0, len(m.deployments))
	for id := range m.deployments {
		deploymentIDs = append(deploymentIDs, id)
	}
	checks := make([]HealthCheck, len(m.checks))
	copy(checks, m.checks)
	m.mu.RUnlock()

	for _, deploymentID := range deploymentIDs {
		results := make([]*CheckResult, 0, len(checks))
		for _, check := range checks {
			result, err := check.Check(ctx, deploymentID)
			if err != nil {
				results = append(results, &CheckResult{
					Name:      check.Name(),
					Healthy:   false,
					Score:     0,
					Message:   err.Error(),
					CheckedAt: time.Now().UTC(),
				})
				continue
			}
			results = append(results, result)
		}

		overall := m.scorer.ComputeScore(results)
		metrics := map[string]float64{}
		for _, result := range results {
			for k, v := range result.Metrics {
				metrics[k] = v
			}
		}
		health := &DeploymentHealth{
			DeploymentID: deploymentID,
			Overall:      overall,
			Healthy:      overall >= m.threshold,
			Checks:       results,
			Metrics:      metrics,
			EvaluatedAt:  time.Now().UTC(),
		}

		m.mu.Lock()
		m.deployments[deploymentID] = health
		m.mu.Unlock()

		// Notify listeners.
		m.mu.RLock()
		listeners := make([]HealthListener, len(m.listeners))
		copy(listeners, m.listeners)
		m.mu.RUnlock()

		for _, listener := range listeners {
			listener.OnHealthUpdate(ctx, health)
		}
	}
}
