package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentStatus represents the connection state of a sidecar agent.
type AgentStatus string

const (
	// AgentStatusConnected indicates the agent is actively reporting.
	AgentStatusConnected AgentStatus = "connected"
	// AgentStatusStale indicates the agent has not reported recently.
	AgentStatusStale AgentStatus = "stale"
	// AgentStatusDisconnected indicates the agent is no longer reporting.
	AgentStatusDisconnected AgentStatus = "disconnected"
)

// Agent represents a registered sidecar agent.
type Agent struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	AppID          uuid.UUID       `json:"app_id" db:"app_id"`
	EnvironmentID  uuid.UUID       `json:"environment_id" db:"environment_id"`
	Status         AgentStatus     `json:"status" db:"status"`
	Version        string          `json:"version" db:"version"`
	UpstreamConfig json.RawMessage `json:"upstream_config" db:"upstream_config"`
	LastSeenAt     time.Time       `json:"last_seen_at" db:"last_seen_at"`
	RegisteredAt   time.Time       `json:"registered_at" db:"registered_at"`
}

// AgentHeartbeat represents a periodic health/traffic report from a sidecar agent.
type AgentHeartbeat struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	AgentID      uuid.UUID       `json:"agent_id" db:"agent_id"`
	DeploymentID *uuid.UUID      `json:"deployment_id,omitempty" db:"deployment_id"`
	Payload      json.RawMessage `json:"payload" db:"payload"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

// HeartbeatPayload is the structured content of an agent heartbeat report.
type HeartbeatPayload struct {
	AgentID        uuid.UUID              `json:"agent_id"`
	DeploymentID   *uuid.UUID             `json:"deployment_id,omitempty"`
	ConfigVersion  int                    `json:"config_version"`
	ActualTraffic  map[string]float64     `json:"actual_traffic"`
	Upstreams      map[string]UpstreamMetrics `json:"upstreams"`
	ActiveRules    ActiveRules            `json:"active_rules"`
	EnvoyHealthy   bool                   `json:"envoy_healthy"`
}

// UpstreamMetrics contains traffic metrics for a single upstream.
type UpstreamMetrics struct {
	RPS       float64 `json:"rps"`
	ErrorRate float64 `json:"error_rate"`
	P99Ms     float64 `json:"p99_ms"`
	P50Ms     float64 `json:"p50_ms"`
}

// ActiveRules describes the traffic rules currently applied by the agent.
type ActiveRules struct {
	Weights         map[string]int  `json:"weights"`
	HeaderOverrides []HeaderOverride `json:"header_overrides"`
	StickySessions  *StickyConfig   `json:"sticky_sessions,omitempty"`
}

// HeaderOverride routes traffic to a specific upstream when a header matches.
type HeaderOverride struct {
	Header   string `json:"header"`
	Value    string `json:"value"`
	Upstream string `json:"upstream"`
}

// StickyConfig configures session affinity for traffic routing.
type StickyConfig struct {
	Enabled  bool   `json:"enabled"`
	Strategy string `json:"strategy"`
	TTL      string `json:"ttl"`
}
