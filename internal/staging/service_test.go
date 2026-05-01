package staging

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

func TestStagedNewValue_AnnotatesObjectInPlace(t *testing.T) {
	stagedAt := time.Date(2026, 5, 1, 10, 30, 0, 0, time.UTC)
	got := stagedNewValue([]byte(`{"enabled":true}`), stagedAt)
	if !strings.Contains(got, `"_staged_at":"2026-05-01T10:30:00Z"`) {
		t.Fatalf("expected _staged_at in object, got %s", got)
	}
	if !strings.Contains(got, `"enabled":true`) {
		t.Fatalf("expected original payload preserved, got %s", got)
	}
	// Output must be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, got)
	}
}

func TestStagedNewValue_HandlesEmptyObject(t *testing.T) {
	got := stagedNewValue([]byte(`{}`), time.Now())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("empty-object splice produced invalid JSON: %v\n%s", err, got)
	}
}

func TestStagedNewValue_WrapsNonObjectPayload(t *testing.T) {
	got := stagedNewValue([]byte(`"control"`), time.Now())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("non-object wrap produced invalid JSON: %v\n%s", err, got)
	}
	if parsed["value"] != "control" {
		t.Fatalf("expected wrapped value=control, got %v", parsed["value"])
	}
}

func TestStagedNewValue_HandlesEmptyInput(t *testing.T) {
	got := stagedNewValue(nil, time.Now())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("nil input produced invalid JSON: %v\n%s", err, got)
	}
	if _, ok := parsed["_staged_at"]; !ok {
		t.Fatal("expected _staged_at key for nil input")
	}
}

func TestBuildAuditEntry_PassthroughResourceID(t *testing.T) {
	rid := uuid.New()
	row := &models.StagedChange{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		ResourceType: "flag",
		ResourceID:   &rid,
		Action:       "toggle",
		NewValue:     json.RawMessage(`{"enabled":true}`),
		CreatedAt:    time.Now(),
	}
	entry := buildAuditEntry(row, uuid.New(), "flag.toggled")
	if entry.EntityID != rid {
		t.Fatalf("expected EntityID=%s, got %s", rid, entry.EntityID)
	}
	if entry.EntityType != "flag" {
		t.Fatalf("expected EntityType=flag, got %s", entry.EntityType)
	}
}
