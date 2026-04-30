package flags

import (
	"context"
	"encoding/json"
	"errors"
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
	archivedID   uuid.UUID
	unarchivedID uuid.UUID
	toggledID    uuid.UUID
	toggledVal   bool
	updatedFlag  *models.FeatureFlag
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

func TestRevertFlagUpdated(t *testing.T) {
	flagID := uuid.New()

	t.Run("malformed JSON in OldValue", func(t *testing.T) {
		svc := &fakeFlagService{flag: &models.FeatureFlag{ID: flagID}}
		handler := revertFlagUpdated(svc)
		entry := makeEntry(flagID, "flag.updated", "{not json", `{"name":"new"}`)
		_, err := handler(context.Background(), entry, false)
		if err == nil {
			t.Fatal("expected error for malformed payload")
		}
		if !contains(err.Error(), "malformed payload") {
			t.Errorf("expected 'malformed payload' in error, got: %v", err)
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
		if !contains(err.Error(), "malformed payload") {
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
		if !contains(err.Error(), "connection refused") {
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
		if !contains(err.Error(), "malformed payload") {
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

// contains is a simple substring check for error message assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
