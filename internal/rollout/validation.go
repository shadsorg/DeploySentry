// Package rollout provides management for rollout strategy templates,
// scope defaults, and onboarding policies. The rollout execution engine
// lives in sub-packages added in a later phase.
package rollout

import (
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
)

var validAbortOperators = map[string]bool{
	">":  true,
	">=": true,
	"<":  true,
	"<=": true,
	"==": true,
	"!=": true,
}

// ValidateStrategy returns a non-nil error when the Strategy is not
// acceptable for persistence. It checks required fields, value ranges,
// step monotonicity (percent increases across steps), and per-step
// duration/threshold constraints.
func ValidateStrategy(s *models.Strategy) error {
	if s == nil {
		return fmt.Errorf("strategy is nil")
	}
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch s.TargetType {
	case models.TargetTypeDeploy, models.TargetTypeConfig, models.TargetTypeAny:
	default:
		return fmt.Errorf("invalid target_type: %q", s.TargetType)
	}
	if s.DefaultHealthThreshold < 0 || s.DefaultHealthThreshold > 1 {
		return fmt.Errorf("default_health_threshold must be in [0,1], got %g", s.DefaultHealthThreshold)
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	var prevPercent float64 = -1
	for i, st := range s.Steps {
		if st.Percent < 0 || st.Percent > 100 {
			return fmt.Errorf("step %d: percent must be in [0,100], got %g", i, st.Percent)
		}
		if st.Percent < prevPercent {
			return fmt.Errorf("step %d: percent %g < previous %g (steps must be non-decreasing)", i, st.Percent, prevPercent)
		}
		prevPercent = st.Percent
		if st.MinDuration < 0 || st.MaxDuration < 0 || st.BakeTimeHealthy < 0 {
			return fmt.Errorf("step %d: negative durations are not allowed", i)
		}
		if st.MinDuration > st.MaxDuration {
			return fmt.Errorf("step %d: min_duration %s > max_duration %s", i, st.MinDuration, st.MaxDuration)
		}
		if st.BakeTimeHealthy > st.MaxDuration && st.MaxDuration > 0 {
			return fmt.Errorf("step %d: bake_time_healthy %s > max_duration %s", i, st.BakeTimeHealthy, st.MaxDuration)
		}
		if st.HealthThreshold != nil && (*st.HealthThreshold < 0 || *st.HealthThreshold > 1) {
			return fmt.Errorf("step %d: health_threshold must be in [0,1]", i)
		}
		if st.Approval != nil && st.Approval.Timeout < 0 {
			return fmt.Errorf("step %d: approval.timeout must be non-negative", i)
		}
		for j, ac := range st.AbortConditions {
			if ac.Metric == "" {
				return fmt.Errorf("step %d abort[%d]: metric is required", i, j)
			}
			if !validAbortOperators[ac.Operator] {
				return fmt.Errorf("step %d abort[%d]: invalid operator %q", i, j, ac.Operator)
			}
			if ac.Window <= 0 {
				return fmt.Errorf("step %d abort[%d]: window must be positive", i, j)
			}
		}
	}
	if s.Steps[len(s.Steps)-1].Percent != 100 {
		return fmt.Errorf("final step must reach percent=100")
	}
	return nil
}
