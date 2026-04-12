package deploysentry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixtureDir() string {
	return filepath.Join("..", "testdata")
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir(), name))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

// TestContract_AuthHeaderPrefix verifies that the auth_request.json fixture
// declares the same header prefix ("ApiKey ") that the SDK uses.
func TestContract_AuthHeaderPrefix(t *testing.T) {
	data := loadFixture(t, "auth_request.json")

	var fixture struct {
		HeaderValuePrefix string `json:"header_value_prefix"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal auth fixture: %v", err)
	}

	const expected = "ApiKey "
	if fixture.HeaderValuePrefix != expected {
		t.Errorf("auth header prefix = %q; want %q", fixture.HeaderValuePrefix, expected)
	}
}

// TestContract_EvaluateResponse verifies that evaluate_response.json can be
// parsed into the SDK's evaluateResponse struct.
func TestContract_EvaluateResponse(t *testing.T) {
	data := loadFixture(t, "evaluate_response.json")

	// The fixture wraps the response in a "body" key.
	var wrapper struct {
		Body evaluateResponse `json:"body"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("unmarshal evaluate_response.json: %v", err)
	}

	resp := wrapper.Body
	if resp.FlagKey != "dark-mode" {
		t.Errorf("FlagKey = %q; want %q", resp.FlagKey, "dark-mode")
	}
	if resp.Reason != "TARGETING_MATCH" {
		t.Errorf("Reason = %q; want %q", resp.Reason, "TARGETING_MATCH")
	}
	if resp.FlagType != FlagTypeBoolean {
		t.Errorf("FlagType = %q; want %q", resp.FlagType, FlagTypeBoolean)
	}
	if !resp.Enabled {
		t.Error("Enabled = false; want true")
	}
	if resp.Metadata.Category != CategoryFeature {
		t.Errorf("Metadata.Category = %q; want %q", resp.Metadata.Category, CategoryFeature)
	}
	if len(resp.Metadata.Owners) != 1 || resp.Metadata.Owners[0] != "frontend-team" {
		t.Errorf("Metadata.Owners = %v; want [frontend-team]", resp.Metadata.Owners)
	}

	// Verify the raw value round-trips to a boolean true.
	var boolVal bool
	if err := json.Unmarshal(resp.Value, &boolVal); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if !boolVal {
		t.Error("Value = false; want true")
	}
}

// TestContract_ListFlagsResponse verifies that list_flags_response.json can
// be parsed into the SDK's listFlagsResponse struct.
func TestContract_ListFlagsResponse(t *testing.T) {
	data := loadFixture(t, "list_flags_response.json")

	var wrapper struct {
		Body listFlagsResponse `json:"body"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("unmarshal list_flags_response.json: %v", err)
	}

	flags := wrapper.Body.Flags
	if len(flags) != 3 {
		t.Fatalf("got %d flags; want 3", len(flags))
	}

	// Spot-check each flag key and type.
	expected := []struct {
		key      string
		flagType FlagType
	}{
		{"dark-mode", FlagTypeBoolean},
		{"new-checkout", FlagTypeString},
		{"max-items", FlagTypeInt},
	}

	for i, want := range expected {
		got := flags[i]
		if got.Key != want.key {
			t.Errorf("flags[%d].Key = %q; want %q", i, got.Key, want.key)
		}
		if got.FlagType != want.flagType {
			t.Errorf("flags[%d].FlagType = %q; want %q", i, got.FlagType, want.flagType)
		}
	}
}

// TestContract_BatchEvaluateResponse verifies that
// batch_evaluate_response.json can be parsed into the SDK's
// batchEvaluateResponse struct.
func TestContract_BatchEvaluateResponse(t *testing.T) {
	data := loadFixture(t, "batch_evaluate_response.json")

	var wrapper struct {
		Body batchEvaluateResponse `json:"body"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("unmarshal batch_evaluate_response.json: %v", err)
	}

	results := wrapper.Body.Results
	if len(results) != 3 {
		t.Fatalf("got %d results; want 3", len(results))
	}

	// Verify each result's flag_key and reason.
	expectations := []struct {
		flagKey string
		reason  string
	}{
		{"dark-mode", "TARGETING_MATCH"},
		{"new-checkout", "PERCENTAGE_ROLLOUT"},
		{"max-items", "DEFAULT_VALUE"},
	}

	for i, want := range expectations {
		got := results[i]
		if got.FlagKey != want.flagKey {
			t.Errorf("results[%d].FlagKey = %q; want %q", i, got.FlagKey, want.flagKey)
		}
		if got.Reason != want.reason {
			t.Errorf("results[%d].Reason = %q; want %q", i, got.Reason, want.reason)
		}
	}
}
