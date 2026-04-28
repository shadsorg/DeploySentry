// Package engine drives rollouts through their strategy phases. This file
// implements the gate logic (abort conditions, approval evaluation) used by
// the main phase loop.
package engine

import (
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/rollout/applicator"
)

// EvaluateAbortConditions returns the first abort condition whose threshold
// is tripped by the current HealthScore, or nil if no condition trips.
// Unknown metric names are silently ignored (no trip).
func EvaluateAbortConditions(step models.Step, score applicator.HealthScore) *models.StepAbortCondition {
	for _, ac := range step.AbortConditions {
		val, ok := metricValue(ac.Metric, score)
		if !ok {
			continue
		}
		if compare(val, ac.Operator, ac.Threshold) {
			ac := ac
			return &ac
		}
	}
	return nil
}

func metricValue(name string, s applicator.HealthScore) (float64, bool) {
	switch name {
	case "score":
		return s.Score, true
	case "error_rate":
		return s.ErrorRate, true
	case "latency_p99_ms":
		return s.LatencyP99Ms, true
	case "latency_p50_ms":
		return s.LatencyP50Ms, true
	case "request_rate":
		return s.RequestRate, true
	}
	return 0, false
}

func compare(val float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return val > threshold
	case ">=":
		return val >= threshold
	case "<":
		return val < threshold
	case "<=":
		return val <= threshold
	case "==":
		return val == threshold
	case "!=":
		return val != threshold
	}
	return false
}

// EffectiveHealthThreshold returns the step's health threshold, defaulting to
// the strategy's default_health_threshold when the step has no override.
func EffectiveHealthThreshold(strat *models.Strategy, step models.Step) float64 {
	if step.HealthThreshold != nil {
		return *step.HealthThreshold
	}
	return strat.DefaultHealthThreshold
}
