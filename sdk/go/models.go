package deploysentry

import (
	"encoding/json"
	"time"
)

// FlagCategory represents the classification of a feature flag.
type FlagCategory string

const (
	CategoryRelease    FlagCategory = "release"
	CategoryFeature    FlagCategory = "feature"
	CategoryExperiment FlagCategory = "experiment"
	CategoryOps        FlagCategory = "ops"
	CategoryPermission FlagCategory = "permission"
)

// FlagType represents the data type of a flag's value.
type FlagType string

const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeString  FlagType = "string"
	FlagTypeInt     FlagType = "integer"
	FlagTypeJSON    FlagType = "json"
)

// FlagMetadata contains rich descriptive information about a flag.
type FlagMetadata struct {
	Category    FlagCategory `json:"category"`
	Purpose     string       `json:"purpose"`
	Owners      []string     `json:"owners"`
	IsPermanent bool         `json:"is_permanent"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Tags        []string     `json:"tags"`
}

// Flag represents a feature flag as returned by the DeploySentry API.
type Flag struct {
	ID           string          `json:"id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Category     FlagCategory    `json:"category"`
	Purpose      string          `json:"purpose"`
	Owners       []string        `json:"owners"`
	IsPermanent  bool            `json:"is_permanent"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	FlagType     FlagType        `json:"flag_type"`
	Enabled      bool            `json:"enabled"`
	DefaultValue string          `json:"default_value"`
	Tags         []string        `json:"tags"`
	Metadata     FlagMetadata    `json:"metadata"`
	RawValue     json.RawMessage `json:"-"`
}

// EvaluationResult contains the full result of evaluating a flag, including
// the resolved value, the reason for the resolution, and flag metadata.
type EvaluationResult struct {
	// FlagKey is the key of the evaluated flag.
	FlagKey string `json:"flag_key"`

	// Value is the resolved value for this evaluation.
	Value interface{} `json:"value"`

	// Reason describes why this value was returned (e.g., "TARGETING_MATCH",
	// "DEFAULT", "ERROR", "CACHED").
	Reason string `json:"reason"`

	// Metadata contains the flag's rich metadata.
	Metadata FlagMetadata `json:"metadata"`

	// FlagType is the declared type of the flag.
	FlagType FlagType `json:"flag_type"`

	// Enabled indicates whether the flag is globally enabled.
	Enabled bool `json:"enabled"`
}

// evaluateRequest is the JSON body sent to the evaluation endpoint.
type evaluateRequest struct {
	FlagKey     string             `json:"flag_key"`
	Context     *EvaluationContext `json:"context,omitempty"`
	Environment string             `json:"environment,omitempty"`
	ProjectID   string             `json:"project_id,omitempty"`
	SessionID   string             `json:"session_id,omitempty"`
}

// evaluateResponse is the JSON body returned from the evaluation endpoint.
type evaluateResponse struct {
	FlagKey  string          `json:"flag_key"`
	Value    json.RawMessage `json:"value"`
	Reason   string          `json:"reason"`
	FlagType FlagType        `json:"flag_type"`
	Enabled  bool            `json:"enabled"`
	Metadata FlagMetadata    `json:"metadata"`
}

// batchEvaluateRequest is the JSON body sent to the batch evaluation endpoint.
type batchEvaluateRequest struct {
	FlagKeys    []string           `json:"flag_keys"`
	Context     *EvaluationContext `json:"context,omitempty"`
	Environment string             `json:"environment,omitempty"`
	ProjectID   string             `json:"project_id,omitempty"`
	SessionID   string             `json:"session_id,omitempty"`
}

// batchEvaluateResponse is the JSON body returned from the batch evaluation endpoint.
type batchEvaluateResponse struct {
	Results []evaluateResponse `json:"results"`
}

// listFlagsResponse is the JSON body returned from the list-flags endpoint.
type listFlagsResponse struct {
	Flags []Flag `json:"flags"`
}
