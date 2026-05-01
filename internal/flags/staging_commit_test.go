package flags

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// stubCommitSvc embeds FlagService so unused methods panic — keeps the test
// double tight. Only the methods exercised by the commit handlers are
// implemented.
type stubCommitSvc struct {
	FlagService

	toggleCalled  func(uuid.UUID, bool) error
	updateCalled  func(*models.FeatureFlag) error
	archiveCalled func(uuid.UUID) error
	restoreCalled func(uuid.UUID) error
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

func ridPtr(id uuid.UUID) *uuid.UUID { return &id }

// ---- Registration shape ----

func TestFlagCommitHandlers_RegistersExpectedTuples(t *testing.T) {
	svc := &stubCommitSvc{}
	tuples := FlagCommitHandlers(svc)
	wantActions := map[string]bool{
		"toggle":  true,
		"update":  true,
		"archive": true,
		"restore": true,
	}
	if len(tuples) != len(wantActions) {
		t.Fatalf("expected %d tuples, got %d", len(wantActions), len(tuples))
	}
	for _, tup := range tuples {
		if tup.ResourceType != "flag" {
			t.Fatalf("tuple has wrong resource_type: %+v", tup)
		}
		if !wantActions[tup.Action] {
			t.Fatalf("unexpected action %q", tup.Action)
		}
		if tup.Handler == nil {
			t.Fatalf("handler for action %q is nil", tup.Action)
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
