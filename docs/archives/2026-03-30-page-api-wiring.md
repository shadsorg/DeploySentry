# Page API Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the 6 remaining mock-data pages to real backend API endpoints and add the missing `GET /flags/:id/rules` endpoint.

**Architecture:** Backend-first — add `ListRules` to the flag service interface + handler, then wire each frontend page one-by-one (remove mock data, add `useEffect`/`useState` for API calls, add loading/error states). Finally clean up unused mock constants.

**Tech Stack:** Go 1.22+, Gin, React 18, TypeScript, Vite

---

## Task 1: Backend — Add ListRules to FlagService Interface + Implementation

**Files:**
- Modify: `internal/flags/service.go`

- [ ] **Step 1: Add ListRules to FlagService interface**

In `internal/flags/service.go`, add the `ListRules` method to the `FlagService` interface (after the `DeleteRule` method at line 66):

```go
	// ListRules returns all targeting rules for a flag.
	ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)
```

- [ ] **Step 2: Add ListRules implementation on flagService struct**

Add the pass-through method on the `flagService` struct (after the `DeleteRule` implementation, around line 311):

```go
// ListRules returns all targeting rules for a given flag.
func (s *flagService) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	rules, err := s.repo.ListRules(ctx, flagID)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}
	return rules, nil
}
```

- [ ] **Step 3: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/flags/...`
Expected: SUCCESS (no errors)

---

## Task 2: Backend — Add listRules Handler + Route

**Files:**
- Modify: `internal/flags/handler.go`

- [ ] **Step 1: Add listRules handler method**

Add the `listRules` handler method after the `deleteRule` handler (after line 754):

```go
func (h *Handler) listRules(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	rules, err := h.service.ListRules(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}
```

- [ ] **Step 2: Register the GET route**

In `RegisterRoutes`, inside the `rules` group (line 74-79), add the GET route before the existing POST:

```go
		rules := flags.Group("/:id/rules")
		{
			rules.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRules)
			rules.POST("", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.addRule)
			rules.PUT("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateRule)
			rules.DELETE("/:ruleId", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.deleteRule)
		}
```

- [ ] **Step 3: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/`
Expected: SUCCESS

---

## Task 3: Backend — Add ListRules Test

**Files:**
- Modify: `internal/flags/handler_test.go`

- [ ] **Step 1: Add listRulesFn to mockFlagService**

In `internal/flags/handler_test.go`, add the field to the `mockFlagService` struct (after `deleteRuleFn` on line 34):

```go
	listRulesFn   func(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)
```

- [ ] **Step 2: Add ListRules mock method**

Add the method implementation after the `DeleteRule` mock method (after line 105):

```go
func (m *mockFlagService) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	if m.listRulesFn != nil {
		return m.listRulesFn(ctx, flagID)
	}
	return []*models.TargetingRule{}, nil
}
```

- [ ] **Step 3: Add TestListRules_Valid test**

Add after the `TestDeleteRule_InvalidRuleID` test (after line 675):

```go
// ---------------------------------------------------------------------------
// GET /flags/:id/rules  (listRules)
// ---------------------------------------------------------------------------

func TestListRules_Valid(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()
	svc := &mockFlagService{
		listRulesFn: func(_ context.Context, fID uuid.UUID) ([]*models.TargetingRule, error) {
			assert.Equal(t, flagID, fID)
			pct := 50
			return []*models.TargetingRule{
				{
					ID:         ruleID,
					FlagID:     fID,
					RuleType:   "percentage",
					Priority:   1,
					Value:      "on",
					Percentage: &pct,
					Enabled:    true,
				},
			}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String()+"/rules", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "rules")

	var rules []models.TargetingRule
	assert.NoError(t, json.Unmarshal(resp["rules"], &rules))
	assert.Len(t, rules, 1)
	assert.Equal(t, ruleID, rules[0].ID)
}

func TestListRules_InvalidFlagID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodGet, "/api/flags/bad-uuid/rules", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListRules_ServiceError(t *testing.T) {
	svc := &mockFlagService{
		listRulesFn: func(_ context.Context, _ uuid.UUID) ([]*models.TargetingRule, error) {
			return nil, errors.New("db down")
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+uuid.New().String()+"/rules", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -run TestListRules -v`
Expected: 3 tests PASS

- [ ] **Step 5: Commit backend changes**

```bash
cd /Users/sgamel/git/DeploySentry && git add internal/flags/service.go internal/flags/handler.go internal/flags/handler_test.go
git commit -m "feat: add GET /flags/:id/rules endpoint for listing targeting rules"
```

---

## Task 4: Frontend — Add listRules to API module

**Files:**
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add listRules to flagsApi**

In `web/src/api.ts`, add `listRules` to the `flagsApi` object (after the `deleteRule` method, around line 50):

```typescript
  listRules: (flagId: string) =>
    request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No errors (or only pre-existing errors)

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/api.ts
git commit -m "feat: add flagsApi.listRules() for GET /flags/:id/rules"
```

---

## Task 5: Frontend — Wire FlagListPage

**Files:**
- Modify: `web/src/pages/FlagListPage.tsx`

- [ ] **Step 1: Replace FlagListPage with API-wired version**

Replace the entire contents of `web/src/pages/FlagListPage.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { Link, useParams } from 'react-router-dom';
import type { Flag, FlagCategory } from '@/types';
import { entitiesApi, flagsApi } from '@/api';

type StatusFilter = 'all' | 'enabled' | 'disabled' | 'archived';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export default function FlagListPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const contextName = appSlug ?? projectSlug ?? '';
  const heading = appSlug ? `${contextName} — Flags` : contextName ? `${contextName} — Feature Flags` : 'Feature Flags';

  const createPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/new`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags/new`;

  const flagDetailPath = (flagId: string) => appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/${flagId}`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags/${flagId}`;

  const [flags, setFlags] = useState<Flag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<'all' | FlagCategory>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    setLoading(true);
    setError(null);

    entitiesApi.getProject(orgSlug, projectSlug)
      .then((project) => flagsApi.list(project.id))
      .then((result) => setFlags(result.flags))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  const filtered = flags.filter((flag) => {
    if (search) {
      const q = search.toLowerCase();
      if (
        !flag.name.toLowerCase().includes(q) &&
        !flag.key.toLowerCase().includes(q)
      ) {
        return false;
      }
    }
    if (categoryFilter !== 'all' && flag.category !== categoryFilter) {
      return false;
    }
    if (statusFilter === 'enabled' && (!flag.enabled || flag.archived)) return false;
    if (statusFilter === 'disabled' && (flag.enabled || flag.archived)) return false;
    if (statusFilter === 'archived' && !flag.archived) return false;
    return true;
  });

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">{heading}</h1>
        <Link to={createPath} className="btn btn-primary">
          Create Flag
        </Link>
      </div>

      <div className="filter-bar">
        <input
          type="text"
          className="form-input"
          placeholder="Search flags..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="form-select"
          value={categoryFilter}
          onChange={(e) => setCategoryFilter(e.target.value as 'all' | FlagCategory)}
        >
          <option value="all">All Categories</option>
          <option value="release">Release</option>
          <option value="feature">Feature</option>
          <option value="experiment">Experiment</option>
          <option value="ops">Ops</option>
          <option value="permission">Permission</option>
        </select>
        <select
          className="form-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
        >
          <option value="all">All Statuses</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
          <option value="archived">Archived</option>
        </select>
      </div>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th>Name / Key</th>
              <th>Category</th>
              <th>Status</th>
              <th>Owners</th>
              <th>Expires</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((flag) => (
              <tr key={flag.id}>
                <td>
                  <Link to={flagDetailPath(flag.id)}>{flag.name}</Link>
                  <div className="font-mono text-muted">{flag.key}</div>
                </td>
                <td>
                  <span className={`badge badge-${flag.category}`}>
                    {flag.category}
                  </span>
                </td>
                <td>
                  {flag.archived ? (
                    <span className="badge badge-archived">archived</span>
                  ) : flag.enabled ? (
                    <span className="badge badge-enabled">enabled</span>
                  ) : (
                    <span className="badge badge-disabled">disabled</span>
                  )}
                </td>
                <td>{flag.owners.join(', ')}</td>
                <td>
                  {flag.is_permanent
                    ? 'Permanent'
                    : flag.expires_at
                      ? formatDate(flag.expires_at)
                      : '\u2014'}
                </td>
                <td>{formatDate(flag.updated_at)}</td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={6} style={{ textAlign: 'center' }}>
                  No flags match your filters.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/pages/FlagListPage.tsx
git commit -m "feat: wire FlagListPage to real API"
```

---

## Task 6: Frontend — Wire FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Replace FlagDetailPage with API-wired version**

Replace the entire contents of `web/src/pages/FlagDetailPage.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Flag, TargetingRule } from '@/types';
import { flagsApi } from '@/api';
import { MOCK_APPLICATIONS } from '@/mocks/hierarchy';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function describeConditions(rule: TargetingRule): string {
  switch (rule.rule_type) {
    case 'percentage':
      return `${rule.percentage}% of users`;
    case 'user_target':
      return `Users: ${rule.target_values?.join(', ') ?? '\u2014'}`;
    case 'attribute':
      return `${rule.attribute} ${rule.operator} ${rule.target_values?.join(', ') ?? '\u2014'}`;
    case 'segment':
      return `Segment: ${rule.segment_id ?? '\u2014'}`;
    case 'schedule':
      return `${rule.start_time ? formatDateTime(rule.start_time) : '\u2014'} to ${rule.end_time ? formatDateTime(rule.end_time) : '\u2014'}`;
    default:
      return '\u2014';
  }
}

function getAppNameById(appId: string): string {
  return MOCK_APPLICATIONS.find((a) => a.id === appId)?.name ?? appId;
}

export default function FlagDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  const [flag, setFlag] = useState<Flag | null>(null);
  const [rules, setRules] = useState<TargetingRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'rules' | 'environments'>('rules');

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    Promise.all([
      flagsApi.get(id),
      flagsApi.listRules(id).then((r) => r.rules),
    ])
      .then(([flagData, rulesData]) => {
        setFlag(flagData);
        setRules(rulesData);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!flag) return <div>Flag not found.</div>;

  const handleToggle = () => {
    setFlag((prev) => prev ? { ...prev, enabled: !prev.enabled } : prev);
  };

  const handleArchive = () => {
    setFlag((prev) => prev ? { ...prev, archived: true } : prev);
  };

  return (
    <div>
      {/* Header Section */}
      <div className="detail-header">
        <Link to={backPath}>&larr; Back to Flags</Link>

        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{flag.name}</h1>
            <span className="detail-header-subtitle">{flag.key}</span>
          </div>
          <div className="detail-header-badges">
            <label className="toggle">
              <input
                type="checkbox"
                checked={flag.enabled}
                onChange={handleToggle}
              />
              <span>{flag.enabled ? 'Enabled' : 'Disabled'}</span>
            </label>
            <span className={`badge badge-${flag.category}`}>{flag.category}</span>
            <button className="btn btn-secondary">Edit</button>
          </div>
        </div>

        <div className="detail-chips">
          <span>Type: {flag.flag_type}</span>
          <span>Owners: {flag.owners.join(', ')}</span>
          <span>Expires: {flag.is_permanent ? 'Permanent' : flag.expires_at ? formatDate(flag.expires_at) : '\u2014'}</span>
          <span>Default Value: <span className="font-mono">{flag.default_value}</span></span>
          <span>Scope: {flag.application_id ? getAppNameById(flag.application_id) : 'Project-wide'}</span>
          {flag.purpose && <span>Purpose: {flag.purpose}</span>}
          {flag.tags.length > 0 && <span>Tags: {flag.tags.join(', ')}</span>}
        </div>

        <div className="detail-secondary">
          <span>Created by {flag.created_by}</span>
          <span>Created {formatDateTime(flag.created_at)}</span>
          <span>Updated {formatDateTime(flag.updated_at)}</span>
        </div>

        {flag.description && (
          <div className="detail-description">{flag.description}</div>
        )}
      </div>

      {/* Tabs */}
      <div className="detail-tabs">
        <button
          className={`detail-tab${activeTab === 'rules' ? ' active' : ''}`}
          onClick={() => setActiveTab('rules')}
        >
          Targeting Rules
        </button>
        <button
          className={`detail-tab${activeTab === 'environments' ? ' active' : ''}`}
          onClick={() => setActiveTab('environments')}
        >
          Environments
        </button>
      </div>

      {/* Tab: Targeting Rules */}
      {activeTab === 'rules' && (
        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <span>{rules.length} rule{rules.length !== 1 ? 's' : ''}</span>
            <button className="btn btn-secondary">Add Rule</button>
          </div>
          <table>
            <thead>
              <tr>
                <th>Priority</th>
                <th>Type</th>
                <th>Condition</th>
                <th>Value</th>
                <th>Enabled</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td>{rule.priority}</td>
                  <td>{rule.rule_type}</td>
                  <td>{describeConditions(rule)}</td>
                  <td className="font-mono">{rule.value}</td>
                  <td>
                    <span className={`badge ${rule.enabled ? 'badge-enabled' : 'badge-disabled'}`}>
                      {rule.enabled ? 'enabled' : 'disabled'}
                    </span>
                  </td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ textAlign: 'center' }}>
                    No targeting rules defined.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Tab: Environments */}
      {activeTab === 'environments' && (
        <div className="card">
          <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
            No environment data available
          </p>
        </div>
      )}

      {/* Danger Zone */}
      <div className="danger-zone">
        <h2>Danger Zone</h2>
        <p className="text-muted">
          Archiving a flag disables it permanently and removes it from active use.
          This action cannot be easily undone.
        </p>
        <button
          className="btn btn-danger"
          onClick={handleArchive}
          disabled={flag.archived}
        >
          {flag.archived ? 'Archived' : 'Archive Flag'}
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: wire FlagDetailPage to real API with rules listing"
```

---

## Task 7: Frontend — Wire DeploymentsPage

**Files:**
- Modify: `web/src/pages/DeploymentsPage.tsx`

- [ ] **Step 1: Replace DeploymentsPage with API-wired version**

Replace the entire contents of `web/src/pages/DeploymentsPage.tsx`:

```tsx
import React, { useState, useMemo, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Deployment, DeployStrategy, DeployStatus } from '@/types';
import { entitiesApi, deploymentsApi } from '@/api';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function strategyBadgeClass(strategy: DeployStrategy): string {
  switch (strategy) {
    case 'canary':
      return 'badge badge-experiment';
    case 'blue-green':
      return 'badge badge-release';
    case 'rolling':
      return 'badge badge-ops';
  }
}

function statusBadgeClass(status: DeployStatus): string {
  switch (status) {
    case 'running':
      return 'badge badge-active';
    case 'promoting':
      return 'badge badge-active';
    case 'completed':
      return 'badge badge-completed';
    case 'failed':
      return 'badge badge-failed';
    case 'rolled_back':
      return 'badge badge-rolling-back';
    case 'paused':
      return 'badge badge-pending';
    case 'pending':
      return 'badge badge-pending';
    case 'cancelled':
      return 'badge badge-disabled';
    default:
      return 'badge';
  }
}

function statusLabel(status: DeployStatus): string {
  switch (status) {
    case 'rolled_back':
      return 'Rolled Back';
    case 'promoting':
      return 'Promoting';
    case 'cancelled':
      return 'Cancelled';
    default:
      return status.charAt(0).toUpperCase() + status.slice(1);
  }
}

function healthColor(score: number): string {
  if (score >= 95) return 'text-success';
  if (score >= 80) return 'text-warning';
  return 'text-danger';
}

function trafficBarColor(score: number): string {
  if (score >= 95) return 'var(--color-success)';
  if (score >= 80) return 'var(--color-warning)';
  return 'var(--color-danger)';
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function computeDuration(start: string, end: string | null): string {
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const mins = Math.round((e - s) / 60000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  const rem = mins % 60;
  return `${hours}h ${rem}m`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const DeploymentsPage: React.FC = () => {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const appName = appSlug ?? '';

  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [strategyFilter, setStrategyFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);

    entitiesApi.getApp(orgSlug, projectSlug, appSlug)
      .then((app) => deploymentsApi.list(app.id))
      .then((result) => setDeployments(result.deployments))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  const filtered = useMemo(() => {
    return deployments.filter((d) => {
      if (search && !d.version.toLowerCase().includes(search.toLowerCase())) {
        return false;
      }
      if (strategyFilter !== 'all' && d.strategy !== strategyFilter) {
        return false;
      }
      if (statusFilter !== 'all' && d.status !== statusFilter) {
        return false;
      }
      return true;
    });
  }, [deployments, search, strategyFilter, statusFilter]);

  if (!appSlug) {
    return (
      <div>
        <h1 className="page-header">Deployments</h1>
        <p>Select an application to view deployments</p>
      </div>
    );
  }

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      {/* Page header */}
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>{appName ? `${appName} — Deployments` : 'Deployments'}</h1>
          <p>Monitor and manage application deployments across environments</p>
        </div>
        <button className="btn btn-primary">+ New Deployment</button>
      </div>

      {/* Filter bar */}
      <div className="filter-bar">
        <input
          className="form-input"
          type="text"
          placeholder="Search by version..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="form-select"
          value={strategyFilter}
          onChange={(e) => setStrategyFilter(e.target.value)}
        >
          <option value="all">All Strategies</option>
          <option value="canary">Canary</option>
          <option value="blue-green">Blue/Green</option>
          <option value="rolling">Rolling</option>
        </select>
        <select
          className="form-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="all">All Statuses</option>
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="promoting">Promoting</option>
          <option value="paused">Paused</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="rolled_back">Rolled Back</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      {/* Table */}
      <div className="card">
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Version</th>
                <th>Strategy</th>
                <th>Status</th>
                <th>Traffic %</th>
                <th>Health</th>
                <th>Started</th>
                <th>Duration</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <div className="empty-state">
                      <h3>No deployments found</h3>
                      <p>Try adjusting your filters or search term.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((dep) => (
                  <tr key={dep.id}>
                    <td style={{ fontWeight: 500 }}>
                      <code className="font-mono text-sm">{dep.version}</code>
                    </td>
                    <td>
                      <span className={strategyBadgeClass(dep.strategy)}>
                        {dep.strategy}
                      </span>
                    </td>
                    <td>
                      <span className={statusBadgeClass(dep.status)}>
                        {statusLabel(dep.status)}
                      </span>
                    </td>
                    <td>
                      <div className="flex items-center gap-2">
                        <div
                          style={{
                            width: 60,
                            height: 6,
                            borderRadius: 3,
                            background: 'var(--color-bg)',
                            overflow: 'hidden',
                          }}
                        >
                          <div
                            style={{
                              width: `${dep.traffic_percent}%`,
                              height: '100%',
                              borderRadius: 3,
                              background: trafficBarColor(dep.health_score),
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </div>
                        <span className="text-sm">{dep.traffic_percent}%</span>
                      </div>
                    </td>
                    <td>
                      <span className={healthColor(dep.health_score)}>
                        {dep.health_score.toFixed(1)}%
                      </span>
                    </td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatTime(dep.created_at)}
                    </td>
                    <td className="text-sm text-muted" style={{ whiteSpace: 'nowrap' }}>
                      {computeDuration(dep.created_at, dep.completed_at)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default DeploymentsPage;
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/pages/DeploymentsPage.tsx
git commit -m "feat: wire DeploymentsPage to real API"
```

---

## Task 8: Frontend — Wire DeploymentDetailPage

**Files:**
- Modify: `web/src/pages/DeploymentDetailPage.tsx`

- [ ] **Step 1: Replace DeploymentDetailPage with API-wired version**

Replace the entire contents of `web/src/pages/DeploymentDetailPage.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import { deploymentsApi } from '@/api';
import type { Deployment, DeployStatus } from '@/types';

function getDeployActions(status: DeployStatus) {
  const noop = () => {};
  switch (status) {
    case 'pending':
      return { primaryAction: { label: 'Start', onClick: noop }, secondaryActions: [{ label: 'Cancel', onClick: noop }] };
    case 'running':
      return { primaryAction: { label: 'Promote', onClick: noop }, secondaryActions: [{ label: 'Pause', onClick: noop }, { label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    case 'promoting':
      return { secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    case 'paused':
      return { primaryAction: { label: 'Resume', onClick: noop }, secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }, { label: 'Cancel', onClick: noop }] };
    case 'failed':
      return { primaryAction: { label: 'Rollback', onClick: noop, variant: 'danger' as const } };
    default:
      return {};
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function computeDuration(start?: string, end?: string | null): string {
  if (!start) return '—';
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const mins = Math.floor((e - s) / 60000);
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
}

function strategyBadgeClass(strategy: string): string {
  switch (strategy) {
    case 'canary': return 'badge badge-experiment';
    case 'blue-green': return 'badge badge-release';
    case 'rolling': return 'badge badge-ops';
    default: return 'badge';
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'running': case 'promoting': return 'badge badge-active';
    case 'completed': return 'badge badge-completed';
    case 'failed': return 'badge badge-danger';
    case 'rolled_back': return 'badge badge-rolling-back';
    case 'paused': return 'badge badge-ops';
    case 'pending': return 'badge badge-pending';
    case 'cancelled': return 'badge badge-disabled';
    default: return 'badge';
  }
}

function healthColorClass(score: number): string {
  if (score >= 99) return 'text-success';
  if (score >= 95) return 'text-warning';
  return 'text-danger';
}

export default function DeploymentDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/deployments`;

  const [dep, setDep] = useState<Deployment | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    deploymentsApi.get(id)
      .then((data) => setDep(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!dep) return <div>Deployment not found.</div>;

  const actions = getDeployActions(dep.status);
  const artifactHostname = dep.artifact ? (() => { try { return new URL(dep.artifact).hostname; } catch { return undefined; } })() : undefined;

  return (
    <div className="page">
      <div className="detail-header">
        <Link to={backPath} className="back-link">← Deployments</Link>
        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{dep.version}</h1>
            <div className="detail-header-subtitle">
              {dep.commit_sha && <span>{dep.commit_sha.slice(0, 7)}</span>}
              {artifactHostname && (
                <>
                  {' · '}
                  <a href={dep.artifact} target="_blank" rel="noopener noreferrer">{artifactHostname}</a>
                </>
              )}
              {' · '}
              <span className={strategyBadgeClass(dep.strategy)}>{dep.strategy}</span>
              {' · '}
              <span>{dep.traffic_percent}% traffic</span>
            </div>
            <div className="detail-header-badges">
              <span className={statusBadgeClass(dep.status)}>{dep.status.replace('_', ' ')}</span>
            </div>
          </div>
          <ActionBar {...actions} />
        </div>
      </div>

      <div className="info-cards">
        <div className="info-card">
          <div className="info-card-label">Traffic</div>
          <div className="info-card-value">{dep.traffic_percent}%</div>
          <div className="info-card-bar">
            <div className="info-card-bar-fill" style={{ width: `${dep.traffic_percent}%` }} />
          </div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Health</div>
          <div className={`info-card-value ${healthColorClass(dep.health_score)}`}>{dep.health_score}%</div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Duration</div>
          <div className="info-card-value">{computeDuration(dep.started_at, dep.completed_at)}</div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Created by</div>
          <div className="info-card-value">{dep.created_by}</div>
        </div>
      </div>

      <div className="activity-log">
        <h2 className="activity-log-title">Activity Log</h2>
        <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
          No events data available
        </p>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/pages/DeploymentDetailPage.tsx
git commit -m "feat: wire DeploymentDetailPage to real API"
```

---

## Task 9: Frontend — Wire ReleasesPage

**Files:**
- Modify: `web/src/pages/ReleasesPage.tsx`

- [ ] **Step 1: Replace ReleasesPage with API-wired version**

Replace the entire contents of `web/src/pages/ReleasesPage.tsx`:

```tsx
import React, { useState, useMemo, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Release, ReleaseStatus } from '@/types';
import { entitiesApi, releasesApi } from '@/api';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const TABS = ['all', 'draft', 'rolling_out', 'paused', 'completed', 'rolled_back'] as const;

type TabKey = (typeof TABS)[number];

function tabLabel(tab: TabKey): string {
  switch (tab) {
    case 'all':
      return 'All';
    case 'draft':
      return 'Draft';
    case 'rolling_out':
      return 'Rolling Out';
    case 'paused':
      return 'Paused';
    case 'completed':
      return 'Completed';
    case 'rolled_back':
      return 'Rolled Back';
  }
}

function statusBadgeClass(status: ReleaseStatus): string {
  switch (status) {
    case 'draft':
      return 'badge badge-pending';
    case 'rolling_out':
      return 'badge badge-active';
    case 'paused':
      return 'badge badge-ops';
    case 'completed':
      return 'badge badge-completed';
    case 'rolled_back':
      return 'badge badge-danger';
  }
}

function statusLabel(status: ReleaseStatus): string {
  switch (status) {
    case 'rolling_out':
      return 'Rolling Out';
    case 'rolled_back':
      return 'Rolled Back';
    default:
      return status.charAt(0).toUpperCase() + status.slice(1);
  }
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const ReleasesPage: React.FC = () => {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const appName = appSlug ?? '';

  const [releases, setReleases] = useState<Release[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);

    entitiesApi.getApp(orgSlug, projectSlug, appSlug)
      .then((app) => releasesApi.list(app.id))
      .then((result) => setReleases(result.releases))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  const filtered = useMemo(() => {
    if (activeTab === 'all') return releases;
    return releases.filter((r) => r.status === activeTab);
  }, [releases, activeTab]);

  if (!appSlug) {
    return (
      <div>
        <h1 className="page-header">Releases</h1>
        <p>Select an application to view releases</p>
      </div>
    );
  }

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      {/* Page header */}
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>{appName ? `${appName} — Releases` : 'Releases'}</h1>
          <p>Coordinate flag changes across environments with managed releases</p>
        </div>
        <button className="btn btn-primary">+ Create Release</button>
      </div>

      {/* Tabs */}
      <div className="tabs">
        {TABS.map((tab) => (
          <button
            key={tab}
            className={`tab${activeTab === tab ? ' active' : ''}`}
            onClick={() => setActiveTab(tab)}
          >
            {tabLabel(tab)}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="card">
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Traffic %</th>
                <th>Description</th>
                <th>Created By</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <div className="empty-state">
                      <h3>No releases found</h3>
                      <p>There are no releases matching the selected filter.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((rel) => (
                  <tr key={rel.id}>
                    <td style={{ fontWeight: 500 }}>{rel.name}</td>
                    <td>
                      <span className={statusBadgeClass(rel.status)}>
                        {statusLabel(rel.status)}
                      </span>
                    </td>
                    <td>
                      <span className="text-sm">{rel.traffic_percent}%</span>
                    </td>
                    <td className="text-sm" style={{ maxWidth: 300 }}>
                      <span className="truncate" style={{ display: 'block' }}>
                        {rel.description}
                      </span>
                    </td>
                    <td className="text-sm text-secondary">{rel.created_by}</td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatDate(rel.created_at)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default ReleasesPage;
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/pages/ReleasesPage.tsx
git commit -m "feat: wire ReleasesPage to real API"
```

---

## Task 10: Frontend — Wire ReleaseDetailPage

**Files:**
- Modify: `web/src/pages/ReleaseDetailPage.tsx`

- [ ] **Step 1: Replace ReleaseDetailPage with API-wired version**

Replace the entire contents of `web/src/pages/ReleaseDetailPage.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import ActionBar from '@/components/ActionBar';
import { releasesApi } from '@/api';
import type { Release, ReleaseStatus } from '@/types';

function getReleaseActions(status: ReleaseStatus) {
  const noop = () => {};
  switch (status) {
    case 'draft':
      return { primaryAction: { label: 'Start Rollout', onClick: noop }, secondaryActions: [{ label: 'Delete', onClick: noop, variant: 'danger' as const }] };
    case 'rolling_out':
      return { primaryAction: { label: 'Promote', onClick: noop }, secondaryActions: [{ label: 'Pause', onClick: noop }, { label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    case 'paused':
      return { primaryAction: { label: 'Resume', onClick: noop }, secondaryActions: [{ label: 'Rollback', onClick: noop, variant: 'danger' as const }] };
    default:
      return {};
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'rolling_out': return 'badge badge-active';
    case 'completed': return 'badge badge-completed';
    case 'rolled_back': return 'badge badge-danger';
    case 'paused': return 'badge badge-ops';
    case 'draft': return 'badge badge-pending';
    default: return 'badge';
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

export default function ReleaseDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/releases`;

  const [release, setRelease] = useState<Release | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    releasesApi.get(id)
      .then((data) => setRelease(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!release) return <div>Release not found.</div>;

  const actions = getReleaseActions(release.status);

  return (
    <div className="page">
      <div className="detail-header">
        <Link to={backPath} className="back-link">&larr; Releases</Link>
        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{release.name}</h1>
            {release.description && (
              <p className="detail-description">{release.description}</p>
            )}
            <div className="detail-header-badges">
              <span className={statusBadgeClass(release.status)}>{release.status.replace('_', ' ')}</span>
              <span>{release.traffic_percent}% traffic</span>
              {release.session_sticky && (
                <span className="sticky-badge">&#128274; Session sticky: {release.sticky_header}</span>
              )}
            </div>
          </div>
          <ActionBar {...actions} />
        </div>
      </div>

      <div className="table-section">
        <h2>Flag Changes</h2>
        <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
          No flag changes data available
        </p>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/pages/ReleaseDetailPage.tsx
git commit -m "feat: wire ReleaseDetailPage to real API"
```

---

## Task 11: Frontend — Clean Up Unused Mock Constants

**Files:**
- Modify: `web/src/mocks/hierarchy.ts`

- [ ] **Step 1: Remove unused constants and imports from hierarchy.ts**

Remove these constants from `web/src/mocks/hierarchy.ts`:
- `MOCK_FLAG_ENV_STATE` (was used by FlagDetailPage — now shows empty state)
- `MOCK_DEPLOYMENT_DETAIL` (was used by DeploymentDetailPage — now fetches from API)
- `MOCK_DEPLOYMENT_EVENTS` (was used by DeploymentDetailPage — now shows empty state)
- `MOCK_RELEASE_DETAIL` (was used by ReleaseDetailPage — now fetches from API)
- `MOCK_RELEASE_FLAG_CHANGES` (was used by ReleaseDetailPage — now shows empty state)

Remove the `FlagEnvState` and `DeploymentEvent` type imports from the import statement at the top, since they're only used by the removed constants. The import should become:

```typescript
import type { Application, Release, ReleaseFlagChange, Member, Group, OrgEnvironment, ApiKey } from '@/types';
```

Wait — `ReleaseFlagChange` and `Release` are used by the removed constants too. After removal, check which types are still referenced. The remaining constants use: `Application`, `OrgEnvironment`, `Member`, `Group`, `ApiKey`. Update to:

```typescript
import type { Application, Member, Group, OrgEnvironment, ApiKey } from '@/types';
```

Keep these constants (still in use):
- `MOCK_APPLICATIONS` (used by FlagDetailPage and MembersPage)
- `MOCK_ENVIRONMENTS` (used by other pages)
- `getMockEnvironments`, `getEnvironmentName` (used by other pages)
- `MOCK_MEMBERS`, `MOCK_GROUPS`, `MOCK_API_KEYS` (used by MembersPage/SettingsPage)

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors. If there are errors about missing exports, check which files still import the removed constants.

- [ ] **Step 3: Verify full frontend build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build 2>&1 | tail -10`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/mocks/hierarchy.ts
git commit -m "chore: remove unused mock constants after API wiring"
```

---

## Review Checkpoint

After all 11 tasks, verify:

- [ ] `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/` compiles
- [ ] `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -v` passes all tests
- [ ] `cd /Users/sgamel/git/DeploySentry/web && npm run build` succeeds
- [ ] No pages still import removed mock constants (grep for `MOCK_FLAG_ENV_STATE`, `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS`, `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES`)
