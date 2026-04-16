# Targeting Rules Per-Environment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-environment activation for targeting rules — rules are created as definitions, then enabled/disabled independently per environment. The flag detail page gets an inline Add Rule form and the Environments tab gets per-rule toggles inside accordions.

**Architecture:** New `rule_environment_state` join table links rules to environments with an enabled boolean. The evaluator loads per-environment rule states and only applies rules enabled for the current environment. The frontend Targeting Rules tab gets CRUD, and the Environments tab gets expandable rule toggles per environment.

**Tech Stack:** Go (backend), PostgreSQL (migration), React + TypeScript (frontend)

**Spec:** `docs/superpowers/specs/2026-04-16-targeting-rules-per-environment-design.md`

---

### Task 1: Migration — create `rule_environment_state` table

**Files:**
- Create: `migrations/040_create_rule_environment_state.up.sql`
- Create: `migrations/040_create_rule_environment_state.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
CREATE TABLE rule_environment_state (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID NOT NULL REFERENCES flag_targeting_rules(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (rule_id, environment_id)
);

CREATE INDEX idx_rule_env_state_rule_id ON rule_environment_state(rule_id);
CREATE INDEX idx_rule_env_state_env_id ON rule_environment_state(environment_id);
```

- [ ] **Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS rule_environment_state;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/040_create_rule_environment_state.up.sql migrations/040_create_rule_environment_state.down.sql
git commit -m "feat: add rule_environment_state table for per-env rule activation"
```

---

### Task 2: Add `RuleEnvironmentState` model and repository methods

**Files:**
- Modify: `internal/models/flag.go` — add struct
- Modify: `internal/flags/repository.go` — add interface methods
- Modify: `internal/platform/database/postgres/flags.go` — add implementations

- [ ] **Step 1: Add `RuleEnvironmentState` struct to models**

In `internal/models/flag.go`, add after the `TargetingRule` struct:

```go
// RuleEnvironmentState tracks whether a targeting rule is enabled for a
// specific environment.
type RuleEnvironmentState struct {
	ID            uuid.UUID `json:"id" db:"id"`
	RuleID        uuid.UUID `json:"rule_id" db:"rule_id"`
	EnvironmentID uuid.UUID `json:"environment_id" db:"environment_id"`
	Enabled       bool      `json:"enabled" db:"enabled"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}
```

- [ ] **Step 2: Add repository interface methods**

In `internal/flags/repository.go`, add to the `FlagRepository` interface:

```go
// SetRuleEnvironmentState upserts the enabled state for a rule in an environment.
SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error)

// ListRuleEnvironmentStates returns all rule-environment states for a flag's rules.
ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error)

// ListRuleEnvironmentStatesByEnv returns enabled rule IDs for a flag in a specific environment.
ListRuleEnvironmentStatesByEnv(ctx context.Context, flagID uuid.UUID, environmentID uuid.UUID) (map[uuid.UUID]bool, error)
```

- [ ] **Step 3: Implement `SetRuleEnvironmentState`**

In `internal/platform/database/postgres/flags.go`, add:

```go
func (r *FlagRepository) SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	const q = `
		INSERT INTO rule_environment_state (rule_id, environment_id, enabled, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (rule_id, environment_id)
		DO UPDATE SET enabled = $3, updated_at = now()
		RETURNING id, rule_id, environment_id, enabled, created_at, updated_at`

	var s models.RuleEnvironmentState
	err := r.pool.QueryRow(ctx, q, ruleID, environmentID, enabled).Scan(
		&s.ID, &s.RuleID, &s.EnvironmentID, &s.Enabled, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres.SetRuleEnvironmentState: %w", err)
	}
	return &s, nil
}
```

- [ ] **Step 4: Implement `ListRuleEnvironmentStates`**

```go
func (r *FlagRepository) ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
	const q = `
		SELECT res.id, res.rule_id, res.environment_id, res.enabled, res.created_at, res.updated_at
		FROM rule_environment_state res
		JOIN flag_targeting_rules ftr ON res.rule_id = ftr.id
		WHERE ftr.flag_id = $1
		ORDER BY res.rule_id, res.environment_id`

	rows, err := r.pool.Query(ctx, q, flagID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListRuleEnvironmentStates: %w", err)
	}
	defer rows.Close()

	var result []*models.RuleEnvironmentState
	for rows.Next() {
		var s models.RuleEnvironmentState
		if err := rows.Scan(&s.ID, &s.RuleID, &s.EnvironmentID, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres.ListRuleEnvironmentStates: %w", err)
		}
		result = append(result, &s)
	}
	return result, rows.Err()
}
```

- [ ] **Step 5: Implement `ListRuleEnvironmentStatesByEnv`**

```go
func (r *FlagRepository) ListRuleEnvironmentStatesByEnv(ctx context.Context, flagID uuid.UUID, environmentID uuid.UUID) (map[uuid.UUID]bool, error) {
	const q = `
		SELECT res.rule_id, res.enabled
		FROM rule_environment_state res
		JOIN flag_targeting_rules ftr ON res.rule_id = ftr.id
		WHERE ftr.flag_id = $1 AND res.environment_id = $2`

	rows, err := r.pool.Query(ctx, q, flagID, environmentID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListRuleEnvironmentStatesByEnv: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]bool)
	for rows.Next() {
		var ruleID uuid.UUID
		var enabled bool
		if err := rows.Scan(&ruleID, &enabled); err != nil {
			return nil, fmt.Errorf("postgres.ListRuleEnvironmentStatesByEnv: %w", err)
		}
		result[ruleID] = enabled
	}
	return result, rows.Err()
}
```

- [ ] **Step 6: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add internal/models/flag.go internal/flags/repository.go internal/platform/database/postgres/flags.go
git commit -m "feat: add RuleEnvironmentState model and repository methods"
```

---

### Task 3: Add service and handler methods for rule environment state

**Files:**
- Modify: `internal/flags/service.go` — add service methods
- Modify: `internal/flags/handler.go` — add handlers and routes

- [ ] **Step 1: Add service interface methods**

In `internal/flags/service.go`, find the `FlagService` interface. Add:

```go
// SetRuleEnvironmentState sets the enabled state for a rule in an environment.
SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error)

// ListRuleEnvironmentStates returns all rule-environment states for a flag's rules.
ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error)
```

- [ ] **Step 2: Add service implementations**

Find the `flagService` struct implementation section. Add:

```go
func (s *flagService) SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	return s.repo.SetRuleEnvironmentState(ctx, ruleID, environmentID, enabled)
}

func (s *flagService) ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
	return s.repo.ListRuleEnvironmentStates(ctx, flagID)
}
```

- [ ] **Step 3: Add handler for `setRuleEnvState`**

In `internal/flags/handler.go`, add:

```go
type setRuleEnvStateRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) setRuleEnvState(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("ruleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	envID, err := uuid.Parse(c.Param("envId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}

	var req setRuleEnvStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state, err := h.service.SetRuleEnvironmentState(c.Request.Context(), ruleID, envID, req.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
```

- [ ] **Step 4: Add handler for `listRuleEnvStates`**

```go
func (h *Handler) listRuleEnvStates(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	states, err := h.service.ListRuleEnvironmentStates(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rule environment states"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rule_environment_states": states})
}
```

- [ ] **Step 5: Register routes**

In `RegisterRoutes`, find the `rules` group (around line 82-88). Add two new routes:

```go
rules.PUT("/:ruleId/environments/:envId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.setRuleEnvState)
rules.GET("/environment-states", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRuleEnvStates)
```

Add the GET **before** the `/:ruleId` routes to avoid Gin matching `environment-states` as a `:ruleId` param.

- [ ] **Step 6: Update mock service in handler tests**

In `internal/flags/handler_test.go`, add mock methods for `SetRuleEnvironmentState` and `ListRuleEnvironmentStates` to the mock service struct so it compiles.

- [ ] **Step 7: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add internal/flags/service.go internal/flags/handler.go internal/flags/handler_test.go
git commit -m "feat: add handlers and routes for per-environment rule state"
```

---

### Task 4: Update evaluator to use per-environment rule states

**Files:**
- Modify: `internal/flags/evaluator.go`

- [ ] **Step 1: Update the Evaluate method**

In `internal/flags/evaluator.go`, in the `Evaluate` method, after loading targeting rules (around line 153), add a step to load per-environment rule states and filter rules:

Replace the rule application block:

```go
// Apply rules in priority order (lower priority number = higher precedence).
for _, rule := range rules {
    if !rule.Enabled {
        continue
    }
```

With:

```go
// Load per-environment rule activation states.
enabledRules, err := e.repo.ListRuleEnvironmentStatesByEnv(ctx, flag.ID, environmentID)
if err != nil {
    // Non-fatal: if we can't load states, skip all rules and return default.
    enabledRules = make(map[uuid.UUID]bool)
}

// Apply rules in priority order (lower priority number = higher precedence).
// Only apply rules that are explicitly enabled for this environment.
for _, rule := range rules {
    if enabled, ok := enabledRules[rule.ID]; !ok || !enabled {
        continue
    }
```

This replaces the global `rule.Enabled` check with a per-environment check. Rules with no `rule_environment_state` row for this environment are skipped.

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`

- [ ] **Step 3: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/...`
Expected: Tests pass (the mock repo needs to implement the new method — add a no-op implementation if needed)

- [ ] **Step 4: Commit**

```bash
git add internal/flags/evaluator.go
git commit -m "feat: evaluator uses per-environment rule states instead of global enabled"
```

---

### Task 5: Update evaluator mock repos in tests

**Files:**
- Modify: `internal/flags/evaluator_test.go`
- Modify: `internal/flags/service_test.go`

- [ ] **Step 1: Add `ListRuleEnvironmentStatesByEnv` to mock repos**

In both test files, find the mock repo structs and add the new method. Return an empty map (all rules disabled) by default, or a map that enables all rules for backward compatibility with existing tests:

```go
func (m *mockFlagRepo) SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
    return &models.RuleEnvironmentState{RuleID: ruleID, EnvironmentID: environmentID, Enabled: enabled}, nil
}

func (m *mockFlagRepo) ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
    return nil, nil
}

func (m *mockFlagRepo) ListRuleEnvironmentStatesByEnv(ctx context.Context, flagID uuid.UUID, environmentID uuid.UUID) (map[uuid.UUID]bool, error) {
    // Default: enable all rules for backward compat with existing tests
    rules, _ := m.ListRules(ctx, flagID)
    result := make(map[uuid.UUID]bool)
    for _, r := range rules {
        result[r.ID] = true
    }
    return result, nil
}
```

- [ ] **Step 2: Run all flag tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/...`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/flags/evaluator_test.go internal/flags/service_test.go
git commit -m "test: add RuleEnvironmentState mock methods to test repos"
```

---

### Task 6: Add frontend types and API methods

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add `RuleEnvironmentState` type**

In `web/src/types.ts`, after the `TargetingRule` interface, add:

```typescript
export interface RuleEnvironmentState {
  id: string;
  rule_id: string;
  environment_id: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export type RuleOperator = 'equals' | 'not_equals' | 'in' | 'not_in' | 'contains' | 'starts_with' | 'ends_with' | 'greater_than' | 'less_than';
```

- [ ] **Step 2: Add API methods**

In `web/src/api.ts`, add to the `flagsApi` object:

```typescript
setRuleEnvState: (flagId: string, ruleId: string, envId: string, data: { enabled: boolean }) =>
  request<RuleEnvironmentState>(`/flags/${flagId}/rules/${ruleId}/environments/${envId}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  }),
listRuleEnvStates: (flagId: string) =>
  request<{ rule_environment_states: RuleEnvironmentState[] }>(`/flags/${flagId}/rules/environment-states`),
```

Import `RuleEnvironmentState` at the top of the file.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add RuleEnvironmentState type and API methods"
```

---

### Task 7: Rebuild Targeting Rules tab with Add Rule form

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Add state and form for rule creation**

Read the file first. Add state variables for the add-rule form:

```typescript
const [showAddRule, setShowAddRule] = useState(false);
const [newRule, setNewRule] = useState({
  attribute: '',
  operator: 'equals',
  target_values: '',
  value: '',
  priority: 0,
});
```

Update the `priority` default when rules load:
```typescript
// After rules are fetched, set default priority
useEffect(() => {
  setNewRule((prev) => ({ ...prev, priority: rules.length + 1 }));
}, [rules]);
```

- [ ] **Step 2: Add the submit handler**

```typescript
const handleAddRule = async () => {
  if (!id || !newRule.attribute || !newRule.value) return;
  try {
    const targetValues = ['in', 'not_in'].includes(newRule.operator)
      ? newRule.target_values.split(',').map((s) => s.trim()).filter(Boolean)
      : [newRule.target_values.trim()];
    await flagsApi.addRule(id, {
      rule_type: 'attribute',
      attribute: newRule.attribute,
      operator: newRule.operator,
      target_values: targetValues,
      value: newRule.value,
      priority: newRule.priority,
    });
    // Refetch rules
    const res = await flagsApi.listRules(id);
    setRules(res.rules ?? []);
    setShowAddRule(false);
    setNewRule({ attribute: '', operator: 'equals', target_values: '', value: '', priority: (res.rules?.length ?? 0) + 1 });
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to add rule');
  }
};
```

- [ ] **Step 3: Add the delete handler**

```typescript
const handleDeleteRule = async (ruleId: string) => {
  if (!id) return;
  if (!window.confirm('Delete this targeting rule?')) return;
  try {
    await flagsApi.deleteRule(id, ruleId);
    setRules((prev) => prev.filter((r) => r.id !== ruleId));
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to delete rule');
  }
};
```

- [ ] **Step 4: Replace the Targeting Rules tab JSX**

Replace the current rules tab content with:

```tsx
{activeTab === 'rules' && (
  <div className="card">
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
      <span>{rules.length} rule{rules.length !== 1 ? 's' : ''}</span>
      <button className="btn btn-secondary" onClick={() => setShowAddRule(!showAddRule)}>
        {showAddRule ? 'Cancel' : 'Add Rule'}
      </button>
    </div>

    {showAddRule && (
      <div style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)', borderRadius: 6, padding: 16, marginBottom: 16 }}>
        <div className="form-row">
          <div className="form-group">
            <label className="form-label">Attribute</label>
            <input className="form-input" type="text" placeholder="e.g. userType" value={newRule.attribute} onChange={(e) => setNewRule({ ...newRule, attribute: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Operator</label>
            <select className="form-select" value={newRule.operator} onChange={(e) => setNewRule({ ...newRule, operator: e.target.value })}>
              <option value="equals">equals</option>
              <option value="not_equals">not equals</option>
              <option value="in">in</option>
              <option value="not_in">not in</option>
              <option value="contains">contains</option>
              <option value="starts_with">starts with</option>
              <option value="ends_with">ends with</option>
              <option value="greater_than">greater than</option>
              <option value="less_than">less than</option>
            </select>
          </div>
        </div>
        <div className="form-row">
          <div className="form-group">
            <label className="form-label">Target Value(s)</label>
            <input className="form-input" type="text" placeholder={['in', 'not_in'].includes(newRule.operator) ? 'comma-separated values' : 'value'} value={newRule.target_values} onChange={(e) => setNewRule({ ...newRule, target_values: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Serve Value</label>
            <input className="form-input" type="text" placeholder="e.g. true" value={newRule.value} onChange={(e) => setNewRule({ ...newRule, value: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Priority</label>
            <input className="form-input" type="number" min={1} value={newRule.priority} onChange={(e) => setNewRule({ ...newRule, priority: parseInt(e.target.value) || 1 })} />
          </div>
        </div>
        <button className="btn btn-primary" onClick={handleAddRule} disabled={!newRule.attribute || !newRule.value}>
          Create Rule
        </button>
      </div>
    )}

    <table>
      <thead>
        <tr>
          <th>Priority</th>
          <th>Condition</th>
          <th>Serve Value</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {rules.map((rule) => (
          <tr key={rule.id}>
            <td>{rule.priority}</td>
            <td>{rule.attribute} {rule.operator} {(rule.target_values ?? []).join(', ')}</td>
            <td className="font-mono">{rule.value}</td>
            <td>
              <button className="btn btn-sm btn-danger" onClick={() => handleDeleteRule(rule.id)}>Delete</button>
            </td>
          </tr>
        ))}
        {rules.length === 0 && (
          <tr><td colSpan={4} style={{ textAlign: 'center' }}>No targeting rules defined.</td></tr>
        )}
      </tbody>
    </table>
  </div>
)}
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: targeting rules tab with inline add form and delete"
```

---

### Task 8: Rebuild Environments tab with per-rule toggles

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Add state for rule environment states and expanded accordions**

```typescript
const [ruleEnvStates, setRuleEnvStates] = useState<RuleEnvironmentState[]>([]);
const [expandedEnvs, setExpandedEnvs] = useState<Set<string>>(new Set());
```

Import `RuleEnvironmentState` from types and `flagsApi` methods.

- [ ] **Step 2: Fetch rule environment states**

In the existing `useEffect` that fetches environments and env states (the one keyed on `[orgSlug, id]`), add:

```typescript
flagsApi.listRuleEnvStates(id)
  .then((res) => setRuleEnvStates(res.rule_environment_states ?? []))
  .catch(() => {});
```

- [ ] **Step 3: Add toggle handler for rule env state**

```typescript
const handleRuleEnvToggle = async (ruleId: string, envId: string, currentEnabled: boolean) => {
  if (!id) return;
  const nextEnabled = !currentEnabled;
  setRuleEnvStates((prev) => {
    const existing = prev.find((s) => s.rule_id === ruleId && s.environment_id === envId);
    if (existing) {
      return prev.map((s) => s.rule_id === ruleId && s.environment_id === envId ? { ...s, enabled: nextEnabled } : s);
    }
    return [...prev, { id: '', rule_id: ruleId, environment_id: envId, enabled: nextEnabled, created_at: '', updated_at: '' }];
  });
  try {
    await flagsApi.setRuleEnvState(id, ruleId, envId, { enabled: nextEnabled });
  } catch (err) {
    setRuleEnvStates((prev) =>
      prev.map((s) => s.rule_id === ruleId && s.environment_id === envId ? { ...s, enabled: currentEnabled } : s),
    );
    setError(err instanceof Error ? err.message : 'Failed to toggle rule');
  }
};

const toggleEnvAccordion = (envId: string) => {
  setExpandedEnvs((prev) => {
    const next = new Set(prev);
    if (next.has(envId)) next.delete(envId);
    else next.add(envId);
    return next;
  });
};
```

- [ ] **Step 4: Replace the Environments tab JSX**

```tsx
{activeTab === 'environments' && (
  <div className="card">
    {environments.length === 0 ? (
      <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
        No environments configured. Add environments in org settings.
      </p>
    ) : (
      <div>
        {environments.map((env) => {
          const state = envStates.find((s) => s.environment_id === env.id);
          const isEnabled = state?.enabled ?? false;
          const isExpanded = expandedEnvs.has(env.id);
          return (
            <div key={env.id} style={{ borderBottom: '1px solid var(--color-border)', paddingBottom: 12, marginBottom: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <button
                    onClick={() => toggleEnvAccordion(env.id)}
                    style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text)', fontSize: 14 }}
                  >
                    {isExpanded ? '▾' : '▸'}
                  </button>
                  <strong>{env.name}</strong>
                  {env.is_production && <span className="badge badge-enabled" style={{ fontSize: 11 }}>production</span>}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
                  <span className="font-mono" style={{ fontSize: 13 }}>
                    {state?.value != null ? String(state.value) : '—'}
                  </span>
                  <label className="toggle">
                    <input type="checkbox" checked={isEnabled} onChange={() => handleEnvToggle(env.id, isEnabled)} />
                    <span>{isEnabled ? 'Enabled' : 'Disabled'}</span>
                  </label>
                </div>
              </div>

              {isExpanded && (
                <div style={{ marginLeft: 28, marginTop: 8 }}>
                  {rules.length === 0 ? (
                    <p className="text-muted" style={{ fontSize: 13 }}>No targeting rules defined.</p>
                  ) : (
                    <table style={{ fontSize: 13 }}>
                      <thead>
                        <tr>
                          <th>Rule</th>
                          <th>Serve Value</th>
                          <th>Enabled</th>
                        </tr>
                      </thead>
                      <tbody>
                        {rules.map((rule) => {
                          const ruleState = ruleEnvStates.find(
                            (s) => s.rule_id === rule.id && s.environment_id === env.id,
                          );
                          const ruleEnabled = ruleState?.enabled ?? false;
                          return (
                            <tr key={rule.id}>
                              <td>{rule.attribute} {rule.operator} {(rule.target_values ?? []).join(', ')}</td>
                              <td className="font-mono">{rule.value}</td>
                              <td>
                                <label className="toggle">
                                  <input
                                    type="checkbox"
                                    checked={ruleEnabled}
                                    onChange={() => handleRuleEnvToggle(rule.id, env.id, ruleEnabled)}
                                  />
                                  <span>{ruleEnabled ? 'On' : 'Off'}</span>
                                </label>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    )}
  </div>
)}
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: environments tab with per-rule per-environment toggles"
```

---

### Task 9: Run migration and full verification

- [ ] **Step 1: Run migration**

Run: `cd /Users/sgamel/git/DeploySentry && make migrate-up`
Expected: Migration 040 applied

- [ ] **Step 2: Rebuild API binary**

Run: `cd /Users/sgamel/git/DeploySentry && go build -o bin/deploysentry-api ./cmd/api/main.go`

- [ ] **Step 3: Run Go tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/...`
Expected: All tests pass

- [ ] **Step 4: Verify TypeScript**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 5: Test in browser**

Restart API server. Test the flow:
1. Go to a flag detail page
2. Targeting Rules tab: click "Add Rule", fill in attribute=`userType`, operator=`equals`, target_values=`beta-tester`, value=`true`, submit
3. Rule appears in the table
4. Switch to Environments tab
5. Expand an environment accordion
6. See the new rule with a toggle (off by default)
7. Toggle it on
8. The rule is now active for that environment only
