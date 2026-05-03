package rollout

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// ---- Repo fakes (implement just enough of the interfaces) ----

type fakeStrategyRepo struct {
	StrategyRepository
}
type fakeStrategyDefaultRepo struct {
	StrategyDefaultRepository

	upserted *models.StrategyDefault
	deleted  *uuid.UUID
}

func (f *fakeStrategyDefaultRepo) Upsert(_ context.Context, d *models.StrategyDefault) error {
	f.upserted = d
	return nil
}
func (f *fakeStrategyDefaultRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = &id
	return nil
}

type fakeRolloutPolicyRepo struct {
	RolloutPolicyRepository

	upserted *models.RolloutPolicy
	deleted  *uuid.UUID
}

func (f *fakeRolloutPolicyRepo) Upsert(_ context.Context, p *models.RolloutPolicy) error {
	f.upserted = p
	return nil
}
func (f *fakeRolloutPolicyRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = &id
	return nil
}

func ridPtr(id uuid.UUID) *uuid.UUID { return &id }

// ---- Tuples ----

func TestRolloutCommitHandlers_Tuples(t *testing.T) {
	strategies := newStrategyService(nil, &fakeStrategyRepo{}, nil)
	defaults := NewStrategyDefaultService(&fakeStrategyDefaultRepo{})
	policies := NewRolloutPolicyService(&fakeRolloutPolicyRepo{})
	tuples := RolloutCommitHandlers(strategies, defaults, policies)
	want := map[string]bool{
		"strategy.update":          true,
		"strategy.delete":          true,
		"strategy_default.upsert":  true,
		"strategy_default.delete":  true,
		"rollout_policy.upsert":    true,
		"rollout_policy.delete":    true,
	}
	if len(tuples) != len(want) {
		t.Fatalf("expected %d tuples, got %d", len(want), len(tuples))
	}
	for _, tup := range tuples {
		key := tup.ResourceType + "." + tup.Action
		if !want[key] {
			t.Fatalf("unexpected tuple %s", key)
		}
		if tup.Handler == nil {
			t.Fatalf("nil handler for %s", key)
		}
	}
}

// ---- strategy.update — request-shape only (happy path requires the
//      service's full ValidateStrategy + IsSystem check, which is covered
//      by the rollout package's own tests). ----

func TestCommitStrategyUpdate_RequiresResourceID(t *testing.T) {
	svc := newStrategyService(nil, &fakeStrategyRepo{}, nil)
	row := &models.StagedChange{Action: "update", NewValue: json.RawMessage(`{}`)}
	_, err := commitStrategyUpdate(svc)(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

func TestCommitStrategyUpdate_RequiresNewValue(t *testing.T) {
	svc := newStrategyService(nil, &fakeStrategyRepo{}, nil)
	row := &models.StagedChange{Action: "update", ResourceID: ridPtr(uuid.New())}
	_, err := commitStrategyUpdate(svc)(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "new_value required") {
		t.Fatalf("expected new_value error, got %v", err)
	}
}

// ---- strategy.delete — request-shape only ----

func TestCommitStrategyDelete_RequiresResourceID(t *testing.T) {
	svc := newStrategyService(nil, &fakeStrategyRepo{}, nil)
	row := &models.StagedChange{Action: "delete"}
	_, err := commitStrategyDelete(svc)(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

// ---- strategy_default.upsert — happy path + body-id override ----

func TestCommitStrategyDefaultUpsert_OverridesIDFromResourceID(t *testing.T) {
	intended := uuid.New()
	bodyHadDifferent := uuid.New()
	repo := &fakeStrategyDefaultRepo{}
	svc := NewStrategyDefaultService(repo)
	body, _ := json.Marshal(&models.StrategyDefault{ID: bodyHadDifferent, ScopeID: uuid.New(), StrategyID: uuid.New()})
	row := &models.StagedChange{
		ResourceType: "strategy_default", Action: "upsert",
		ResourceID: ridPtr(intended), NewValue: body,
	}
	action, err := commitStrategyDefaultUpsert(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "strategy_default.upserted" {
		t.Fatalf("expected strategy_default.upserted, got %s", action)
	}
	if repo.upserted == nil || repo.upserted.ID != intended {
		t.Fatalf("upsert should have used row.ResourceID, got %+v", repo.upserted)
	}
}

func TestCommitStrategyDefaultUpsert_RequiresResourceID(t *testing.T) {
	svc := NewStrategyDefaultService(&fakeStrategyDefaultRepo{})
	row := &models.StagedChange{Action: "upsert", NewValue: json.RawMessage(`{}`)}
	_, err := commitStrategyDefaultUpsert(svc)(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

// ---- strategy_default.delete — happy path ----

func TestCommitStrategyDefaultDelete_CallsRepo(t *testing.T) {
	id := uuid.New()
	repo := &fakeStrategyDefaultRepo{}
	svc := NewStrategyDefaultService(repo)
	row := &models.StagedChange{Action: "delete", ResourceID: ridPtr(id)}
	action, err := commitStrategyDefaultDelete(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "strategy_default.deleted" {
		t.Fatalf("expected strategy_default.deleted, got %s", action)
	}
	if repo.deleted == nil || *repo.deleted != id {
		t.Fatalf("delete should have called repo with id=%s, got %v", id, repo.deleted)
	}
}

// ---- rollout_policy.upsert — happy path + body-id override ----

func TestCommitRolloutPolicyUpsert_OverridesIDFromResourceID(t *testing.T) {
	intended := uuid.New()
	bodyHadDifferent := uuid.New()
	repo := &fakeRolloutPolicyRepo{}
	svc := NewRolloutPolicyService(repo)
	body, _ := json.Marshal(&models.RolloutPolicy{
		ID: bodyHadDifferent, ScopeID: uuid.New(), Enabled: true, Policy: models.PolicyMandate,
	})
	row := &models.StagedChange{
		ResourceType: "rollout_policy", Action: "upsert",
		ResourceID: ridPtr(intended), NewValue: body,
	}
	action, err := commitRolloutPolicyUpsert(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "rollout_policy.upserted" {
		t.Fatalf("expected rollout_policy.upserted, got %s", action)
	}
	if repo.upserted == nil || repo.upserted.ID != intended {
		t.Fatalf("upsert should have used row.ResourceID, got %+v", repo.upserted)
	}
	if repo.upserted.Policy != models.PolicyMandate {
		t.Fatalf("payload fields not propagated: %+v", repo.upserted)
	}
}

func TestCommitRolloutPolicyUpsert_RequiresNewValue(t *testing.T) {
	svc := NewRolloutPolicyService(&fakeRolloutPolicyRepo{})
	row := &models.StagedChange{Action: "upsert", ResourceID: ridPtr(uuid.New())}
	_, err := commitRolloutPolicyUpsert(svc)(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "new_value required") {
		t.Fatalf("expected new_value error, got %v", err)
	}
}

// ---- rollout_policy.delete — happy path ----

func TestCommitRolloutPolicyDelete_CallsRepo(t *testing.T) {
	id := uuid.New()
	repo := &fakeRolloutPolicyRepo{}
	svc := NewRolloutPolicyService(repo)
	row := &models.StagedChange{Action: "delete", ResourceID: ridPtr(id)}
	action, err := commitRolloutPolicyDelete(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "rollout_policy.deleted" {
		t.Fatalf("expected rollout_policy.deleted, got %s", action)
	}
	if repo.deleted == nil || *repo.deleted != id {
		t.Fatalf("delete should have called repo with id=%s, got %v", id, repo.deleted)
	}
}

// ---- StrategyCreateHandlers + commitStrategyCreate ----

func TestStrategyCreateHandlers_RegistersExpectedTuples(t *testing.T) {
	svc := newStrategyService(&mockTxBeginner{}, &fakeStratRepo{}, nil)
	tuples := StrategyCreateHandlers(svc)
	if len(tuples) != 1 {
		t.Fatalf("expected 1 tuple, got %d", len(tuples))
	}
	if tuples[0].ResourceType != "strategy" || tuples[0].Action != "create" {
		t.Errorf("unexpected tuple: %s.%s", tuples[0].ResourceType, tuples[0].Action)
	}
	if tuples[0].Handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestCommitStrategyCreate_MintsRealID(t *testing.T) {
	wantReal := uuid.New()
	repo := &fakeStratRepo{
		createTxFn: func(s *models.Strategy) error {
			s.ID = wantReal
			return nil
		},
	}
	svc := newStrategyService(&mockTxBeginner{}, repo, nil)

	prov := staging.NewProvisional()
	payload, _ := json.Marshal(&models.Strategy{
		ScopeType:  models.ScopeOrg,
		ScopeID:    uuid.New(),
		Name:       "canary",
		TargetType: models.TargetTypeDeploy,
		Steps:      []models.Step{{Percent: 100}},
	})
	row := &models.StagedChange{
		ResourceType:  "strategy",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      payload,
	}

	realID, action, hook, err := commitStrategyCreate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if realID != wantReal {
		t.Errorf("realID: got %v want %v", realID, wantReal)
	}
	if staging.IsProvisional(realID) {
		t.Fatal("realID is provisional")
	}
	if action != "strategy.created" {
		t.Errorf("action: %v", action)
	}
	if hook != nil {
		t.Error("expected nil post-commit hook for strategy.create")
	}
}

func TestCommitStrategyCreate_RequiresNewValue(t *testing.T) {
	svc := newStrategyService(&mockTxBeginner{}, &fakeStratRepo{}, nil)
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "strategy",
		Action:        "create",
		ProvisionalID: &prov,
	}
	_, _, _, err := commitStrategyCreate(svc)(context.Background(), nil, row)
	if err == nil {
		t.Fatal("expected error for missing new_value")
	}
}
