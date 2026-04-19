package rollout

import (
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

// StrategyOverrides captures per-rollout tweaks to a template at attach time.
// All pointer fields are optional; nil means "keep template value".
type StrategyOverrides struct {
	DefaultHealthThreshold   *float64             `json:"default_health_threshold,omitempty"`
	DefaultRollbackOnFailure *bool                `json:"default_rollback_on_failure,omitempty"`
	Steps                    map[int]StepOverride `json:"steps,omitempty"`
}

// StepOverride is an optional tweak to a single step. Field names mirror the
// Step struct; unset fields leave the template's step field unchanged.
type StepOverride struct {
	Percent          *float64 `json:"percent,omitempty"`
	MinDurationMs    *int64   `json:"min_duration_ms,omitempty"`
	MaxDurationMs    *int64   `json:"max_duration_ms,omitempty"`
	BakeTimeMs       *int64   `json:"bake_time_healthy_ms,omitempty"`
	HealthThreshold  *float64 `json:"health_threshold,omitempty"`
}

// BuildSnapshot returns a deep copy of the template with overrides applied.
func BuildSnapshot(tmpl *models.Strategy, overrides *StrategyOverrides) *models.Strategy {
	snap := &models.Strategy{
		ID:                       tmpl.ID,
		ScopeType:                tmpl.ScopeType,
		ScopeID:                  tmpl.ScopeID,
		Name:                     tmpl.Name,
		Description:              tmpl.Description,
		TargetType:               tmpl.TargetType,
		DefaultHealthThreshold:   tmpl.DefaultHealthThreshold,
		DefaultRollbackOnFailure: tmpl.DefaultRollbackOnFailure,
		Version:                  tmpl.Version,
		IsSystem:                 tmpl.IsSystem,
	}
	snap.Steps = make([]models.Step, len(tmpl.Steps))
	for i, st := range tmpl.Steps {
		snap.Steps[i] = st
		if st.AbortConditions != nil {
			cp := make([]models.StepAbortCondition, len(st.AbortConditions))
			copy(cp, st.AbortConditions)
			snap.Steps[i].AbortConditions = cp
		}
		if st.Approval != nil {
			a := *st.Approval
			snap.Steps[i].Approval = &a
		}
		if st.Notify != nil {
			n := *st.Notify
			snap.Steps[i].Notify = &n
		}
	}
	if overrides == nil {
		return snap
	}
	if overrides.DefaultHealthThreshold != nil {
		snap.DefaultHealthThreshold = *overrides.DefaultHealthThreshold
	}
	if overrides.DefaultRollbackOnFailure != nil {
		snap.DefaultRollbackOnFailure = *overrides.DefaultRollbackOnFailure
	}
	for idx, so := range overrides.Steps {
		if idx < 0 || idx >= len(snap.Steps) {
			continue
		}
		st := &snap.Steps[idx]
		if so.Percent != nil {
			st.Percent = *so.Percent
		}
		if so.MinDurationMs != nil {
			st.MinDuration = time.Duration(*so.MinDurationMs) * time.Millisecond
		}
		if so.MaxDurationMs != nil {
			st.MaxDuration = time.Duration(*so.MaxDurationMs) * time.Millisecond
		}
		if so.BakeTimeMs != nil {
			st.BakeTimeHealthy = time.Duration(*so.BakeTimeMs) * time.Millisecond
		}
		if so.HealthThreshold != nil {
			th := *so.HealthThreshold
			st.HealthThreshold = &th
		}
	}
	return snap
}
