package rollout

import (
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
)

func validStrategy() *models.Strategy {
	return &models.Strategy{
		Name:       "ok",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: time.Minute, MaxDuration: 10 * time.Minute, BakeTimeHealthy: time.Minute},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
}

func TestValidateStrategy_OK(t *testing.T) {
	if err := ValidateStrategy(validStrategy()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStrategy_RequiresName(t *testing.T) {
	s := validStrategy()
	s.Name = ""
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStrategy_RequiresSteps(t *testing.T) {
	s := validStrategy()
	s.Steps = nil
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStrategy_PercentRange(t *testing.T) {
	s := validStrategy()
	s.Steps[0].Percent = 150
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for percent > 100")
	}
	s.Steps[0].Percent = -1
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for percent < 0")
	}
}

func TestValidateStrategy_StepsMonotonic(t *testing.T) {
	s := validStrategy()
	s.Steps = []models.Step{
		{Percent: 50, MinDuration: time.Minute, MaxDuration: time.Minute, BakeTimeHealthy: 0},
		{Percent: 25, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
	}
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error: percent not monotonic")
	}
}

func TestValidateStrategy_MinLessThanMax(t *testing.T) {
	s := validStrategy()
	s.Steps[0].MinDuration = 20 * time.Minute
	s.Steps[0].MaxDuration = 10 * time.Minute
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error: min > max")
	}
}

func TestValidateStrategy_BakeWithinBounds(t *testing.T) {
	s := validStrategy()
	s.Steps[0].BakeTimeHealthy = 30 * time.Minute
	s.Steps[0].MaxDuration = 10 * time.Minute
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error: bake > max")
	}
}

func TestValidateStrategy_HealthThresholdRange(t *testing.T) {
	s := validStrategy()
	th := 1.5
	s.Steps[0].HealthThreshold = &th
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for health threshold > 1")
	}
}

func TestValidateStrategy_AbortConditionOperators(t *testing.T) {
	s := validStrategy()
	s.Steps[0].AbortConditions = []models.StepAbortCondition{
		{Metric: "error_rate", Operator: "?", Threshold: 0.01, Window: time.Minute},
	}
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected error for bogus operator")
	}
}

func TestValidateStrategy_RejectsUnknownAbortMetric(t *testing.T) {
	s := validStrategy()
	s.Steps[0].AbortConditions = []models.StepAbortCondition{
		{Metric: "cpu_usage", Operator: ">", Threshold: 0.8, Window: time.Minute},
	}
	if err := ValidateStrategy(s); err == nil {
		t.Fatalf("expected unknown metric error")
	}
}

func TestValidateStrategy_AcceptsCustomMetric(t *testing.T) {
	s := validStrategy()
	s.Steps[0].AbortConditions = []models.StepAbortCondition{
		{Metric: "custom:queue_depth", Operator: ">", Threshold: 100, Window: time.Minute},
	}
	if err := ValidateStrategy(s); err != nil {
		t.Fatalf("custom:* metric should be accepted, got %v", err)
	}
}
