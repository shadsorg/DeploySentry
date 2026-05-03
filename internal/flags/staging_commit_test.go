package flags

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// stubCommitSvc embeds FlagService so unused methods panic — keeps the test
// double tight. Only the methods exercised by the commit handlers are
// implemented.
type stubCommitSvc struct {
	FlagService

	toggleCalled              func(uuid.UUID, bool) error
	updateCalled              func(*models.FeatureFlag) error
	archiveCalled             func(uuid.UUID) error
	restoreCalled             func(uuid.UUID) error
	updateRuleCalled          func(*models.TargetingRule) error
	deleteRuleCalled          func(uuid.UUID) error
	setRuleEnvStateCalled     func(uuid.UUID, uuid.UUID, bool) error
	setFlagEnvStateCalled     func(*models.FlagEnvironmentState) error
	createFlagTxCalled        func(*models.FeatureFlag) (uuid.UUID, error)
	publishCreatedCalled      func(*models.FeatureFlag)
	addRuleTxCalled           func(*models.TargetingRule) (uuid.UUID, error)
	invalidateFlagCacheCalled func(uuid.UUID) error
}

func (s *stubCommitSvc) ToggleFlag(_ context.Context, id uuid.UUID, enabled bool) error {
	if s.toggleCalled == nil {
		return nil
	}
	return s.toggleCalled(id, enabled)
}

func (s *stubCommitSvc) UpdateFlag(_ context.Context, flag *models.FeatureFlag) error {
	if s.updateCalled == nil {
		return nil
	}
	return s.updateCalled(flag)
}

func (s *stubCommitSvc) ArchiveFlag(_ context.Context, id uuid.UUID) error {
	if s.archiveCalled == nil {
		return nil
	}
	return s.archiveCalled(id)
}

func (s *stubCommitSvc) RestoreFlag(_ context.Context, id uuid.UUID) error {
	if s.restoreCalled == nil {
		return nil
	}
	return s.restoreCalled(id)
}

func (s *stubCommitSvc) UpdateRule(_ context.Context, rule *models.TargetingRule) error {
	if s.updateRuleCalled == nil {
		return nil
	}
	return s.updateRuleCalled(rule)
}

func (s *stubCommitSvc) DeleteRule(_ context.Context, ruleID uuid.UUID) error {
	if s.deleteRuleCalled == nil {
		return nil
	}
	return s.deleteRuleCalled(ruleID)
}

func (s *stubCommitSvc) SetRuleEnvironmentState(_ context.Context, ruleID, envID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	if s.setRuleEnvStateCalled != nil {
		if err := s.setRuleEnvStateCalled(ruleID, envID, enabled); err != nil {
			return nil, err
		}
	}
	return &models.RuleEnvironmentState{RuleID: ruleID, EnvironmentID: envID, Enabled: enabled}, nil
}

func (s *stubCommitSvc) SetFlagEnvState(_ context.Context, state *models.FlagEnvironmentState) error {
	if s.setFlagEnvStateCalled == nil {
		return nil
	}
	return s.setFlagEnvStateCalled(state)
}

func ridPtr(id uuid.UUID) *uuid.UUID { return &id }

// ---- Registration shape ----

func TestFlagCommitHandlers_RegistersExpectedTuples(t *testing.T) {
	svc := &stubCommitSvc{}
	tuples := FlagCommitHandlers(svc)
	type key struct{ rt, action string }
	want := map[key]bool{
		{"flag", "toggle"}:                          true,
		{"flag", "update"}:                          true,
		{"flag", "archive"}:                         true,
		{"flag", "restore"}:                         true,
		{"flag_rule", "update"}:                     true,
		{"flag_rule", "delete"}:                     true,
		{"flag_rule_env_state", "update"}:           true,
		{"flag_env_state", "update"}:                true,
	}
	if len(tuples) != len(want) {
		t.Fatalf("expected %d tuples, got %d", len(want), len(tuples))
	}
	for _, tup := range tuples {
		if !want[key{tup.ResourceType, tup.Action}] {
			t.Fatalf("unexpected tuple %s.%s", tup.ResourceType, tup.Action)
		}
		if tup.Handler == nil {
			t.Fatalf("handler for %s.%s is nil", tup.ResourceType, tup.Action)
		}
	}
}

// ---- toggle (Phase A regression check) ----

func TestCommitFlagToggle_DispatchesEnabledFromPayload(t *testing.T) {
	flagID := uuid.New()
	gotEnabled := false
	svc := &stubCommitSvc{
		toggleCalled: func(id uuid.UUID, enabled bool) error {
			if id != flagID {
				t.Fatalf("expected id=%s, got %s", flagID, id)
			}
			gotEnabled = enabled
			return nil
		},
	}
	row := &models.StagedChange{
		ResourceType: "flag",
		Action:       "toggle",
		ResourceID:   ridPtr(flagID),
		NewValue:     json.RawMessage(`{"enabled":true}`),
	}
	action, err := commitFlagToggle(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.toggled" {
		t.Fatalf("expected audit action flag.toggled, got %s", action)
	}
	if !gotEnabled {
		t.Fatal("expected toggle to receive enabled=true")
	}
}

// ---- update ----

func TestCommitFlagUpdate_OverridesIDFromResourceID(t *testing.T) {
	intendedID := uuid.New()
	bodyHadDifferentID := uuid.New()
	var got *models.FeatureFlag
	svc := &stubCommitSvc{
		updateCalled: func(flag *models.FeatureFlag) error {
			got = flag
			return nil
		},
	}
	body, _ := json.Marshal(&models.FeatureFlag{ID: bodyHadDifferentID, Key: "x"})
	row := &models.StagedChange{
		ResourceType: "flag",
		Action:       "update",
		ResourceID:   ridPtr(intendedID),
		NewValue:     body,
	}
	action, err := commitFlagUpdate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.updated" {
		t.Fatalf("expected flag.updated, got %s", action)
	}
	if got == nil || got.ID != intendedID {
		t.Fatalf("update should have used row.ResourceID, got %+v", got)
	}
}

func TestCommitFlagUpdate_RequiresResourceID(t *testing.T) {
	row := &models.StagedChange{ResourceType: "flag", Action: "update", NewValue: json.RawMessage(`{}`)}
	_, err := commitFlagUpdate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

func TestCommitFlagUpdate_RequiresNewValue(t *testing.T) {
	row := &models.StagedChange{ResourceType: "flag", Action: "update", ResourceID: ridPtr(uuid.New())}
	_, err := commitFlagUpdate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "new_value required") {
		t.Fatalf("expected new_value error, got %v", err)
	}
}

func TestCommitFlagUpdate_PropagatesServiceError(t *testing.T) {
	boom := errors.New("invalid lifecycle state")
	svc := &stubCommitSvc{updateCalled: func(*models.FeatureFlag) error { return boom }}
	row := &models.StagedChange{
		ResourceType: "flag", Action: "update",
		ResourceID: ridPtr(uuid.New()),
		NewValue:   json.RawMessage(`{"key":"x"}`),
	}
	_, err := commitFlagUpdate(svc)(context.Background(), nil, row)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped service error, got %v", err)
	}
}

// ---- archive ----

func TestCommitFlagArchive_CallsArchive(t *testing.T) {
	flagID := uuid.New()
	called := false
	svc := &stubCommitSvc{
		archiveCalled: func(id uuid.UUID) error {
			called = true
			if id != flagID {
				t.Fatalf("expected id=%s, got %s", flagID, id)
			}
			return nil
		},
	}
	row := &models.StagedChange{ResourceType: "flag", Action: "archive", ResourceID: ridPtr(flagID)}
	action, err := commitFlagArchive(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.archived" {
		t.Fatalf("expected flag.archived, got %s", action)
	}
	if !called {
		t.Fatal("ArchiveFlag was not invoked")
	}
}

func TestCommitFlagArchive_RequiresResourceID(t *testing.T) {
	_, err := commitFlagArchive(&stubCommitSvc{})(context.Background(), nil, &models.StagedChange{Action: "archive"})
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

// ---- restore ----

func TestCommitFlagRestore_CallsRestore(t *testing.T) {
	flagID := uuid.New()
	called := false
	svc := &stubCommitSvc{
		restoreCalled: func(id uuid.UUID) error {
			called = true
			if id != flagID {
				t.Fatalf("expected id=%s, got %s", flagID, id)
			}
			return nil
		},
	}
	row := &models.StagedChange{ResourceType: "flag", Action: "restore", ResourceID: ridPtr(flagID)}
	action, err := commitFlagRestore(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.restored" {
		t.Fatalf("expected flag.restored, got %s", action)
	}
	if !called {
		t.Fatal("RestoreFlag was not invoked")
	}
}

func TestCommitFlagRestore_RequiresResourceID(t *testing.T) {
	_, err := commitFlagRestore(&stubCommitSvc{})(context.Background(), nil, &models.StagedChange{Action: "restore"})
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

// ---- flag_rule.update ----

func TestCommitFlagRuleUpdate_OverridesIDFromResourceID(t *testing.T) {
	intended := uuid.New()
	bodyHadDifferent := uuid.New()
	var got *models.TargetingRule
	svc := &stubCommitSvc{
		updateRuleCalled: func(r *models.TargetingRule) error {
			got = r
			return nil
		},
	}
	body, _ := json.Marshal(&models.TargetingRule{ID: bodyHadDifferent, Value: "x"})
	row := &models.StagedChange{ResourceType: "flag_rule", Action: "update", ResourceID: ridPtr(intended), NewValue: body}
	action, err := commitFlagRuleUpdate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.rule.updated" {
		t.Fatalf("expected flag.rule.updated, got %s", action)
	}
	if got == nil || got.ID != intended {
		t.Fatalf("update should have used row.ResourceID, got %+v", got)
	}
}

func TestCommitFlagRuleUpdate_RequiresResourceIDAndNewValue(t *testing.T) {
	cases := []struct {
		name string
		row  *models.StagedChange
		want string
	}{
		{"no resource_id", &models.StagedChange{Action: "update", NewValue: json.RawMessage(`{}`)}, "resource_id required"},
		{"no new_value", &models.StagedChange{Action: "update", ResourceID: ridPtr(uuid.New())}, "new_value required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := commitFlagRuleUpdate(&stubCommitSvc{})(context.Background(), nil, c.row)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected %q error, got %v", c.want, err)
			}
		})
	}
}

// ---- flag_rule.delete ----

func TestCommitFlagRuleDelete_CallsService(t *testing.T) {
	ruleID := uuid.New()
	called := false
	svc := &stubCommitSvc{
		deleteRuleCalled: func(id uuid.UUID) error {
			called = true
			if id != ruleID {
				t.Fatalf("expected id=%s, got %s", ruleID, id)
			}
			return nil
		},
	}
	row := &models.StagedChange{ResourceType: "flag_rule", Action: "delete", ResourceID: ridPtr(ruleID)}
	action, err := commitFlagRuleDelete(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.rule.deleted" {
		t.Fatalf("expected flag.rule.deleted, got %s", action)
	}
	if !called {
		t.Fatal("DeleteRule was not invoked")
	}
}

func TestCommitFlagRuleDelete_RequiresResourceID(t *testing.T) {
	_, err := commitFlagRuleDelete(&stubCommitSvc{})(context.Background(), nil, &models.StagedChange{Action: "delete"})
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

// ---- flag_rule_env_state.update ----

func TestCommitFlagRuleEnvStateUpdate_DispatchesPayloadFields(t *testing.T) {
	ruleID := uuid.New()
	envID := uuid.New()
	var gotRule, gotEnv uuid.UUID
	var gotEnabled bool
	svc := &stubCommitSvc{
		setRuleEnvStateCalled: func(rid, eid uuid.UUID, enabled bool) error {
			gotRule, gotEnv, gotEnabled = rid, eid, enabled
			return nil
		},
	}
	body, _ := json.Marshal(ruleEnvStatePayload{EnvironmentID: envID, Enabled: true})
	row := &models.StagedChange{
		ResourceType: "flag_rule_env_state",
		Action:       "update",
		ResourceID:   ridPtr(ruleID),
		NewValue:     body,
	}
	action, err := commitFlagRuleEnvStateUpdate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.rule.env_state.updated" {
		t.Fatalf("expected flag.rule.env_state.updated, got %s", action)
	}
	if gotRule != ruleID || gotEnv != envID || !gotEnabled {
		t.Fatalf("dispatch passed wrong args: rule=%s env=%s enabled=%v", gotRule, gotEnv, gotEnabled)
	}
}

func TestCommitFlagRuleEnvStateUpdate_RequiresEnvironmentID(t *testing.T) {
	body, _ := json.Marshal(ruleEnvStatePayload{Enabled: true}) // EnvironmentID = uuid.Nil
	row := &models.StagedChange{
		ResourceType: "flag_rule_env_state",
		Action:       "update",
		ResourceID:   ridPtr(uuid.New()),
		NewValue:     body,
	}
	_, err := commitFlagRuleEnvStateUpdate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "environment_id required") {
		t.Fatalf("expected environment_id error, got %v", err)
	}
}

// ---- flag_env_state.update ----

func TestCommitFlagEnvStateUpdate_OverridesFlagIDFromResourceID(t *testing.T) {
	intendedFlagID := uuid.New()
	bodyHadDifferent := uuid.New()
	envID := uuid.New()
	var got *models.FlagEnvironmentState
	svc := &stubCommitSvc{
		setFlagEnvStateCalled: func(s *models.FlagEnvironmentState) error {
			got = s
			return nil
		},
	}
	body, _ := json.Marshal(&models.FlagEnvironmentState{
		FlagID: bodyHadDifferent, EnvironmentID: envID, Enabled: true,
	})
	row := &models.StagedChange{
		ResourceType: "flag_env_state",
		Action:       "update",
		ResourceID:   ridPtr(intendedFlagID),
		NewValue:     body,
	}
	action, err := commitFlagEnvStateUpdate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if action != "flag.env_state.updated" {
		t.Fatalf("expected flag.env_state.updated, got %s", action)
	}
	if got == nil || got.FlagID != intendedFlagID {
		t.Fatalf("FlagID should come from ResourceID, got %+v", got)
	}
	if got.EnvironmentID != envID || !got.Enabled {
		t.Fatalf("payload fields not propagated: %+v", got)
	}
}

func TestCommitFlagEnvStateUpdate_RequiresEnvironmentID(t *testing.T) {
	body, _ := json.Marshal(&models.FlagEnvironmentState{Enabled: true}) // no env id
	row := &models.StagedChange{
		ResourceType: "flag_env_state",
		Action:       "update",
		ResourceID:   ridPtr(uuid.New()),
		NewValue:     body,
	}
	_, err := commitFlagEnvStateUpdate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "environment_id required") {
		t.Fatalf("expected environment_id error, got %v", err)
	}
}

func (s *stubCommitSvc) CreateFlagTx(_ context.Context, _ pgx.Tx, flag *models.FeatureFlag) (uuid.UUID, error) {
	if s.createFlagTxCalled == nil {
		return flag.ID, nil
	}
	return s.createFlagTxCalled(flag)
}

func (s *stubCommitSvc) PublishCreated(_ context.Context, flag *models.FeatureFlag) {
	if s.publishCreatedCalled != nil {
		s.publishCreatedCalled(flag)
	}
}

func (s *stubCommitSvc) AddRuleTx(_ context.Context, _ pgx.Tx, rule *models.TargetingRule) (uuid.UUID, error) {
	if s.addRuleTxCalled == nil {
		return rule.ID, nil
	}
	return s.addRuleTxCalled(rule)
}

func (s *stubCommitSvc) InvalidateFlagCache(_ context.Context, flagID uuid.UUID) error {
	if s.invalidateFlagCacheCalled == nil {
		return nil
	}
	return s.invalidateFlagCacheCalled(flagID)
}

// ---- create ----

func TestFlagCreateHandlers_RegistersExpectedTuples(t *testing.T) {
	svc := &stubCommitSvc{}
	tuples := FlagCreateHandlers(svc)
	type key struct{ rt, action string }
	want := map[key]bool{
		{"flag", "create"}:      true,
		{"flag_rule", "create"}: true,
	}
	if len(tuples) != len(want) {
		t.Fatalf("expected %d tuples, got %d", len(want), len(tuples))
	}
	for _, tup := range tuples {
		if !want[key{tup.ResourceType, tup.Action}] {
			t.Fatalf("unexpected tuple %s.%s", tup.ResourceType, tup.Action)
		}
		if tup.Handler == nil {
			t.Fatalf("handler for %s.%s is nil", tup.ResourceType, tup.Action)
		}
	}
}

func TestCommitFlagCreate_MintsRealIDAndDefersHook(t *testing.T) {
	var calls []string
	wantReal := uuid.New()
	svc := &stubCommitSvc{
		createFlagTxCalled: func(flag *models.FeatureFlag) (uuid.UUID, error) {
			calls = append(calls, "createFlagTx")
			flag.ID = wantReal
			return wantReal, nil
		},
		publishCreatedCalled: func(flag *models.FeatureFlag) {
			calls = append(calls, "publishCreated")
		},
	}
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "flag",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      json.RawMessage(`{"key":"new","name":"New"}`),
	}
	realID, action, hook, err := commitFlagCreate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if realID != wantReal {
		t.Fatalf("realID: got %v want %v", realID, wantReal)
	}
	if staging.IsProvisional(realID) {
		t.Fatalf("realID is provisional")
	}
	if action != "flag.created" {
		t.Errorf("action: %v", action)
	}
	if hook == nil {
		t.Fatal("hook should be non-nil")
	}
	if got := strings.Join(calls, ","); got != "createFlagTx" {
		t.Errorf("hook fired inside handler: %v", got)
	}
	hook(context.Background())
	if got := strings.Join(calls, ","); got != "createFlagTx,publishCreated" {
		t.Errorf("hook did not publish post-commit: %v", got)
	}
}

func TestCommitFlagCreate_RequiresNewValue(t *testing.T) {
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "flag",
		Action:        "create",
		ProvisionalID: &prov,
	}
	_, _, _, err := commitFlagCreate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil {
		t.Fatal("expected error for missing new_value")
	}
}

// ---- flag_rule.create ----

func TestCommitFlagRuleCreate_MintsRealIDAndInvalidates(t *testing.T) {
	var calls []string
	wantReal := uuid.New()
	flagID := uuid.New()
	svc := &stubCommitSvc{
		addRuleTxCalled: func(rule *models.TargetingRule) (uuid.UUID, error) {
			calls = append(calls, "addRuleTx")
			rule.ID = wantReal
			return wantReal, nil
		},
		invalidateFlagCacheCalled: func(fID uuid.UUID) error {
			calls = append(calls, "invalidate")
			if fID != flagID {
				t.Errorf("invalidate target mismatch: got %v want %v", fID, flagID)
			}
			return nil
		},
	}
	prov := staging.NewProvisional()
	payload := `{"flag_id":"` + flagID.String() + `","priority":1}`
	row := &models.StagedChange{
		ResourceType:  "flag_rule",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      json.RawMessage(payload),
	}
	realID, action, hook, err := commitFlagRuleCreate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if realID != wantReal {
		t.Errorf("realID: got %v want %v", realID, wantReal)
	}
	if action != "flag.rule.created" {
		t.Errorf("action: %v", action)
	}
	if hook == nil {
		t.Fatal("hook should be non-nil")
	}
	if got := strings.Join(calls, ","); got != "addRuleTx" {
		t.Errorf("hook fired inside handler: %v", got)
	}
	hook(context.Background())
	if got := strings.Join(calls, ","); got != "addRuleTx,invalidate" {
		t.Errorf("hook did not invalidate post-commit: %v", got)
	}
}

func TestCommitFlagRuleCreate_RejectsProvisionalFlagID(t *testing.T) {
	// The resolver should have rewritten flag_id before this handler runs.
	// If a provisional UUID survives to here, MustNotBeProvisional must panic.
	prov := staging.NewProvisional()
	payload := `{"flag_id":"` + prov.String() + `","priority":1}`
	row := &models.StagedChange{
		ResourceType:  "flag_rule",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      json.RawMessage(payload),
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from MustNotBeProvisional")
		}
	}()
	_, _, _, _ = commitFlagRuleCreate(&stubCommitSvc{})(context.Background(), nil, row)
}

func TestCommitFlagRuleCreate_RequiresNewValue(t *testing.T) {
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "flag_rule",
		Action:        "create",
		ProvisionalID: &prov,
	}
	_, _, _, err := commitFlagRuleCreate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil {
		t.Fatal("expected error for missing new_value")
	}
}

func TestCommitFlagRuleCreate_RequiresFlagID(t *testing.T) {
	prov := staging.NewProvisional()
	row := &models.StagedChange{
		ResourceType:  "flag_rule",
		Action:        "create",
		ProvisionalID: &prov,
		NewValue:      json.RawMessage(`{"priority":1}`),
	}
	_, _, _, err := commitFlagRuleCreate(&stubCommitSvc{})(context.Background(), nil, row)
	if err == nil {
		t.Fatal("expected error for missing flag_id")
	}
}
