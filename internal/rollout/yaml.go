package rollout

import (
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"gopkg.in/yaml.v3"
)

// yamlStrategy is the on-disk shape; durations are strings.
type yamlStrategy struct {
	Name                     string     `yaml:"name"`
	Description              string     `yaml:"description,omitempty"`
	TargetType               string     `yaml:"target_type"`
	DefaultHealthThreshold   float64    `yaml:"default_health_threshold"`
	DefaultRollbackOnFailure bool       `yaml:"default_rollback_on_failure"`
	Steps                    []yamlStep `yaml:"steps"`
}

type yamlStep struct {
	Percent         float64              `yaml:"percent"`
	MinDuration     string               `yaml:"min_duration"`
	MaxDuration     string               `yaml:"max_duration"`
	BakeTimeHealthy string               `yaml:"bake_time_healthy"`
	HealthThreshold *float64             `yaml:"health_threshold,omitempty"`
	Approval        *yamlApproval        `yaml:"approval,omitempty"`
	Notify          *yamlNotify          `yaml:"notify,omitempty"`
	AbortConditions []yamlAbortCondition `yaml:"abort_conditions,omitempty"`
	SignalOverride  *yamlSignalSource    `yaml:"signal_override,omitempty"`
}

type yamlApproval struct {
	RequiredRole string `yaml:"required_role"`
	Timeout      string `yaml:"timeout"`
}

type yamlNotify struct {
	OnEntry []string `yaml:"on_entry,omitempty"`
	OnExit  []string `yaml:"on_exit,omitempty"`
}

type yamlAbortCondition struct {
	Metric    string  `yaml:"metric"`
	Operator  string  `yaml:"operator"`
	Threshold float64 `yaml:"threshold"`
	Window    string  `yaml:"window"`
}

type yamlSignalSource struct {
	Kind string `yaml:"kind"`
}

// MarshalStrategyYAML converts a strategy to YAML bytes.
func MarshalStrategyYAML(s *models.Strategy) ([]byte, error) {
	y := yamlStrategy{
		Name: s.Name, Description: s.Description,
		TargetType:               string(s.TargetType),
		DefaultHealthThreshold:   s.DefaultHealthThreshold,
		DefaultRollbackOnFailure: s.DefaultRollbackOnFailure,
	}
	for _, st := range s.Steps {
		ys := yamlStep{
			Percent:         st.Percent,
			MinDuration:     st.MinDuration.String(),
			MaxDuration:     st.MaxDuration.String(),
			BakeTimeHealthy: st.BakeTimeHealthy.String(),
			HealthThreshold: st.HealthThreshold,
		}
		if st.Approval != nil {
			ys.Approval = &yamlApproval{RequiredRole: st.Approval.RequiredRole, Timeout: st.Approval.Timeout.String()}
		}
		if st.Notify != nil {
			ys.Notify = &yamlNotify{OnEntry: st.Notify.OnEntry, OnExit: st.Notify.OnExit}
		}
		for _, ac := range st.AbortConditions {
			ys.AbortConditions = append(ys.AbortConditions, yamlAbortCondition{
				Metric: ac.Metric, Operator: ac.Operator, Threshold: ac.Threshold, Window: ac.Window.String(),
			})
		}
		if st.SignalOverride != nil {
			ys.SignalOverride = &yamlSignalSource{Kind: st.SignalOverride.Kind}
		}
		y.Steps = append(y.Steps, ys)
	}
	return yaml.Marshal(y)
}

// UnmarshalStrategyYAML parses YAML bytes into a strategy (scope/ID left empty).
func UnmarshalStrategyYAML(data []byte) (*models.Strategy, error) {
	var y yamlStrategy
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	s := &models.Strategy{
		Name: y.Name, Description: y.Description,
		TargetType:               models.TargetType(y.TargetType),
		DefaultHealthThreshold:   y.DefaultHealthThreshold,
		DefaultRollbackOnFailure: y.DefaultRollbackOnFailure,
	}
	for i, ys := range y.Steps {
		st, err := yamlStepToStep(ys, i)
		if err != nil {
			return nil, err
		}
		s.Steps = append(s.Steps, st)
	}
	return s, nil
}

func yamlStepToStep(ys yamlStep, idx int) (models.Step, error) {
	minD, err := parseDur(ys.MinDuration, "min_duration", idx)
	if err != nil {
		return models.Step{}, err
	}
	maxD, err := parseDur(ys.MaxDuration, "max_duration", idx)
	if err != nil {
		return models.Step{}, err
	}
	bake, err := parseDur(ys.BakeTimeHealthy, "bake_time_healthy", idx)
	if err != nil {
		return models.Step{}, err
	}
	st := models.Step{
		Percent: ys.Percent, MinDuration: minD, MaxDuration: maxD, BakeTimeHealthy: bake,
		HealthThreshold: ys.HealthThreshold,
	}
	if ys.Approval != nil {
		d, err := parseDur(ys.Approval.Timeout, "approval.timeout", idx)
		if err != nil {
			return models.Step{}, err
		}
		st.Approval = &models.StepApproval{RequiredRole: ys.Approval.RequiredRole, Timeout: d}
	}
	if ys.Notify != nil {
		st.Notify = &models.StepNotify{OnEntry: ys.Notify.OnEntry, OnExit: ys.Notify.OnExit}
	}
	for j, ac := range ys.AbortConditions {
		w, err := parseDur(ac.Window, fmt.Sprintf("abort_conditions[%d].window", j), idx)
		if err != nil {
			return models.Step{}, err
		}
		st.AbortConditions = append(st.AbortConditions, models.StepAbortCondition{
			Metric: ac.Metric, Operator: ac.Operator, Threshold: ac.Threshold, Window: w,
		})
	}
	if ys.SignalOverride != nil {
		st.SignalOverride = &models.SignalSource{Kind: ys.SignalOverride.Kind}
	}
	return st, nil
}

func parseDur(s, field string, idx int) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("step %d: %s: %w", idx, field, err)
	}
	return d, nil
}
