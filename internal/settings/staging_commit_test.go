package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// stubSettingSvc implements just enough of SettingService to exercise the
// commit handlers. Unimplemented methods panic via the embedded interface.
type stubSettingSvc struct {
	SettingService

	setCalled    func(*models.Setting) error
	deleteCalled func(uuid.UUID) error
}

func (s *stubSettingSvc) Set(_ context.Context, setting *models.Setting) error {
	if s.setCalled == nil {
		return nil
	}
	return s.setCalled(setting)
}

func (s *stubSettingSvc) Delete(_ context.Context, id uuid.UUID) error {
	if s.deleteCalled == nil {
		return nil
	}
	return s.deleteCalled(id)
}

func ridPtr(id uuid.UUID) *uuid.UUID { return &id }

func TestSettingCommitHandlers_Tuples(t *testing.T) {
	svc := &stubSettingSvc{}
	tuples := SettingCommitHandlers(svc)
	got := map[string]bool{}
	for _, tup := range tuples {
		if tup.ResourceType != "setting" {
			t.Fatalf("unexpected resource_type %s", tup.ResourceType)
		}
		if tup.Handler == nil {
			t.Fatalf("nil handler for action %s", tup.Action)
		}
		got[tup.Action] = true
	}
	for _, want := range []string{"update", "delete"} {
		if !got[want] {
			t.Fatalf("missing tuple for action %q", want)
		}
	}
	if len(tuples) != 2 {
		t.Fatalf("expected 2 tuples, got %d", len(tuples))
	}
}

func TestCommitSettingUpdate_OverridesIDFromResourceID(t *testing.T) {
	intended := uuid.New()
	bodyHadDifferent := uuid.New()
	var got *models.Setting
	svc := &stubSettingSvc{
		setCalled: func(s *models.Setting) error {
			got = s
			return nil
		},
	}
	body, _ := json.Marshal(&models.Setting{ID: bodyHadDifferent, Key: "k", Value: json.RawMessage(`"v"`)})
	row := &models.StagedChange{
		ResourceType: "setting", Action: "update",
		ResourceID: ridPtr(intended), NewValue: body,
	}
	action, err := commitSettingUpdate(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "setting.updated" {
		t.Fatalf("expected setting.updated, got %s", action)
	}
	if got == nil || got.ID != intended {
		t.Fatalf("update should have used row.ResourceID, got %+v", got)
	}
}

func TestCommitSettingUpdate_RequiresResourceIDAndNewValue(t *testing.T) {
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
			_, err := commitSettingUpdate(&stubSettingSvc{})(context.Background(), nil, c.row)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected %q error, got %v", c.want, err)
			}
		})
	}
}

func TestCommitSettingUpdate_PropagatesServiceError(t *testing.T) {
	boom := errors.New("validation: scope required")
	svc := &stubSettingSvc{setCalled: func(*models.Setting) error { return boom }}
	body, _ := json.Marshal(&models.Setting{Key: "k", Value: json.RawMessage(`"v"`)})
	row := &models.StagedChange{
		ResourceType: "setting", Action: "update",
		ResourceID: ridPtr(uuid.New()), NewValue: body,
	}
	_, err := commitSettingUpdate(svc)(context.Background(), nil, row)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped service error, got %v", err)
	}
}

func TestCommitSettingDelete_CallsService(t *testing.T) {
	id := uuid.New()
	called := false
	svc := &stubSettingSvc{
		deleteCalled: func(got uuid.UUID) error {
			called = true
			if got != id {
				t.Fatalf("expected id=%s, got %s", id, got)
			}
			return nil
		},
	}
	row := &models.StagedChange{ResourceType: "setting", Action: "delete", ResourceID: ridPtr(id)}
	action, err := commitSettingDelete(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "setting.deleted" {
		t.Fatalf("expected setting.deleted, got %s", action)
	}
	if !called {
		t.Fatal("Delete was not invoked")
	}
}

func TestCommitSettingDelete_RequiresResourceID(t *testing.T) {
	_, err := commitSettingDelete(&stubSettingSvc{})(context.Background(), nil, &models.StagedChange{Action: "delete"})
	if err == nil || !strings.Contains(err.Error(), "resource_id required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}
