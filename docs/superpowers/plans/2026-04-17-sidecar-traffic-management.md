# Sidecar Traffic Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a DeploySentry Agent sidecar that bridges the deployment engine's desired state to actual traffic routing via Envoy xDS, with real-time observability on the dashboard.

**Architecture:** The agent is a Go binary that receives desired state via SSE, implements an Envoy xDS control plane (CDS/EDS/RDS via `go-control-plane`), and reports actual traffic metrics back via heartbeats. The API gets agent registry endpoints and heartbeat storage. The dashboard gets a traffic panel showing desired vs. actual state. SDKs auto-detect `SERVICE_COLOR` for flag canaries.

**Tech Stack:** Go 1.25, `go-control-plane` (xDS), Envoy v1.31, PostgreSQL (pgx), Gin, React + TypeScript, Docker Compose

---

## File Structure

### New Files

```
cmd/agent/main.go                          — Agent binary entrypoint
internal/agent/config.go                   — Agent configuration (env vars)
internal/agent/xds/server.go               — xDS gRPC server + SnapshotCache
internal/agent/xds/snapshot.go             — Envoy snapshot builder (clusters, routes, listeners)
internal/agent/xds/server_test.go          — xDS server unit tests
internal/agent/sse/client.go               — SSE client for desired-state stream
internal/agent/sse/client_test.go          — SSE client unit tests
internal/agent/reporter/reporter.go        — Heartbeat reporter + Envoy stats collector
internal/agent/reporter/reporter_test.go   — Reporter unit tests
internal/agent/registry/repository.go      — Agent registry repository interface
internal/agent/registry/service.go         — Agent registry service
internal/agent/registry/handler.go         — Agent registry HTTP handler
internal/agent/registry/handler_test.go    — Handler tests
internal/platform/database/postgres/agents.go — Postgres agent repository
migrations/043_create_agents.up.sql        — Agents + heartbeats tables
migrations/043_create_agents.down.sql      — Drop agents + heartbeats tables
deploy/docker/docker-compose.deploy.yml    — Multi-instance compose with Envoy + agent
deploy/docker/envoy-bootstrap.yaml         — Envoy bootstrap pointing to agent xDS
deploy/docker/Dockerfile.agent             — Agent Docker image
```

### Modified Files

```
cmd/api/main.go                            — Wire agent registry handler
web/src/api.ts                             — Add agent API functions
web/src/types.ts                           — Add Agent, AgentHeartbeat types
web/src/pages/DeploymentDetailPage.tsx     — Add traffic panel, agent status, traffic rules
sdk/go/client.go                           — Auto-detect SERVICE_COLOR env var
Makefile                                   — Add dev-deploy target
```

---

### Task 1: Database Migration — Agents and Heartbeats Tables

**Files:**
- Create: `migrations/043_create_agents.up.sql`
- Create: `migrations/043_create_agents.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- 043_create_agents.up.sql
CREATE TABLE agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'connected' CHECK (status IN ('connected', 'stale', 'disconnected')),
    version         TEXT NOT NULL DEFAULT '',
    upstream_config JSONB NOT NULL DEFAULT '{}',
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_app_id ON agents(app_id);
CREATE INDEX idx_agents_status ON agents(status);

CREATE TABLE agent_heartbeats (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    deployment_id   UUID REFERENCES deployments(id) ON DELETE SET NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_heartbeats_agent_id ON agent_heartbeats(agent_id);
CREATE INDEX idx_agent_heartbeats_agent_deployment ON agent_heartbeats(agent_id, deployment_id);
```

- [ ] **Step 2: Write the down migration**

```sql
-- 043_create_agents.down.sql
DROP TABLE IF EXISTS agent_heartbeats;
DROP TABLE IF EXISTS agents;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration 043 applies successfully.

- [ ] **Step 4: Verify tables exist**

Run: `psql "postgres://deploysentry:deploysentry@localhost:5432/deploysentry?search_path=deploy" -c "\dt agents; \dt agent_heartbeats;"`
Expected: Both tables listed.

- [ ] **Step 5: Commit**

```bash
git add migrations/043_create_agents.up.sql migrations/043_create_agents.down.sql
git commit -m "feat: add agents and agent_heartbeats tables (migration 043)"
```

---

### Task 2: Agent Models

**Files:**
- Create: `internal/models/agent.go`

- [ ] **Step 1: Write agent model**

```go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentStatus represents the connection state of a DeploySentry agent.
type AgentStatus string

const (
	AgentStatusConnected    AgentStatus = "connected"
	AgentStatusStale        AgentStatus = "stale"
	AgentStatusDisconnected AgentStatus = "disconnected"
)

// Agent represents a registered DeploySentry sidecar agent.
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

// AgentHeartbeat represents a single heartbeat from an agent.
type AgentHeartbeat struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	AgentID      uuid.UUID       `json:"agent_id" db:"agent_id"`
	DeploymentID *uuid.UUID      `json:"deployment_id,omitempty" db:"deployment_id"`
	Payload      json.RawMessage `json:"payload" db:"payload"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

// HeartbeatPayload is the structured content of an agent heartbeat.
type HeartbeatPayload struct {
	AgentID        uuid.UUID                    `json:"agent_id"`
	DeploymentID   *uuid.UUID                   `json:"deployment_id,omitempty"`
	ConfigVersion  int                          `json:"config_version"`
	ActualTraffic  map[string]float64           `json:"actual_traffic"`
	Upstreams      map[string]UpstreamMetrics   `json:"upstreams"`
	ActiveRules    ActiveRules                  `json:"active_rules"`
	EnvoyHealthy   bool                         `json:"envoy_healthy"`
}

// UpstreamMetrics contains per-upstream traffic metrics.
type UpstreamMetrics struct {
	RPS       float64 `json:"rps"`
	ErrorRate float64 `json:"error_rate"`
	P99Ms     float64 `json:"p99_ms"`
	P50Ms     float64 `json:"p50_ms"`
}

// ActiveRules describes the current routing rules applied in Envoy.
type ActiveRules struct {
	Weights         map[string]int    `json:"weights"`
	HeaderOverrides []HeaderOverride  `json:"header_overrides,omitempty"`
	StickySessions  *StickyConfig     `json:"sticky_sessions,omitempty"`
}

// HeaderOverride routes requests matching a specific header to an upstream.
type HeaderOverride struct {
	Header   string `json:"header"`
	Value    string `json:"value"`
	Upstream string `json:"upstream"`
}

// StickyConfig defines sticky session routing behavior.
type StickyConfig struct {
	Enabled  bool   `json:"enabled"`
	Strategy string `json:"strategy,omitempty"` // "cookie" or "header"
	TTL      string `json:"ttl,omitempty"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/models/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/agent.go
git commit -m "feat: add Agent and AgentHeartbeat models"
```

---

### Task 3: Agent Postgres Repository

**Files:**
- Create: `internal/agent/registry/repository.go`
- Create: `internal/platform/database/postgres/agents.go`

- [ ] **Step 1: Write the repository interface**

```go
package registry

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Repository persists agent registrations and heartbeats.
type Repository interface {
	CreateAgent(ctx context.Context, a *models.Agent) error
	GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	ListAgentsByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error)
	UpdateAgentStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error
	UpdateLastSeen(ctx context.Context, id uuid.UUID) error
	DeleteAgent(ctx context.Context, id uuid.UUID) error
	MarkStaleAgents(ctx context.Context, staleDuration, disconnectDuration int) error

	InsertHeartbeat(ctx context.Context, hb *models.AgentHeartbeat) error
	ListHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error)
	PruneHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, keep int) error
}
```

- [ ] **Step 2: Write the Postgres implementation**

```go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/deploysentry/deploysentry/internal/agent/registry"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRepository implements registry.Repository using PostgreSQL.
type AgentRepository struct {
	pool *pgxpool.Pool
}

// NewAgentRepository creates a new AgentRepository backed by the given pool.
func NewAgentRepository(pool *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{pool: pool}
}

func (r *AgentRepository) CreateAgent(ctx context.Context, a *models.Agent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agents (id, app_id, environment_id, status, version, upstream_config, last_seen_at, registered_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.AppID, a.EnvironmentID, a.Status, a.Version, a.UpstreamConfig, a.LastSeenAt, a.RegisteredAt,
	)
	return err
}

func (r *AgentRepository) GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	var a models.Agent
	err := r.pool.QueryRow(ctx,
		`SELECT id, app_id, environment_id, status, version, upstream_config, last_seen_at, registered_at
		 FROM agents WHERE id = $1`, id,
	).Scan(&a.ID, &a.AppID, &a.EnvironmentID, &a.Status, &a.Version, &a.UpstreamConfig, &a.LastSeenAt, &a.RegisteredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (r *AgentRepository) ListAgentsByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, app_id, environment_id, status, version, upstream_config, last_seen_at, registered_at
		 FROM agents WHERE app_id = $1 ORDER BY registered_at DESC`, appID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.AppID, &a.EnvironmentID, &a.Status, &a.Version, &a.UpstreamConfig, &a.LastSeenAt, &a.RegisteredAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (r *AgentRepository) UpdateAgentStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE agents SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *AgentRepository) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE agents SET last_seen_at = NOW(), status = 'connected' WHERE id = $1`, id)
	return err
}

func (r *AgentRepository) DeleteAgent(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id)
	return err
}

func (r *AgentRepository) MarkStaleAgents(ctx context.Context, staleSec, disconnectSec int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agents SET status = CASE
			WHEN last_seen_at < NOW() - make_interval(secs => $1::double precision) THEN 'disconnected'
			WHEN last_seen_at < NOW() - make_interval(secs => $2::double precision) THEN 'stale'
			ELSE status
		 END
		 WHERE status != 'disconnected'`,
		disconnectSec, staleSec,
	)
	return err
}

func (r *AgentRepository) InsertHeartbeat(ctx context.Context, hb *models.AgentHeartbeat) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agent_heartbeats (id, agent_id, deployment_id, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		hb.ID, hb.AgentID, hb.DeploymentID, hb.Payload, hb.CreatedAt,
	)
	return err
}

func (r *AgentRepository) ListHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error) {
	var rows pgx.Rows
	var err error
	if deploymentID != nil {
		rows, err = r.pool.Query(ctx,
			`SELECT id, agent_id, deployment_id, payload, created_at
			 FROM agent_heartbeats WHERE agent_id = $1 AND deployment_id = $2
			 ORDER BY created_at DESC LIMIT $3`, agentID, *deploymentID, limit,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, agent_id, deployment_id, payload, created_at
			 FROM agent_heartbeats WHERE agent_id = $1
			 ORDER BY created_at DESC LIMIT $2`, agentID, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var heartbeats []models.AgentHeartbeat
	for rows.Next() {
		var hb models.AgentHeartbeat
		if err := rows.Scan(&hb.ID, &hb.AgentID, &hb.DeploymentID, &hb.Payload, &hb.CreatedAt); err != nil {
			return nil, err
		}
		heartbeats = append(heartbeats, hb)
	}
	return heartbeats, rows.Err()
}

func (r *AgentRepository) PruneHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, keep int) error {
	if deploymentID != nil {
		_, err := r.pool.Exec(ctx,
			`DELETE FROM agent_heartbeats WHERE agent_id = $1 AND deployment_id = $2
			 AND id NOT IN (
				SELECT id FROM agent_heartbeats WHERE agent_id = $1 AND deployment_id = $2
				ORDER BY created_at DESC LIMIT $3
			 )`, agentID, *deploymentID, keep,
		)
		return err
	}
	_, err := r.pool.Exec(ctx,
		`DELETE FROM agent_heartbeats WHERE agent_id = $1
		 AND id NOT IN (
			SELECT id FROM agent_heartbeats WHERE agent_id = $1
			ORDER BY created_at DESC LIMIT $2
		 )`, agentID, keep,
	)
	return err
}

// Compile-time interface check.
var _ registry.Repository = (*AgentRepository)(nil)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/agent/registry/... ./internal/platform/database/postgres/...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/registry/repository.go internal/platform/database/postgres/agents.go
git commit -m "feat: add agent registry repository interface and Postgres implementation"
```

---

### Task 4: Agent Registry Service

**Files:**
- Create: `internal/agent/registry/service.go`

- [ ] **Step 1: Write the failing test**

Create `internal/agent/registry/service_test.go`:

```go
package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/agent/registry"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// mockRepo implements registry.Repository for testing.
type mockRepo struct {
	agents     map[uuid.UUID]*models.Agent
	heartbeats []models.AgentHeartbeat
}

func newMockRepo() *mockRepo {
	return &mockRepo{agents: make(map[uuid.UUID]*models.Agent)}
}

func (m *mockRepo) CreateAgent(_ context.Context, a *models.Agent) error {
	m.agents[a.ID] = a
	return nil
}

func (m *mockRepo) GetAgent(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	a, ok := m.agents[id]
	if !ok {
		return nil, registry.ErrAgentNotFound
	}
	return a, nil
}

func (m *mockRepo) ListAgentsByApp(_ context.Context, appID uuid.UUID) ([]models.Agent, error) {
	var result []models.Agent
	for _, a := range m.agents {
		if a.AppID == appID {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (m *mockRepo) UpdateAgentStatus(_ context.Context, id uuid.UUID, status models.AgentStatus) error {
	if a, ok := m.agents[id]; ok {
		a.Status = status
	}
	return nil
}

func (m *mockRepo) UpdateLastSeen(_ context.Context, id uuid.UUID) error {
	if a, ok := m.agents[id]; ok {
		a.LastSeenAt = time.Now()
		a.Status = models.AgentStatusConnected
	}
	return nil
}

func (m *mockRepo) DeleteAgent(_ context.Context, id uuid.UUID) error {
	delete(m.agents, id)
	return nil
}

func (m *mockRepo) MarkStaleAgents(_ context.Context, _, _ int) error { return nil }

func (m *mockRepo) InsertHeartbeat(_ context.Context, hb *models.AgentHeartbeat) error {
	m.heartbeats = append(m.heartbeats, *hb)
	return nil
}

func (m *mockRepo) ListHeartbeats(_ context.Context, _ uuid.UUID, _ *uuid.UUID, limit int) ([]models.AgentHeartbeat, error) {
	if limit > len(m.heartbeats) {
		limit = len(m.heartbeats)
	}
	return m.heartbeats[:limit], nil
}

func (m *mockRepo) PruneHeartbeats(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ int) error {
	return nil
}

func TestRegisterAgent(t *testing.T) {
	repo := newMockRepo()
	svc := registry.NewService(repo)

	appID := uuid.New()
	envID := uuid.New()
	upstreams := json.RawMessage(`{"blue":"localhost:8081","green":"localhost:8082"}`)

	agent, err := svc.Register(context.Background(), appID, envID, "1.0.0", upstreams)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if agent.AppID != appID {
		t.Errorf("AppID = %v, want %v", agent.AppID, appID)
	}
	if agent.Status != models.AgentStatusConnected {
		t.Errorf("Status = %v, want connected", agent.Status)
	}
}

func TestHeartbeat(t *testing.T) {
	repo := newMockRepo()
	svc := registry.NewService(repo)

	appID := uuid.New()
	envID := uuid.New()
	agent, _ := svc.Register(context.Background(), appID, envID, "1.0.0", json.RawMessage(`{}`))

	payload := models.HeartbeatPayload{
		AgentID:       agent.ID,
		ConfigVersion: 1,
		EnvoyHealthy:  true,
		ActualTraffic: map[string]float64{"blue": 95.0, "green": 5.0},
		Upstreams:     map[string]models.UpstreamMetrics{},
		ActiveRules:   models.ActiveRules{Weights: map[string]int{"blue": 95, "green": 5}},
	}

	err := svc.Heartbeat(context.Background(), agent.ID, nil, payload)
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if len(repo.heartbeats) != 1 {
		t.Errorf("heartbeats count = %d, want 1", len(repo.heartbeats))
	}
}

func TestDeregister(t *testing.T) {
	repo := newMockRepo()
	svc := registry.NewService(repo)

	agent, _ := svc.Register(context.Background(), uuid.New(), uuid.New(), "1.0.0", json.RawMessage(`{}`))

	err := svc.Deregister(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if len(repo.agents) != 0 {
		t.Errorf("agents count = %d, want 0", len(repo.agents))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/registry/... -v -run TestRegisterAgent`
Expected: FAIL — `registry.NewService` not defined.

- [ ] **Step 3: Write the service**

```go
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ErrAgentNotFound is returned when an agent lookup finds no match.
var ErrAgentNotFound = errors.New("agent not found")

// Service manages agent registration and heartbeats.
type Service interface {
	Register(ctx context.Context, appID, envID uuid.UUID, version string, upstreams json.RawMessage) (*models.Agent, error)
	Heartbeat(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, payload models.HeartbeatPayload) error
	Deregister(ctx context.Context, agentID uuid.UUID) error
	ListByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error)
	GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, error)
	LatestHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error)
}

type agentService struct {
	repo Repository
}

// NewService creates a new agent registry Service.
func NewService(repo Repository) Service {
	return &agentService{repo: repo}
}

func (s *agentService) Register(ctx context.Context, appID, envID uuid.UUID, version string, upstreams json.RawMessage) (*models.Agent, error) {
	now := time.Now()
	agent := &models.Agent{
		ID:             uuid.New(),
		AppID:          appID,
		EnvironmentID:  envID,
		Status:         models.AgentStatusConnected,
		Version:        version,
		UpstreamConfig: upstreams,
		LastSeenAt:     now,
		RegisteredAt:   now,
	}
	if err := s.repo.CreateAgent(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *agentService) Heartbeat(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, payload models.HeartbeatPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	hb := &models.AgentHeartbeat{
		ID:           uuid.New(),
		AgentID:      agentID,
		DeploymentID: deploymentID,
		Payload:      payloadBytes,
		CreatedAt:    time.Now(),
	}

	if err := s.repo.InsertHeartbeat(ctx, hb); err != nil {
		return err
	}
	if err := s.repo.UpdateLastSeen(ctx, agentID); err != nil {
		return err
	}
	// Prune old heartbeats — keep last 100 per agent+deployment.
	return s.repo.PruneHeartbeats(ctx, agentID, deploymentID, 100)
}

func (s *agentService) Deregister(ctx context.Context, agentID uuid.UUID) error {
	return s.repo.DeleteAgent(ctx, agentID)
}

func (s *agentService) ListByApp(ctx context.Context, appID uuid.UUID) ([]models.Agent, error) {
	return s.repo.ListAgentsByApp(ctx, appID)
}

func (s *agentService) GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, error) {
	return s.repo.GetAgent(ctx, agentID)
}

func (s *agentService) LatestHeartbeats(ctx context.Context, agentID uuid.UUID, deploymentID *uuid.UUID, limit int) ([]models.AgentHeartbeat, error) {
	return s.repo.ListHeartbeats(ctx, agentID, deploymentID, limit)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/registry/... -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/registry/service.go internal/agent/registry/service_test.go
git commit -m "feat: add agent registry service with register, heartbeat, deregister"
```

---

### Task 5: Agent Registry HTTP Handler

**Files:**
- Create: `internal/agent/registry/handler.go`
- Create: `internal/agent/registry/handler_test.go`

- [ ] **Step 1: Write the handler tests**

```go
package registry_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/agent/registry"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func setupRouter(svc registry.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := registry.NewHandler(svc)
	api := r.Group("/api/v1")
	h.RegisterRoutes(api, nil)
	return r
}

func TestRegisterHandler(t *testing.T) {
	repo := newMockRepo()
	svc := registry.NewService(repo)
	router := setupRouter(svc)

	body, _ := json.Marshal(map[string]interface{}{
		"app_id":         uuid.New().String(),
		"environment_id": uuid.New().String(),
		"version":        "1.0.0",
		"upstreams":      map[string]string{"blue": "localhost:8081", "green": "localhost:8082"},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["id"]; !ok {
		t.Error("response missing 'id' field")
	}
}

func TestHeartbeatHandler(t *testing.T) {
	repo := newMockRepo()
	svc := registry.NewService(repo)
	router := setupRouter(svc)

	// Register first.
	agent, _ := svc.Register(nil, uuid.New(), uuid.New(), "1.0.0", json.RawMessage(`{}`))

	body, _ := json.Marshal(map[string]interface{}{
		"config_version": 1,
		"envoy_healthy":  true,
		"actual_traffic": map[string]float64{"blue": 95, "green": 5},
		"upstreams":      map[string]interface{}{},
		"active_rules":   map[string]interface{}{"weights": map[string]int{"blue": 95, "green": 5}},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agents/"+agent.ID.String()+"/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestListAgentsHandler(t *testing.T) {
	repo := newMockRepo()
	svc := registry.NewService(repo)
	router := setupRouter(svc)

	appID := uuid.New()
	svc.Register(nil, appID, uuid.New(), "1.0.0", json.RawMessage(`{}`))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/applications/"+appID.String()+"/agents", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	agents, ok := resp["agents"].([]interface{})
	if !ok || len(agents) != 1 {
		t.Errorf("expected 1 agent, got %v", resp)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/registry/... -v -run TestRegisterHandler`
Expected: FAIL — `registry.NewHandler` not defined.

- [ ] **Step 3: Write the handler**

```go
package registry

import (
	"encoding/json"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for agent registration and heartbeats.
type Handler struct {
	service Service
}

// NewHandler creates a new agent registry HTTP handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts agent registry API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	agents := rg.Group("/agents")
	{
		agents.POST("/register", h.registerAgent)
		agents.POST("/:id/heartbeat", h.heartbeat)
		agents.DELETE("/:id", h.deregisterAgent)
		agents.GET("/:id/heartbeats", h.listHeartbeats)
	}

	// Application-scoped agent listing.
	apps := rg.Group("/applications")
	{
		apps.GET("/:app_id/agents", h.listAgents)
	}
}

type registerRequest struct {
	AppID         uuid.UUID       `json:"app_id" binding:"required"`
	EnvironmentID uuid.UUID       `json:"environment_id" binding:"required"`
	Version       string          `json:"version"`
	Upstreams     json.RawMessage `json:"upstreams"`
}

func (h *Handler) registerAgent(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent, err := h.service.Register(c.Request.Context(), req.AppID, req.EnvironmentID, req.Version, req.Upstreams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, agent)
}

type heartbeatRequest struct {
	DeploymentID  *uuid.UUID                   `json:"deployment_id"`
	ConfigVersion int                          `json:"config_version"`
	ActualTraffic map[string]float64           `json:"actual_traffic"`
	Upstreams     map[string]models.UpstreamMetrics `json:"upstreams"`
	ActiveRules   models.ActiveRules           `json:"active_rules"`
	EnvoyHealthy  bool                         `json:"envoy_healthy"`
}

func (h *Handler) heartbeat(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req heartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := models.HeartbeatPayload{
		AgentID:       agentID,
		DeploymentID:  req.DeploymentID,
		ConfigVersion: req.ConfigVersion,
		ActualTraffic: req.ActualTraffic,
		Upstreams:     req.Upstreams,
		ActiveRules:   req.ActiveRules,
		EnvoyHealthy:  req.EnvoyHealthy,
	}

	if err := h.service.Heartbeat(c.Request.Context(), agentID, req.DeploymentID, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) deregisterAgent(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	if err := h.service.Deregister(c.Request.Context(), agentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) listAgents(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid app_id"})
		return
	}

	agents, err := h.service.ListByApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if agents == nil {
		agents = []models.Agent{}
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

func (h *Handler) listHeartbeats(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var deploymentID *uuid.UUID
	if did := c.Query("deployment_id"); did != "" {
		parsed, err := uuid.Parse(did)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment_id"})
			return
		}
		deploymentID = &parsed
	}

	heartbeats, err := h.service.LatestHeartbeats(c.Request.Context(), agentID, deploymentID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if heartbeats == nil {
		heartbeats = []models.AgentHeartbeat{}
	}
	c.JSON(http.StatusOK, gin.H{"heartbeats": heartbeats})
}
```

- [ ] **Step 4: Run all handler tests**

Run: `go test ./internal/agent/registry/... -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/registry/handler.go internal/agent/registry/handler_test.go
git commit -m "feat: add agent registry HTTP handler (register, heartbeat, list, deregister)"
```

---

### Task 6: Wire Agent Registry into API Server

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add agent repository and service initialization**

In `cmd/api/main.go`, after the existing repository block (~line 229), add:

```go
agentRepo := postgres.NewAgentRepository(db.Pool)
```

After the existing services block (~line 248), add:

```go
agentService := registry.NewService(agentRepo)
```

- [ ] **Step 2: Add route registration**

After the existing route registrations (~line 356), add:

```go
registry.NewHandler(agentService).RegisterRoutes(api, rbacChecker)
```

- [ ] **Step 3: Add import**

Add to the import block:

```go
"github.com/deploysentry/deploysentry/internal/agent/registry"
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./cmd/api/...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: wire agent registry handler into API server"
```

---

### Task 7: xDS Snapshot Builder

**Files:**
- Create: `internal/agent/xds/snapshot.go`
- Create: `internal/agent/xds/snapshot_test.go`

- [ ] **Step 1: Write the failing test**

```go
package xds_test

import (
	"testing"

	"github.com/deploysentry/deploysentry/internal/agent/xds"
)

func TestBuildSnapshot(t *testing.T) {
	upstreams := map[string]string{
		"blue":  "app-blue:8081",
		"green": "app-green:8082",
	}
	weights := map[string]uint32{
		"blue":  95,
		"green": 5,
	}

	snap, err := xds.BuildSnapshot("1", upstreams, weights, 8080)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}

	if err := snap.Consistent(); err != nil {
		t.Fatalf("snapshot not consistent: %v", err)
	}

	clusters := snap.GetResources("type.googleapis.com/envoy.config.cluster.v3.Cluster")
	if len(clusters) != 2 {
		t.Errorf("clusters count = %d, want 2", len(clusters))
	}

	listeners := snap.GetResources("type.googleapis.com/envoy.config.listener.v3.Listener")
	if len(listeners) != 1 {
		t.Errorf("listeners count = %d, want 1", len(listeners))
	}

	routes := snap.GetResources("type.googleapis.com/envoy.config.route.v3.RouteConfiguration")
	if len(routes) != 1 {
		t.Errorf("routes count = %d, want 1", len(routes))
	}
}

func TestBuildSnapshotWithHeaderOverrides(t *testing.T) {
	upstreams := map[string]string{
		"blue":  "app-blue:8081",
		"green": "app-green:8082",
	}
	weights := map[string]uint32{"blue": 95, "green": 5}
	overrides := []xds.HeaderOverride{
		{Header: "X-Version", Value: "canary", Upstream: "green"},
	}

	snap, err := xds.BuildSnapshotWithOptions("2", upstreams, weights, 8080, xds.SnapshotOptions{
		HeaderOverrides: overrides,
	})
	if err != nil {
		t.Fatalf("BuildSnapshotWithOptions: %v", err)
	}

	if err := snap.Consistent(); err != nil {
		t.Fatalf("snapshot not consistent: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/xds/... -v -run TestBuildSnapshot`
Expected: FAIL — package not found.

- [ ] **Step 3: Write the snapshot builder**

```go
package xds

import (
	"fmt"
	"net"
	"strconv"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// HeaderOverride routes requests matching a specific header value to an upstream.
type HeaderOverride struct {
	Header   string
	Value    string
	Upstream string
}

// SnapshotOptions configures optional routing behavior.
type SnapshotOptions struct {
	HeaderOverrides []HeaderOverride
}

// BuildSnapshot creates an Envoy cache snapshot with weighted clusters.
func BuildSnapshot(version string, upstreams map[string]string, weights map[string]uint32, listenPort uint32) (*cachev3.Snapshot, error) {
	return BuildSnapshotWithOptions(version, upstreams, weights, listenPort, SnapshotOptions{})
}

// BuildSnapshotWithOptions creates an Envoy cache snapshot with weighted clusters
// and optional header overrides.
func BuildSnapshotWithOptions(version string, upstreams map[string]string, weights map[string]uint32, listenPort uint32, opts SnapshotOptions) (*cachev3.Snapshot, error) {
	var clusters []cachev3.Resource
	var endpoints []cachev3.Resource

	for name, addr := range upstreams {
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream address %q: %w", addr, err)
		}
		port, err := strconv.ParseUint(portStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid port in %q: %w", addr, err)
		}

		clusters = append(clusters, &cluster.Cluster{
			Name:                 name,
			ConnectTimeout:       durationpb.New(5 * time.Second),
			ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
			LbPolicy:             cluster.Cluster_ROUND_ROBIN,
			LoadAssignment: &endpoint.ClusterLoadAssignment{
				ClusterName: name,
				Endpoints: []*endpoint.LocalityLbEndpoints{{
					LbEndpoints: []*endpoint.LbEndpoint{{
						HostIdentifier: &endpoint.LbEndpoint_Endpoint{
							Endpoint: &endpoint.Endpoint{
								Address: &core.Address{
									Address: &core.Address_SocketAddress{
										SocketAddress: &core.SocketAddress{
											Address: host,
											PortSpecifier: &core.SocketAddress_PortValue{
												PortValue: uint32(port),
											},
										},
									},
								},
							},
						},
					}},
				}},
			},
		})
	}

	// Build weighted clusters for the default route.
	var weightedClusters []*route.WeightedCluster_ClusterWeight
	for name, w := range weights {
		weightedClusters = append(weightedClusters, &route.WeightedCluster_ClusterWeight{
			Name:   name,
			Weight: wrapperspb.UInt32(w),
		})
	}

	// Build route entries: header overrides first, then default weighted route.
	var routes []*route.Route

	for _, override := range opts.HeaderOverrides {
		routes = append(routes, &route.Route{
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
				Headers: []*route.HeaderMatcher{{
					Name: override.Header,
					HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
						ExactMatch: override.Value,
					},
				}},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: override.Upstream,
					},
				},
			},
		})
	}

	// Default weighted route.
	routes = append(routes, &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_WeightedClusters{
					WeightedClusters: &route.WeightedCluster{
						Clusters: weightedClusters,
					},
				},
			},
		},
	})

	routeConfig := &route.RouteConfiguration{
		Name: "local_route",
		VirtualHosts: []*route.VirtualHost{{
			Name:    "local_service",
			Domains: []string{"*"},
			Routes:  routes,
		}},
	}

	hcmConfig := &hcm.HttpConnectionManager{
		StatPrefix: "ingress_http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource: &core.ConfigSource{
					ConfigSourceSpecifier: &core.ConfigSource_Ads{
						Ads: &core.AggregatedConfigSource{},
					},
				},
				RouteConfigName: "local_route",
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: "envoy.filters.http.router",
			ConfigType: &hcm.HttpFilter_TypedConfig{
				TypedConfig: mustMarshalAny(&hcm.HttpFilter{}),
			},
		}},
	}

	hcmAny, err := anypb.New(hcmConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal HCM config: %w", err)
	}

	lis := &listener.Listener{
		Name: "listener_0",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: listenPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: hcmAny,
				},
			}},
		}},
	}

	snap, err := cachev3.NewSnapshot(version, map[resource.Type][]cachev3.Resource{
		resource.ClusterType:  clusters,
		resource.RouteType:    {routeConfig},
		resource.ListenerType: {lis},
		resource.EndpointType: endpoints,
	})
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	return snap, nil
}

func mustMarshalAny(msg interface{}) *anypb.Any {
	// The router filter doesn't need a typed config in newer Envoy versions.
	// Return a placeholder that Envoy accepts.
	return &anypb.Any{
		TypeUrl: "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router",
	}
}
```

- [ ] **Step 4: Install go-control-plane dependency**

Run: `go get github.com/envoyproxy/go-control-plane@latest`
Expected: Module added to go.mod.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/agent/xds/... -v`
Expected: Both tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/xds/snapshot.go internal/agent/xds/snapshot_test.go go.mod go.sum
git commit -m "feat: add Envoy xDS snapshot builder with weighted clusters and header overrides"
```

---

### Task 8: xDS gRPC Server

**Files:**
- Create: `internal/agent/xds/server.go`
- Create: `internal/agent/xds/server_test.go`

- [ ] **Step 1: Write the failing test**

```go
package xds_test

import (
	"context"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/agent/xds"
)

func TestServerStartAndUpdate(t *testing.T) {
	srv, err := xds.NewServer(18000)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)

	// Allow server to start.
	time.Sleep(100 * time.Millisecond)

	upstreams := map[string]string{
		"blue":  "localhost:8081",
		"green": "localhost:8082",
	}
	weights := map[string]uint32{"blue": 95, "green": 5}

	err = srv.UpdateWeights(upstreams, weights, 8080, xds.SnapshotOptions{})
	if err != nil {
		t.Fatalf("UpdateWeights: %v", err)
	}

	if srv.ConfigVersion() != 1 {
		t.Errorf("ConfigVersion = %d, want 1", srv.ConfigVersion())
	}

	// Update again.
	weights["blue"] = 75
	weights["green"] = 25
	err = srv.UpdateWeights(upstreams, weights, 8080, xds.SnapshotOptions{})
	if err != nil {
		t.Fatalf("UpdateWeights: %v", err)
	}

	if srv.ConfigVersion() != 2 {
		t.Errorf("ConfigVersion = %d, want 2", srv.ConfigVersion())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/xds/... -v -run TestServerStartAndUpdate`
Expected: FAIL — `xds.NewServer` not defined.

- [ ] **Step 3: Write the xDS server**

```go
package xds

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync/atomic"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
)

const nodeID = "deploysentry-envoy"

// Server is an xDS gRPC control plane for Envoy.
type Server struct {
	port          int
	cache         cachev3.SnapshotCache
	grpcServer    *grpc.Server
	configVersion atomic.Int64
}

// NewServer creates a new xDS Server that listens on the given port.
func NewServer(port int) (*Server, error) {
	cache := cachev3.NewSnapshotCache(false, cachev3.IDHash{}, nil)
	return &Server{
		port:  port,
		cache: cache,
	}, nil
}

// Start starts the xDS gRPC server. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("xds listen: %w", err)
	}

	xdsServer := serverv3.NewServer(ctx, s.cache, nil)
	s.grpcServer = grpc.NewServer()

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(s.grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(s.grpcServer, xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(s.grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(s.grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(s.grpcServer, xdsServer)

	go func() {
		<-ctx.Done()
		s.grpcServer.GracefulStop()
	}()

	log.Printf("xDS server listening on :%d", s.port)
	return s.grpcServer.Serve(lis)
}

// UpdateWeights pushes a new snapshot to the cache with the given traffic weights.
func (s *Server) UpdateWeights(upstreams map[string]string, weights map[string]uint32, listenPort uint32, opts SnapshotOptions) error {
	version := s.configVersion.Add(1)

	snap, err := BuildSnapshotWithOptions(strconv.FormatInt(version, 10), upstreams, weights, listenPort, opts)
	if err != nil {
		return fmt.Errorf("build snapshot: %w", err)
	}

	if err := s.cache.SetSnapshot(context.Background(), nodeID, snap); err != nil {
		return fmt.Errorf("set snapshot: %w", err)
	}

	log.Printf("xDS snapshot updated to version %d (weights: %v)", version, weights)
	return nil
}

// ConfigVersion returns the current snapshot version counter.
func (s *Server) ConfigVersion() int64 {
	return s.configVersion.Load()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/agent/xds/... -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/xds/server.go internal/agent/xds/server_test.go
git commit -m "feat: add xDS gRPC control plane server with dynamic weight updates"
```

---

### Task 9: Agent SSE Client

**Files:**
- Create: `internal/agent/sse/client.go`
- Create: `internal/agent/sse/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
package sse_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/agent/sse"
)

func TestClientReceivesEvents(t *testing.T) {
	// Fake SSE server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("streaming not supported")
		}

		fmt.Fprintf(w, "event: deployment.phase_changed\ndata: {\"traffic_percent\":5}\n\n")
		flusher.Flush()

		fmt.Fprintf(w, "event: deployment.phase_changed\ndata: {\"traffic_percent\":25}\n\n")
		flusher.Flush()

		// Keep connection open briefly so client can read.
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	var received []int
	callback := func(trafficPercent int) {
		received = append(received, trafficPercent)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := sse.NewClient(server.URL, "", callback)
	go client.Connect(ctx)

	// Wait for events.
	time.Sleep(500 * time.Millisecond)

	if len(received) != 2 {
		t.Fatalf("received %d events, want 2", len(received))
	}
	if received[0] != 5 || received[1] != 25 {
		t.Errorf("received = %v, want [5, 25]", received)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/sse/... -v -run TestClientReceivesEvents`
Expected: FAIL — package not found.

- [ ] **Step 3: Write the SSE client**

```go
package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// TrafficCallback is called when a phase_changed event is received.
type TrafficCallback func(trafficPercent int)

// Client connects to a DeploySentry SSE stream and invokes a callback on
// deployment phase change events.
type Client struct {
	url      string
	apiKey   string
	callback TrafficCallback
	client   *http.Client
}

// NewClient creates a new SSE client.
func NewClient(url, apiKey string, callback TrafficCallback) *Client {
	return &Client{
		url:      url,
		apiKey:   apiKey,
		callback: callback,
		client:   &http.Client{Timeout: 0}, // no timeout for SSE
	}
}

type phaseChangedEvent struct {
	TrafficPercent int `json:"traffic_percent"`
}

// Connect opens the SSE stream and processes events. Blocks until ctx is
// cancelled. Reconnects with exponential backoff on disconnection.
func (c *Client) Connect(ctx context.Context) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := c.stream(ctx)
		if ctx.Err() != nil {
			return
		}

		log.Printf("SSE disconnected: %v — reconnecting in %v", err, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}

		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *Client) stream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", c.apiKey))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if eventType == "deployment.phase_changed" {
				var evt phaseChangedEvent
				if err := json.Unmarshal([]byte(data), &evt); err != nil {
					log.Printf("SSE: failed to parse phase_changed event: %v", err)
					continue
				}
				c.callback(evt.TrafficPercent)
			}

			eventType = ""
			continue
		}

		// Empty line or comment (heartbeat) — reset event state.
		if line == "" || strings.HasPrefix(line, ":") {
			eventType = ""
		}
	}

	return scanner.Err()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/agent/sse/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/sse/client.go internal/agent/sse/client_test.go
git commit -m "feat: add agent SSE client for desired-state streaming"
```

---

### Task 10: Agent Heartbeat Reporter

**Files:**
- Create: `internal/agent/reporter/reporter.go`
- Create: `internal/agent/reporter/reporter_test.go`

- [ ] **Step 1: Write the failing test**

```go
package reporter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/agent/reporter"
	"github.com/google/uuid"
)

func TestReporterSendsHeartbeats(t *testing.T) {
	var count atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path != "" {
			count.Add(1)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
	}))
	defer server.Close()

	agentID := uuid.New()
	r := reporter.New(server.URL, "", agentID, 100*time.Millisecond)
	r.SetWeights(map[string]uint32{"blue": 95, "green": 5})

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	go r.Start(ctx)
	<-ctx.Done()

	// Should have sent ~3 heartbeats in 350ms at 100ms interval.
	c := count.Load()
	if c < 2 || c > 5 {
		t.Errorf("heartbeat count = %d, want 2-5", c)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/reporter/... -v`
Expected: FAIL — package not found.

- [ ] **Step 3: Write the reporter**

```go
package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Reporter periodically sends heartbeats to the DeploySentry API.
type Reporter struct {
	apiURL   string
	apiKey   string
	agentID  uuid.UUID
	interval time.Duration
	client   *http.Client

	mu            sync.RWMutex
	weights       map[string]uint32
	deploymentID  *uuid.UUID
	configVersion int64
}

// New creates a new heartbeat Reporter.
func New(apiURL, apiKey string, agentID uuid.UUID, interval time.Duration) *Reporter {
	return &Reporter{
		apiURL:   apiURL,
		apiKey:   apiKey,
		agentID:  agentID,
		interval: interval,
		client:   &http.Client{Timeout: 5 * time.Second},
		weights:  make(map[string]uint32),
	}
}

// SetWeights updates the current traffic weights for the next heartbeat.
func (r *Reporter) SetWeights(w map[string]uint32) {
	r.mu.Lock()
	r.weights = w
	r.mu.Unlock()
}

// SetDeploymentID sets the active deployment being tracked.
func (r *Reporter) SetDeploymentID(id *uuid.UUID) {
	r.mu.Lock()
	r.deploymentID = id
	r.mu.Unlock()
}

// SetConfigVersion sets the current Envoy config version.
func (r *Reporter) SetConfigVersion(v int64) {
	r.mu.Lock()
	r.configVersion = v
	r.mu.Unlock()
}

// Start begins sending heartbeats at the configured interval. Blocks until ctx is cancelled.
func (r *Reporter) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.sendHeartbeat(ctx); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		}
	}
}

func (r *Reporter) sendHeartbeat(ctx context.Context) error {
	r.mu.RLock()
	actualTraffic := make(map[string]float64, len(r.weights))
	var totalWeight uint32
	for _, w := range r.weights {
		totalWeight += w
	}
	for name, w := range r.weights {
		if totalWeight > 0 {
			actualTraffic[name] = float64(w) / float64(totalWeight) * 100.0
		}
	}
	deploymentID := r.deploymentID
	configVersion := r.configVersion
	r.mu.RUnlock()

	payload := models.HeartbeatPayload{
		AgentID:       r.agentID,
		DeploymentID:  deploymentID,
		ConfigVersion: int(configVersion),
		ActualTraffic: actualTraffic,
		Upstreams:     make(map[string]models.UpstreamMetrics),
		ActiveRules:   models.ActiveRules{Weights: make(map[string]int)},
		EnvoyHealthy:  true,
	}
	for name, w := range r.weights {
		payload.ActiveRules.Weights[name] = int(w)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/agents/%s/heartbeat", r.apiURL, r.agentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", r.apiKey))
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat returned %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/agent/reporter/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/reporter/reporter.go internal/agent/reporter/reporter_test.go
git commit -m "feat: add agent heartbeat reporter"
```

---

### Task 11: Agent Configuration and Binary Entrypoint

**Files:**
- Create: `internal/agent/config.go`
- Create: `cmd/agent/main.go`

- [ ] **Step 1: Write the agent config**

```go
package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Config holds agent configuration loaded from environment variables.
type Config struct {
	APIURL            string
	APIKey            string
	AppID             uuid.UUID
	Environment       string
	Upstreams         map[string]string // name -> host:port
	EnvoyXDSPort      int
	EnvoyListenPort   int
	HeartbeatInterval time.Duration
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (*Config, error) {
	appID, err := uuid.Parse(getEnv("DS_APP_ID", ""))
	if err != nil {
		return nil, fmt.Errorf("DS_APP_ID must be a valid UUID: %w", err)
	}

	upstreams, err := parseUpstreams(getEnv("DS_UPSTREAMS", "blue:localhost:8081,green:localhost:8082"))
	if err != nil {
		return nil, fmt.Errorf("DS_UPSTREAMS: %w", err)
	}

	return &Config{
		APIURL:            getEnv("DS_API_URL", "http://localhost:8080"),
		APIKey:            getEnv("DS_API_KEY", ""),
		AppID:             appID,
		Environment:       getEnv("DS_ENVIRONMENT", "production"),
		Upstreams:         upstreams,
		EnvoyXDSPort:      getEnvInt("DS_ENVOY_XDS_PORT", 18000),
		EnvoyListenPort:   getEnvInt("DS_ENVOY_LISTEN_PORT", 8080),
		HeartbeatInterval: getEnvDuration("DS_HEARTBEAT_INTERVAL", 5*time.Second),
	}, nil
}

// parseUpstreams parses "blue:host:port,green:host:port" format.
func parseUpstreams(s string) (map[string]string, error) {
	result := make(map[string]string)
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Format: name:host:port
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid upstream format %q (expected name:host:port)", entry)
		}
		result[parts[0]] = parts[1] + ":" + parts[2]
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no upstreams configured")
	}
	return result, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
```

- [ ] **Step 2: Write the agent entrypoint**

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/deploysentry/deploysentry/internal/agent"
	"github.com/deploysentry/deploysentry/internal/agent/reporter"
	"github.com/deploysentry/deploysentry/internal/agent/sse"
	"github.com/deploysentry/deploysentry/internal/agent/xds"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := agent.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	log.Printf("deploysentry-agent starting (app=%s, env=%s, xds=:%d)", cfg.AppID, cfg.Environment, cfg.EnvoyXDSPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutdown signal received")
		cancel()
	}()

	// Start xDS server.
	xdsSrv, err := xds.NewServer(cfg.EnvoyXDSPort)
	if err != nil {
		return fmt.Errorf("creating xDS server: %w", err)
	}
	go func() {
		if err := xdsSrv.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("xDS server error: %v", err)
		}
	}()

	// Push initial equal-weight snapshot.
	initialWeights := make(map[string]uint32, len(cfg.Upstreams))
	for name := range cfg.Upstreams {
		initialWeights[name] = 50
	}
	if err := xdsSrv.UpdateWeights(cfg.Upstreams, initialWeights, uint32(cfg.EnvoyListenPort), xds.SnapshotOptions{}); err != nil {
		return fmt.Errorf("initial xDS snapshot: %w", err)
	}

	// Register with the DeploySentry API.
	agentID, err := registerAgent(cfg)
	if err != nil {
		log.Printf("warning: agent registration failed (running unregistered): %v", err)
		agentID = uuid.New()
	}

	// Start heartbeat reporter.
	rep := reporter.New(cfg.APIURL, cfg.APIKey, agentID, cfg.HeartbeatInterval)
	rep.SetWeights(initialWeights)
	go rep.Start(ctx)

	// SSE callback: update xDS weights when desired state changes.
	currentWeights := initialWeights
	sseCallback := func(trafficPercent int) {
		log.Printf("SSE: desired traffic percent = %d%%", trafficPercent)

		// Map traffic_percent to green, remainder to blue.
		newWeights := map[string]uint32{
			"blue":  uint32(100 - trafficPercent),
			"green": uint32(trafficPercent),
		}

		if err := xdsSrv.UpdateWeights(cfg.Upstreams, newWeights, uint32(cfg.EnvoyListenPort), xds.SnapshotOptions{}); err != nil {
			log.Printf("xDS update failed: %v", err)
			return
		}

		currentWeights = newWeights
		rep.SetWeights(currentWeights)
		rep.SetConfigVersion(xdsSrv.ConfigVersion())
		log.Printf("traffic updated: blue=%d%% green=%d%%", 100-trafficPercent, trafficPercent)
	}

	// Start SSE client.
	sseURL := fmt.Sprintf("%s/api/v1/flags/stream?application=%s", cfg.APIURL, cfg.AppID)
	sseClient := sse.NewClient(sseURL, cfg.APIKey, sseCallback)
	go sseClient.Connect(ctx)

	log.Printf("agent running (id=%s)", agentID)

	<-ctx.Done()
	log.Println("agent shutting down")
	return nil
}
```

Add the `registerAgent` helper function after `run()`:

```go
func registerAgent(cfg *agent.Config) (uuid.UUID, error) {
	upstreamsJSON, _ := json.Marshal(cfg.Upstreams)
	body := fmt.Sprintf(`{"app_id":"%s","environment_id":"%s","version":"0.1.0","upstreams":%s}`,
		cfg.AppID, cfg.AppID, string(upstreamsJSON))

	url := fmt.Sprintf("%s/api/v1/agents/register", cfg.APIURL)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", cfg.APIKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return uuid.Nil, fmt.Errorf("registration returned %d", resp.StatusCode)
	}

	var result struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return uuid.Nil, err
	}
	return result.ID, nil
}
```

Add the additional imports to the import block: `"net/http"`, `"strings"`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/agent/...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/config.go cmd/agent/main.go
git commit -m "feat: add deploysentry-agent binary with xDS, SSE, and heartbeat integration"
```

---

### Task 12: Docker Compose Multi-Instance Setup

**Files:**
- Create: `deploy/docker/docker-compose.deploy.yml`
- Create: `deploy/docker/envoy-bootstrap.yaml`
- Create: `deploy/docker/Dockerfile.agent`

- [ ] **Step 1: Write the Envoy bootstrap config**

```yaml
# envoy-bootstrap.yaml — Minimal bootstrap that points Envoy to the agent xDS server.
# All routing config (listeners, routes, clusters, weights) is pushed dynamically via xDS.
node:
  id: deploysentry-envoy
  cluster: deploysentry

dynamic_resources:
  cds_config:
    resource_api_version: V3
    api_config_source:
      api_type: GRPC
      transport_api_version: V3
      grpc_services:
        - envoy_grpc:
            cluster_name: xds_cluster
  lds_config:
    resource_api_version: V3
    api_config_source:
      api_type: GRPC
      transport_api_version: V3
      grpc_services:
        - envoy_grpc:
            cluster_name: xds_cluster

static_resources:
  clusters:
    - name: xds_cluster
      connect_timeout: 1s
      type: STATIC
      typed_extension_protocol_options:
        envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
          "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
          explicit_http_config:
            http2_protocol_options: {}
      load_assignment:
        cluster_name: xds_cluster
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: deploysentry-agent
                      port_value: 18000

admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9901
```

- [ ] **Step 2: Write the agent Dockerfile**

```dockerfile
# Dockerfile.agent — Builds the deploysentry-agent binary.
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /deploysentry-agent ./cmd/agent

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /deploysentry-agent /usr/local/bin/deploysentry-agent
ENTRYPOINT ["deploysentry-agent"]
```

- [ ] **Step 3: Write the deploy compose file**

```yaml
# docker-compose.deploy.yml — Multi-instance setup with Envoy + agent for traffic management.
# Extends the base docker-compose.yml. Use: make dev-deploy

services:
  app-blue:
    image: ${DS_APP_IMAGE:-deploysentry/demo-app:latest}
    container_name: deploysentry-app-blue
    environment:
      - SERVICE_COLOR=blue
      - PORT=8081
    ports:
      - "8081:8081"
    networks:
      - deploysentry-net

  app-green:
    image: ${DS_APP_IMAGE:-deploysentry/demo-app:latest}
    container_name: deploysentry-app-green
    environment:
      - SERVICE_COLOR=green
      - PORT=8082
    ports:
      - "8082:8082"
    networks:
      - deploysentry-net

  envoy:
    image: envoyproxy/envoy:v1.31-latest
    container_name: deploysentry-envoy
    ports:
      - "8080:8080"
      - "9901:9901"
    volumes:
      - ./envoy-bootstrap.yaml:/etc/envoy/envoy.yaml:ro
    depends_on:
      - deploysentry-agent
    networks:
      - deploysentry-net

  deploysentry-agent:
    build:
      context: ../..
      dockerfile: deploy/docker/Dockerfile.agent
    container_name: deploysentry-agent
    environment:
      - DS_API_URL=http://host.docker.internal:8080
      - DS_API_KEY=${DS_API_KEY:-}
      - DS_APP_ID=${DS_APP_ID:-00000000-0000-0000-0000-000000000000}
      - DS_ENVIRONMENT=${DS_ENVIRONMENT:-production}
      - DS_UPSTREAMS=blue:app-blue:8081,green:app-green:8082
      - DS_ENVOY_XDS_PORT=18000
      - DS_ENVOY_LISTEN_PORT=8080
      - DS_HEARTBEAT_INTERVAL=5s
    ports:
      - "18000:18000"
    networks:
      - deploysentry-net

networks:
  deploysentry-net:
    external: true
    name: deploysentry-net
```

- [ ] **Step 4: Add Makefile target**

In `Makefile`, add:

```makefile
dev-deploy: ## Start multi-instance deploy setup (Envoy + agent + blue/green)
	docker compose -f deploy/docker-compose.yml -f deploy/docker/docker-compose.deploy.yml up -d --build
```

- [ ] **Step 5: Verify compose config is valid**

Run: `docker compose -f deploy/docker-compose.yml -f deploy/docker/docker-compose.deploy.yml config --quiet`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add deploy/docker/docker-compose.deploy.yml deploy/docker/envoy-bootstrap.yaml deploy/docker/Dockerfile.agent Makefile
git commit -m "feat: add Docker Compose multi-instance setup with Envoy and agent"
```

---

### Task 13: Web Dashboard — Types and API Functions

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add agent types to `web/src/types.ts`**

Append after the existing `Deployment` interface:

```typescript
export type AgentStatus = 'connected' | 'stale' | 'disconnected';

export interface Agent {
  id: string;
  app_id: string;
  environment_id: string;
  status: AgentStatus;
  version: string;
  upstream_config: Record<string, string>;
  last_seen_at: string;
  registered_at: string;
}

export interface UpstreamMetrics {
  rps: number;
  error_rate: number;
  p99_ms: number;
  p50_ms: number;
}

export interface ActiveRules {
  weights: Record<string, number>;
  header_overrides?: { header: string; value: string; upstream: string }[];
  sticky_sessions?: { enabled: boolean; strategy?: string; ttl?: string };
}

export interface HeartbeatPayload {
  agent_id: string;
  deployment_id?: string;
  config_version: number;
  actual_traffic: Record<string, number>;
  upstreams: Record<string, UpstreamMetrics>;
  active_rules: ActiveRules;
  envoy_healthy: boolean;
}

export interface AgentHeartbeat {
  id: string;
  agent_id: string;
  deployment_id?: string;
  payload: HeartbeatPayload;
  created_at: string;
}
```

- [ ] **Step 2: Add agent API functions to `web/src/api.ts`**

Append after the existing `deploymentsApi` export:

```typescript
export const agentsApi = {
  listByApp: (appId: string) =>
    request<{ agents: Agent[] }>(`/applications/${appId}/agents`),
  heartbeats: (agentId: string, deploymentId?: string) => {
    const qs = deploymentId ? `?deployment_id=${deploymentId}` : '';
    return request<{ heartbeats: AgentHeartbeat[] }>(`/agents/${agentId}/heartbeats${qs}`);
  },
};
```

- [ ] **Step 3: Add the import for new types**

Ensure the `Agent`, `AgentHeartbeat` types are imported where `api.ts` uses them (they're in the same `types.ts` file that's already imported).

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add agent types and API functions to web dashboard"
```

---

### Task 14: Web Dashboard — Traffic Panel on DeploymentDetailPage

**Files:**
- Modify: `web/src/pages/DeploymentDetailPage.tsx`

- [ ] **Step 1: Read the current DeploymentDetailPage**

Read `web/src/pages/DeploymentDetailPage.tsx` in full to understand the existing structure, state management, and component layout before modifying.

- [ ] **Step 2: Add agent data fetching**

Add state and useEffect hooks to fetch agents and heartbeats for the current deployment's application:

```typescript
const [agents, setAgents] = useState<Agent[]>([]);
const [latestHeartbeat, setLatestHeartbeat] = useState<HeartbeatPayload | null>(null);

useEffect(() => {
  if (!deployment?.application_id) return;
  agentsApi.listByApp(deployment.application_id).then(res => {
    setAgents(res.agents);
    // Fetch heartbeats for the first connected agent.
    const connected = res.agents.find(a => a.status === 'connected');
    if (connected) {
      agentsApi.heartbeats(connected.id, deployment.id).then(hbRes => {
        if (hbRes.heartbeats.length > 0) {
          setLatestHeartbeat(hbRes.heartbeats[0].payload);
        }
      });
    }
  });
}, [deployment?.application_id, deployment?.id]);
```

Set up a polling interval to refresh heartbeat data every 5 seconds:

```typescript
useEffect(() => {
  if (!agents.length || !deployment?.id) return;
  const connected = agents.find(a => a.status === 'connected');
  if (!connected) return;

  const interval = setInterval(() => {
    agentsApi.heartbeats(connected.id, deployment.id).then(res => {
      if (res.heartbeats.length > 0) {
        setLatestHeartbeat(res.heartbeats[0].payload);
      }
    });
  }, 5000);

  return () => clearInterval(interval);
}, [agents, deployment?.id]);
```

- [ ] **Step 3: Add the Traffic Distribution component**

Below the existing phase timeline, add a traffic panel section. This is inline JSX, not a separate component file (matches the existing page pattern):

```tsx
{/* Traffic Distribution */}
{latestHeartbeat && (
  <div className="mt-6 rounded-lg border border-gray-700 bg-gray-800 p-4">
    <h3 className="mb-3 text-xs font-medium uppercase tracking-wider text-gray-400">
      Traffic Distribution
    </h3>
    {Object.entries(latestHeartbeat.actual_traffic).map(([name, pct]) => (
      <div key={name} className="mb-3">
        <div className="mb-1 flex justify-between text-sm">
          <span className="font-medium" style={{ color: name === 'blue' ? '#69f0ae' : '#b388ff' }}>
            {name}
          </span>
          <span className="text-gray-400">
            desired: {latestHeartbeat.active_rules.weights[name]}%
            {' | '}
            actual: {pct.toFixed(1)}%
          </span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-gray-900">
          <div
            className="h-full rounded-full"
            style={{
              width: `${pct}%`,
              background: name === 'blue'
                ? 'linear-gradient(90deg, #1a5a3a, #69f0ae)'
                : 'linear-gradient(90deg, #4a1a6a, #b388ff)',
            }}
          />
        </div>
      </div>
    ))}
  </div>
)}
```

- [ ] **Step 4: Add Per-Version Metrics component**

```tsx
{latestHeartbeat && Object.keys(latestHeartbeat.upstreams).length > 0 && (
  <div className="mt-4 grid grid-cols-2 gap-3">
    {Object.entries(latestHeartbeat.upstreams).map(([name, metrics]) => (
      <div key={name} className="rounded-lg border border-gray-700 bg-gray-800 p-3">
        <div className="mb-2 text-sm font-medium" style={{ color: name === 'blue' ? '#69f0ae' : '#b388ff' }}>
          {name}
        </div>
        <div className="grid grid-cols-2 gap-2 text-xs">
          <div>
            <div className="text-gray-500">RPS</div>
            <div className="text-lg font-semibold text-gray-100">{metrics.rps.toLocaleString()}</div>
          </div>
          <div>
            <div className="text-gray-500">Error Rate</div>
            <div className={`text-lg font-semibold ${metrics.error_rate < 1 ? 'text-green-400' : 'text-red-400'}`}>
              {metrics.error_rate.toFixed(2)}%
            </div>
          </div>
          <div>
            <div className="text-gray-500">P99</div>
            <div className="text-lg font-semibold text-gray-100">{metrics.p99_ms}ms</div>
          </div>
          <div>
            <div className="text-gray-500">P50</div>
            <div className="text-lg font-semibold text-gray-100">{metrics.p50_ms}ms</div>
          </div>
        </div>
      </div>
    ))}
  </div>
)}
```

- [ ] **Step 5: Add Agent Status component**

```tsx
{agents.length > 0 && (
  <div className="mt-6 rounded-lg border border-gray-700 bg-gray-800 p-4">
    <h3 className="mb-3 text-xs font-medium uppercase tracking-wider text-gray-400">
      Agent Status
    </h3>
    {agents.map(agent => (
      <div key={agent.id} className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div
            className="h-2 w-2 rounded-full"
            style={{
              background: agent.status === 'connected' ? '#69f0ae'
                : agent.status === 'stale' ? '#ffd740'
                : '#ff5252',
            }}
          />
          <span className="text-sm font-medium text-gray-200">deploysentry-agent</span>
        </div>
        <span className="text-xs text-gray-500">
          {agent.status === 'connected'
            ? `last seen ${Math.round((Date.now() - new Date(agent.last_seen_at).getTime()) / 1000)}s ago`
            : agent.status}
        </span>
      </div>
    ))}
    {latestHeartbeat && (
      <div className="mt-2 flex gap-4 text-xs text-gray-400">
        <span>Envoy: <span className={latestHeartbeat.envoy_healthy ? 'text-green-400' : 'text-red-400'}>
          {latestHeartbeat.envoy_healthy ? 'healthy' : 'unhealthy'}
        </span></span>
        <span>Config v: {latestHeartbeat.config_version}</span>
      </div>
    )}
  </div>
)}
```

- [ ] **Step 6: Add Traffic Rules summary**

```tsx
{latestHeartbeat?.active_rules && (
  <div className="mt-4 rounded-lg border border-gray-700 bg-gray-800 p-4">
    <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-gray-400">
      Traffic Rules
    </h3>
    <div className="text-sm text-gray-300">
      Weighted {Object.entries(latestHeartbeat.active_rules.weights).map(([k, v]) => `${k}:${v}%`).join(' / ')}
      {latestHeartbeat.active_rules.header_overrides?.length
        ? ` + ${latestHeartbeat.active_rules.header_overrides.length} header override(s)`
        : ''}
      {latestHeartbeat.active_rules.sticky_sessions?.enabled
        ? ` + sticky (${latestHeartbeat.active_rules.sticky_sessions.strategy})`
        : ''}
    </div>
    {latestHeartbeat.active_rules.header_overrides?.map((o, i) => (
      <div key={i} className="mt-1 text-xs text-gray-500">
        {o.header}: {o.value} → {o.upstream}
      </div>
    ))}
  </div>
)}
```

- [ ] **Step 7: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 8: Start dev server and visually verify**

Run: `cd web && npm run dev`
Open the deployment detail page in a browser. Without agent data, the new panels should not render (graceful empty state). With mock data or a running agent, the panels should appear.

- [ ] **Step 9: Commit**

```bash
git add web/src/pages/DeploymentDetailPage.tsx
git commit -m "feat: add traffic panel, agent status, and traffic rules to DeploymentDetailPage"
```

---

### Task 15: SDK SERVICE_COLOR Auto-Detection (Go SDK)

**Files:**
- Modify: `sdk/go/client.go`

- [ ] **Step 1: Write the failing test**

Create or append to `sdk/go/client_test.go`:

```go
func TestServiceColorAutoDetected(t *testing.T) {
	t.Setenv("SERVICE_COLOR", "green")

	client := NewClient(WithAPIKey("test"))
	ctx := client.DefaultContext()

	if ctx.Attributes["service_color"] != "green" {
		t.Errorf("service_color = %v, want 'green'", ctx.Attributes["service_color"])
	}
}

func TestServiceColorNotSetWhenEmpty(t *testing.T) {
	// Ensure SERVICE_COLOR is not set.
	t.Setenv("SERVICE_COLOR", "")

	client := NewClient(WithAPIKey("test"))
	ctx := client.DefaultContext()

	if _, ok := ctx.Attributes["service_color"]; ok {
		t.Error("service_color should not be set when SERVICE_COLOR is empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./sdk/go/... -v -run TestServiceColor`
Expected: FAIL — `DefaultContext` method not defined.

- [ ] **Step 3: Add SERVICE_COLOR detection to the client**

In `sdk/go/client.go`, add a `serviceColor` field to the `Client` struct:

```go
serviceColor string // auto-detected from SERVICE_COLOR env var
```

In `NewClient`, after applying options, add:

```go
if sc := os.Getenv("SERVICE_COLOR"); sc != "" {
    c.serviceColor = sc
}
```

Add the `DefaultContext` method:

```go
// DefaultContext returns an EvaluationContext with auto-detected attributes
// (e.g., SERVICE_COLOR). Use this as a base and add user-specific fields.
func (c *Client) DefaultContext() *EvaluationContext {
	ctx := &EvaluationContext{
		Attributes: make(map[string]interface{}),
	}
	if c.serviceColor != "" {
		ctx.Attributes["service_color"] = c.serviceColor
	}
	return ctx
}
```

Also modify `BoolValue`, `StringValue`, `Detail`, and other evaluation methods to merge `serviceColor` into the context if present and not already set:

```go
// mergeServiceColor adds the service_color attribute if configured and not
// already present in the evaluation context.
func (c *Client) mergeServiceColor(ctx *EvaluationContext) *EvaluationContext {
	if c.serviceColor == "" || ctx == nil {
		if c.serviceColor == "" {
			return ctx
		}
		ctx = &EvaluationContext{Attributes: make(map[string]interface{})}
	}
	if ctx.Attributes == nil {
		ctx.Attributes = make(map[string]interface{})
	}
	if _, ok := ctx.Attributes["service_color"]; !ok {
		ctx.Attributes["service_color"] = c.serviceColor
	}
	return ctx
}
```

Call `mergeServiceColor` at the top of each evaluation method.

- [ ] **Step 4: Run tests**

Run: `go test ./sdk/go/... -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add sdk/go/client.go sdk/go/client_test.go
git commit -m "feat: auto-detect SERVICE_COLOR in Go SDK evaluation context"
```

---

### Task 16: Flag-Test Deployment Mode

**Files:**
- Modify: `internal/models/deployment.go`
- Modify: `internal/deploy/handler.go`

- [ ] **Step 1: Add flag_test_key field to Deployment model**

In `internal/models/deployment.go`, add to the `Deployment` struct:

```go
FlagTestKey *string `json:"flag_test_key,omitempty" db:"flag_test_key"`
```

- [ ] **Step 2: Write a migration for the new column**

Create `migrations/044_add_flag_test_key.up.sql`:

```sql
ALTER TABLE deployments ADD COLUMN flag_test_key TEXT;
```

Create `migrations/044_add_flag_test_key.down.sql`:

```sql
ALTER TABLE deployments DROP COLUMN IF EXISTS flag_test_key;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration 044 applies successfully.

- [ ] **Step 4: Update createDeployment handler to accept flag_test_key**

In `internal/deploy/handler.go`, in the `createDeploymentRequest` struct, add:

```go
FlagTestKey *string `json:"flag_test_key"`
```

In the `createDeployment` handler, pass it through to the deployment:

```go
d.FlagTestKey = req.FlagTestKey
```

- [ ] **Step 5: Update the Postgres deploy repository to persist flag_test_key**

In the INSERT and SELECT queries for deployments, include the `flag_test_key` column.

- [ ] **Step 6: Verify it compiles**

Run: `go build ./cmd/api/...`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add internal/models/deployment.go internal/deploy/handler.go internal/platform/database/postgres/deploy.go migrations/044_add_flag_test_key.up.sql migrations/044_add_flag_test_key.down.sql
git commit -m "feat: add flag_test_key to deployments for flag canary mode"
```

---

### Task 17: Dashboard — Flags Under Test Section

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/pages/DeploymentDetailPage.tsx`

- [ ] **Step 1: Add flag_test_key to Deployment type**

In `web/src/types.ts`, add to the `Deployment` interface:

```typescript
flag_test_key?: string;
```

- [ ] **Step 2: Add Flags Under Test section to DeploymentDetailPage**

In `web/src/pages/DeploymentDetailPage.tsx`, after the Traffic Rules section, add:

```tsx
{deployment.flag_test_key && (
  <div className="mt-4 rounded-lg border border-amber-800/50 bg-amber-900/20 p-4">
    <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-amber-400">
      Flag Under Test
    </h3>
    <div className="flex items-center gap-2">
      <span className="rounded bg-amber-800/30 px-2 py-0.5 text-sm font-mono text-amber-300">
        {deployment.flag_test_key}
      </span>
      <span className="text-xs text-gray-400">
        Canary testing via SERVICE_COLOR targeting
      </span>
    </div>
    <div className="mt-2 text-xs text-gray-500">
      Blue (stable) receives the flag's default value. Green (canary) receives the flag with{' '}
      <code className="text-amber-400">service_color eq green</code> targeting rule.
    </div>
    {latestHeartbeat && (
      <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
        <div className="rounded bg-gray-800 p-2">
          <span style={{ color: '#69f0ae' }}>blue</span>: flag off (baseline)
        </div>
        <div className="rounded bg-gray-800 p-2">
          <span style={{ color: '#b388ff' }}>green</span>: flag on (testing)
        </div>
      </div>
    )}
  </div>
)}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/pages/DeploymentDetailPage.tsx
git commit -m "feat: add Flags Under Test section to deployment detail page"
```

---

### Task 18: Update Documentation

> Note: This was previously Task 16 before flag-test tasks were added.

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Read the current initiatives file**

Read `docs/Current_Initiatives.md` to see the current format and entries.

- [ ] **Step 2: Add the sidecar traffic management initiative**

Add a row to the initiatives table:

```markdown
| Sidecar Traffic Management | Design | [Spec](./superpowers/specs/2026-04-17-sidecar-traffic-management-design.md) / [Plan](./superpowers/plans/2026-04-17-sidecar-traffic-management.md) |
```

- [ ] **Step 3: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add sidecar traffic management to current initiatives"
```

---

## Task Dependency Summary

```
Task 1 (migration) ──────┐
Task 2 (models)    ──────┤
                          ├──▶ Task 3 (postgres repo) ──▶ Task 4 (service) ──▶ Task 5 (handler) ──▶ Task 6 (wire API)
                          │
Task 7 (xDS snapshot) ───┤
                          ├──▶ Task 8 (xDS server) ──┐
Task 9 (SSE client) ─────┤                           ├──▶ Task 11 (agent binary)
Task 10 (reporter) ──────┘                           │
                                                      ├──▶ Task 12 (Docker Compose)
Task 13 (web types/API) ──▶ Task 14 (dashboard) ──▶ Task 17 (flags under test UI)
Task 15 (SDK SERVICE_COLOR) — independent
Task 16 (flag-test deploy mode) — independent of agent, needs migration
Task 18 (docs) — independent, do last
```

**Parallel tracks:**
- Track A: Tasks 1→2→3→4→5→6 (API: models, repo, service, handler, wiring)
- Track B: Tasks 7→8 (xDS: snapshot builder, gRPC server)
- Track C: Task 9 (SSE client)
- Track D: Task 10 (heartbeat reporter)
- Track E: Tasks 13→14→17 (web dashboard + flags under test)
- Track F: Task 15 (SDK)
- Track G: Task 16 (flag-test deploy mode)
- Merge: Task 11 (agent binary — needs A, B, C, D)
- Merge: Task 12 (Docker Compose — needs 11)
- Final: Task 18 (docs)
