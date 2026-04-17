# Flag Detail Page Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance FlagDetailPage with a Settings tab, History (audit log) tab, fix created_by display, improve header spacing, and reorder tabs.

**Architecture:** Backend changes add audit log writes to all flag mutation handlers and resolve user names in flag/audit responses. Frontend adds two new tabs (Settings form, History timeline) and fixes the header display. All changes build on existing infrastructure (audit_log table, flagsApi.update, AuditLogRepository).

**Tech Stack:** Go/Gin (backend), React/TypeScript (frontend), PostgreSQL (audit_log table)

**Spec:** `docs/superpowers/specs/2026-04-17-flag-detail-enhancements-design.md`

---

## File Map

### Backend
- **Modify:** `internal/models/audit.go` — Add `ActorName` field, add `WriteAuditLog` to interface concept
- **Modify:** `internal/platform/database/postgres/audit.go` — Add `WriteAuditLog` insert method, add actor name LEFT JOIN, add `resource_id` filter
- **Modify:** `internal/auth/audit_handler.go` — Add `resource_id` query param to `AuditLogFilter` and handler
- **Modify:** `internal/flags/handler.go` — Add `AuditWriter` interface and field, inject via constructor, add audit writes to all mutation handlers, add user name resolution in `getFlag`
- **Modify:** `cmd/api/main.go` — Pass `auditRepo` to flags handler constructor

### Frontend
- **Modify:** `web/src/types.ts` — Add `created_by_name` to Flag, add `AuditLogEntry` type
- **Modify:** `web/src/api.ts` — Add `auditApi.query()` function
- **Modify:** `web/src/pages/FlagDetailPage.tsx` — Reorder tabs, add Settings tab, add History tab, fix created_by, fix header spacing
- **Modify:** `web/src/styles/globals.css` — Add gap/separator to `detail-secondary`

---

### Task 1: Add `WriteAuditLog` method to audit repository

**Files:**
- Modify: `internal/platform/database/postgres/audit.go`
- Modify: `internal/models/audit.go`

- [ ] **Step 1: Add `ActorName` to the AuditLogEntry model**

In `internal/models/audit.go`, add the `ActorName` field:

```go
type AuditLogEntry struct {
	ID         uuid.UUID `json:"id" db:"id"`
	OrgID      uuid.UUID `json:"org_id" db:"org_id"`
	ProjectID  uuid.UUID `json:"project_id,omitempty" db:"project_id"`
	ActorID    uuid.UUID `json:"actor_id" db:"actor_id"`
	ActorName  string    `json:"actor_name,omitempty" db:"-"`
	Action     string    `json:"action" db:"action"`
	EntityType string    `json:"entity_type" db:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id" db:"entity_id"`
	OldValue   string    `json:"old_value,omitempty" db:"old_value"`
	NewValue   string    `json:"new_value,omitempty" db:"new_value"`
	IPAddress  string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
```

- [ ] **Step 2: Add `WriteAuditLog` method to `AuditLogRepository`**

In `internal/platform/database/postgres/audit.go`, add after the existing `QueryAuditLogs` method:

```go
// WriteAuditLog inserts a single audit log entry.
func (r *AuditLogRepository) WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	const q = `
		INSERT INTO audit_log (id, org_id, project_id, user_id, action, resource_type, resource_id, old_value, new_value, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::inet, $11, $12)`
	_, err := r.pool.Exec(ctx, q,
		entry.ID, entry.OrgID, entry.ProjectID, entry.ActorID,
		entry.Action, entry.EntityType, entry.EntityID,
		nullIfEmpty(entry.OldValue), nullIfEmpty(entry.NewValue),
		nullIfEmpty(entry.IPAddress), entry.UserAgent, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.WriteAuditLog: %w", err)
	}
	return nil
}

// nullIfEmpty returns nil for empty strings (so postgres stores NULL instead of '').
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
```

Add import for `"github.com/google/uuid"` if not already present.

- [ ] **Step 3: Add actor name LEFT JOIN and resource_id filter to `QueryAuditLogs`**

In `internal/platform/database/postgres/audit.go`, update the `QueryAuditLogs` method:

Add to the filter handling block (after the `ResourceType` filter):

```go
if filter.ResourceID != nil {
	wb.Add("a.resource_id = $%d", *filter.ResourceID)
}
```

Update the count query to use the alias:
```go
countQ := `SELECT COUNT(*) FROM audit_log a` + where
```

Update the SELECT query to join users and use alias:
```go
selectQ := `
	SELECT
		a.id, a.org_id,
		COALESCE(a.project_id, '00000000-0000-0000-0000-000000000000'::uuid),
		a.user_id, COALESCE(u.name, ''), a.action, a.resource_type,
		COALESCE(a.resource_id, '00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(a.old_value::text, ''),
		COALESCE(a.new_value::text, ''),
		COALESCE(a.ip_address::text, ''),
		COALESCE(a.user_agent, ''),
		a.created_at
	FROM audit_log a
	LEFT JOIN users u ON u.id = a.user_id` + where + ` ORDER BY a.created_at DESC` + pagClause
```

Update the Scan to include ActorName:
```go
if err := rows.Scan(
	&e.ID,
	&e.OrgID,
	&e.ProjectID,
	&e.ActorID,
	&e.ActorName,
	&e.Action,
	&e.EntityType,
	&e.EntityID,
	&e.OldValue,
	&e.NewValue,
	&e.IPAddress,
	&e.UserAgent,
	&e.CreatedAt,
); err != nil {
```

- [ ] **Step 4: Add `ResourceID` to `AuditLogFilter` and handler**

In `internal/auth/audit_handler.go`, add to the `AuditLogFilter` struct:

```go
ResourceID   *uuid.UUID `json:"resource_id,omitempty"`
```

Add to the handler's `queryAuditLog` function, after the `user_id` parsing block:

```go
// Parse optional resource_id filter.
if ridStr := c.Query("resource_id"); ridStr != "" {
	rid, err := uuid.Parse(ridStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	filter.ResourceID = &rid
}
```

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/models/audit.go internal/platform/database/postgres/audit.go internal/auth/audit_handler.go
git commit -m "feat: add WriteAuditLog method, actor name join, resource_id filter"
```

---

### Task 2: Wire audit writer into flags handler

**Files:**
- Modify: `internal/flags/handler.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add AuditWriter interface and field to handler**

In `internal/flags/handler.go`, add the interface after the existing `EnvironmentSlugResolver` interface:

```go
// AuditWriter persists audit log entries.
type AuditWriter interface {
	WriteAuditLog(ctx context.Context, entry *models.AuditLogEntry) error
}
```

Add `auditWriter` field to the `Handler` struct:

```go
type Handler struct {
	service      FlagService
	rbac         *auth.RBACChecker
	sse          *SSEBroker
	webhookSvc   *webhooks.Service
	analyticsSvc *analytics.Service
	ratingSvc    FlagRatingSvc
	entityRepo   EntityResolver
	envResolver  EnvironmentSlugResolver
	auditWriter  AuditWriter
}
```

Update `NewHandler` signature and body:

```go
func NewHandler(service FlagService, rbac *auth.RBACChecker, webhookSvc *webhooks.Service, analyticsSvc *analytics.Service, entityRepo EntityResolver, envResolver EnvironmentSlugResolver, auditWriter AuditWriter) *Handler {
	return &Handler{
		service:      service,
		rbac:         rbac,
		sse:          NewSSEBroker(),
		webhookSvc:   webhookSvc,
		analyticsSvc: analyticsSvc,
		entityRepo:   entityRepo,
		envResolver:  envResolver,
		auditWriter:  auditWriter,
	}
}
```

- [ ] **Step 2: Add audit helper method**

Add a helper to reduce boilerplate in each handler. Place it near the `broadcastEvent` method:

```go
// writeAudit records an audit log entry. Failures are logged but do not fail the request.
func (h *Handler) writeAudit(c *gin.Context, action, entityType string, entityID uuid.UUID, oldValue, newValue string) {
	if h.auditWriter == nil {
		return
	}
	var actorID uuid.UUID
	if uid, exists := c.Get("user_id"); exists {
		actorID, _ = uid.(uuid.UUID)
	}
	var orgID uuid.UUID
	if oid := c.GetString("org_id"); oid != "" {
		orgID, _ = uuid.Parse(oid)
	}

	entry := &models.AuditLogEntry{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldValue,
		NewValue:   newValue,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		CreatedAt:  time.Now(),
	}
	if err := h.auditWriter.WriteAuditLog(c.Request.Context(), entry); err != nil {
		log.Printf("failed to write audit log: %v", err)
	}
}
```

- [ ] **Step 3: Update main.go**

In `cmd/api/main.go`, update the `flags.NewHandler` call (line ~339):

```go
flagHandler := flags.NewHandler(flagService, rbacChecker, webhookService, analyticsService, entityRepo, envRepo, auditRepo)
```

`auditRepo` is already created at line 219.

- [ ] **Step 4: Fix test files**

In `internal/flags/handler_test.go` and `internal/flags/segment_handler_test.go`, update all `NewHandler` calls from 6 args to 7:

```go
NewHandler(svc, rbac, nil, nil, nil, nil, nil)
```

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./internal/flags/...`
Expected: Clean build, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/flags/handler.go cmd/api/main.go internal/flags/handler_test.go internal/flags/segment_handler_test.go
git commit -m "feat: wire audit writer into flags handler"
```

---

### Task 3: Instrument all flag mutation handlers with audit writes

**Files:**
- Modify: `internal/flags/handler.go`

- [ ] **Step 1: Add audit write to `createFlag` handler**

After the successful flag creation (after the webhook block, before the `c.JSON` response), add:

```go
newVal, _ := json.Marshal(map[string]interface{}{
	"key": flag.Key, "name": flag.Name, "flag_type": flag.FlagType,
	"category": flag.Category, "default_value": flag.DefaultValue,
})
h.writeAudit(c, "flag.created", "flag", flag.ID, "", string(newVal))
```

- [ ] **Step 2: Add audit write to `updateFlag` handler**

Before the update is applied (after `flag` is fetched but before fields are mutated), capture the old state. After the update succeeds, write the audit entry.

Add before the field mutations (after line ~329 where `flag` is fetched):

```go
oldVal, _ := json.Marshal(map[string]interface{}{
	"name": flag.Name, "description": flag.Description, "category": flag.Category,
	"purpose": flag.Purpose, "owners": flag.Owners, "is_permanent": flag.IsPermanent,
	"expires_at": flag.ExpiresAt, "default_value": flag.DefaultValue, "tags": flag.Tags,
})
```

After the successful update (after `h.service.UpdateFlag` and before the webhook block), add:

```go
newVal, _ := json.Marshal(map[string]interface{}{
	"name": flag.Name, "description": flag.Description, "category": flag.Category,
	"purpose": flag.Purpose, "owners": flag.Owners, "is_permanent": flag.IsPermanent,
	"expires_at": flag.ExpiresAt, "default_value": flag.DefaultValue, "tags": flag.Tags,
})
h.writeAudit(c, "flag.updated", "flag", flag.ID, string(oldVal), string(newVal))
```

- [ ] **Step 3: Add audit write to `toggleFlag` handler**

After successful toggle (after `h.service.ToggleFlag` and before the broadcast), add:

```go
h.writeAudit(c, "flag.toggled", "flag", id,
	fmt.Sprintf(`{"enabled":%v}`, !req.Enabled),
	fmt.Sprintf(`{"enabled":%v}`, req.Enabled))
```

- [ ] **Step 4: Add audit write to `archiveFlag` handler**

After successful archive (after `h.service.ArchiveFlag` and before the broadcast), add:

```go
h.writeAudit(c, "flag.archived", "flag", id, "", "")
```

- [ ] **Step 5: Add audit write to `setFlagEnvState` handler**

In the `setFlagEnvState` handler, before the upsert, fetch the current state for old value. After success, write the audit.

After `envID` is parsed and before the JSON binding, add:

```go
// Fetch current state for audit old value.
currentState, _ := h.service.GetFlagEnvState(c.Request.Context(), flagID, envID)
```

Wait — we need `GetFlagEnvState` on the service. Check if it exists:

The service interface has `ListFlagEnvStates` but not `GetFlagEnvState`. The handler can compute the old value from the list or we can just capture what we have. Simpler: after the successful upsert, write the audit with old/new. We already have the request data. For the old value, query the list:

Actually, we added `GetFlagEnvState` to the repository in a previous session fix. But the service doesn't expose it. Simpler approach — just record the new state in the audit entry. The old state isn't critical for env toggles since the history itself shows the progression.

After the successful `h.service.SetFlagEnvState` call, add:

```go
newVal, _ := json.Marshal(map[string]interface{}{
	"environment_id": envID, "enabled": req.Enabled, "value": req.Value,
})
h.writeAudit(c, "flag.env_state.updated", "flag", flagID, "", string(newVal))
```

- [ ] **Step 6: Add audit write to `addRule` handler**

After the successful rule creation (after `h.service.CreateRule` and before the broadcast), add:

```go
newVal, _ := json.Marshal(map[string]interface{}{
	"rule_id": rule.ID, "attribute": rule.Attribute, "operator": rule.Operator,
	"target_values": rule.TargetValues, "value": rule.Value, "priority": rule.Priority,
})
h.writeAudit(c, "flag.rule.created", "flag", flagID, "", string(newVal))
```

Note: `flagID` is parsed from `c.Param("id")` in the addRule handler.

- [ ] **Step 7: Add audit write to `deleteRule` handler**

In the `deleteRule` handler, after the successful deletion, add:

```go
h.writeAudit(c, "flag.rule.deleted", "flag", flagID, fmt.Sprintf(`{"rule_id":"%s"}`, ruleID), "")
```

Note: `flagID` is available as `c.Param("id")` — parse it at the top of the handler if not already parsed.

Check the current deleteRule handler:

```go
func (h *Handler) deleteRule(c *gin.Context) {
```

Read lines 885-910 to see if flagID is already parsed. If not, add:

```go
flagID, _ := uuid.Parse(c.Param("id"))
```

- [ ] **Step 8: Add audit write to `setRuleEnvState` handler**

After the successful `h.service.SetRuleEnvironmentState` call, add:

```go
flagID, _ := uuid.Parse(c.Param("id"))
newVal, _ := json.Marshal(map[string]interface{}{
	"rule_id": ruleID, "environment_id": envID, "enabled": req.Enabled,
})
h.writeAudit(c, "flag.rule.env_state.updated", "flag", flagID, "", string(newVal))
```

- [ ] **Step 9: Build and test**

Run: `go build ./... && go test ./internal/flags/...`
Expected: Clean build, all tests pass.

- [ ] **Step 10: Commit**

```bash
git add internal/flags/handler.go
git commit -m "feat: instrument all flag mutation handlers with audit log writes"
```

---

### Task 4: Resolve created_by name in getFlag response

**Files:**
- Modify: `internal/flags/handler.go`

- [ ] **Step 1: Add UserNameResolver interface**

Add to the interfaces section of `internal/flags/handler.go`:

```go
// UserNameResolver looks up a user's display name by ID.
type UserNameResolver interface {
	GetUserName(ctx context.Context, id uuid.UUID) (string, error)
}
```

Add `userResolver` field to Handler struct and update NewHandler to accept it.

Actually — this adds complexity to the constructor (already 7 args). Simpler approach: do a direct SQL query in the getFlag handler using the existing entityRepo pattern, OR add the name to the JSON response as a wrapper.

Simplest approach: add a `created_by_name` field to the `flagWithRatings` response wrapper and resolve it in `getFlag`. We need a user lookup. The `entityRepo` (postgres.EntityRepository) has access to the pool. Let's add `GetUserName` to the `EntityResolver` interface.

Add to the `EntityResolver` interface:

```go
type EntityResolver interface {
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	GetUserName(ctx context.Context, id uuid.UUID) (string, error)
}
```

- [ ] **Step 2: Implement `GetUserName` on EntityRepository**

In `internal/platform/database/postgres/entities.go`, add:

```go
// GetUserName returns the display name for a user by ID.
func (r *EntityRepository) GetUserName(ctx context.Context, id uuid.UUID) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(name, email) FROM users WHERE id = $1`, id).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}
```

- [ ] **Step 3: Add `created_by_name` to getFlag response**

In `internal/flags/handler.go`, in the `getFlag` handler, after the flag is fetched (line ~211), resolve the name:

```go
createdByName := ""
if flag.CreatedBy != uuid.Nil {
	if name, err := h.entityRepo.GetUserName(c.Request.Context(), flag.CreatedBy); err == nil {
		createdByName = name
	}
}
```

The `getFlag` handler returns either a `flagWithRatings` wrapper or the raw flag. We need to add `created_by_name` to the response in both cases. Update the `flagWithRatings` struct:

```go
type flagWithRatings struct {
	*models.FeatureFlag
	CreatedByName string               `json:"created_by_name,omitempty"`
	RatingSummary *models.RatingSummary `json:"rating_summary,omitempty"`
	ErrorRate     *models.ErrorSummary  `json:"error_rate,omitempty"`
}
```

In the `getFlag` handler, set `resp.CreatedByName = createdByName` before returning the `flagWithRatings` response. For the non-ratings path, wrap the flag in a `flagWithRatings` as well so the field is always present:

Replace the final `c.JSON(http.StatusOK, flag)` with:

```go
c.JSON(http.StatusOK, &flagWithRatings{FeatureFlag: flag, CreatedByName: createdByName})
```

- [ ] **Step 4: Update mock in tests**

Add `GetUserName` to the mock in `internal/flags/handler_test.go`:

The tests use `nil` for `entityRepo`, so no mock method needed — nil interface won't be called. But if there's an explicit mock struct implementing `EntityResolver`, add:

```go
func (m *mockEntityResolver) GetUserName(ctx context.Context, id uuid.UUID) (string, error) {
	return "", nil
}
```

Check if there's a mock — if tests pass `nil`, this step can be skipped.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./internal/flags/...`
Expected: Clean build, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/flags/handler.go internal/platform/database/postgres/entities.go
git commit -m "feat: resolve created_by user name in flag detail response"
```

---

### Task 5: Frontend — add types and API functions

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add `created_by_name` to Flag type**

In `web/src/types.ts`, add to the `Flag` interface after `created_by`:

```typescript
created_by_name?: string;
```

- [ ] **Step 2: Add AuditLogEntry type**

In `web/src/types.ts`, add:

```typescript
export interface AuditLogEntry {
  id: string;
  org_id: string;
  project_id: string;
  actor_id: string;
  actor_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  old_value: string;
  new_value: string;
  created_at: string;
}
```

- [ ] **Step 3: Add audit log API function**

In `web/src/api.ts`, add:

```typescript
export const auditApi = {
  query: (params: { resource_type?: string; resource_id?: string; limit?: number; offset?: number }) => {
    const qs = new URLSearchParams();
    if (params.resource_type) qs.set('resource_type', params.resource_type);
    if (params.resource_id) qs.set('resource_id', params.resource_id);
    if (params.limit) qs.set('limit', String(params.limit));
    if (params.offset) qs.set('offset', String(params.offset));
    return request<{ entries: AuditLogEntry[]; total: number }>(`/audit-log?${qs}`);
  },
};
```

Add `AuditLogEntry` to the imports from `@/types` at the top of `api.ts` (or wherever types are imported).

- [ ] **Step 4: Type-check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add audit log types and API function for flag history"
```

---

### Task 6: Frontend — reorder tabs, fix header, add Settings tab

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`
- Modify: `web/src/styles/globals.css`

- [ ] **Step 1: Fix header spacing CSS**

In `web/src/styles/globals.css`, update the `.detail-secondary` rule:

```css
.detail-secondary {
  margin-top: 8px;
  font-size: 11px;
  color: var(--color-text-muted);
  display: flex;
  gap: 16px;
  align-items: center;
}

.detail-secondary span + span::before {
  content: '\00b7';
  margin-right: 16px;
  color: var(--color-text-muted);
}
```

- [ ] **Step 2: Fix created_by display**

In `web/src/pages/FlagDetailPage.tsx`, update line 277:

```tsx
<span>Created by {flag.created_by_name || flag.created_by}</span>
```

- [ ] **Step 3: Update activeTab state type and default**

Change the `activeTab` state declaration:

```tsx
const [activeTab, setActiveTab] = useState<'environments' | 'rules' | 'yaml' | 'settings' | 'history'>('environments');
```

- [ ] **Step 4: Reorder tab buttons and add new ones**

Replace the entire `{/* Tabs */}` section with:

```tsx
{/* Tabs */}
<div className="detail-tabs">
  <button
    className={`detail-tab${activeTab === 'environments' ? ' active' : ''}`}
    onClick={() => setActiveTab('environments')}
  >
    Environments
  </button>
  <button
    className={`detail-tab${activeTab === 'rules' ? ' active' : ''}`}
    onClick={() => setActiveTab('rules')}
  >
    Targeting Rules
  </button>
  <button
    className={`detail-tab${activeTab === 'yaml' ? ' active' : ''}`}
    onClick={() => setActiveTab('yaml')}
  >
    YAML
  </button>
  <button
    className={`detail-tab${activeTab === 'settings' ? ' active' : ''}`}
    onClick={() => setActiveTab('settings')}
  >
    Settings
  </button>
  <button
    className={`detail-tab${activeTab === 'history' ? ' active' : ''}`}
    onClick={() => setActiveTab('history')}
  >
    History
  </button>
</div>
```

- [ ] **Step 5: Add settings form state**

Add state for the settings form, after the existing state declarations:

```tsx
const [settingsForm, setSettingsForm] = useState<{
  name: string;
  description: string;
  category: string;
  purpose: string;
  owners: string;
  is_permanent: boolean;
  expires_at: string;
  default_value: string;
  tags: string;
} | null>(null);
const [settingsSaving, setSettingsSaving] = useState(false);
const [settingsSuccess, setSettingsSuccess] = useState(false);
```

Add an effect to initialize the form when the flag loads:

```tsx
useEffect(() => {
  if (flag) {
    setSettingsForm({
      name: flag.name,
      description: flag.description ?? '',
      category: flag.category,
      purpose: flag.purpose ?? '',
      owners: (flag.owners ?? []).join(', '),
      is_permanent: flag.is_permanent,
      expires_at: flag.expires_at ? flag.expires_at.slice(0, 16) : '',
      default_value: flag.default_value,
      tags: (flag.tags ?? []).join(', '),
    });
  }
}, [flag]);
```

- [ ] **Step 6: Add settings save handler**

Add the save handler function (near the other handlers):

```tsx
const handleSettingsSave = async () => {
  if (!flag || !settingsForm || !id) return;
  setSettingsSaving(true);
  setSettingsSuccess(false);
  setError(null);
  try {
    const updated = await flagsApi.update(id, {
      name: settingsForm.name,
      description: settingsForm.description,
      category: settingsForm.category as FlagCategory,
      purpose: settingsForm.purpose,
      owners: settingsForm.owners.split(',').map((s) => s.trim()).filter(Boolean),
      is_permanent: settingsForm.is_permanent,
      expires_at: settingsForm.is_permanent ? undefined : settingsForm.expires_at ? settingsForm.expires_at + ':00Z' : undefined,
      default_value: settingsForm.default_value,
    });
    setFlag(updated);
    setSettingsSuccess(true);
    setTimeout(() => setSettingsSuccess(false), 3000);
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to update flag');
  } finally {
    setSettingsSaving(false);
  }
};
```

- [ ] **Step 7: Add Settings tab JSX**

Add after the YAML tab content block (`{activeTab === 'yaml' && ...}`):

```tsx
{/* Tab: Settings */}
{activeTab === 'settings' && settingsForm && (
  <div className="card">
    <div className="form-group">
      <label className="form-label">Name</label>
      <input className="form-input" value={settingsForm.name}
        onChange={(e) => setSettingsForm({ ...settingsForm, name: e.target.value })} />
    </div>
    <div className="form-group">
      <label className="form-label">Description</label>
      <textarea className="form-input" rows={3} value={settingsForm.description}
        onChange={(e) => setSettingsForm({ ...settingsForm, description: e.target.value })} />
    </div>
    <div className="form-row">
      <div className="form-group">
        <label className="form-label">Category</label>
        <select className="form-select" value={settingsForm.category}
          onChange={(e) => setSettingsForm({ ...settingsForm, category: e.target.value })}>
          <option value="release">Release</option>
          <option value="feature">Feature</option>
          <option value="experiment">Experiment</option>
          <option value="ops">Ops</option>
          <option value="permission">Permission</option>
        </select>
      </div>
      <div className="form-group">
        <label className="form-label">Default Value</label>
        {flag.flag_type === 'boolean' ? (
          <select className="form-select" value={settingsForm.default_value}
            onChange={(e) => setSettingsForm({ ...settingsForm, default_value: e.target.value })}>
            <option value="true">true</option>
            <option value="false">false</option>
          </select>
        ) : (
          <input className="form-input" value={settingsForm.default_value}
            onChange={(e) => setSettingsForm({ ...settingsForm, default_value: e.target.value })} />
        )}
      </div>
    </div>
    <div className="form-group">
      <label className="form-label">Purpose</label>
      <input className="form-input" value={settingsForm.purpose}
        onChange={(e) => setSettingsForm({ ...settingsForm, purpose: e.target.value })} />
    </div>
    <div className="form-group">
      <label className="form-label">Owners (comma-separated)</label>
      <input className="form-input" value={settingsForm.owners}
        onChange={(e) => setSettingsForm({ ...settingsForm, owners: e.target.value })} />
    </div>
    <div className="form-row">
      <div className="form-group">
        <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <input type="checkbox" checked={settingsForm.is_permanent}
            onChange={(e) => setSettingsForm({ ...settingsForm, is_permanent: e.target.checked })} />
          Permanent flag
        </label>
      </div>
      {!settingsForm.is_permanent && (
        <div className="form-group">
          <label className="form-label">Expires At</label>
          <input className="form-input" type="datetime-local" value={settingsForm.expires_at}
            onChange={(e) => setSettingsForm({ ...settingsForm, expires_at: e.target.value })} />
        </div>
      )}
    </div>
    <div className="form-group">
      <label className="form-label">Tags (comma-separated)</label>
      <input className="form-input" value={settingsForm.tags}
        onChange={(e) => setSettingsForm({ ...settingsForm, tags: e.target.value })} />
    </div>
    <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
      <button className="btn btn-primary" onClick={handleSettingsSave} disabled={settingsSaving}>
        {settingsSaving ? 'Saving...' : 'Save Changes'}
      </button>
      {settingsSuccess && <span style={{ color: 'var(--color-success)', fontSize: 13 }}>Saved successfully</span>}
    </div>
  </div>
)}
```

- [ ] **Step 8: Type-check and verify in browser**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

Start dev server (`make run-web`) and verify:
- Tabs appear in correct order: Environments, Targeting Rules, YAML, Settings, History
- Default tab is Environments
- Created by shows a name (or UUID fallback)
- Created by, Created date, Updated date have proper spacing with separators
- Settings tab shows all fields populated from flag data
- Saving settings updates the header

- [ ] **Step 9: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx web/src/styles/globals.css
git commit -m "feat: add Settings tab, reorder tabs, fix header display"
```

---

### Task 7: Frontend — add History tab

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Add imports and state**

Add `auditApi` to the imports from `@/api`:

```tsx
import { flagsApi, entitiesApi, flagEnvStateApi, auditApi } from '@/api';
```

Add `AuditLogEntry` to the imports from `@/types`:

```tsx
import type { Flag, TargetingRule, OrgEnvironment, FlagEnvironmentState, RuleEnvironmentState, AuditLogEntry } from '@/types';
```

Add state for history:

```tsx
const [history, setHistory] = useState<AuditLogEntry[]>([]);
const [historyLoading, setHistoryLoading] = useState(false);
```

- [ ] **Step 2: Add history fetch effect**

Fetch history when the History tab is selected:

```tsx
useEffect(() => {
  if (activeTab !== 'history' || !id) return;
  setHistoryLoading(true);
  auditApi
    .query({ resource_type: 'flag', resource_id: id, limit: 50 })
    .then((res) => setHistory(res.entries ?? []))
    .catch(() => setHistory([]))
    .finally(() => setHistoryLoading(false));
}, [activeTab, id]);
```

- [ ] **Step 3: Add action description helper**

Add a helper function to format action strings into human-readable descriptions:

```tsx
function describeAction(entry: AuditLogEntry): string {
  switch (entry.action) {
    case 'flag.created': return 'Created flag';
    case 'flag.updated': return 'Updated flag settings';
    case 'flag.toggled': return 'Toggled flag';
    case 'flag.archived': return 'Archived flag';
    case 'flag.env_state.updated': return 'Updated environment state';
    case 'flag.rule.created': return 'Added targeting rule';
    case 'flag.rule.deleted': return 'Deleted targeting rule';
    case 'flag.rule.env_state.updated': return 'Updated rule environment state';
    default: return entry.action;
  }
}
```

- [ ] **Step 4: Add History tab JSX**

Add after the Settings tab content block:

```tsx
{/* Tab: History */}
{activeTab === 'history' && (
  <div className="card">
    {historyLoading ? (
      <p style={{ textAlign: 'center', padding: '2rem 0' }}>Loading history...</p>
    ) : history.length === 0 ? (
      <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
        No history recorded yet.
      </p>
    ) : (
      <table>
        <thead>
          <tr>
            <th>Time</th>
            <th>User</th>
            <th>Action</th>
            <th>Details</th>
          </tr>
        </thead>
        <tbody>
          {history.map((entry) => (
            <tr key={entry.id}>
              <td style={{ whiteSpace: 'nowrap', fontSize: 12 }}>{formatDateTime(entry.created_at)}</td>
              <td>{entry.actor_name || 'System'}</td>
              <td>{describeAction(entry)}</td>
              <td style={{ fontSize: 12 }}>
                {entry.old_value && (
                  <div>
                    <span className="text-muted">Before: </span>
                    <code style={{ fontSize: 11 }}>{entry.old_value}</code>
                  </div>
                )}
                {entry.new_value && (
                  <div>
                    <span className="text-muted">After: </span>
                    <code style={{ fontSize: 11 }}>{entry.new_value}</code>
                  </div>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    )}
  </div>
)}
```

- [ ] **Step 5: Type-check and verify in browser**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

Start dev server and verify:
- History tab loads entries for the flag
- Entries show timestamp, user name, action description, before/after values
- Empty state shows "No history recorded yet." message

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: add History tab with audit log timeline"
```

---

### Task 8: End-to-end verification

- [ ] **Step 1: Restart API server**

Run: `make run-api`

- [ ] **Step 2: Test Settings tab**

Navigate to a flag detail page. Go to Settings tab. Change the description and default value. Click Save. Verify:
- Header updates with new values
- No errors in console

- [ ] **Step 3: Test History tab**

Go to History tab. Verify:
- The settings change from Step 2 appears as an entry
- Toggle an environment on the Environments tab
- Return to History — the toggle appears
- Add a targeting rule — verify it appears in History

- [ ] **Step 4: Test created_by display**

Verify the flag header shows a user name instead of a UUID for "Created by".

- [ ] **Step 5: Test tab order**

Verify tabs are: Environments, Targeting Rules, YAML, Settings, History. Default is Environments.

- [ ] **Step 6: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "fix: cleanup from end-to-end verification"
```
