package staging

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

func TestResolverBindRejectsNonProvisionalSource(t *testing.T) {
	r := NewResolver()
	defer func() {
		if recover() == nil {
			t.Fatal("Bind should panic when source is not provisional")
		}
	}()
	r.Bind(uuid.New(), uuid.New())
}

func TestResolverBindRejectsProvisionalDestination(t *testing.T) {
	r := NewResolver()
	defer func() {
		if recover() == nil {
			t.Fatal("Bind should panic when destination is provisional")
		}
	}()
	r.Bind(NewProvisional(), NewProvisional())
}

func TestResolverLookupRoundTrip(t *testing.T) {
	r := NewResolver()
	prov := NewProvisional()
	real := uuid.New()
	r.Bind(prov, real)

	got, ok := r.Lookup(prov)
	if !ok || got != real {
		t.Fatalf("Lookup(prov) = (%v,%v), want (%v,true)", got, ok, real)
	}
	if _, ok := r.Lookup(uuid.New()); ok {
		t.Fatal("Lookup of unknown UUID should return ok=false")
	}
}

func TestRewriteUUIDsInJSONSubstitutesNested(t *testing.T) {
	r := NewResolver()
	prov := NewProvisional()
	real := uuid.New()
	r.Bind(prov, real)

	in := []byte(`{"flag_id":"` + prov.String() + `","nested":{"id":"` + prov.String() + `"},"arr":["` + prov.String() + `","not-a-uuid"]}`)
	out, err := r.RewriteUUIDsInJSON(in)
	if err != nil {
		t.Fatalf("RewriteUUIDsInJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got["flag_id"] != real.String() {
		t.Errorf("top-level not rewritten: %v", got["flag_id"])
	}
	if got["nested"].(map[string]any)["id"] != real.String() {
		t.Errorf("nested not rewritten: %v", got["nested"])
	}
	arr := got["arr"].([]any)
	if arr[0] != real.String() || arr[1] != "not-a-uuid" {
		t.Errorf("array rewrite mismatch: %v", arr)
	}
}

func TestRewriteUUIDsInJSONLeavesUnknownUUIDsAlone(t *testing.T) {
	r := NewResolver()
	r.Bind(NewProvisional(), uuid.New()) // bind something else
	other := uuid.New()
	in := []byte(`{"flag_id":"` + other.String() + `"}`)
	out, err := r.RewriteUUIDsInJSON(in)
	if err != nil {
		t.Fatalf("RewriteUUIDsInJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got["flag_id"] != other.String() {
		t.Errorf("unknown UUID was rewritten: %v", got["flag_id"])
	}
}

func TestRewriteRowResolvesResourceIDAndJSON(t *testing.T) {
	r := NewResolver()
	prov := NewProvisional()
	real := uuid.New()
	r.Bind(prov, real)

	row := &models.StagedChange{
		ResourceID: &prov,
		NewValue:   []byte(`{"flag_id":"` + prov.String() + `"}`),
	}
	if err := r.RewriteRow(row); err != nil {
		t.Fatalf("RewriteRow: %v", err)
	}
	if row.ResourceID == nil || *row.ResourceID != real {
		t.Errorf("ResourceID not rewritten: %v", row.ResourceID)
	}
	var got map[string]any
	if err := json.Unmarshal(row.NewValue, &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if got["flag_id"] != real.String() {
		t.Errorf("NewValue not rewritten: %v", got)
	}
}
