package staging

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

func TestPlanBatchHappyPathOrdersCreatesFirst(t *testing.T) {
	provFlag := NewProvisional()
	rowCreateFlag := &models.StagedChange{
		ID:            uuid.New(),
		ResourceType:  "flag",
		Action:        "create",
		ProvisionalID: &provFlag,
		NewValue:      []byte(`{"key":"x"}`),
	}
	realRule := uuid.New()
	rowMutateRule := &models.StagedChange{
		ID:           uuid.New(),
		ResourceType: "flag_rule",
		Action:       "update",
		ResourceID:   &realRule,
		NewValue:     []byte(`{"flag_id":"` + provFlag.String() + `"}`),
	}
	plan, err := planBatch([]*models.StagedChange{rowMutateRule, rowCreateFlag})
	if err != nil {
		t.Fatalf("planBatch: %v", err)
	}
	if len(plan.ordered) != 2 {
		t.Fatalf("ordered len = %d, want 2", len(plan.ordered))
	}
	if plan.ordered[0].Action != "create" {
		t.Errorf("first row should be create, got %v", plan.ordered[0].Action)
	}
	if _, ok := plan.knownProvs[provFlag]; !ok {
		t.Errorf("knownProvs missing flag provisional")
	}
}

func TestPlanBatchRejectsUnresolvedProvisional(t *testing.T) {
	dangling := NewProvisional()
	row := &models.StagedChange{
		ID:           uuid.New(),
		ResourceType: "flag_rule",
		Action:       "update",
		ResourceID:   ptrUUID(uuid.New()),
		NewValue:     []byte(`{"flag_id":"` + dangling.String() + `"}`),
	}
	_, err := planBatch([]*models.StagedChange{row})
	var unresolved *ErrUnresolvedProvisional
	if !errors.As(err, &unresolved) {
		t.Fatalf("expected *ErrUnresolvedProvisional, got %v", err)
	}
	if unresolved.ProvUUID != dangling {
		t.Errorf("ProvUUID mismatch: got %v want %v", unresolved.ProvUUID, dangling)
	}
}

func TestPlanBatchPreservesOrderForIsolatedMutations(t *testing.T) {
	a := &models.StagedChange{ID: uuid.New(), ResourceType: "flag", Action: "toggle", ResourceID: ptrUUID(uuid.New())}
	b := &models.StagedChange{ID: uuid.New(), ResourceType: "flag", Action: "update", ResourceID: ptrUUID(uuid.New())}
	plan, err := planBatch([]*models.StagedChange{a, b})
	if err != nil {
		t.Fatalf("planBatch: %v", err)
	}
	if plan.ordered[0] != a || plan.ordered[1] != b {
		t.Fatalf("input order not preserved")
	}
}

func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }
