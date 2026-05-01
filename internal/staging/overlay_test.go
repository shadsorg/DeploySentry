package staging

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

type testRow struct {
	ID    uuid.UUID
	Name  string
	Color string
}

func rowID(r testRow) uuid.UUID { return r.ID }

// patcher applies field_path-keyed updates ("name", "color") plus deletes.
func patcher(prod testRow, s *models.StagedChange) (testRow, bool) {
	if s.Action == "delete" {
		return testRow{}, true
	}
	switch s.FieldPath {
	case "name":
		var v string
		_ = json.Unmarshal(s.NewValue, &v)
		prod.Name = v
	case "color":
		var v string
		_ = json.Unmarshal(s.NewValue, &v)
		prod.Color = v
	}
	return prod, false
}

func synth(s *models.StagedChange) (testRow, bool) {
	if s.ProvisionalID == nil {
		return testRow{}, false
	}
	var payload struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	_ = json.Unmarshal(s.NewValue, &payload)
	return testRow{ID: *s.ProvisionalID, Name: payload.Name, Color: payload.Color}, true
}

func ridPtr(t *testing.T, id uuid.UUID) *uuid.UUID { t.Helper(); return &id }

func TestOverlayList_NoStagedReturnsProduction(t *testing.T) {
	prod := []testRow{{ID: uuid.New(), Name: "a"}}
	got := OverlayList(prod, nil, rowID, patcher, synth)
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("expected production passthrough, got %+v", got)
	}
}

func TestOverlayList_PatchesUpdate(t *testing.T) {
	id := uuid.New()
	prod := []testRow{{ID: id, Name: "old", Color: "red"}}
	staged := []*models.StagedChange{
		{ResourceID: ridPtr(t, id), Action: "update", FieldPath: "name", NewValue: json.RawMessage(`"new"`)},
		{ResourceID: ridPtr(t, id), Action: "update", FieldPath: "color", NewValue: json.RawMessage(`"blue"`)},
	}
	got := OverlayList(prod, staged, rowID, patcher, synth)
	if len(got) != 1 || got[0].Name != "new" || got[0].Color != "blue" {
		t.Fatalf("expected both fields patched, got %+v", got)
	}
}

func TestOverlayList_DropsDelete(t *testing.T) {
	keepID, dropID := uuid.New(), uuid.New()
	prod := []testRow{{ID: keepID, Name: "keep"}, {ID: dropID, Name: "drop"}}
	staged := []*models.StagedChange{
		{ResourceID: ridPtr(t, dropID), Action: "delete"},
	}
	got := OverlayList(prod, staged, rowID, patcher, synth)
	if len(got) != 1 || got[0].ID != keepID {
		t.Fatalf("expected only keepID to survive, got %+v", got)
	}
}

func TestOverlayList_AppendsSyntheticCreate(t *testing.T) {
	id1 := uuid.New()
	prod := []testRow{{ID: id1, Name: "a"}}
	prov := NewProvisional()
	staged := []*models.StagedChange{
		{ProvisionalID: &prov, Action: "create", NewValue: json.RawMessage(`{"name":"new","color":"green"}`)},
	}
	got := OverlayList(prod, staged, rowID, patcher, synth)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows (production + synthetic), got %d: %+v", len(got), got)
	}
	if got[1].ID != prov || got[1].Name != "new" || got[1].Color != "green" {
		t.Fatalf("synthetic row malformed: %+v", got[1])
	}
}

func TestOverlayList_MultiActionInOneCall(t *testing.T) {
	patchID := uuid.New()
	dropID := uuid.New()
	keepID := uuid.New()
	prov := NewProvisional()
	prod := []testRow{
		{ID: keepID, Name: "keep"},
		{ID: patchID, Name: "old"},
		{ID: dropID, Name: "doomed"},
	}
	staged := []*models.StagedChange{
		{ResourceID: ridPtr(t, patchID), Action: "update", FieldPath: "name", NewValue: json.RawMessage(`"new"`)},
		{ResourceID: ridPtr(t, dropID), Action: "delete"},
		{ProvisionalID: &prov, Action: "create", NewValue: json.RawMessage(`{"name":"created"}`)},
	}
	got := OverlayList(prod, staged, rowID, patcher, synth)
	if len(got) != 3 {
		t.Fatalf("expected keep+patched+synthetic, got %+v", got)
	}
	if got[0].ID != keepID || got[1].Name != "new" || got[2].ID != prov {
		t.Fatalf("ordering or content wrong: %+v", got)
	}
}

func TestOverlayDetail_Patches(t *testing.T) {
	id := uuid.New()
	prod := testRow{ID: id, Name: "old", Color: "red"}
	staged := []*models.StagedChange{
		{ResourceID: ridPtr(t, id), Action: "update", FieldPath: "color", NewValue: json.RawMessage(`"blue"`)},
		// Unrelated row for a different resource — should be ignored.
		{ResourceID: ridPtr(t, uuid.New()), Action: "update", FieldPath: "color", NewValue: json.RawMessage(`"yellow"`)},
	}
	got, dropped := OverlayDetail(prod, staged, rowID, patcher)
	if dropped {
		t.Fatal("expected dropped=false")
	}
	if got.Color != "blue" {
		t.Fatalf("expected color patched, got %+v", got)
	}
}

func TestOverlayDetail_DeleteSignalsDropped(t *testing.T) {
	id := uuid.New()
	prod := testRow{ID: id, Name: "x"}
	staged := []*models.StagedChange{{ResourceID: ridPtr(t, id), Action: "delete"}}
	_, dropped := OverlayDetail(prod, staged, rowID, patcher)
	if !dropped {
		t.Fatal("expected dropped=true")
	}
}
