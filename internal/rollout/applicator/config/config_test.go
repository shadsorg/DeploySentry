package config

import (
	"context"
	"errors"
	"testing"

	"github.com/shadsorg/deploysentry/internal/health"
	"github.com/shadsorg/deploysentry/internal/models"
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
	app := NewApplicator(u, nil, nil, nil)
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 25}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(u.calls) != 1 || u.calls[0] != 25 {
		t.Fatalf("expected [25], got %v", u.calls)
	}
}

func TestConfigApplicator_Apply_MissingRuleID(t *testing.T) {
	app := NewApplicator(&stubRuleUpdater{}, nil, nil, nil)
	ro := &models.Rollout{TargetType: models.TargetTypeConfig}
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 10}); err == nil {
		t.Fatalf("expected error on missing rule_id")
	}
}

func TestConfigApplicator_Revert_RestoresPrevious(t *testing.T) {
	u := &stubRuleUpdater{}
	app := NewApplicator(u, nil, nil, nil)
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
	app := NewApplicator(u, nil, nil, nil)
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
	app := NewApplicator(&stubRuleUpdater{}, nil, nil, nil)
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
	app := NewApplicator(u, nil, nil, nil)
	ro := makeRolloutWithRule(uuid.NewString(), 0)
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 50}); err == nil {
		t.Fatalf("expected error propagation")
	}
}

// --- new health-signal tests ---

type stubRuleFlag struct {
	rule *models.TargetingRule
	flag *models.FeatureFlag
}

func (s *stubRuleFlag) GetRule(_ context.Context, _ uuid.UUID) (*models.TargetingRule, error) {
	return s.rule, nil
}

func (s *stubRuleFlag) GetFlag(_ context.Context, _ uuid.UUID) (*models.FeatureFlag, error) {
	return s.flag, nil
}

type stubFinder struct{ dep *models.Deployment }

func (s *stubFinder) CurrentDeploymentFor(_ context.Context, _, _ uuid.UUID) (*models.Deployment, error) {
	return s.dep, nil
}

type stubHealthLookup struct{ h *health.DeploymentHealth }

func (s *stubHealthLookup) GetHealth(_ uuid.UUID) (*health.DeploymentHealth, error) {
	return s.h, nil
}

func TestConfigApplicator_CurrentSignal_MapsHealth(t *testing.T) {
	appID, envID, depID := uuid.New(), uuid.New(), uuid.New()
	rule := &models.TargetingRule{ID: uuid.New(), FlagID: uuid.New()}
	flag := &models.FeatureFlag{ID: rule.FlagID, ApplicationID: &appID, EnvironmentID: &envID}
	dep := &models.Deployment{ID: depID}
	h := &health.DeploymentHealth{Overall: 0.92, Metrics: map[string]float64{"error_rate": 0.03, "latency_p99_ms": 180}}

	app := NewApplicator(
		&stubRuleUpdater{},
		&stubRuleFlag{rule: rule, flag: flag},
		&stubHealthLookup{h: h},
		&stubFinder{dep: dep},
	)

	ruleIDStr := rule.ID.String()
	ro := &models.Rollout{TargetType: models.TargetTypeConfig, TargetRef: models.RolloutTargetRef{RuleID: &ruleIDStr}}
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 0.92 || s.ErrorRate != 0.03 || s.LatencyP99Ms != 180 {
		t.Fatalf("wrong mapping: %+v", s)
	}
}

func TestConfigApplicator_CurrentSignal_FallsBackWhenDepsNil(t *testing.T) {
	app := NewApplicator(&stubRuleUpdater{}, nil, nil, nil)
	ro := &models.Rollout{TargetType: models.TargetTypeConfig}
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 1.0 {
		t.Fatalf("expected healthy fallback, got %g", s.Score)
	}
}
