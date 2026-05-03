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

func TestOverlayListMarkedAttachesEnvelopeForCreatesAndUpdates(t *testing.T) {
	type flagDTO struct {
		ID     uuid.UUID `json:"id"`
		Staged *Marker   `json:"_staged,omitempty"`
		Key    string    `json:"key"`
	}

	prov := NewProvisional()
	realID := uuid.New()
	staged := []*models.StagedChange{
		{ID: uuid.New(), Action: "create", ProvisionalID: &prov, NewValue: []byte(`{"key":"new"}`)},
		{ID: uuid.New(), Action: "update", ResourceID: &realID, NewValue: []byte(`{"key":"updated"}`)},
	}
	production := []flagDTO{{ID: realID, Key: "original"}}

	got := OverlayListMarked(
		production,
		staged,
		func(f flagDTO) uuid.UUID { return f.ID },
		func(f flagDTO, s *models.StagedChange) (flagDTO, bool) {
			// Naive update: replace key from new_value
			var p struct{ Key string `json:"key"` }
			_ = json.Unmarshal(s.NewValue, &p)
			f.Key = p.Key
			return f, false
		},
		func(s *models.StagedChange) (flagDTO, bool) {
			var p struct{ Key string `json:"key"` }
			_ = json.Unmarshal(s.NewValue, &p)
			return flagDTO{ID: *s.ProvisionalID, Key: p.Key}, true
		},
		func(f *flagDTO, m Marker) { f.Staged = &m },
	)

	if len(got) != 2 {
		t.Fatalf("len: %d, want 2", len(got))
	}
	// First row: prod row with update applied + marker
	if got[0].ID != realID || got[0].Key != "updated" {
		t.Errorf("updated row: %+v", got[0])
	}
	if got[0].Staged == nil || got[0].Staged.Action != "update" {
		t.Errorf("update row missing _staged marker: %+v", got[0].Staged)
	}
	// Second row: synthetic create + marker with provisional id
	if got[1].ID != prov || got[1].Key != "new" {
		t.Errorf("synthetic row: %+v", got[1])
	}
	if got[1].Staged == nil || got[1].Staged.Action != "create" {
		t.Errorf("create row missing _staged marker: %+v", got[1].Staged)
	}
	if got[1].Staged.ProvisionalID == nil || *got[1].Staged.ProvisionalID != prov {
		t.Errorf("create marker should carry provisional id, got %+v", got[1].Staged)
	}
}

func TestOverlayListMarkedNoStagedReturnsProductionUnchanged(t *testing.T) {
	type flagDTO struct {
		ID     uuid.UUID `json:"id"`
		Staged *Marker   `json:"_staged,omitempty"`
	}
	production := []flagDTO{{ID: uuid.New()}, {ID: uuid.New()}}
	got := OverlayListMarked(
		production, nil,
		func(f flagDTO) uuid.UUID { return f.ID },
		nil, nil, nil,
	)
	if len(got) != 2 {
		t.Errorf("expected production unchanged, got len=%d", len(got))
	}
	if got[0].Staged != nil || got[1].Staged != nil {
		t.Error("no marker should be set when staged is empty")
	}
}
