package config

import (
	"context"
	"errors"
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

type stubRuleUpdater struct {
	calls []int // percentages applied, in order
	err   error
}

func (s *stubRuleUpdater) UpdateRulePercentage(_ context.Context, _ uuid.UUID, pct int) error {
	s.calls = append(s.calls, pct)
	return s.err
}

func makeRolloutWithRule(ruleID string, prev int) *models.Rollout {
	p := prev
	return &models.Rollout{
		TargetType: models.TargetTypeConfig,
		TargetRef: models.RolloutTargetRef{
			RuleID:             &ruleID,
			PreviousPercentage: &p,
		},
	}
}

func TestConfigApplicator_Apply_SetsPercentage(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u)
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 25}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(u.calls) != 1 || u.calls[0] != 25 {
		t.Fatalf("expected [25], got %v", u.calls)
	}
}

func TestConfigApplicator_Apply_MissingRuleID(t *testing.T) {
	app := NewApplicator(&stubRuleUpdater{})
	ro := &models.Rollout{TargetType: models.TargetTypeConfig}
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 10}); err == nil {
		t.Fatalf("expected error on missing rule_id")
	}
}

func TestConfigApplicator_Revert_RestoresPrevious(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u)
	ro := makeRolloutWithRule(uuid.NewString(), 7)
	if err := app.Revert(context.Background(), ro); err != nil {
		t.Fatal(err)
	}
	if len(u.calls) != 1 || u.calls[0] != 7 {
		t.Fatalf("expected revert to 7, got %v", u.calls)
	}
}

func TestConfigApplicator_Revert_DefaultsToZero(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u)
	// No PreviousPercentage set → Revert should set to 0.
	ro := &models.Rollout{TargetType: models.TargetTypeConfig, TargetRef: models.RolloutTargetRef{RuleID: func() *string { s := uuid.NewString(); return &s }()}}
	if err := app.Revert(context.Background(), ro); err != nil {
		t.Fatal(err)
	}
	if len(u.calls) != 1 || u.calls[0] != 0 {
		t.Fatalf("expected revert to 0, got %v", u.calls)
	}
}

func TestConfigApplicator_CurrentSignal_AlwaysHealthy(t *testing.T) {
	app := NewApplicator(&stubRuleUpdater{})
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 1.0 {
		t.Fatalf("expected healthy noop signal (1.0), got %g", s.Score)
	}
}

func TestConfigApplicator_Apply_ErrorPropagates(t *testing.T) {
	u := &stubRuleUpdater{err: errors.New("db fail")}
	app := NewApplicator(u)
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 50}); err == nil {
		t.Fatalf("expected error propagation")
	}
}
