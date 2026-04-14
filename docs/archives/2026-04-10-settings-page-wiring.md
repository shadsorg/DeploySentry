# SettingsPage Backend Wiring — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire all 5 SettingsPage tabs (Environments, Webhooks, Notifications, Project General, App General) to real backend APIs, replacing all mock data and local-only state.

**Architecture:** Three backend additions (environment CRUD, notification preferences handler, environment migration) plus frontend wiring for all tabs. Environments are org-level with app-level overrides. Notification preferences merge with env var defaults. Webhooks backend already exists — frontend-only wiring.

**Tech Stack:** Go (Gin, sqlx, PostgreSQL), React/TypeScript, REST APIs

**Spec:** `docs/superpowers/specs/2026-04-10-settings-page-wiring-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `migrations/035_create_environments.up.sql` | Environments + app overrides tables |
| Create | `migrations/035_create_environments.down.sql` | Rollback |
| Create | `internal/entities/environment_repository.go` | Environment DB queries |
| Modify | `internal/entities/service.go:13-30` | Add environment methods to EntityService interface + implementation |
| Modify | `internal/entities/handler.go:24-50` | Add environment CRUD routes |
| Create | `internal/notifications/preferences_handler.go` | Notification preferences HTTP handler |
| Modify | `cmd/api/main.go:284-305` | Register notification preferences routes |
| Modify | `web/src/api.ts:275-350` | Add webhooksApi, notificationsApi, environmentsApi |
| Create | `web/src/hooks/useWebhooks.ts` | Webhook data hook |
| Create | `web/src/hooks/useNotifications.ts` | Notification preferences hook |
| Modify | `web/src/hooks/useEntities.ts:81-110` | Update useEnvironments to use org-level endpoint |
| Modify | `web/src/pages/SettingsPage.tsx` | Wire all 5 tabs to real APIs |

---

### Task 1: Environment Database Migration

**Files:**
- Create: `migrations/035_create_environments.up.sql`
- Create: `migrations/035_create_environments.down.sql`

- [ ] **Step 1: Create up migration**

Write `migrations/035_create_environments.up.sql`:

```sql
CREATE TABLE environments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL,
    is_production BOOLEAN NOT NULL DEFAULT false,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

CREATE INDEX idx_environments_org_id ON environments(org_id);

CREATE TABLE app_environment_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_id, environment_id)
);

CREATE INDEX idx_app_env_overrides_app_id ON app_environment_overrides(app_id);
```

- [ ] **Step 2: Create down migration**

Write `migrations/035_create_environments.down.sql`:

```sql
DROP TABLE IF EXISTS app_environment_overrides;
DROP TABLE IF EXISTS environments;
```

- [ ] **Step 3: Run migrations**

```bash
cd /Users/sgamel/git/DeploySentry && make migrate-up
```

Expected: Migration 035 applied successfully.

- [ ] **Step 4: Commit**

```bash
git add migrations/035_create_environments.up.sql migrations/035_create_environments.down.sql
git commit -m "feat: add environments and app_environment_overrides tables (035)"
```

---

### Task 2: Environment Repository

**Files:**
- Create: `internal/entities/environment_repository.go`

- [ ] **Step 1: Create environment repository**

Write `internal/entities/environment_repository.go`:

```go
package entities

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Environment represents an org-level environment.
type Environment struct {
	ID           uuid.UUID `json:"id" db:"id"`
	OrgID        uuid.UUID `json:"org_id" db:"org_id"`
	Name         string    `json:"name" db:"name"`
	Slug         string    `json:"slug" db:"slug"`
	IsProduction bool      `json:"is_production" db:"is_production"`
	SortOrder    int       `json:"sort_order" db:"sort_order"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// AppEnvironmentOverride represents an app-level environment override.
type AppEnvironmentOverride struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	AppID         uuid.UUID              `json:"app_id" db:"app_id"`
	EnvironmentID uuid.UUID              `json:"environment_id" db:"environment_id"`
	Config        map[string]interface{} `json:"config" db:"config"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
}

// EnvironmentRepository handles environment persistence.
type EnvironmentRepository struct {
	db *sqlx.DB
}

// NewEnvironmentRepository creates a new EnvironmentRepository.
func NewEnvironmentRepository(db *sqlx.DB) *EnvironmentRepository {
	return &EnvironmentRepository{db: db}
}

func (r *EnvironmentRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Environment, error) {
	var envs []Environment
	err := r.db.SelectContext(ctx, &envs,
		`SELECT id, org_id, name, slug, is_production, sort_order, created_at, updated_at
		 FROM environments WHERE org_id = $1 ORDER BY sort_order, name`, orgID)
	if err != nil {
		return nil, err
	}
	return envs, nil
}

func (r *EnvironmentRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Environment, error) {
	var env Environment
	err := r.db.GetContext(ctx, &env,
		`SELECT id, org_id, name, slug, is_production, sort_order, created_at, updated_at
		 FROM environments WHERE org_id = $1 AND slug = $2`, orgID, slug)
	if err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *EnvironmentRepository) Create(ctx context.Context, env *Environment) error {
	env.ID = uuid.New()
	now := time.Now().UTC()
	env.CreatedAt = now
	env.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO environments (id, org_id, name, slug, is_production, sort_order, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		env.ID, env.OrgID, env.Name, env.Slug, env.IsProduction, env.SortOrder, env.CreatedAt, env.UpdatedAt)
	return err
}

func (r *EnvironmentRepository) Update(ctx context.Context, env *Environment) error {
	env.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`UPDATE environments SET name = $1, slug = $2, is_production = $3, sort_order = $4, updated_at = $5
		 WHERE id = $6`,
		env.Name, env.Slug, env.IsProduction, env.SortOrder, env.UpdatedAt, env.ID)
	return err
}

func (r *EnvironmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM environments WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/sgamel/git/DeploySentry && go build ./internal/entities/...
```

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/entities/environment_repository.go
git commit -m "feat: add environment repository with CRUD operations"
```

---

### Task 3: Environment Service and Handler

**Files:**
- Modify: `internal/entities/service.go:13-30`
- Modify: `internal/entities/handler.go:24-50`

- [ ] **Step 1: Add environment methods to EntityService interface**

In `internal/entities/service.go`, add these methods to the `EntityService` interface (after the existing App methods):

```go
	ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]Environment, error)
	CreateEnvironment(ctx context.Context, env *Environment) error
	GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Environment, error)
	UpdateEnvironment(ctx context.Context, env *Environment) error
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
```

- [ ] **Step 2: Add envRepo field and update constructor**

Add `envRepo *EnvironmentRepository` to the `entityService` struct. Update `NewEntityService` to accept and store it:

```go
type entityService struct {
	repo    EntityRepository
	envRepo *EnvironmentRepository
}

func NewEntityService(repo EntityRepository, envRepo *EnvironmentRepository) EntityService {
	return &entityService{repo: repo, envRepo: envRepo}
}
```

- [ ] **Step 3: Add environment service method implementations**

Add after the existing App methods in `service.go`:

```go
func (s *entityService) ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]Environment, error) {
	return s.envRepo.ListByOrg(ctx, orgID)
}

func (s *entityService) CreateEnvironment(ctx context.Context, env *Environment) error {
	return s.envRepo.Create(ctx, env)
}

func (s *entityService) GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Environment, error) {
	return s.envRepo.GetBySlug(ctx, orgID, slug)
}

func (s *entityService) UpdateEnvironment(ctx context.Context, env *Environment) error {
	return s.envRepo.Update(ctx, env)
}

func (s *entityService) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	return s.envRepo.Delete(ctx, id)
}
```

- [ ] **Step 4: Add environment handler methods**

Add to `internal/entities/handler.go` after the `listEnvironments` method:

```go
func (h *Handler) listOrgEnvironments(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	envs, err := h.service.ListEnvironments(c.Request.Context(), org.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"environments": envs})
}

func (h *Handler) createEnvironment(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	var req struct {
		Name         string `json:"name" binding:"required"`
		Slug         string `json:"slug" binding:"required"`
		IsProduction bool   `json:"is_production"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	env := &Environment{
		OrgID:        org.ID,
		Name:         req.Name,
		Slug:         req.Slug,
		IsProduction: req.IsProduction,
	}
	if err := h.service.CreateEnvironment(c.Request.Context(), env); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, env)
}

func (h *Handler) updateEnvironment(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	env, err := h.service.GetEnvironmentBySlug(c.Request.Context(), org.ID, c.Param("envSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	var req struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		IsProduction *bool  `json:"is_production"`
		SortOrder    *int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != "" {
		env.Name = req.Name
	}
	if req.Slug != "" {
		env.Slug = req.Slug
	}
	if req.IsProduction != nil {
		env.IsProduction = *req.IsProduction
	}
	if req.SortOrder != nil {
		env.SortOrder = *req.SortOrder
	}
	if err := h.service.UpdateEnvironment(c.Request.Context(), env); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, env)
}

func (h *Handler) deleteEnvironment(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	env, err := h.service.GetEnvironmentBySlug(c.Request.Context(), org.ID, c.Param("envSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if err := h.service.DeleteEnvironment(c.Request.Context(), env.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
```

- [ ] **Step 5: Register environment routes**

In `RegisterRoutes` in `handler.go`, add inside the orgs group (after the projects group block):

```go
		envs := orgs.Group("/:orgSlug/environments")
		{
			envs.GET("", h.listOrgEnvironments)
			envs.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.createEnvironment)
			envs.PUT("/:envSlug", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateEnvironment)
			envs.DELETE("/:envSlug", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.deleteEnvironment)
		}
```

- [ ] **Step 6: Update main.go to pass envRepo to service**

In `cmd/api/main.go`, update the entityService construction. Add the environment repository:

```go
envRepo := entities.NewEnvironmentRepository(db)
entityService := entities.NewEntityService(entityRepo, envRepo)
```

- [ ] **Step 7: Verify build**

```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Build succeeds.

- [ ] **Step 8: Commit**

```bash
git add internal/entities/service.go internal/entities/handler.go cmd/api/main.go
git commit -m "feat: add environment CRUD service, handler, and routes"
```

---

### Task 4: Notification Preferences Handler

**Files:**
- Create: `internal/notifications/preferences_handler.go`
- Modify: `cmd/api/main.go:284-305`

- [ ] **Step 1: Create preferences handler**

Write `internal/notifications/preferences_handler.go`:

```go
package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"deploysentry/internal/auth"
)

// ChannelConfig represents a notification channel's configuration.
type ChannelConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url,omitempty"`
	Channel    string `json:"channel,omitempty"`
	SMTPHost   string `json:"smtp_host,omitempty"`
	SMTPPort   int    `json:"smtp_port,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	From       string `json:"from,omitempty"`
	RoutingKey string `json:"routing_key,omitempty"`
	Source     string `json:"source"`
}

// PreferencesResponse is the GET response for notification preferences.
type PreferencesResponse struct {
	Channels     map[string]ChannelConfig `json:"channels"`
	EventRouting map[string][]string      `json:"event_routing"`
}

// PreferencesRequest is the PUT request to save notification preferences.
type PreferencesRequest struct {
	Channels     map[string]ChannelConfig `json:"channels,omitempty"`
	EventRouting map[string][]string      `json:"event_routing,omitempty"`
}

// PreferencesHandler handles HTTP requests for notification preferences.
type PreferencesHandler struct {
	store   PreferenceStore
	service *NotificationService
	rbac    *auth.RBACChecker
}

// NewPreferencesHandler creates a new PreferencesHandler.
func NewPreferencesHandler(store PreferenceStore, service *NotificationService, rbac *auth.RBACChecker) *PreferencesHandler {
	return &PreferencesHandler{store: store, service: service, rbac: rbac}
}

// RegisterRoutes registers notification preferences routes.
func (h *PreferencesHandler) RegisterRoutes(rg *gin.RouterGroup) {
	prefs := rg.Group("/notifications")
	{
		prefs.GET("/preferences", auth.RequirePermission(h.rbac, auth.PermSettingsRead), h.getPreferences)
		prefs.PUT("/preferences", auth.RequirePermission(h.rbac, auth.PermSettingsWrite), h.savePreferences)
		prefs.DELETE("/preferences", auth.RequirePermission(h.rbac, auth.PermSettingsWrite), h.resetPreferences)
	}
}

func (h *PreferencesHandler) getPreferences(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	orgIDStr, _ := orgID.(string)

	prefs, err := h.store.GetPreferences(c.Request.Context(), "", orgIDStr)
	if err != nil {
		prefs = &NotificationPreferences{
			ProjectID: orgIDStr,
			Rules:     make(map[EventType]ChannelPreferences),
		}
	}

	channels := h.buildChannelConfigs(prefs)
	eventRouting := h.buildEventRouting(prefs)

	c.JSON(http.StatusOK, PreferencesResponse{
		Channels:     channels,
		EventRouting: eventRouting,
	})
}

func (h *PreferencesHandler) savePreferences(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	orgIDStr, _ := orgID.(string)

	var req PreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.store.GetPreferences(c.Request.Context(), "", orgIDStr)
	if err != nil || existing == nil {
		existing = &NotificationPreferences{
			ProjectID: orgIDStr,
			Rules:     make(map[EventType]ChannelPreferences),
		}
	}

	if req.EventRouting != nil {
		for eventStr, channels := range req.EventRouting {
			et := EventType(eventStr)
			cp := ChannelPreferences{Channels: make(map[string]bool)}
			for _, ch := range channels {
				cp.Channels[ch] = true
			}
			existing.Rules[et] = cp
		}
	}

	if err := h.store.SavePreferences(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"saved": true})
}

func (h *PreferencesHandler) resetPreferences(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	orgIDStr, _ := orgID.(string)

	if err := h.store.DeletePreferences(c.Request.Context(), "", orgIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reset": true})
}

func (h *PreferencesHandler) buildChannelConfigs(prefs *NotificationPreferences) map[string]ChannelConfig {
	configs := make(map[string]ChannelConfig)

	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	for _, ch := range h.service.channels {
		name := ch.Name()
		cfg := ChannelConfig{Source: "config"}
		hasEvents := false
		for _, cp := range prefs.Rules {
			if cp.Channels[name] {
				hasEvents = true
				break
			}
		}
		cfg.Enabled = hasEvents
		configs[name] = cfg
	}

	if _, ok := configs["slack"]; !ok {
		configs["slack"] = ChannelConfig{Enabled: false, Source: "config"}
	}
	if _, ok := configs["email"]; !ok {
		configs["email"] = ChannelConfig{Enabled: false, Source: "config"}
	}
	if _, ok := configs["pagerduty"]; !ok {
		configs["pagerduty"] = ChannelConfig{Enabled: false, Source: "config"}
	}

	return configs
}

func (h *PreferencesHandler) buildEventRouting(prefs *NotificationPreferences) map[string][]string {
	routing := make(map[string][]string)
	for eventType, cp := range prefs.Rules {
		var channels []string
		for ch, enabled := range cp.Channels {
			if enabled {
				channels = append(channels, ch)
			}
		}
		if len(channels) > 0 {
			routing[string(eventType)] = channels
		}
	}
	return routing
}
```

- [ ] **Step 2: Register preferences routes in main.go**

In `cmd/api/main.go`, after the existing route registrations (after line 305), add:

```go
	prefStore := notifications.NewInMemoryPreferenceStore()
	notifications.NewPreferencesHandler(prefStore, notificationService, rbacChecker).RegisterRoutes(api)
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/notifications/preferences_handler.go cmd/api/main.go
git commit -m "feat: add notification preferences HTTP handler"
```

---

### Task 5: Frontend API Modules

**Files:**
- Modify: `web/src/api.ts:275-350`

- [ ] **Step 1: Add webhooksApi module**

In `web/src/api.ts`, add after the `settingsApi` block:

```typescript
// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

export interface Webhook {
  id: string;
  url: string;
  events: string[];
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export const webhooksApi = {
  list: () => request<{ webhooks: Webhook[] }>('/api/v1/webhooks'),
  get: (id: string) => request<Webhook>(`/api/v1/webhooks/${id}`),
  create: (data: { url: string; events: string[]; is_active?: boolean }) =>
    request<Webhook>('/api/v1/webhooks', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (id: string, data: Partial<{ url: string; events: string[]; is_active: boolean }>) =>
    request<Webhook>(`/api/v1/webhooks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  delete: (id: string) =>
    request<{ deleted: boolean }>(`/api/v1/webhooks/${id}`, { method: 'DELETE' }),
  test: (id: string) =>
    request<{ success: boolean; status_code: number }>(`/api/v1/webhooks/${id}/test`, {
      method: 'POST',
    }),
  deliveries: (id: string) =>
    request<{ deliveries: unknown[] }>(`/api/v1/webhooks/${id}/deliveries`),
};
```

- [ ] **Step 2: Add notificationsApi module**

Add after `webhooksApi`:

```typescript
// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

export interface ChannelConfig {
  enabled: boolean;
  webhook_url?: string;
  channel?: string;
  smtp_host?: string;
  smtp_port?: number;
  username?: string;
  password?: string;
  from?: string;
  routing_key?: string;
  source: 'config' | 'api';
}

export interface NotificationPreferences {
  channels: Record<string, ChannelConfig>;
  event_routing: Record<string, string[]>;
}

export const notificationsApi = {
  getPreferences: () =>
    request<NotificationPreferences>('/api/v1/notifications/preferences'),
  savePreferences: (data: {
    channels?: Record<string, Partial<ChannelConfig>>;
    event_routing?: Record<string, string[]>;
  }) =>
    request<{ saved: boolean }>('/api/v1/notifications/preferences', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  resetPreferences: () =>
    request<{ reset: boolean }>('/api/v1/notifications/preferences', {
      method: 'DELETE',
    }),
};
```

- [ ] **Step 3: Add environmentsApi functions to entitiesApi**

In the existing `entitiesApi` block, add after the `listEnvironments` function:

```typescript
  listOrgEnvironments: (orgSlug: string) =>
    request<{ environments: OrgEnvironment[] }>(`/api/v1/orgs/${orgSlug}/environments`),
  createEnvironment: (orgSlug: string, data: { name: string; slug: string; is_production: boolean }) =>
    request<OrgEnvironment>(`/api/v1/orgs/${orgSlug}/environments`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateEnvironment: (orgSlug: string, envSlug: string, data: Partial<{ name: string; slug: string; is_production: boolean; sort_order: number }>) =>
    request<OrgEnvironment>(`/api/v1/orgs/${orgSlug}/environments/${envSlug}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteEnvironment: (orgSlug: string, envSlug: string) =>
    request<{ deleted: boolean }>(`/api/v1/orgs/${orgSlug}/environments/${envSlug}`, {
      method: 'DELETE',
    }),
```

- [ ] **Step 4: Commit**

```bash
git add web/src/api.ts
git commit -m "feat: add webhooksApi, notificationsApi, and environment CRUD to api.ts"
```

---

### Task 6: Frontend Hooks

**Files:**
- Create: `web/src/hooks/useWebhooks.ts`
- Create: `web/src/hooks/useNotifications.ts`
- Modify: `web/src/hooks/useEntities.ts:81-110`

- [ ] **Step 1: Create useWebhooks hook**

Write `web/src/hooks/useWebhooks.ts`:

```typescript
import { useCallback, useEffect, useState } from 'react';
import { webhooksApi, Webhook } from '../api';

export function useWebhooks() {
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    setError(null);
    webhooksApi
      .list()
      .then((res) => setWebhooks(res.webhooks ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { webhooks, loading, error, refresh };
}
```

- [ ] **Step 2: Create useNotifications hook**

Write `web/src/hooks/useNotifications.ts`:

```typescript
import { useCallback, useEffect, useState } from 'react';
import { notificationsApi, NotificationPreferences } from '../api';

export function useNotifications() {
  const [preferences, setPreferences] = useState<NotificationPreferences | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const refresh = useCallback(() => {
    setLoading(true);
    setError(null);
    notificationsApi
      .getPreferences()
      .then((res) => setPreferences(res))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const save = useCallback(
    async (data: Parameters<typeof notificationsApi.savePreferences>[0]) => {
      setSaving(true);
      try {
        await notificationsApi.savePreferences(data);
        await refresh();
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : 'Save failed';
        setError(message);
        throw err;
      } finally {
        setSaving(false);
      }
    },
    [refresh],
  );

  const reset = useCallback(async () => {
    setSaving(true);
    try {
      await notificationsApi.resetPreferences();
      await refresh();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Reset failed';
      setError(message);
      throw err;
    } finally {
      setSaving(false);
    }
  }, [refresh]);

  return { preferences, loading, error, saving, refresh, save, reset };
}
```

- [ ] **Step 3: Update useEnvironments hook to use org-level endpoint**

In `web/src/hooks/useEntities.ts`, update the `useEnvironments` hook to accept just `orgSlug` and call the org-level endpoint:

Replace the existing `useEnvironments` function with:

```typescript
export function useEnvironments(orgSlug?: string) {
  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    entitiesApi
      .listOrgEnvironments(orgSlug)
      .then((res) => setEnvironments(res.environments ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { environments, loading, error, refresh };
}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useWebhooks.ts web/src/hooks/useNotifications.ts web/src/hooks/useEntities.ts
git commit -m "feat: add useWebhooks, useNotifications hooks, update useEnvironments"
```

---

### Task 7: Wire Environments Tab

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx:100-104,203-321`

- [ ] **Step 1: Replace local environment state with hook**

At the top of the SettingsPage component, remove the local environment state variables (lines 100-104):

```typescript
// REMOVE these lines:
const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
const [newEnvName, setNewEnvName] = useState('');
const [newEnvSlug, setNewEnvSlug] = useState('');
const [newEnvIsProd, setNewEnvIsProd] = useState(false);
const [confirmDeleteEnv, setConfirmDeleteEnv] = useState<string | null>(null);
```

Replace with:

```typescript
const { environments, loading: envsLoading, error: envsError, refresh: refreshEnvs } = useEnvironments(orgSlug);
const [newEnvName, setNewEnvName] = useState('');
const [newEnvSlug, setNewEnvSlug] = useState('');
const [newEnvIsProd, setNewEnvIsProd] = useState(false);
const [confirmDeleteEnv, setConfirmDeleteEnv] = useState<string | null>(null);
const [envSaving, setEnvSaving] = useState(false);
```

Add the import at the top of the file:

```typescript
import { useEnvironments } from '../hooks/useEntities';
import { entitiesApi } from '../api';
```

- [ ] **Step 2: Update environment handlers to call API**

Replace `handleAddEnvironment` with:

```typescript
const handleAddEnvironment = async () => {
  if (!newEnvName.trim() || !orgSlug) return;
  setEnvSaving(true);
  try {
    await entitiesApi.createEnvironment(orgSlug, {
      name: newEnvName,
      slug: newEnvSlug || newEnvName.toLowerCase().replace(/\s+/g, '-'),
      is_production: newEnvIsProd,
    });
    setNewEnvName('');
    setNewEnvSlug('');
    setNewEnvIsProd(false);
    refreshEnvs();
  } catch (err) {
    console.error('Failed to create environment:', err);
  } finally {
    setEnvSaving(false);
  }
};
```

Replace `handleDeleteEnvironment` with:

```typescript
const handleDeleteEnvironment = async (envSlug: string) => {
  if (!orgSlug) return;
  try {
    await entitiesApi.deleteEnvironment(orgSlug, envSlug);
    setConfirmDeleteEnv(null);
    refreshEnvs();
  } catch (err) {
    console.error('Failed to delete environment:', err);
  }
};
```

- [ ] **Step 3: Add loading/error states to environments tab rendering**

In the environments tab section (around line 203), add at the top of the tab content:

```typescript
{envsLoading && <div className="text-gray-400 py-4">Loading environments...</div>}
{envsError && (
  <div className="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4">
    <span className="text-red-400">Error: {envsError}</span>
    <button onClick={refreshEnvs} className="ml-2 text-sm text-blue-400 hover:underline">Retry</button>
  </div>
)}
```

- [ ] **Step 4: Remove the "local to this session" warning note**

Find and remove the line that says "Environment changes are local to this session. Backend persistence coming soon."

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: wire environments tab to real API"
```

---

### Task 8: Wire Webhooks Tab

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx:55-82,324-371`

- [ ] **Step 1: Remove mock data and add hook**

Remove the `MOCK_WEBHOOKS` constant (lines 67-82) and the local `Webhook` interface (lines 55-61) from SettingsPage.

Add imports:

```typescript
import { useWebhooks } from '../hooks/useWebhooks';
import { webhooksApi, Webhook } from '../api';
```

Add inside the component:

```typescript
const { webhooks, loading: webhooksLoading, error: webhooksError, refresh: refreshWebhooks } = useWebhooks();
const [addingWebhook, setAddingWebhook] = useState(false);
const [editingWebhookId, setEditingWebhookId] = useState<string | null>(null);
const [webhookUrl, setWebhookUrl] = useState('');
const [webhookEvents, setWebhookEvents] = useState<string[]>([]);
const [webhookActive, setWebhookActive] = useState(true);
const [webhookSaving, setWebhookSaving] = useState(false);
const [testingWebhookId, setTestingWebhookId] = useState<string | null>(null);
const [testResult, setTestResult] = useState<{ id: string; success: boolean } | null>(null);
```

- [ ] **Step 2: Add webhook handler functions**

Add inside the component:

```typescript
const handleSaveWebhook = async () => {
  if (!webhookUrl.trim()) return;
  setWebhookSaving(true);
  try {
    if (editingWebhookId) {
      await webhooksApi.update(editingWebhookId, {
        url: webhookUrl,
        events: webhookEvents,
        is_active: webhookActive,
      });
    } else {
      await webhooksApi.create({
        url: webhookUrl,
        events: webhookEvents,
        is_active: webhookActive,
      });
    }
    setAddingWebhook(false);
    setEditingWebhookId(null);
    setWebhookUrl('');
    setWebhookEvents([]);
    setWebhookActive(true);
    refreshWebhooks();
  } catch (err) {
    console.error('Failed to save webhook:', err);
  } finally {
    setWebhookSaving(false);
  }
};

const handleDeleteWebhook = async (id: string) => {
  try {
    await webhooksApi.delete(id);
    refreshWebhooks();
  } catch (err) {
    console.error('Failed to delete webhook:', err);
  }
};

const handleTestWebhook = async (id: string) => {
  setTestingWebhookId(id);
  setTestResult(null);
  try {
    const res = await webhooksApi.test(id);
    setTestResult({ id, success: res.success });
  } catch {
    setTestResult({ id, success: false });
  } finally {
    setTestingWebhookId(null);
  }
};

const startEditWebhook = (wh: Webhook) => {
  setEditingWebhookId(wh.id);
  setWebhookUrl(wh.url);
  setWebhookEvents(wh.events);
  setWebhookActive(wh.is_active);
  setAddingWebhook(false);
};

const cancelWebhookForm = () => {
  setAddingWebhook(false);
  setEditingWebhookId(null);
  setWebhookUrl('');
  setWebhookEvents([]);
  setWebhookActive(true);
};
```

- [ ] **Step 3: Replace webhooks tab rendering**

Replace the webhooks tab section (lines 324-371) with:

```tsx
{activeTab === 'webhooks' && (
  <div>
    <div className="flex justify-between items-center mb-4">
      <h3 className="text-lg font-semibold text-white">Webhooks</h3>
      {!addingWebhook && !editingWebhookId && (
        <button
          onClick={() => setAddingWebhook(true)}
          className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded"
        >
          Add Webhook
        </button>
      )}
    </div>

    {webhooksLoading && <div className="text-gray-400 py-4">Loading webhooks...</div>}
    {webhooksError && (
      <div className="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4">
        <span className="text-red-400">Error: {webhooksError}</span>
        <button onClick={refreshWebhooks} className="ml-2 text-sm text-blue-400 hover:underline">Retry</button>
      </div>
    )}

    <div className="space-y-2">
      {addingWebhook && (
        <div className="bg-gray-800 border border-gray-700 rounded p-4 space-y-3">
          <input
            type="url"
            placeholder="https://example.com/webhook"
            value={webhookUrl}
            onChange={(e) => setWebhookUrl(e.target.value)}
            className="w-full bg-gray-900 border border-gray-700 rounded px-3 py-2 text-white text-sm"
          />
          <input
            type="text"
            placeholder="Events (comma-separated, e.g. deploy.completed,deploy.failed)"
            value={webhookEvents.join(',')}
            onChange={(e) => setWebhookEvents(e.target.value.split(',').map((s) => s.trim()).filter(Boolean))}
            className="w-full bg-gray-900 border border-gray-700 rounded px-3 py-2 text-white text-sm"
          />
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm text-gray-300">
              <input type="checkbox" checked={webhookActive} onChange={(e) => setWebhookActive(e.target.checked)} />
              Active
            </label>
            <div className="flex-1" />
            <button onClick={cancelWebhookForm} className="px-3 py-1.5 text-gray-400 hover:text-white text-sm">Cancel</button>
            <button
              onClick={handleSaveWebhook}
              disabled={webhookSaving || !webhookUrl.trim()}
              className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded"
            >
              {webhookSaving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      )}

      {webhooks.map((wh) =>
        editingWebhookId === wh.id ? (
          <div key={wh.id} className="bg-gray-800 border border-blue-500/30 rounded p-4 space-y-3">
            <input
              type="url"
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              className="w-full bg-gray-900 border border-gray-700 rounded px-3 py-2 text-white text-sm"
            />
            <input
              type="text"
              value={webhookEvents.join(',')}
              onChange={(e) => setWebhookEvents(e.target.value.split(',').map((s) => s.trim()).filter(Boolean))}
              className="w-full bg-gray-900 border border-gray-700 rounded px-3 py-2 text-white text-sm"
            />
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 text-sm text-gray-300">
                <input type="checkbox" checked={webhookActive} onChange={(e) => setWebhookActive(e.target.checked)} />
                Active
              </label>
              <div className="flex-1" />
              <button onClick={cancelWebhookForm} className="px-3 py-1.5 text-gray-400 hover:text-white text-sm">Cancel</button>
              <button
                onClick={handleSaveWebhook}
                disabled={webhookSaving}
                className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded"
              >
                {webhookSaving ? 'Saving...' : 'Update'}
              </button>
            </div>
          </div>
        ) : (
          <div key={wh.id} className="bg-gray-800 border border-gray-700 rounded p-4 flex items-center justify-between">
            <div>
              <div className="text-white text-sm font-mono">{wh.url}</div>
              <div className="text-gray-400 text-xs mt-1">
                {wh.events.join(', ')} &middot;{' '}
                <span className={wh.is_active ? 'text-green-400' : 'text-gray-500'}>
                  {wh.is_active ? 'Active' : 'Inactive'}
                </span>
              </div>
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => handleTestWebhook(wh.id)}
                disabled={testingWebhookId === wh.id}
                className="px-2 py-1 text-xs text-gray-400 hover:text-white border border-gray-600 rounded"
              >
                {testingWebhookId === wh.id ? 'Testing...' : 'Test'}
              </button>
              {testResult?.id === wh.id && (
                <span className={`text-xs py-1 ${testResult.success ? 'text-green-400' : 'text-red-400'}`}>
                  {testResult.success ? 'OK' : 'Failed'}
                </span>
              )}
              <button onClick={() => startEditWebhook(wh)} className="px-2 py-1 text-xs text-blue-400 hover:text-blue-300 border border-gray-600 rounded">Edit</button>
              <button onClick={() => handleDeleteWebhook(wh.id)} className="px-2 py-1 text-xs text-red-400 hover:text-red-300 border border-gray-600 rounded">Delete</button>
            </div>
          </div>
        ),
      )}

      {!webhooksLoading && webhooks.length === 0 && !addingWebhook && (
        <div className="text-gray-500 text-sm py-4">No webhooks configured.</div>
      )}
    </div>
  </div>
)}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: wire webhooks tab to real API with inline CRUD"
```

---

### Task 9: Wire Notifications Tab

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx:84-90,107-120,374-545`

- [ ] **Step 1: Add notifications hook and remove hardcoded events**

Remove the `NOTIFICATION_EVENTS` constant (lines 84-90).

Add import:

```typescript
import { useNotifications } from '../hooks/useNotifications';
```

Replace notification state variables (lines 107-120) with:

```typescript
const { preferences: notifPrefs, loading: notifLoading, error: notifError, saving: notifSaving, save: saveNotifPrefs, reset: resetNotifPrefs } = useNotifications();
const [notifSuccess, setNotifSuccess] = useState(false);
```

- [ ] **Step 2: Replace notifications tab rendering**

Replace the notifications tab section (lines 374-545) with:

```tsx
{activeTab === 'notifications' && (
  <div>
    <h3 className="text-lg font-semibold text-white mb-4">Notification Settings</h3>

    {notifLoading && <div className="text-gray-400 py-4">Loading notification preferences...</div>}
    {notifError && (
      <div className="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4">
        <span className="text-red-400">Error: {notifError}</span>
      </div>
    )}

    {notifPrefs && (
      <div className="space-y-6">
        {/* Channels */}
        {Object.entries(notifPrefs.channels).map(([name, cfg]) => (
          <div key={name} className="bg-gray-800 border border-gray-700 rounded p-4">
            <div className="flex items-center justify-between mb-3">
              <h4 className="text-white font-medium capitalize">{name}</h4>
              <span className={`text-xs px-2 py-0.5 rounded ${cfg.source === 'config' ? 'bg-gray-700 text-gray-400' : 'bg-blue-900 text-blue-300'}`}>
                {cfg.source === 'config' ? 'From config' : 'Custom'}
              </span>
            </div>
            <div className="text-gray-400 text-sm">
              {cfg.enabled ? 'Enabled' : 'Not enabled'} for events
            </div>
          </div>
        ))}

        {/* Event Routing */}
        <div className="bg-gray-800 border border-gray-700 rounded p-4">
          <h4 className="text-white font-medium mb-3">Event Routing</h4>
          <div className="space-y-2">
            {Object.entries(notifPrefs.event_routing).length === 0 && (
              <div className="text-gray-500 text-sm">No event routing configured.</div>
            )}
            {Object.entries(notifPrefs.event_routing).map(([event, channels]) => (
              <div key={event} className="flex items-center justify-between text-sm">
                <span className="text-gray-300 font-mono">{event}</span>
                <span className="text-gray-400">{channels.join(', ')}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Actions */}
        <div className="flex gap-3">
          <button
            onClick={async () => {
              try {
                await saveNotifPrefs({});
                setNotifSuccess(true);
                setTimeout(() => setNotifSuccess(false), 3000);
              } catch {}
            }}
            disabled={notifSaving}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded"
          >
            {notifSaving ? 'Saving...' : 'Save Notification Settings'}
          </button>
          <button
            onClick={async () => {
              if (confirm('Reset all notification preferences to defaults?')) {
                await resetNotifPrefs();
              }
            }}
            disabled={notifSaving}
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 disabled:opacity-50 text-gray-300 text-sm rounded"
          >
            Reset to Defaults
          </button>
          {notifSuccess && <span className="text-green-400 text-sm self-center">Saved!</span>}
        </div>
      </div>
    )}
  </div>
)}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: wire notifications tab to preferences API"
```

---

### Task 10: Wire Project General and App General/Danger Tabs

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx:123-130,548-656`

- [ ] **Step 1: Add settings and entities imports**

Ensure these imports are present at the top of SettingsPage:

```typescript
import { settingsApi, entitiesApi } from '../api';
```

- [ ] **Step 2: Add save handlers for project settings**

Add state for save feedback:

```typescript
const [settingsSaving, setSettingsSaving] = useState(false);
const [settingsSuccess, setSettingsSuccess] = useState(false);
```

Add handler:

```typescript
const handleSaveProjectSettings = async () => {
  if (!projectSlug || !orgSlug) return;
  setSettingsSaving(true);
  try {
    const project = await entitiesApi.getProject(orgSlug, projectSlug);
    await entitiesApi.updateProject(orgSlug, projectSlug, {
      name: projectName || project.name,
    });
    setSettingsSuccess(true);
    setTimeout(() => setSettingsSuccess(false), 3000);
  } catch (err) {
    console.error('Failed to save project settings:', err);
  } finally {
    setSettingsSaving(false);
  }
};
```

- [ ] **Step 3: Wire project general Save button**

In the project general tab section (around line 548-592), find the Save button and update it:

```tsx
<button
  onClick={handleSaveProjectSettings}
  disabled={settingsSaving}
  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded"
>
  {settingsSaving ? 'Saving...' : 'Save Project Settings'}
</button>
{settingsSuccess && <span className="text-green-400 text-sm ml-3">Saved!</span>}
```

- [ ] **Step 4: Add save handler for app settings**

```typescript
const handleSaveAppSettings = async () => {
  if (!orgSlug || !projectSlug || !appSlug) return;
  setSettingsSaving(true);
  try {
    await entitiesApi.updateApp(orgSlug, projectSlug, appSlug, {
      name: appName,
      description: appDescription,
      repository_url: appRepoUrl,
    });
    setSettingsSuccess(true);
    setTimeout(() => setSettingsSuccess(false), 3000);
  } catch (err) {
    console.error('Failed to save app settings:', err);
  } finally {
    setSettingsSaving(false);
  }
};
```

- [ ] **Step 5: Wire app general Save button**

In the app general tab section (around line 595-644), find the Save button and update it:

```tsx
<button
  onClick={handleSaveAppSettings}
  disabled={settingsSaving}
  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm rounded"
>
  {settingsSaving ? 'Saving...' : 'Save App Settings'}
</button>
{settingsSuccess && <span className="text-green-400 text-sm ml-3">Saved!</span>}
```

- [ ] **Step 6: Wire app Delete button in danger zone**

```typescript
const handleDeleteApp = async () => {
  if (!orgSlug || !projectSlug || !appSlug) return;
  const confirmed = prompt(`Type "${appSlug}" to confirm deletion:`);
  if (confirmed !== appSlug) return;
  try {
    await entitiesApi.deleteApp(orgSlug, projectSlug, appSlug);
    window.location.href = `/orgs/${orgSlug}/projects/${projectSlug}`;
  } catch (err) {
    console.error('Failed to delete app:', err);
  }
};
```

Update the Delete button in the danger zone section:

```tsx
<button
  onClick={handleDeleteApp}
  className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white text-sm rounded"
>
  Delete Application
</button>
```

- [ ] **Step 7: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: wire project general, app general, and danger zone tabs"
```

---

### Task 11: Verify Build and Clean Up

**Files:**
- Verify: all modified files

- [ ] **Step 1: Verify Go build**

```bash
cd /Users/sgamel/git/DeploySentry && go build ./...
```

Expected: Build succeeds with no errors.

- [ ] **Step 2: Verify frontend build**

```bash
cd /Users/sgamel/git/DeploySentry/web && npm run build 2>&1 | tail -5
```

Expected: Build succeeds (or shows only pre-existing warnings).

- [ ] **Step 3: Run Go tests**

```bash
cd /Users/sgamel/git/DeploySentry && go test ./... 2>&1 | tail -20
```

Expected: All tests pass.

- [ ] **Step 4: Verify no MOCK_ constants remain in SettingsPage**

```bash
grep -n "MOCK_" web/src/pages/SettingsPage.tsx
```

Expected: No matches.

- [ ] **Step 5: Verify no "local to this session" warning remains**

```bash
grep -n "local to this session" web/src/pages/SettingsPage.tsx
```

Expected: No matches.

- [ ] **Step 6: Update plan doc phase**

Update `docs/Current_Initiatives.md` to reflect SettingsPage wiring is complete.

- [ ] **Step 7: Commit any cleanup**

```bash
git add -A
git commit -m "chore: clean up SettingsPage wiring, update docs"
```
