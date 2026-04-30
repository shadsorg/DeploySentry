package engine

import (
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/rollout/applicator"
)

func TestEvaluateAbortConditions_Trips(t *testing.T) {
	step := models.Step{AbortConditions: []models.StepAbortCondition{
		{Metric: "error_rate", Operator: ">", Threshold: 0.02, Window: time.Minute},
	}}
	score := applicator.HealthScore{ErrorRate: 0.05}
	got := EvaluateAbortConditions(step, score)
	if got == nil {
		t.Fatalf("expected trip, got nil")
	} else if got.Metric != "error_rate" {
		t.Fatalf("wrong metric: %+v", got)
	}
}

func TestEvaluateAbortConditions_DoesNotTrip(t *testing.T) {
	step := models.Step{AbortConditions: []models.StepAbortCondition{
		{Metric: "error_rate", Operator: ">", Threshold: 0.10, Window: time.Minute},
	}}
	score := applicator.HealthScore{ErrorRate: 0.05}
	if got := EvaluateAbortConditions(step, score); got != nil {
		t.Fatalf("unexpected trip: %+v", got)
	}
}

func TestEvaluateAbortConditions_MultipleOperators(t *testing.T) {
	cases := []struct {
		op string
		thr, val float64
		wantTrip bool
	}{
		{">", 1, 2, true}, {">", 1, 1, false},
		{">=", 1, 1, true},
		{"<", 2, 1, true}, {"<", 1, 1, false},
		{"<=", 1, 1, true},
		{"==", 1, 1, true}, {"==", 1, 1.0001, false},
		{"!=", 1, 2, true}, {"!=", 1, 1, false},
	}
	for _, c := range cases {
		step := models.Step{AbortConditions: []models.StepAbortCondition{
			{Metric: "latency_p99_ms", Operator: c.op, Threshold: c.thr, Window: time.Second},
		}}
		score := applicator.HealthScore{LatencyP99Ms: c.val}
		got := EvaluateAbortConditions(step, score)
		if (got != nil) != c.wantTrip {
			t.Errorf("op %s thr %g val %g: got trip=%v want %v", c.op, c.thr, c.val, got != nil, c.wantTrip)
		}
	}
}

func TestEvaluateAbortConditions_UnknownMetric(t *testing.T) {
	step := models.Step{AbortConditions: []models.StepAbortCondition{
		{Metric: "unknown_metric", Operator: ">", Threshold: 0, Window: time.Second},
	}}
	if got := EvaluateAbortConditions(step, applicator.HealthScore{}); got != nil {
		t.Fatalf("unknown metric should not trip; got %+v", got)
	}
}
