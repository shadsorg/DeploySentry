package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Supported provider identifiers for deploy-event ingestion.
const (
	DeployProviderGeneric       = "generic"
	DeployProviderRailway       = "railway"
	DeployProviderRender        = "render"
	DeployProviderFly           = "fly"
	DeployProviderHeroku        = "heroku"
	DeployProviderVercel        = "vercel"
	DeployProviderNetlify       = "netlify"
	DeployProviderGitHubActions = "github-actions"
)

// DeployIntegrationAuth is the auth mode applied to inbound webhook requests.
const (
	DeployIntegrationAuthHMAC   = "hmac"
	DeployIntegrationAuthBearer = "bearer"
)

// Canonical event types emitted by adapters into the shared ingestion path.
const (
	DeployEventSucceeded = "deploy.succeeded"
	DeployEventFailed    = "deploy.failed"
	DeployEventCrashed   = "deploy.crashed"
	DeployEventStarted   = "deploy.started"
)

// DeployIntegration is the per-application configuration for a provider
// webhook (or the generic canonical endpoint).
type DeployIntegration struct {
	ID                 uuid.UUID         `json:"id" db:"id"`
	ApplicationID      uuid.UUID         `json:"application_id" db:"application_id"`
	Provider           string            `json:"provider" db:"provider"`
	AuthMode           string            `json:"auth_mode" db:"auth_mode"`
	WebhookSecret      string            `json:"-" db:"-"` // only ever populated in-memory
	WebhookSecretEnc   []byte            `json:"-" db:"webhook_secret_enc"`
	ProviderConfig     map[string]any    `json:"provider_config" db:"provider_config"`
	EnvMapping         map[string]uuid.UUID `json:"env_mapping" db:"env_mapping"`
	VersionExtractors  []string          `json:"version_extractors" db:"version_extractors"`
	Enabled            bool              `json:"enabled" db:"enabled"`
	CreatedAt          time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at" db:"updated_at"`
}

// DeployIntegrationEvent is an audit + idempotency record for every inbound
// webhook delivery.
type DeployIntegrationEvent struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	IntegrationID uuid.UUID  `json:"integration_id" db:"integration_id"`
	EventType     string     `json:"event_type" db:"event_type"`
	DedupKey      string     `json:"dedup_key" db:"dedup_key"`
	DeploymentID  *uuid.UUID `json:"deployment_id,omitempty" db:"deployment_id"`
	PayloadJSON   []byte     `json:"payload_json" db:"payload_json"`
	ReceivedAt    time.Time  `json:"received_at" db:"received_at"`
}

// DeployEvent is the canonical in-memory shape produced by every adapter
// before the shared ingestion path takes over. Adapters normalize provider
// payloads into this struct.
type DeployEvent struct {
	EventType   string          `json:"event_type"`
	Environment string          `json:"environment"`
	Version     string          `json:"version"`
	CommitSHA   string          `json:"commit_sha,omitempty"`
	Artifact    string          `json:"artifact,omitempty"`
	OccurredAt  time.Time       `json:"occurred_at,omitempty"`
	URL         string          `json:"url,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// DeployEventDedupKey produces the sha256 hex key used for webhook
// idempotency. Deliberately stable across time — the same (app, env,
// version, event_type) tuple always maps to the same key.
func DeployEventDedupKey(appID, envID uuid.UUID, version, eventType string) string {
	sum := sha256.Sum256([]byte(appID.String() + "|" + envID.String() + "|" + version + "|" + eventType))
	return hex.EncodeToString(sum[:])
}
