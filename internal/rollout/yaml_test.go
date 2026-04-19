package rollout

import (
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
)

func TestYAML_RoundTrip(t *testing.T) {
	s := &models.Strategy{
		Name: "prod-canary-standard", Description: "d",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: 5 * time.Minute, MaxDuration: 30 * time.Minute, BakeTimeHealthy: 5 * time.Minute,
				AbortConditions: []models.StepAbortCondition{{Metric: "error_rate", Operator: ">", Threshold: 0.02, Window: time.Minute}}},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	}
	b, err := MarshalStrategyYAML(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalStrategyYAML(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != s.Name || len(got.Steps) != 2 || got.Steps[0].MinDuration != 5*time.Minute ||
		got.Steps[0].AbortConditions[0].Metric != "error_rate" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestYAML_DurationStrings(t *testing.T) {
	y := []byte(`
name: s
target_type: deploy
default_health_threshold: 0.95
default_rollback_on_failure: true
steps:
  - percent: 1
    min_duration: 5m
    max_duration: 1h
    bake_time_healthy: 10s
  - percent: 100
    min_duration: 0s
    max_duration: 0s
    bake_time_healthy: 0s
`)
	got, err := UnmarshalStrategyYAML(y)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Steps[0].MinDuration != 5*time.Minute || got.Steps[0].MaxDuration != time.Hour {
		t.Fatalf("duration parse: %+v", got.Steps[0])
	}
}
