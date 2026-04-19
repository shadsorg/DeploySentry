package models

import (
	"encoding/json"
	"testing"
)

func TestRolloutStatusStrings(t *testing.T) {
	for s, want := range map[RolloutStatus]string{
		RolloutPending:         "pending",
		RolloutActive:          "active",
		RolloutPaused:          "paused",
		RolloutAwaitingApproval: "awaiting_approval",
		RolloutSucceeded:       "succeeded",
		RolloutRolledBack:      "rolled_back",
		RolloutAborted:         "aborted",
		RolloutSuperseded:      "superseded",
	} {
		if string(s) != want {
			t.Errorf("%v: got %q want %q", s, string(s), want)
		}
	}
}

func TestRolloutPhaseStatusStrings(t *testing.T) {
	for s, want := range map[PhaseStatus]string{
		PhasePending:          "pending",
		PhaseActive:           "active",
		PhaseAwaitingApproval: "awaiting_approval",
		PhasePassed:           "passed",
		PhaseFailed:           "failed",
		PhaseRolledBack:       "rolled_back",
	} {
		if string(s) != want {
			t.Errorf("%v: got %q want %q", s, string(s), want)
		}
	}
}

func TestRolloutTargetRefJSON(t *testing.T) {
	r := Rollout{TargetType: TargetTypeDeploy, TargetRef: RolloutTargetRef{DeploymentID: ptrString("abc")}}
	b, err := json.Marshal(r.TargetRef)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"deployment_id":"abc"}` {
		t.Fatalf("unexpected json: %s", string(b))
	}
}

func TestRolloutTargetRef_RuleJSON(t *testing.T) {
	pp := 10
	r := Rollout{TargetType: TargetTypeConfig, TargetRef: RolloutTargetRef{
		RuleID:             ptrString("11111111-1111-1111-1111-111111111111"),
		PreviousPercentage: &pp,
	}}
	b, err := json.Marshal(r.TargetRef)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"rule_id":"11111111-1111-1111-1111-111111111111","previous_percentage":10}` {
		t.Fatalf("unexpected json: %s", string(b))
	}
}

func ptrString(s string) *string { return &s }
