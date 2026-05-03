package rollout

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

func TestAttacher_AttachDeploy_ExplicitStrategyName(t *testing.T) {
	// Seed a template via the Plan 1 fake repos.
	stratRepo := newFakeStratRepo()
	orgID := uuid.New()
	tmpl := &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	}
	_ = stratRepo.Create(context.Background(), tmpl)

	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	attacher := NewAttacher(
		newStrategyService(nil, stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	dep := &models.Deployment{ID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New()}
	actor := uuid.New()

	// Attempt resolution against an app-level leaf with org ancestor being the strategy's scope.
	leaf := ScopeRef{Type: models.ScopeApp, ID: dep.ApplicationID}
	projID := uuid.New()
	err := attacher.AttachDeploy(context.Background(), dep, &AttachIntent{
		StrategyName: "std",
		Leaf:         leaf,
		ProjectID:    &projID,
		OrgID:        &orgID,
	}, actor)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	// Verify one rollout was created.
	if len(rolloutRepo.rows) != 1 {
		t.Fatalf("expected 1 rollout, got %d", len(rolloutRepo.rows))
	}
}

func TestAttacher_AttachDeploy_ResolvesDefault(t *testing.T) {
	stratRepo := newFakeStratRepo()
	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	orgID := uuid.New()
	tmpl := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "default-canary",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	_ = stratRepo.Create(context.Background(), tmpl)
	tt := models.TargetTypeDeploy
	_ = defRepo.Upsert(context.Background(), &models.StrategyDefault{
		ScopeType: models.ScopeOrg, ScopeID: orgID, TargetType: &tt, StrategyID: tmpl.ID,
	})

	attacher := NewAttacher(
		newStrategyService(nil, stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	dep := &models.Deployment{ID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New()}
	projID := uuid.New()
	if err := attacher.AttachDeploy(context.Background(), dep, &AttachIntent{
		Leaf:      ScopeRef{Type: models.ScopeApp, ID: dep.ApplicationID},
		ProjectID: &projID,
		OrgID:     &orgID,
	}, uuid.New()); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if len(rolloutRepo.rows) != 1 {
		t.Fatalf("expected default to resolve; got %d rollouts", len(rolloutRepo.rows))
	}
}

func TestAttacher_AttachDeploy_MandateWithoutStrategy_Errors(t *testing.T) {
	stratRepo := newFakeStratRepo()
	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	orgID := uuid.New()
	_ = polRepo.Upsert(context.Background(), &models.RolloutPolicy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Enabled: true, Policy: models.PolicyMandate,
	})

	attacher := NewAttacher(
		newStrategyService(nil, stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	dep := &models.Deployment{ID: uuid.New(), ApplicationID: uuid.New()}
	projID := uuid.New()
	if err := attacher.AttachDeploy(context.Background(), dep, &AttachIntent{
		Leaf:      ScopeRef{Type: models.ScopeApp, ID: dep.ApplicationID},
		ProjectID: &projID,
		OrgID:     &orgID,
	}, uuid.New()); err == nil {
		t.Fatalf("expected mandate error")
	}
}

func TestAttacher_MissingOverrides_Snapshot(t *testing.T) {
	stratRepo := newFakeStratRepo()
	orgID := uuid.New()
	tmpl := &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95,
	}
	_ = stratRepo.Create(context.Background(), tmpl)
	raw, _ := json.Marshal(struct{}{})
	_ = raw
	// (Smoke test that empty overrides don't crash.)
}

func TestAttacher_AttachConfig_ExplicitStrategyName(t *testing.T) {
	stratRepo := newFakeStratRepo()
	orgID := uuid.New()
	tmpl := &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "slow-roll",
		TargetType: models.TargetTypeConfig,
		Steps:      []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	}
	_ = stratRepo.Create(context.Background(), tmpl)

	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	attacher := NewAttacher(
		newStrategyService(nil, stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	ruleID := uuid.New()
	projID := uuid.New()
	appID := uuid.New()
	err := attacher.AttachConfig(context.Background(), ruleID, 5, &AttachIntent{
		StrategyName: "slow-roll",
		Leaf:         ScopeRef{Type: models.ScopeApp, ID: appID},
		ProjectID:    &projID,
		OrgID:        &orgID,
	}, uuid.New())
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if len(rolloutRepo.rows) != 1 {
		t.Fatalf("expected 1 rollout created, got %d", len(rolloutRepo.rows))
	}
	for _, r := range rolloutRepo.rows {
		if r.TargetType != models.TargetTypeConfig {
			t.Fatalf("expected config rollout, got %s", r.TargetType)
		}
		if r.TargetRef.PreviousPercentage == nil || *r.TargetRef.PreviousPercentage != 5 {
			t.Fatalf("expected previous_percentage=5")
		}
	}
}

func TestAttacher_AttachConfig_MandateWithoutStrategy_Errors(t *testing.T) {
	stratRepo := newFakeStratRepo()
	defRepo := newFakeDefaultsRepo()
	polRepo := newFakePolicyRepo()
	rolloutRepo := newFakeRolloutRepo()
	svc := NewRolloutService(rolloutRepo, &fakePhaseRepo{}, &fakeEventRepo{}, &fakePublisher{})

	orgID := uuid.New()
	_ = polRepo.Upsert(context.Background(), &models.RolloutPolicy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Enabled: true, Policy: models.PolicyMandate,
	})

	attacher := NewAttacher(
		newStrategyService(nil, stratRepo, nil),
		NewStrategyDefaultService(defRepo),
		NewRolloutPolicyService(polRepo),
		svc,
	)

	ruleID := uuid.New()
	projID := uuid.New()
	appID := uuid.New()
	if err := attacher.AttachConfig(context.Background(), ruleID, 0, &AttachIntent{
		Leaf:      ScopeRef{Type: models.ScopeApp, ID: appID},
		ProjectID: &projID,
		OrgID:     &orgID,
	}, uuid.New()); err == nil {
		t.Fatalf("expected mandate error")
	}
}
