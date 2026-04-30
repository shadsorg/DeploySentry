package flags

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// fakeFlagService is a minimal stub that implements only the FlagService methods
// exercised by the revert handlers. Unimplemented methods panic so tests that
// accidentally call them are caught immediately.
type fakeFlagService struct {
	FlagService // embed interface; unimplemented methods panic

	flag          *models.FeatureFlag
	flagErr       error
	archiveErr    error
	unarchiveErr  error
	toggleErr     error
	updateErr     error
	rule          *models.TargetingRule
	ruleErr       error
	deleteRuleErr error
	addRuleErr    error
	ruleEnvStates []*models.RuleEnvironmentState
	ruleEnvErr    error
	flagEnvStates []*models.FlagEnvironmentState
	flagEnvErr    error
	setEnvErr     error
	setRuleEnvErr error

	// capture calls
	archivedID    uuid.UUID
	unarchivedID  uuid.UUID
	toggledID     uuid.UUID
	toggledVal    bool
	updatedFlag   *models.FeatureFlag
	deletedRuleID uuid.UUID
	addedRule     *models.TargetingRule
}

func (f *fakeFlagService) GetFlag(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
	return f.flag, f.flagErr
}

func (f *fakeFlagService) ArchiveFlag(_ context.Context, id uuid.UUID) error {
	f.archivedID = id
	return f.archiveErr
}

func (f *fakeFlagService) UnarchiveFlag(_ context.Context, id uuid.UUID) error {
	f.unarchivedID = id
	return f.unarchiveErr
}

func (f *fakeFlagService) ToggleFlag(_ context.Context, id uuid.UUID, enabled bool) error {
	f.toggledID = id
	f.toggledVal = enabled
	return f.toggleErr
}

func (f *fakeFlagService) UpdateFlag(_ context.Context, flag *models.FeatureFlag) error {
	f.updatedFlag = flag
	return f.updateErr
}

func (f *fakeFlagService) GetRule(_ context.Context, ruleID uuid.UUID) (*models.TargetingRule, error) {
	return f.rule, f.ruleErr
}

func (f *fakeFlagService) DeleteRule(_ context.Context, ruleID uuid.UUID) error {
	f.deletedRuleID = ruleID
	return f.deleteRuleErr
}

func (f *fakeFlagService) AddRule(_ context.Context, rule *models.TargetingRule) error {
	f.addedRule = rule
	return f.addRuleErr
}

func (f *fakeFlagService) ListRuleEnvironmentStates(_ context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
	return f.ruleEnvStates, f.ruleEnvErr
}

func (f *fakeFlagService) SetRuleEnvironmentState(_ context.Context, ruleID, envID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	if f.setRuleEnvErr != nil {
		return nil, f.setRuleEnvErr
	}
	return &models.RuleEnvironmentState{RuleID: ruleID, EnvironmentID: envID, Enabled: enabled}, nil
}

func (f *fakeFlagService) ListFlagEnvStates(_ context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error) {
	return f.flagEnvStates, f.flagEnvErr
}

func (f *fakeFlagService) SetFlagEnvState(_ context.Context, state *models.FlagEnvironmentState) error {
	return f.setEnvErr
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeEntry(entityID uuid.UUID, action, oldValue, newValue string) *models.AuditLogEntry {
	return &models.AuditLogEntry{
		ID:         uuid.New(),
		EntityType: "flag",
		EntityID:   entityID,
		Action:     action,
		OldValue:   oldValue,
		NewValue:   newValue,
		CreatedAt:  time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRevertFlagArchived(t *testing.T) {
	flagID := uuid.New()

	t.Run("success", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Archived: true}}
		handler := revertFlagArchived(svc)
		newAction, err := handler(context.Background(), makeEntry(flagID, "flag.archived", "", ""), false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.archived.reverted" {
			t.Errorf("expected %q, got %q", "flag.archived.reverted", newAction)
		}
		if svc.unarchivedID != flagID {
			t.Errorf("expected UnarchiveFlag called with %s, got %s", flagID, svc.unarchivedID)
		}
	})

	t.Run("race: flag already unarchived", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Archived: false}}
		handler := revertFlagArchived(svc)
		_, err := handler(context.Background(), makeEntry(flagID, "flag.archived", "", ""), false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})

	t.Run("force overrides race", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Archived: false}}
		handler := revertFlagArchived(svc)
		newAction, err := handler(context.Background(), makeEntry(flagID, "flag.archived", "", ""), true)
		if err != nil {
			t.Fatalf("expected no error with force=true, got %v", err)
		}
		if newAction != "flag.archived.reverted" {
			t.Errorf("expected %q, got %q", "flag.archived.reverted", newAction)
		}
	})
}

func TestRevertFlagCreated(t *testing.T) {
	flagID := uuid.New()

	t.Run("success: archives active flag", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Archived: false}}
		handler := revertFlagCreated(svc)
		newAction, err := handler(context.Background(), makeEntry(flagID, "flag.created", "", ""), false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.created.reverted" {
			t.Errorf("expected %q, got %q", "flag.created.reverted", newAction)
		}
		if svc.archivedID != flagID {
			t.Errorf("expected ArchiveFlag called with %s, got %s", flagID, svc.archivedID)
		}
	})

	t.Run("race: flag already archived", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Archived: true}}
		handler := revertFlagCreated(svc)
		_, err := handler(context.Background(), makeEntry(flagID, "flag.created", "", ""), false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})
}

func TestRevertFlagUpdated(t *testing.T) {
	flagID := uuid.New()

	t.Run("malformed JSON in OldValue", func(t *testing.T) {
		// Use force=true so race detection is skipped and the handler reaches the OldValue parse.
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Name: "new"}}
		handler := revertFlagUpdated(svc)
		entry := makeEntry(flagID, "flag.updated", "{not json", `{"name":"new"}`)
		_, err := handler(context.Background(), entry, true)
		if err == nil {
			t.Fatal("expected error for malformed payload")
		}
		if !strings.Contains(err.Error(), "malformed") {
			t.Errorf("expected 'malformed' in error, got: %v", err)
		}
	})

	t.Run("malformed JSON in NewValue", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID}}
		handler := revertFlagUpdated(svc)
		entry := makeEntry(flagID, "flag.updated", `{"name":"old"}`, "{not json")
		_, err := handler(context.Background(), entry, false)
		if err == nil {
			t.Fatal("expected error for malformed payload")
		}
		if !strings.Contains(err.Error(), "malformed payload") {
			t.Errorf("expected 'malformed payload' in error, got: %v", err)
		}
	})

	t.Run("empty old or new value", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID}}
		handler := revertFlagUpdated(svc)
		entry := makeEntry(flagID, "flag.updated", "", "")
		_, err := handler(context.Background(), entry, false)
		if err == nil {
			t.Fatal("expected error for empty payload")
		}
	})

	t.Run("success_path_restores_old_values", func(t *testing.T) {
		// Current flag has name="new", description="new desc"
		currentFlag := &models.FeatureFlag{
			ID:          flagID,
			Name:        "new",
			Description: "new desc",
			Category:    models.FlagCategoryFeature,
		}
		svc := &fakeFlagService{flag: currentFlag}
		handler := revertFlagUpdated(svc)

		oldFlag := models.FeatureFlag{
			ID:          flagID,
			Name:        "old",
			Description: "old desc",
			Category:    models.FlagCategoryFeature,
		}
		oldJSON, _ := json.Marshal(oldFlag)
		newFlag := models.FeatureFlag{
			ID:          flagID,
			Name:        "new",
			Description: "new desc",
			Category:    models.FlagCategoryFeature,
		}
		newJSON, _ := json.Marshal(newFlag)

		entry := makeEntry(flagID, "flag.updated", string(oldJSON), string(newJSON))
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.updated.reverted" {
			t.Errorf("expected %q, got %q", "flag.updated.reverted", newAction)
		}
		if svc.updatedFlag == nil {
			t.Fatal("expected UpdateFlag to be called")
		}
		if svc.updatedFlag.Name != "old" {
			t.Errorf("expected Name=%q, got %q", "old", svc.updatedFlag.Name)
		}
		if svc.updatedFlag.Description != "old desc" {
			t.Errorf("expected Description=%q, got %q", "old desc", svc.updatedFlag.Description)
		}
		// Identifying fields must be preserved from current
		if svc.updatedFlag.ID != flagID {
			t.Errorf("expected ID=%s preserved, got %s", flagID, svc.updatedFlag.ID)
		}
	})

	t.Run("success_restores_expires_at", func(t *testing.T) {
		// Regression test for the critical bug: expires_at must round-trip.
		expiresAt := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

		// Current flag has no expires_at (it was cleared in the "new" edit)
		currentFlag := &models.FeatureFlag{
			ID:       flagID,
			Name:     "myflag",
			Category: models.FlagCategoryFeature,
			// ExpiresAt is nil
		}
		svc := &fakeFlagService{flag: currentFlag}
		handler := revertFlagUpdated(svc)

		// Old value had an expires_at
		oldFlag := models.FeatureFlag{
			ID:        flagID,
			Name:      "myflag",
			Category:  models.FlagCategoryFeature,
			ExpiresAt: &expiresAt,
		}
		oldJSON, _ := json.Marshal(oldFlag)
		// New value cleared expires_at
		newFlag := models.FeatureFlag{
			ID:       flagID,
			Name:     "myflag",
			Category: models.FlagCategoryFeature,
			// ExpiresAt is nil
		}
		newJSON, _ := json.Marshal(newFlag)

		entry := makeEntry(flagID, "flag.updated", string(oldJSON), string(newJSON))
		_, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if svc.updatedFlag == nil {
			t.Fatal("expected UpdateFlag to be called")
		}
		if svc.updatedFlag.ExpiresAt == nil {
			t.Fatal("expected ExpiresAt to be restored, got nil")
		}
		if !svc.updatedFlag.ExpiresAt.Equal(expiresAt) {
			t.Errorf("expected ExpiresAt=%v, got %v", expiresAt, *svc.updatedFlag.ExpiresAt)
		}
	})

	t.Run("race_detected_when_field_differs_from_new_value", func(t *testing.T) {
		// current.Name="something_else" but new_value says name="intermediate"
		currentFlag := &models.FeatureFlag{
			ID:       flagID,
			Name:     "something_else",
			Category: models.FlagCategoryFeature,
		}
		svc := &fakeFlagService{flag: currentFlag}
		handler := revertFlagUpdated(svc)

		oldFlag := models.FeatureFlag{ID: flagID, Name: "original", Category: models.FlagCategoryFeature}
		oldJSON, _ := json.Marshal(oldFlag)
		newFlag := models.FeatureFlag{ID: flagID, Name: "intermediate", Category: models.FlagCategoryFeature}
		newJSON, _ := json.Marshal(newFlag)

		entry := makeEntry(flagID, "flag.updated", string(oldJSON), string(newJSON))
		_, err := handler(context.Background(), entry, false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})

	t.Run("force_overrides_race", func(t *testing.T) {
		// Same setup as race test but force=true
		currentFlag := &models.FeatureFlag{
			ID:       flagID,
			Name:     "something_else",
			Category: models.FlagCategoryFeature,
		}
		svc := &fakeFlagService{flag: currentFlag}
		handler := revertFlagUpdated(svc)

		oldFlag := models.FeatureFlag{ID: flagID, Name: "original", Category: models.FlagCategoryFeature}
		oldJSON, _ := json.Marshal(oldFlag)
		newFlag := models.FeatureFlag{ID: flagID, Name: "intermediate", Category: models.FlagCategoryFeature}
		newJSON, _ := json.Marshal(newFlag)

		entry := makeEntry(flagID, "flag.updated", string(oldJSON), string(newJSON))
		newAction, err := handler(context.Background(), entry, true)
		if err != nil {
			t.Fatalf("expected no error with force=true, got %v", err)
		}
		if newAction != "flag.updated.reverted" {
			t.Errorf("expected %q, got %q", "flag.updated.reverted", newAction)
		}
		if svc.updatedFlag == nil {
			t.Fatal("expected UpdateFlag to be called")
		}
	})
}

func TestRevertFlagToggled(t *testing.T) {
	flagID := uuid.New()

	t.Run("race: current state differs from new_value", func(t *testing.T) {
		// current.Enabled=true, new_value says enabled=false → race
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Enabled: true}}
		handler := revertFlagToggled(svc)
		entry := makeEntry(flagID, "flag.toggled",
			`{"enabled":true}`,
			`{"enabled":false}`,
		)
		_, err := handler(context.Background(), entry, false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})

	t.Run("success: restores old enabled state", func(t *testing.T) {
		// current.Enabled=false matches new_value; should toggle back to old=true
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID, Enabled: false}}
		handler := revertFlagToggled(svc)
		entry := makeEntry(flagID, "flag.toggled",
			`{"enabled":true}`,
			`{"enabled":false}`,
		)
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.toggled.reverted" {
			t.Errorf("expected %q, got %q", "flag.toggled.reverted", newAction)
		}
		if svc.toggledVal != true {
			t.Errorf("expected ToggleFlag called with enabled=true, got enabled=%v", svc.toggledVal)
		}
	})
}

func TestRevertFlagRuleCreated(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()

	newVal, _ := json.Marshal(map[string]interface{}{
		"rule_id": ruleID, "value": "x", "priority": 1,
	})

	t.Run("already deleted is idempotent success", func(t *testing.T) {
		// GetRule returns a wrapped "not found" error (service layer wraps postgres.ErrNotFound).
		svc := &fakeFlagService{ruleErr: errors.New("getting rule: not found")}
		handler := revertFlagRuleCreated(svc)
		entry := makeEntry(flagID, "flag.rule.created", "", string(newVal))
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected idempotent success, got %v", err)
		}
		if newAction != "flag.rule.created.reverted" {
			t.Errorf("expected %q, got %q", "flag.rule.created.reverted", newAction)
		}
		// DeleteRule should NOT have been called
		if svc.deletedRuleID != uuid.Nil {
			t.Error("expected DeleteRule NOT to be called when rule already gone")
		}
	})

	t.Run("transient error propagates (not idempotent)", func(t *testing.T) {
		// GetRule returns a wrapped transient error that is NOT "not found".
		svc := &fakeFlagService{ruleErr: errors.New("getting rule: connection refused")}
		handler := revertFlagRuleCreated(svc)
		entry := makeEntry(flagID, "flag.rule.created", "", string(newVal))
		_, err := handler(context.Background(), entry, false)
		if err == nil {
			t.Fatal("expected error for transient DB error, got nil")
		}
		// Error should contain the original message
		if !strings.Contains(err.Error(), "connection refused") {
			t.Errorf("expected error to mention 'connection refused', got: %v", err)
		}
		// DeleteRule should NOT have been called (error occurred before deletion)
		if svc.deletedRuleID != uuid.Nil {
			t.Error("expected DeleteRule NOT to be called when GetRule fails with transient error")
		}
	})

	t.Run("success: deletes existing rule", func(t *testing.T) {
		existingRule := &models.TargetingRule{ID: ruleID, FlagID: flagID}
		svc := &fakeFlagService{rule: existingRule}
		handler := revertFlagRuleCreated(svc)
		entry := makeEntry(flagID, "flag.rule.created", "", string(newVal))
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.rule.created.reverted" {
			t.Errorf("expected %q, got %q", "flag.rule.created.reverted", newAction)
		}
		if svc.deletedRuleID != ruleID {
			t.Errorf("expected DeleteRule(%s), got %s", ruleID, svc.deletedRuleID)
		}
	})
}

func TestRevertFlagRuleDeleted(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()

	t.Run("malformed old_value (pre-fix audit entry)", func(t *testing.T) {
		svc := &fakeFlagService{ruleErr: errors.New("not found")}
		handler := revertFlagRuleDeleted(svc)
		// Old format: only rule_id, not a full TargetingRule struct → ID will be Nil after unmarshal
		entry := makeEntry(flagID, "flag.rule.deleted", `{"rule_id":"not-a-uuid"}`, "")
		_, err := handler(context.Background(), entry, false)
		if err == nil {
			t.Fatal("expected error for malformed payload")
		}
		if !strings.Contains(err.Error(), "malformed payload") {
			t.Errorf("expected 'malformed payload' in error, got: %v", err)
		}
	})

	t.Run("success: re-creates rule", func(t *testing.T) {
		rule := &models.TargetingRule{
			ID:       ruleID,
			FlagID:   flagID,
			RuleType: models.RuleTypePercentage,
			Value:    "true",
		}
		oldVal, _ := json.Marshal(rule)
		// GetRule returns not-found so no race
		svc := &fakeFlagService{ruleErr: errors.New("not found")}
		handler := revertFlagRuleDeleted(svc)
		entry := makeEntry(flagID, "flag.rule.deleted", string(oldVal), "")
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.rule.deleted.reverted" {
			t.Errorf("expected %q, got %q", "flag.rule.deleted.reverted", newAction)
		}
		if svc.addedRule == nil || svc.addedRule.ID != ruleID {
			t.Errorf("expected AddRule called with rule.ID=%s", ruleID)
		}
	})

	t.Run("transient_error_propagates", func(t *testing.T) {
		rule := &models.TargetingRule{
			ID:       ruleID,
			FlagID:   flagID,
			RuleType: models.RuleTypePercentage,
			Value:    "true",
		}
		oldVal, _ := json.Marshal(rule)
		svc := &fakeFlagService{ruleErr: errors.New("connection refused")}
		handler := revertFlagRuleDeleted(svc)
		entry := makeEntry(flagID, "flag.rule.deleted", string(oldVal), "")
		_, err := handler(context.Background(), entry, false)
		if err == nil {
			t.Fatal("expected error for transient DB error, got nil")
		}
		if !strings.Contains(err.Error(), "connection refused") {
			t.Errorf("expected 'connection refused' in error, got: %v", err)
		}
		// AddRule should NOT have been called
		if svc.addedRule != nil {
			t.Error("expected AddRule NOT to be called when GetRule fails with transient error")
		}
	})

	t.Run("race_detected_when_rule_exists", func(t *testing.T) {
		rule := &models.TargetingRule{
			ID:       ruleID,
			FlagID:   flagID,
			RuleType: models.RuleTypePercentage,
			Value:    "true",
		}
		oldVal, _ := json.Marshal(rule)
		// GetRule returns the rule (it still exists → someone re-created it)
		svc := &fakeFlagService{rule: rule}
		handler := revertFlagRuleDeleted(svc)
		entry := makeEntry(flagID, "flag.rule.deleted", string(oldVal), "")
		_, err := handler(context.Background(), entry, false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})
}

func TestRevertFlagRuleEnvStateUpdated(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()
	envID := uuid.New()

	oldPayload, _ := json.Marshal(map[string]interface{}{
		"rule_id": ruleID, "environment_id": envID, "enabled": true,
	})
	newPayload, _ := json.Marshal(map[string]interface{}{
		"rule_id": ruleID, "environment_id": envID, "enabled": false,
	})

	t.Run("success: restores old enabled state", func(t *testing.T) {
		// current state matches new_value (enabled=false) → no race; restore old (enabled=true)
		svc := &fakeFlagService{
			ruleEnvStates: []*models.RuleEnvironmentState{
				{RuleID: ruleID, EnvironmentID: envID, Enabled: false},
			},
		}
		handler := revertFlagRuleEnvStateUpdated(svc)
		entry := makeEntry(flagID, "flag.rule.env_state.updated", string(oldPayload), string(newPayload))
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.rule.env_state.updated.reverted" {
			t.Errorf("expected %q, got %q", "flag.rule.env_state.updated.reverted", newAction)
		}
	})

	t.Run("race: current state differs from new_value", func(t *testing.T) {
		// current enabled=true but new_value says enabled=false → race
		svc := &fakeFlagService{
			ruleEnvStates: []*models.RuleEnvironmentState{
				{RuleID: ruleID, EnvironmentID: envID, Enabled: true},
			},
		}
		handler := revertFlagRuleEnvStateUpdated(svc)
		entry := makeEntry(flagID, "flag.rule.env_state.updated", string(oldPayload), string(newPayload))
		_, err := handler(context.Background(), entry, false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})
}

func TestRevertFlagEnvStateUpdated(t *testing.T) {
	flagID := uuid.New()
	envID := uuid.New()

	rawTrue := json.RawMessage(`true`)
	rawFalse := json.RawMessage(`false`)

	type envPayload struct {
		EnvironmentID uuid.UUID        `json:"environment_id"`
		Enabled       bool             `json:"enabled"`
		Value         *json.RawMessage `json:"value"`
	}

	oldEnvPayload, _ := json.Marshal(envPayload{EnvironmentID: envID, Enabled: true, Value: &rawTrue})
	newEnvPayload, _ := json.Marshal(envPayload{EnvironmentID: envID, Enabled: false, Value: &rawFalse})

	t.Run("success: restores old state", func(t *testing.T) {
		// current state matches new_value (enabled=false, value=false) → no race; restore old
		svc := &fakeFlagService{
			flagEnvStates: []*models.FlagEnvironmentState{
				{FlagID: flagID, EnvironmentID: envID, Enabled: false, Value: &rawFalse},
			},
		}
		handler := revertFlagEnvStateUpdated(svc)
		entry := makeEntry(flagID, "flag.env_state.updated", string(oldEnvPayload), string(newEnvPayload))
		newAction, err := handler(context.Background(), entry, false)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if newAction != "flag.env_state.updated.reverted" {
			t.Errorf("expected %q, got %q", "flag.env_state.updated.reverted", newAction)
		}
	})

	t.Run("race: enabled differs from new_value", func(t *testing.T) {
		// current enabled=true but new_value says enabled=false → race
		svc := &fakeFlagService{
			flagEnvStates: []*models.FlagEnvironmentState{
				{FlagID: flagID, EnvironmentID: envID, Enabled: true, Value: &rawFalse},
			},
		}
		handler := revertFlagEnvStateUpdated(svc)
		entry := makeEntry(flagID, "flag.env_state.updated", string(oldEnvPayload), string(newEnvPayload))
		_, err := handler(context.Background(), entry, false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})

	t.Run("race: value differs from new_value", func(t *testing.T) {
		rawOther := json.RawMessage(`"something_else"`)
		// enabled matches but value differs → race
		svc := &fakeFlagService{
			flagEnvStates: []*models.FlagEnvironmentState{
				{FlagID: flagID, EnvironmentID: envID, Enabled: false, Value: &rawOther},
			},
		}
		handler := revertFlagEnvStateUpdated(svc)
		entry := makeEntry(flagID, "flag.env_state.updated", string(oldEnvPayload), string(newEnvPayload))
		_, err := handler(context.Background(), entry, false)
		if !errors.Is(err, auth.ErrRevertRace) {
			t.Errorf("expected ErrRevertRace, got %v", err)
		}
	})
}

func TestFlagRevertHandlersConstructor(t *testing.T) {
	svc := &fakeFlagService{}
	tuples := FlagRevertHandlers(svc)
	if len(tuples) != 8 {
		t.Fatalf("expected 8 tuples, got %d", len(tuples))
	}

	expectedActions := []string{
		"flag.archived",
		"flag.created",
		"flag.updated",
		"flag.toggled",
		"flag.rule.created",
		"flag.rule.deleted",
		"flag.rule.env_state.updated",
		"flag.env_state.updated",
	}
	for i, want := range expectedActions {
		if tuples[i].Action != want {
			t.Errorf("tuple[%d]: expected action %q, got %q", i, want, tuples[i].Action)
		}
		if tuples[i].EntityType != "flag" {
			t.Errorf("tuple[%d]: expected entity_type %q, got %q", i, "flag", tuples[i].EntityType)
		}
		if tuples[i].Handler == nil {
			t.Errorf("tuple[%d]: handler is nil", i)
		}
	}
}
