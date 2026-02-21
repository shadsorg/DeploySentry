package health

import (
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Signal names used in the default health scoring configuration.
const (
	SignalErrorRate5xx    = "error_rate_5xx"
	SignalLatencyP99     = "latency_p99"
	SignalErrorTracking  = "error_tracking"
	SignalCustomMetrics  = "custom_metrics"
	SignalSyntheticChecks = "synthetic_checks"
)

// Weight defines a named weight for a health check in the scoring formula.
type Weight struct {
	CheckName string  `json:"check_name"`
	Weight    float64 `json:"weight"`
}

// DefaultWeights returns the checklist-specified default weights for health
// scoring signals:
//   - Error rate (5xx): 30%
//   - Latency p99: 20%
//   - Error tracking (new errors): 20%
//   - Custom metrics: 15%
//   - Synthetic checks: 15%
func DefaultWeights() []Weight {
	return []Weight{
		{CheckName: SignalErrorRate5xx, Weight: 0.30},
		{CheckName: SignalLatencyP99, Weight: 0.20},
		{CheckName: SignalErrorTracking, Weight: 0.20},
		{CheckName: SignalCustomMetrics, Weight: 0.15},
		{CheckName: SignalSyntheticChecks, Weight: 0.15},
	}
}

// HealthScorer computes weighted health scores from a set of check results.
// Each check can have a configurable weight, and the final score is the
// weighted average of all check scores normalized to [0.0, 1.0].
type HealthScorer struct {
	weights       map[string]float64
	defaultWeight float64
}

// NewHealthScorer creates a new HealthScorer with the given weight configuration.
// Checks not present in the weights map receive the default weight.
func NewHealthScorer(weights []Weight, defaultWeight float64) *HealthScorer {
	wm := make(map[string]float64, len(weights))
	for _, w := range weights {
		wm[w.CheckName] = w.Weight
	}
	if defaultWeight <= 0 {
		defaultWeight = 1.0
	}
	return &HealthScorer{
		weights:       wm,
		defaultWeight: defaultWeight,
	}
}

// NewDefaultHealthScorer creates a new HealthScorer with the default weights.
func NewDefaultHealthScorer() *HealthScorer {
	return NewHealthScorer(DefaultWeights(), 0.10)
}

// ComputeScore calculates the weighted average health score from the given
// check results. Returns a value in [0.0, 1.0] where 1.0 is fully healthy.
// If no results are provided, returns 1.0 (healthy by default).
func (s *HealthScorer) ComputeScore(results []*CheckResult) float64 {
	if len(results) == 0 {
		return 1.0
	}

	var totalWeight float64
	var weightedSum float64

	for _, r := range results {
		w := s.weightFor(r.Name)
		totalWeight += w
		weightedSum += r.Score * w
	}

	if totalWeight == 0 {
		return 1.0
	}

	score := weightedSum / totalWeight
	return clamp(score, 0.0, 1.0)
}

// weightFor returns the configured weight for the named check,
// falling back to the default weight.
func (s *HealthScorer) weightFor(name string) float64 {
	if w, ok := s.weights[name]; ok {
		return w
	}
	return s.defaultWeight
}

// ComputeSignalScore computes a health signal score using the formula:
//
//	100 * max(0, 1 - (current_value / threshold_value))
//
// This yields a score in [0, 100] where 100 means the signal is perfectly
// healthy (current_value is 0) and 0 means the signal has met or exceeded
// the threshold. If the threshold is zero or negative, returns 100.0.
func ComputeSignalScore(currentValue, thresholdValue float64) float64 {
	if thresholdValue <= 0 {
		return 100.0
	}
	return 100.0 * math.Max(0, 1-(currentValue/thresholdValue))
}

// HealthHistoryEntry records a single health score observation at a point in time.
type HealthHistoryEntry struct {
	DeploymentID uuid.UUID `json:"deployment_id"`
	Score        float64   `json:"score"`
	Healthy      bool      `json:"healthy"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// HealthHistory stores time-series health scores for deployments. It provides
// thread-safe recording and retrieval of health score observations, suitable
// for trend analysis and dashboard display.
type HealthHistory struct {
	mu      sync.RWMutex
	entries map[uuid.UUID][]HealthHistoryEntry
	maxSize int
}

// NewHealthHistory creates a new HealthHistory with the given maximum entries
// per deployment. If maxSize is zero or negative, it defaults to 1000.
func NewHealthHistory(maxSize int) *HealthHistory {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &HealthHistory{
		entries: make(map[uuid.UUID][]HealthHistoryEntry),
		maxSize: maxSize,
	}
}

// RecordScore records a health score observation for the given deployment.
// If the history for a deployment exceeds the max size, the oldest entries
// are trimmed.
func (h *HealthHistory) RecordScore(deploymentID uuid.UUID, score float64, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := HealthHistoryEntry{
		DeploymentID: deploymentID,
		Score:        score,
		Healthy:      healthy,
		RecordedAt:   time.Now().UTC(),
	}

	entries := h.entries[deploymentID]
	entries = append(entries, entry)

	// Trim oldest entries if we exceed max size.
	if len(entries) > h.maxSize {
		entries = entries[len(entries)-h.maxSize:]
	}

	h.entries[deploymentID] = entries
}

// GetHistory returns all recorded health score entries for a deployment,
// ordered chronologically (oldest first).
func (h *HealthHistory) GetHistory(deploymentID uuid.UUID) []HealthHistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := h.entries[deploymentID]
	if entries == nil {
		return nil
	}

	result := make([]HealthHistoryEntry, len(entries))
	copy(result, entries)
	return result
}

// GetHistorySince returns health score entries for a deployment that were
// recorded at or after the given time.
func (h *HealthHistory) GetHistorySince(deploymentID uuid.UUID, since time.Time) []HealthHistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := h.entries[deploymentID]
	var result []HealthHistoryEntry
	for _, e := range entries {
		if !e.RecordedAt.Before(since) {
			result = append(result, e)
		}
	}
	return result
}

// clamp restricts a value to the range [min, max].
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
