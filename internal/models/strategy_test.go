package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStrategyJSONRoundTrip(t *testing.T) {
	s := &Strategy{
		Name:        "prod-canary-standard",
		Description: "Standard prod canary",
		TargetType:  TargetTypeDeploy,
		Steps: []Step{
			{Percent: 1, MinDuration: 5 * time.Minute, MaxDuration: 30 * time.Minute, BakeTimeHealthy: 5 * time.Minute},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Strategy
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Steps[0].Percent != 1 || got.Steps[0].MinDuration != 5*time.Minute {
		t.Fatalf("roundtrip mismatch: %+v", got.Steps[0])
	}
}

func TestStepOptionalFieldsOmitEmpty(t *testing.T) {
	s := Step{Percent: 5, MinDuration: time.Minute, MaxDuration: time.Hour, BakeTimeHealthy: time.Minute}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, field := range []string{"approval", "notify", "abort_conditions", "health_threshold", "signal_override"} {
		if containsField(js, field) {
			t.Errorf("expected %q to be omitted when empty: %s", field, js)
		}
	}
}

func containsField(js, name string) bool {
	return indexOf(js, `"`+name+`":`) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
