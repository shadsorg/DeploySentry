# Rollout Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the web UI for the rollout system shipped in Plans 1–4: pages to manage strategies, view and control live rollouts (with the six runtime actions), view and create rollout groups, and configure per-scope policies/defaults. Plus targeted integration points in the existing deploy-create and flag-rule-edit forms.

**Architecture:** New pages under `web/src/pages/`, new API client functions in `web/src/api.ts`, new TS types in `web/src/types.ts`, and new route entries in `App.tsx` under the existing org/project/app hierarchy layout. Uses the same React + TypeScript + plain-CSS utility-class pattern already established by `DeploymentsPage`, `FlagDetailPage`, etc. — no new UI library.

**Tech Stack:** React 18, TypeScript, Vite, react-router-dom v6, existing CSS utility classes (`badge badge-*`, layout primitives). Vitest for unit tests, Playwright available but NOT required for this plan (smoke-test with real API manually).

**Spec:** `docs/superpowers/specs/2026-04-18-configurable-rollout-strategies-design.md`
**Prior plans:** Plans 1–4 (all merged — templates, deploy engine, config rollouts, rollout groups + coordination).

---

## File Structure

**New files under `web/src/`:**

```
api/                               # split from monolithic api.ts (optional; see Task 1 note)
  ... (if split; otherwise api.ts grows)

pages/
  StrategiesPage.tsx               # list + create + delete + YAML import/export
  StrategyEditor.tsx               # modal/form: step-list editor + YAML tab
  RolloutsPage.tsx                 # list with status/target-type filter
  RolloutDetailPage.tsx            # phase timeline, 6 action buttons, event log
  RolloutGroupsPage.tsx            # list groups + create
  RolloutGroupDetailPage.tsx       # group with member rollouts + edit policy
  PolicyAndDefaultsTab.tsx         # a sub-tab that slots inside SettingsPage

components/rollout/
  RolloutStatusBadge.tsx           # colored pill per RolloutStatus
  PhaseTimeline.tsx                # visual timeline of phases
  StepEditorRow.tsx                # one-row editor for a strategy Step
  StrategyPicker.tsx               # dropdown + link to strategies page (for integration points)
  ReasonModal.tsx                  # prompt for reason (used by rollback + force-promote)
  ActiveRolloutsCard.tsx           # dashboard summary
```

**Modified files:**

```
web/src/api.ts                     # add strategies/rollouts/groups/policy/defaults functions
web/src/types.ts                   # add types for Strategy, Step, Rollout, RolloutGroup, etc.
web/src/App.tsx                    # add routes for new pages
web/src/pages/DeploymentsPage.tsx  # extend deploy-create form with StrategyPicker + --apply-immediately
web/src/pages/FlagDetailPage.tsx   # extend rule edit with StrategyPicker + 202 handling
web/src/pages/SettingsPage.tsx     # embed PolicyAndDefaultsTab
```

**New routes (under existing `/orgs/:orgSlug` hierarchy layout):**

```
/orgs/:orgSlug/strategies                      # StrategiesPage
/orgs/:orgSlug/rollouts                        # RolloutsPage (list)
/orgs/:orgSlug/rollouts/:id                    # RolloutDetailPage
/orgs/:orgSlug/rollout-groups                  # RolloutGroupsPage
/orgs/:orgSlug/rollout-groups/:id              # RolloutGroupDetailPage
# Settings tabs gain "Policy & Defaults" — new route not needed, new tab
```

(Project/app-scoped equivalents of StrategiesPage/PolicyAndDefaultsTab are **deferred** to keep scope tight. Plan 5 ships org-level surface only; project/app scopes remain reachable via API + CLI.)

**Smoke test baseline (manual, after Task 10):**
Boot the API (with Plans 1–4 merged), open `http://localhost:3000`, sign in to a seed org, navigate to `/orgs/<slug>/strategies` and confirm the seeded `system-canary` shows. Create a new strategy via YAML. Open `/orgs/<slug>/rollouts`, confirm empty state. Create a deploy with strategy attached via CLI → refresh, confirm the rollout appears. Click into it, try Pause → Resume → Force-promote (with reason). Observe phase timeline advance.

**Not in scope:**
- Playwright E2E tests (too flaky for fast shipping; manual smoke in DoD).
- Live SSE for rollout events (use polling with short interval; upgrade to SSE later).
- Project/app-scoped Strategies/Policy/Defaults pages (org-level only).
- Timeline graph of health signals (just status badges + textual phase log).
- Rollout `amend` UI (spec says `amend` is a hint only; no endpoint yet).

---

## Task 1: API client — extend `web/src/api.ts`

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/types.ts`

- [ ] **Step 1: Add types to `web/src/types.ts`**

Append:

```typescript
// ---- Strategies + policies + defaults (Plan 1) ----

export type TargetType = 'deploy' | 'config' | 'any';
export type ScopeType = 'org' | 'project' | 'app';
export type PolicyKind = 'off' | 'prompt' | 'mandate';

export interface Step {
  percent: number;
  min_duration: number;    // nanoseconds
  max_duration: number;
  bake_time_healthy: number;
  health_threshold?: number;
  approval?: { required_role: string; timeout: number };
  notify?: { on_entry?: string[]; on_exit?: string[] };
  abort_conditions?: Array<{
    metric: string;
    operator: string;
    threshold: number;
    window: number;
  }>;
  signal_override?: { kind: string };
}

export interface Strategy {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  name: string;
  description: string;
  target_type: TargetType;
  steps: Step[];
  default_health_threshold: number;
  default_rollback_on_failure: boolean;
  version: number;
  is_system: boolean;
  created_by?: string;
  updated_by?: string;
  created_at: string;
  updated_at: string;
}

export interface EffectiveStrategy {
  strategy: Strategy;
  origin_scope: { type: ScopeType; id: string };
  is_inherited: boolean;
}

export interface RolloutPolicy {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  environment?: string;
  target_type?: TargetType;
  enabled: boolean;
  policy: PolicyKind;
  created_at: string;
  updated_at: string;
}

export interface StrategyDefault {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  environment?: string;
  target_type?: TargetType;
  strategy_id: string;
  created_at: string;
  updated_at: string;
}

// ---- Rollouts (Plan 2+3) ----

export type RolloutStatus =
  | 'pending'
  | 'active'
  | 'paused'
  | 'awaiting_approval'
  | 'succeeded'
  | 'rolled_back'
  | 'aborted'
  | 'superseded';

export type PhaseStatus =
  | 'pending'
  | 'active'
  | 'awaiting_approval'
  | 'passed'
  | 'failed'
  | 'rolled_back';

export interface RolloutTargetRef {
  deployment_id?: string;
  flag_key?: string;
  env?: string;
  rule_id?: string;
  previous_percentage?: number;
}

export interface Rollout {
  id: string;
  release_id?: string;
  target_type: 'deploy' | 'config';
  target_ref: RolloutTargetRef;
  strategy_snapshot: Strategy;
  signal_source: { kind: string };
  status: RolloutStatus;
  current_phase_index: number;
  current_phase_started_at?: string;
  last_healthy_since?: string;
  rollback_reason?: string;
  created_by?: string;
  created_at: string;
  completed_at?: string;
}

export interface RolloutEvent {
  id: string;
  rollout_id: string;
  event_type: string;
  actor_type: 'user' | 'system';
  actor_id?: string;
  reason?: string;
  payload: Record<string, unknown>;
  occurred_at: string;
}

// ---- Rollout groups (Plan 4) ----

export type CoordinationPolicy = 'independent' | 'pause_on_sibling_abort' | 'cascade_abort';

export interface RolloutGroup {
  id: string;
  scope_type: ScopeType;
  scope_id: string;
  name: string;
  description: string;
  coordination_policy: CoordinationPolicy;
  created_by?: string;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Add API client namespace to `web/src/api.ts`**

Read the existing file first: `grep -n "^export const.*Api" web/src/api.ts | head` — existing pattern is `export const deploymentsApi = { ... }`, `export const entitiesApi = { ... }`.

Append a new namespaced group:

```typescript
// ---- Strategies ----
export const strategiesApi = {
  list: (orgSlug: string) =>
    request<{ items: EffectiveStrategy[] }>(`/orgs/${orgSlug}/strategies`),
  get: (orgSlug: string, name: string) =>
    request<Strategy>(`/orgs/${orgSlug}/strategies/${name}`),
  create: (orgSlug: string, body: {
    name: string; description: string; target_type: TargetType;
    steps: Step[]; default_health_threshold: number; default_rollback_on_failure: boolean;
  }) => request<Strategy>(`/orgs/${orgSlug}/strategies`, {
    method: 'POST', body: JSON.stringify(body),
  }),
  update: (orgSlug: string, name: string, body: {
    description: string; target_type: TargetType; steps: Step[];
    default_health_threshold: number; default_rollback_on_failure: boolean;
    expected_version: number;
  }) => request<Strategy>(`/orgs/${orgSlug}/strategies/${name}`, {
    method: 'PUT', body: JSON.stringify(body),
  }),
  delete: (orgSlug: string, name: string) =>
    request<void>(`/orgs/${orgSlug}/strategies/${name}`, { method: 'DELETE' }),
  importYAML: (orgSlug: string, yaml: string) =>
    request<Strategy>(`/orgs/${orgSlug}/strategies/import`, {
      method: 'POST',
      body: yaml,
      headers: { 'Content-Type': 'application/yaml' },
    }),
  exportYAML: (orgSlug: string, name: string) =>
    fetch(`${BASE}/orgs/${orgSlug}/strategies/${name}/export`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('ds_token') || ''}` },
    }).then(r => r.text()),
};

// ---- Strategy defaults + rollout policy ----
export const strategyDefaultsApi = {
  list: (orgSlug: string) =>
    request<{ items: StrategyDefault[] }>(`/orgs/${orgSlug}/strategy-defaults`),
  set: (orgSlug: string, body: {
    environment?: string; target_type?: TargetType; strategy_name?: string; strategy_id?: string;
  }) => request<StrategyDefault>(`/orgs/${orgSlug}/strategy-defaults`, {
    method: 'PUT', body: JSON.stringify(body),
  }),
  delete: (orgSlug: string, id: string) =>
    request<void>(`/orgs/${orgSlug}/strategy-defaults/${id}`, { method: 'DELETE' }),
};

export const rolloutPolicyApi = {
  list: (orgSlug: string) =>
    request<{ items: RolloutPolicy[] }>(`/orgs/${orgSlug}/rollout-policy`),
  set: (orgSlug: string, body: {
    environment?: string; target_type?: TargetType; enabled: boolean; policy: PolicyKind;
  }) => request<RolloutPolicy>(`/orgs/${orgSlug}/rollout-policy`, {
    method: 'PUT', body: JSON.stringify(body),
  }),
};

// ---- Rollouts (runtime control) ----
export const rolloutsApi = {
  list: (orgSlug: string, opts?: { status?: RolloutStatus; target_type?: string; limit?: number }) => {
    const params = new URLSearchParams();
    if (opts?.status) params.set('status', opts.status);
    if (opts?.target_type) params.set('target_type', opts.target_type);
    if (opts?.limit) params.set('limit', String(opts.limit));
    const qs = params.toString();
    return request<{ items: Rollout[] }>(`/orgs/${orgSlug}/rollouts${qs ? '?' + qs : ''}`);
  },
  get: (orgSlug: string, id: string) =>
    request<Rollout>(`/orgs/${orgSlug}/rollouts/${id}`),
  pause: (orgSlug: string, id: string, reason?: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollouts/${id}/pause`, {
      method: 'POST', body: JSON.stringify({ reason: reason || '' }),
    }),
  resume: (orgSlug: string, id: string, reason?: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollouts/${id}/resume`, {
      method: 'POST', body: JSON.stringify({ reason: reason || '' }),
    }),
  promote: (orgSlug: string, id: string, reason?: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollouts/${id}/promote`, {
      method: 'POST', body: JSON.stringify({ reason: reason || '' }),
    }),
  approve: (orgSlug: string, id: string, reason?: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollouts/${id}/approve`, {
      method: 'POST', body: JSON.stringify({ reason: reason || '' }),
    }),
  rollback: (orgSlug: string, id: string, reason: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollouts/${id}/rollback`, {
      method: 'POST', body: JSON.stringify({ reason }),
    }),
  forcePromote: (orgSlug: string, id: string, reason: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollouts/${id}/force-promote`, {
      method: 'POST', body: JSON.stringify({ reason }),
    }),
  events: (orgSlug: string, id: string, limit = 100) =>
    request<{ items: RolloutEvent[] }>(`/orgs/${orgSlug}/rollouts/${id}/events?limit=${limit}`),
};

// ---- Rollout groups ----
export const rolloutGroupsApi = {
  list: (orgSlug: string) =>
    request<{ items: RolloutGroup[] }>(`/orgs/${orgSlug}/rollout-groups`),
  get: (orgSlug: string, id: string) =>
    request<{ group: RolloutGroup; members: Rollout[] }>(`/orgs/${orgSlug}/rollout-groups/${id}`),
  create: (orgSlug: string, body: { name: string; description?: string; coordination_policy?: CoordinationPolicy }) =>
    request<RolloutGroup>(`/orgs/${orgSlug}/rollout-groups`, {
      method: 'POST', body: JSON.stringify(body),
    }),
  update: (orgSlug: string, id: string, body: { name: string; description: string; coordination_policy: CoordinationPolicy }) =>
    request<RolloutGroup>(`/orgs/${orgSlug}/rollout-groups/${id}`, {
      method: 'PUT', body: JSON.stringify(body),
    }),
  attach: (orgSlug: string, id: string, rolloutId: string) =>
    request<{ ok: boolean }>(`/orgs/${orgSlug}/rollout-groups/${id}/attach`, {
      method: 'POST', body: JSON.stringify({ rollout_id: rolloutId }),
    }),
};
```

The `request<T>` helper and `BASE` constant already exist in the file from the existing codebase (Plan 2's `deploymentsApi` uses them). Match whichever error-handling pattern they use — `request` already parses JSON and throws on non-2xx.

- [ ] **Step 3: Build check**

```bash
cd /Users/sgamel/git/DeploySentry-web-ui/web
npm run lint
npx tsc --noEmit
```

Both must pass. Fix any TS errors (e.g., if `Step`'s `number` nanosecond fields conflict with existing patterns — adapt to match).

- [ ] **Step 4: Commit**

```bash
git add web/src/api.ts web/src/types.ts
git commit -m "feat(web): API client + types for strategies, rollouts, groups, policy"
```

---

## Task 2: Shared components

**Files:**
- Create: `web/src/components/rollout/RolloutStatusBadge.tsx`
- Create: `web/src/components/rollout/ReasonModal.tsx`
- Create: `web/src/components/rollout/StrategyPicker.tsx`
- Create: `web/src/components/rollout/PhaseTimeline.tsx`

- [ ] **Step 1: RolloutStatusBadge**

Create `web/src/components/rollout/RolloutStatusBadge.tsx`:

```tsx
import type { RolloutStatus } from '@/types';

const CLASS_BY_STATUS: Record<RolloutStatus, string> = {
  pending: 'badge badge-pending',
  active: 'badge badge-active',
  paused: 'badge badge-warning',
  awaiting_approval: 'badge badge-warning',
  succeeded: 'badge badge-completed',
  rolled_back: 'badge badge-rolling-back',
  aborted: 'badge badge-failed',
  superseded: 'badge badge-disabled',
};

const LABEL_BY_STATUS: Record<RolloutStatus, string> = {
  pending: 'Pending',
  active: 'Active',
  paused: 'Paused',
  awaiting_approval: 'Awaiting Approval',
  succeeded: 'Succeeded',
  rolled_back: 'Rolled Back',
  aborted: 'Aborted',
  superseded: 'Superseded',
};

export function RolloutStatusBadge({ status }: { status: RolloutStatus }) {
  return <span className={CLASS_BY_STATUS[status]}>{LABEL_BY_STATUS[status]}</span>;
}
```

- [ ] **Step 2: ReasonModal**

Create `web/src/components/rollout/ReasonModal.tsx`:

```tsx
import { useState } from 'react';

interface Props {
  title: string;
  placeholder?: string;
  required?: boolean;
  onConfirm: (reason: string) => void | Promise<void>;
  onCancel: () => void;
}

export function ReasonModal({ title, placeholder, required = false, onConfirm, onCancel }: Props) {
  const [reason, setReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleConfirm() {
    if (required && !reason.trim()) return;
    setSubmitting(true);
    try {
      await onConfirm(reason.trim());
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{title}</h3>
        <textarea
          autoFocus
          placeholder={placeholder || (required ? 'Reason (required)' : 'Reason (optional)')}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={4}
        />
        <div className="modal-actions">
          <button type="button" onClick={onCancel} disabled={submitting}>
            Cancel
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={submitting || (required && !reason.trim())}
            className="btn-primary"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: StrategyPicker**

Create `web/src/components/rollout/StrategyPicker.tsx`:

```tsx
import { useEffect, useState } from 'react';
import type { EffectiveStrategy, TargetType } from '@/types';
import { strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
  targetType?: TargetType; // filter results
  value: string; // selected strategy name ('' = none)
  onChange: (strategyName: string) => void;
  allowImmediate?: boolean;
  immediate: boolean;
  onImmediateChange: (v: boolean) => void;
}

export function StrategyPicker({ orgSlug, targetType, value, onChange, allowImmediate, immediate, onImmediateChange }: Props) {
  const [options, setOptions] = useState<EffectiveStrategy[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    strategiesApi.list(orgSlug)
      .then((r) => {
        const filtered = targetType
          ? r.items.filter((e) => e.strategy.target_type === targetType || e.strategy.target_type === 'any')
          : r.items;
        setOptions(filtered);
      })
      .finally(() => setLoading(false));
  }, [orgSlug, targetType]);

  if (loading) return <span>Loading strategies…</span>;

  return (
    <div className="strategy-picker">
      {allowImmediate && (
        <label>
          <input
            type="checkbox"
            checked={immediate}
            onChange={(e) => onImmediateChange(e.target.checked)}
          />
          Apply immediately (skip rollout)
        </label>
      )}
      {!immediate && (
        <select value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">— select strategy —</option>
          {options.map((eff) => (
            <option key={eff.strategy.id} value={eff.strategy.name}>
              {eff.strategy.name}
              {eff.is_inherited ? ` (inherited from ${eff.origin_scope.type})` : ''}
            </option>
          ))}
        </select>
      )}
    </div>
  );
}
```

- [ ] **Step 4: PhaseTimeline**

Create `web/src/components/rollout/PhaseTimeline.tsx`:

```tsx
import type { Rollout } from '@/types';

export function PhaseTimeline({ rollout }: { rollout: Rollout }) {
  const steps = rollout.strategy_snapshot.steps;
  return (
    <div className="phase-timeline">
      {steps.map((step, idx) => {
        let state: 'done' | 'current' | 'pending';
        if (idx < rollout.current_phase_index) state = 'done';
        else if (idx === rollout.current_phase_index) state = 'current';
        else state = 'pending';
        return (
          <div key={idx} className={`phase-node phase-${state}`}>
            <div className="phase-index">{idx + 1}</div>
            <div className="phase-percent">{step.percent}%</div>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 5: Build check + commit**

```bash
cd /Users/sgamel/git/DeploySentry-web-ui/web
npx tsc --noEmit
npm run lint
```

Ignore missing CSS class warnings — we'll add `.strategy-picker` / `.phase-timeline` / `.modal-*` utility class styles at the end of Task 3 or via existing class hierarchy. If the project's ESLint is strict on unused imports, fix those.

```bash
git add web/src/components/rollout/
git commit -m "feat(web): shared rollout components (status badge, reason modal, strategy picker, phase timeline)"
```

---

## Task 3: StrategiesPage + StrategyEditor

**Files:**
- Create: `web/src/pages/StrategiesPage.tsx`
- Create: `web/src/pages/StrategyEditor.tsx`

- [ ] **Step 1: StrategiesPage — list + delete + create button**

Create `web/src/pages/StrategiesPage.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { EffectiveStrategy } from '@/types';
import { strategiesApi } from '@/api';
import { StrategyEditor } from './StrategyEditor';

export default function StrategiesPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<EffectiveStrategy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<'new' | string | null>(null); // 'new' or strategy name

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const r = await strategiesApi.list(orgSlug);
      setItems(r.items);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [orgSlug]);

  async function handleDelete(name: string) {
    if (!confirm(`Delete strategy "${name}"?`)) return;
    try {
      await strategiesApi.delete(orgSlug, name);
      await load();
    } catch (e) {
      alert(`Delete failed: ${e}`);
    }
  }

  async function handleImportYAML(file: File) {
    const yaml = await file.text();
    try {
      await strategiesApi.importYAML(orgSlug, yaml);
      await load();
    } catch (e) {
      alert(`Import failed: ${e}`);
    }
  }

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollout Strategies</h1>
        <div className="header-actions">
          <label className="btn">
            Import YAML
            <input
              type="file"
              accept=".yaml,.yml"
              style={{ display: 'none' }}
              onChange={(e) => e.target.files?.[0] && handleImportYAML(e.target.files[0])}
            />
          </label>
          <button className="btn btn-primary" onClick={() => setEditing('new')}>
            New Strategy
          </button>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {error && <p className="error">{error}</p>}

      {!loading && items.length === 0 && (
        <p className="empty-state">No strategies yet. Click "New Strategy" to create one.</p>
      )}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Target</th>
              <th>Version</th>
              <th>Origin</th>
              <th>System</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((eff) => (
              <tr key={eff.strategy.id}>
                <td>
                  <Link to="#" onClick={(e) => { e.preventDefault(); setEditing(eff.strategy.name); }}>
                    {eff.strategy.name}
                  </Link>
                </td>
                <td>{eff.strategy.target_type}</td>
                <td>v{eff.strategy.version}</td>
                <td>
                  {eff.origin_scope.type}
                  {eff.is_inherited && ' (inh)'}
                </td>
                <td>{eff.strategy.is_system ? 'yes' : 'no'}</td>
                <td>
                  <button
                    onClick={async () => {
                      const yaml = await strategiesApi.exportYAML(orgSlug, eff.strategy.name);
                      const blob = new Blob([yaml], { type: 'application/yaml' });
                      const url = URL.createObjectURL(blob);
                      const a = document.createElement('a');
                      a.href = url;
                      a.download = `${eff.strategy.name}.yaml`;
                      a.click();
                      URL.revokeObjectURL(url);
                    }}
                  >
                    Export
                  </button>
                  {!eff.strategy.is_system && !eff.is_inherited && (
                    <button onClick={() => handleDelete(eff.strategy.name)}>Delete</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {editing && (
        <StrategyEditor
          orgSlug={orgSlug}
          strategyName={editing === 'new' ? null : editing}
          onClose={() => { setEditing(null); load(); }}
        />
      )}
    </div>
  );
}
```

- [ ] **Step 2: StrategyEditor — modal form for create/edit**

Create `web/src/pages/StrategyEditor.tsx`:

```tsx
import { useEffect, useState } from 'react';
import type { Step, TargetType } from '@/types';
import { strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
  strategyName: string | null; // null = creating new
  onClose: () => void;
}

const MS = 1_000_000; // nanoseconds per millisecond

function emptyStep(): Step {
  return {
    percent: 0,
    min_duration: 0,
    max_duration: 0,
    bake_time_healthy: 0,
  };
}

export function StrategyEditor({ orgSlug, strategyName, onClose }: Props) {
  const [name, setName] = useState(strategyName ?? '');
  const [description, setDescription] = useState('');
  const [targetType, setTargetType] = useState<TargetType>('deploy');
  const [healthThreshold, setHealthThreshold] = useState(0.95);
  const [rollbackOnFailure, setRollbackOnFailure] = useState(true);
  const [steps, setSteps] = useState<Step[]>([emptyStep()]);
  const [expectedVersion, setExpectedVersion] = useState(1);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (strategyName) {
      strategiesApi.get(orgSlug, strategyName).then((s) => {
        setName(s.name);
        setDescription(s.description);
        setTargetType(s.target_type);
        setHealthThreshold(s.default_health_threshold);
        setRollbackOnFailure(s.default_rollback_on_failure);
        setSteps(s.steps);
        setExpectedVersion(s.version);
      });
    }
  }, [orgSlug, strategyName]);

  function updateStep(idx: number, patch: Partial<Step>) {
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, ...patch } : s)));
  }

  function removeStep(idx: number) {
    setSteps((prev) => prev.filter((_, i) => i !== idx));
  }

  function addStep() {
    setSteps((prev) => [...prev, emptyStep()]);
  }

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      if (strategyName) {
        await strategiesApi.update(orgSlug, strategyName, {
          description,
          target_type: targetType,
          steps,
          default_health_threshold: healthThreshold,
          default_rollback_on_failure: rollbackOnFailure,
          expected_version: expectedVersion,
        });
      } else {
        await strategiesApi.create(orgSlug, {
          name,
          description,
          target_type: targetType,
          steps,
          default_health_threshold: healthThreshold,
          default_rollback_on_failure: rollbackOnFailure,
        });
      }
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal modal-wide" onClick={(e) => e.stopPropagation()}>
        <h3>{strategyName ? `Edit Strategy: ${strategyName}` : 'New Strategy'}</h3>

        {error && <p className="error">{error}</p>}

        <label>Name
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={Boolean(strategyName)}
          />
        </label>

        <label>Description
          <input type="text" value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>

        <label>Target Type
          <select value={targetType} onChange={(e) => setTargetType(e.target.value as TargetType)}>
            <option value="deploy">deploy</option>
            <option value="config">config</option>
            <option value="any">any</option>
          </select>
        </label>

        <label>Default Health Threshold (0–1)
          <input
            type="number"
            min={0}
            max={1}
            step={0.01}
            value={healthThreshold}
            onChange={(e) => setHealthThreshold(Number(e.target.value))}
          />
        </label>

        <label>
          <input
            type="checkbox"
            checked={rollbackOnFailure}
            onChange={(e) => setRollbackOnFailure(e.target.checked)}
          />
          Rollback on failure
        </label>

        <h4>Steps</h4>
        <table className="step-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Percent</th>
              <th>Min (ms)</th>
              <th>Max (ms)</th>
              <th>Bake (ms)</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {steps.map((s, idx) => (
              <tr key={idx}>
                <td>{idx + 1}</td>
                <td><input type="number" value={s.percent} onChange={(e) => updateStep(idx, { percent: Number(e.target.value) })} /></td>
                <td><input type="number" value={s.min_duration / MS} onChange={(e) => updateStep(idx, { min_duration: Number(e.target.value) * MS })} /></td>
                <td><input type="number" value={s.max_duration / MS} onChange={(e) => updateStep(idx, { max_duration: Number(e.target.value) * MS })} /></td>
                <td><input type="number" value={s.bake_time_healthy / MS} onChange={(e) => updateStep(idx, { bake_time_healthy: Number(e.target.value) * MS })} /></td>
                <td>
                  {steps.length > 1 && (
                    <button type="button" onClick={() => removeStep(idx)}>×</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <button type="button" onClick={addStep}>+ Add Step</button>

        <div className="modal-actions">
          <button type="button" onClick={onClose}>Cancel</button>
          <button type="button" onClick={submit} disabled={submitting} className="btn-primary">
            {strategyName ? 'Save' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Route + nav entry**

In `web/src/App.tsx`, add inside the `/orgs/:orgSlug` HierarchyLayout block:

```tsx
<Route path="strategies" element={<StrategiesPage />} />
```

And import `StrategiesPage` at the top (use existing lazy-load pattern if the file uses it, else direct import).

In the sidebar / nav component (probably `web/src/components/Sidebar.tsx`), add an entry for Strategies under the org-level section. Read the existing sidebar file first and match whatever pattern it uses for org-level links.

- [ ] **Step 4: Build check**

```bash
cd web
npx tsc --noEmit
npm run lint
npm run build
```

All must pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/StrategiesPage.tsx web/src/pages/StrategyEditor.tsx web/src/App.tsx web/src/components/Sidebar.tsx
git commit -m "feat(web): StrategiesPage + StrategyEditor with step-builder form"
```

---

## Task 4: RolloutsPage — list view

**Files:**
- Create: `web/src/pages/RolloutsPage.tsx`

- [ ] **Step 1: Implement**

Create `web/src/pages/RolloutsPage.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Rollout, RolloutStatus } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

const STATUS_FILTERS: RolloutStatus[] = ['active', 'paused', 'awaiting_approval', 'succeeded', 'rolled_back'];

export default function RolloutsPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<Rollout[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<RolloutStatus | ''>('');

  async function load() {
    setLoading(true);
    try {
      const r = await rolloutsApi.list(orgSlug, { status: filter || undefined, limit: 100 });
      setItems(r.items || []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, [orgSlug, filter]);

  // Auto-refresh active rollouts.
  useEffect(() => {
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [orgSlug, filter]);

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollouts</h1>
        <div className="header-actions">
          <select value={filter} onChange={(e) => setFilter(e.target.value as RolloutStatus | '')}>
            <option value="">All statuses</option>
            {STATUS_FILTERS.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {!loading && items.length === 0 && <p className="empty-state">No rollouts match the filter.</p>}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Target</th>
              <th>Strategy</th>
              <th>Phase</th>
              <th>Status</th>
              <th>Started</th>
            </tr>
          </thead>
          <tbody>
            {items.map((r) => (
              <tr key={r.id}>
                <td>
                  <Link to={`/orgs/${orgSlug}/rollouts/${r.id}`}>{r.id.slice(0, 8)}</Link>
                </td>
                <td>
                  {r.target_type === 'deploy'
                    ? `deploy ${r.target_ref.deployment_id?.slice(0, 8) ?? ''}`
                    : `rule ${r.target_ref.rule_id?.slice(0, 8) ?? ''}`}
                </td>
                <td>{r.strategy_snapshot.name}</td>
                <td>
                  {r.current_phase_index + 1}/{r.strategy_snapshot.steps.length}
                  {' • '}
                  {r.strategy_snapshot.steps[r.current_phase_index]?.percent ?? '?'}%
                </td>
                <td><RolloutStatusBadge status={r.status} /></td>
                <td>{new Date(r.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Route**

In `web/src/App.tsx`, add inside the `/orgs/:orgSlug` block:

```tsx
<Route path="rollouts" element={<RolloutsPage />} />
```

Import `RolloutsPage` at the top.

Add to sidebar nav.

- [ ] **Step 3: Build check + commit**

```bash
cd web
npx tsc --noEmit && npm run lint && npm run build
git add web/src/pages/RolloutsPage.tsx web/src/App.tsx web/src/components/Sidebar.tsx
git commit -m "feat(web): RolloutsPage list view with status filter + auto-refresh"
```

---

## Task 5: RolloutDetailPage — timeline + 6 actions + event log

**Files:**
- Create: `web/src/pages/RolloutDetailPage.tsx`

- [ ] **Step 1: Implement**

Create `web/src/pages/RolloutDetailPage.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import type { Rollout, RolloutEvent } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';
import { PhaseTimeline } from '@/components/rollout/PhaseTimeline';
import { ReasonModal } from '@/components/rollout/ReasonModal';

type ReasonAction = 'rollback' | 'force-promote';

export default function RolloutDetailPage() {
  const { orgSlug = '', id = '' } = useParams();
  const [rollout, setRollout] = useState<Rollout | null>(null);
  const [events, setEvents] = useState<RolloutEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reasonModal, setReasonModal] = useState<ReasonAction | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [r, e] = await Promise.all([
        rolloutsApi.get(orgSlug, id),
        rolloutsApi.events(orgSlug, id, 100),
      ]);
      setRollout(r);
      setEvents(e.items);
      setError(null);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }, [orgSlug, id]);

  useEffect(() => { load(); }, [load]);

  // Poll while the rollout is non-terminal.
  useEffect(() => {
    if (!rollout) return;
    const terminal = ['succeeded', 'rolled_back', 'aborted', 'superseded'];
    if (terminal.includes(rollout.status)) return;
    const t = setInterval(load, 3000);
    return () => clearInterval(t);
  }, [rollout, load]);

  async function runAction(name: string, fn: () => Promise<unknown>) {
    setBusy(name);
    try {
      await fn();
      await load();
    } catch (e) {
      alert(`${name} failed: ${e}`);
    } finally {
      setBusy(null);
    }
  }

  if (loading) return <div className="page"><p>Loading…</p></div>;
  if (error) return <div className="page"><p className="error">{error}</p></div>;
  if (!rollout) return <div className="page"><p>Rollout not found.</p></div>;

  const canAct = !['succeeded', 'rolled_back', 'aborted', 'superseded'].includes(rollout.status);

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollout {rollout.id.slice(0, 8)}</h1>
        <div>
          <RolloutStatusBadge status={rollout.status} />
        </div>
      </header>

      <section className="card">
        <h3>Strategy: {rollout.strategy_snapshot.name}</h3>
        <p>Target type: {rollout.target_type}</p>
        <p>Phase: {rollout.current_phase_index + 1} / {rollout.strategy_snapshot.steps.length}</p>
        {rollout.rollback_reason && <p className="error">Rollback reason: {rollout.rollback_reason}</p>}
        <PhaseTimeline rollout={rollout} />
      </section>

      {canAct && (
        <section className="card">
          <h3>Actions</h3>
          <div className="action-bar">
            <button
              disabled={busy !== null || rollout.status !== 'active'}
              onClick={() => runAction('pause', () => rolloutsApi.pause(orgSlug, rollout.id))}
            >
              Pause
            </button>
            <button
              disabled={busy !== null || rollout.status !== 'paused'}
              onClick={() => runAction('resume', () => rolloutsApi.resume(orgSlug, rollout.id))}
            >
              Resume
            </button>
            <button
              disabled={busy !== null || !['active', 'paused'].includes(rollout.status)}
              onClick={() => runAction('promote', () => rolloutsApi.promote(orgSlug, rollout.id))}
            >
              Promote
            </button>
            <button
              disabled={busy !== null || rollout.status !== 'awaiting_approval'}
              onClick={() => runAction('approve', () => rolloutsApi.approve(orgSlug, rollout.id))}
            >
              Approve
            </button>
            <button
              disabled={busy !== null}
              className="btn-danger"
              onClick={() => setReasonModal('rollback')}
            >
              Rollback
            </button>
            <button
              disabled={busy !== null}
              className="btn-warning"
              onClick={() => setReasonModal('force-promote')}
            >
              Force Promote
            </button>
          </div>
        </section>
      )}

      <section className="card">
        <h3>Event Log</h3>
        {events.length === 0 ? (
          <p className="empty-state">No events yet.</p>
        ) : (
          <ul className="event-list">
            {events.map((ev) => (
              <li key={ev.id}>
                <span className="event-time">{new Date(ev.occurred_at).toLocaleString()}</span>
                <span className="event-type">{ev.event_type}</span>
                <span className="event-actor">{ev.actor_type}{ev.actor_id ? ` · ${ev.actor_id.slice(0, 8)}` : ''}</span>
                {ev.reason && <span className="event-reason">"{ev.reason}"</span>}
              </li>
            ))}
          </ul>
        )}
      </section>

      {reasonModal === 'rollback' && (
        <ReasonModal
          title="Rollback Rollout"
          placeholder="Why are you rolling back?"
          required
          onConfirm={async (reason) => {
            setReasonModal(null);
            await runAction('rollback', () => rolloutsApi.rollback(orgSlug, rollout.id, reason));
          }}
          onCancel={() => setReasonModal(null)}
        />
      )}
      {reasonModal === 'force-promote' && (
        <ReasonModal
          title="Force Promote (Override Health)"
          placeholder="Explain why health gates are being overridden"
          required
          onConfirm={async (reason) => {
            setReasonModal(null);
            await runAction('force-promote', () => rolloutsApi.forcePromote(orgSlug, rollout.id, reason));
          }}
          onCancel={() => setReasonModal(null)}
        />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Route**

In `web/src/App.tsx`:

```tsx
<Route path="rollouts/:id" element={<RolloutDetailPage />} />
```

- [ ] **Step 3: Build + commit**

```bash
cd web
npx tsc --noEmit && npm run lint && npm run build
git add web/src/pages/RolloutDetailPage.tsx web/src/App.tsx
git commit -m "feat(web): RolloutDetailPage with phase timeline, 6 actions, event log"
```

---

## Task 6: RolloutGroupsPage + RolloutGroupDetailPage

**Files:**
- Create: `web/src/pages/RolloutGroupsPage.tsx`
- Create: `web/src/pages/RolloutGroupDetailPage.tsx`

- [ ] **Step 1: RolloutGroupsPage**

Create `web/src/pages/RolloutGroupsPage.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutGroup, CoordinationPolicy } from '@/types';
import { rolloutGroupsApi } from '@/api';

const POLICIES: CoordinationPolicy[] = ['independent', 'pause_on_sibling_abort', 'cascade_abort'];

export default function RolloutGroupsPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<RolloutGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [policy, setPolicy] = useState<CoordinationPolicy>('independent');

  async function load() {
    setLoading(true);
    try {
      const r = await rolloutGroupsApi.list(orgSlug);
      setItems(r.items || []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, [orgSlug]);

  async function handleCreate() {
    if (!name.trim()) return;
    try {
      await rolloutGroupsApi.create(orgSlug, { name, description, coordination_policy: policy });
      setCreating(false);
      setName(''); setDescription(''); setPolicy('independent');
      await load();
    } catch (e) {
      alert(`Create failed: ${e}`);
    }
  }

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollout Groups</h1>
        <div className="header-actions">
          <button className="btn btn-primary" onClick={() => setCreating(true)}>New Group</button>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {!loading && items.length === 0 && <p className="empty-state">No rollout groups yet.</p>}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr><th>Name</th><th>Policy</th><th>Created</th></tr>
          </thead>
          <tbody>
            {items.map((g) => (
              <tr key={g.id}>
                <td><Link to={`/orgs/${orgSlug}/rollout-groups/${g.id}`}>{g.name}</Link></td>
                <td>{g.coordination_policy}</td>
                <td>{new Date(g.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {creating && (
        <div className="modal-backdrop" onClick={() => setCreating(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>New Rollout Group</h3>
            <label>Name<input value={name} onChange={(e) => setName(e.target.value)} /></label>
            <label>Description<input value={description} onChange={(e) => setDescription(e.target.value)} /></label>
            <label>Coordination Policy
              <select value={policy} onChange={(e) => setPolicy(e.target.value as CoordinationPolicy)}>
                {POLICIES.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </label>
            <div className="modal-actions">
              <button onClick={() => setCreating(false)}>Cancel</button>
              <button className="btn-primary" onClick={handleCreate}>Create</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: RolloutGroupDetailPage**

Create `web/src/pages/RolloutGroupDetailPage.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutGroup, Rollout, CoordinationPolicy } from '@/types';
import { rolloutGroupsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

const POLICIES: CoordinationPolicy[] = ['independent', 'pause_on_sibling_abort', 'cascade_abort'];

export default function RolloutGroupDetailPage() {
  const { orgSlug = '', id = '' } = useParams();
  const [group, setGroup] = useState<RolloutGroup | null>(null);
  const [members, setMembers] = useState<Rollout[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await rolloutGroupsApi.get(orgSlug, id);
      setGroup(r.group);
      setMembers(r.members || []);
    } finally {
      setLoading(false);
    }
  }, [orgSlug, id]);

  useEffect(() => { load(); }, [load]);

  async function savePolicy(next: CoordinationPolicy) {
    if (!group) return;
    try {
      await rolloutGroupsApi.update(orgSlug, group.id, {
        name: group.name,
        description: group.description,
        coordination_policy: next,
      });
      setEditing(false);
      await load();
    } catch (e) {
      alert(`Update failed: ${e}`);
    }
  }

  if (loading) return <div className="page"><p>Loading…</p></div>;
  if (!group) return <div className="page"><p>Group not found.</p></div>;

  return (
    <div className="page">
      <header className="page-header">
        <h1>{group.name}</h1>
      </header>

      <section className="card">
        <p>{group.description || <em>No description.</em>}</p>
        <p>
          Coordination: <strong>{group.coordination_policy}</strong>
          {' '}<button onClick={() => setEditing(!editing)}>Edit</button>
        </p>
        {editing && (
          <select
            defaultValue={group.coordination_policy}
            onChange={(e) => savePolicy(e.target.value as CoordinationPolicy)}
          >
            {POLICIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        )}
      </section>

      <section className="card">
        <h3>Member Rollouts ({members.length})</h3>
        {members.length === 0 ? (
          <p className="empty-state">No rollouts attached.</p>
        ) : (
          <table className="data-table">
            <thead>
              <tr><th>ID</th><th>Target</th><th>Status</th></tr>
            </thead>
            <tbody>
              {members.map((r) => (
                <tr key={r.id}>
                  <td><Link to={`/orgs/${orgSlug}/rollouts/${r.id}`}>{r.id.slice(0, 8)}</Link></td>
                  <td>{r.target_type}</td>
                  <td><RolloutStatusBadge status={r.status} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
```

- [ ] **Step 3: Routes**

In `web/src/App.tsx`:

```tsx
<Route path="rollout-groups" element={<RolloutGroupsPage />} />
<Route path="rollout-groups/:id" element={<RolloutGroupDetailPage />} />
```

- [ ] **Step 4: Build + commit**

```bash
cd web
npx tsc --noEmit && npm run lint && npm run build
git add web/src/pages/RolloutGroupsPage.tsx web/src/pages/RolloutGroupDetailPage.tsx web/src/App.tsx
git commit -m "feat(web): RolloutGroupsPage + RolloutGroupDetailPage"
```

---

## Task 7: Policy & Defaults settings tab

**Files:**
- Create: `web/src/pages/PolicyAndDefaultsTab.tsx`
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: PolicyAndDefaultsTab**

Create `web/src/pages/PolicyAndDefaultsTab.tsx`:

```tsx
import { useEffect, useState } from 'react';
import type { RolloutPolicy, StrategyDefault, PolicyKind, TargetType, EffectiveStrategy } from '@/types';
import { rolloutPolicyApi, strategyDefaultsApi, strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
}

const POLICIES: PolicyKind[] = ['off', 'prompt', 'mandate'];
const TARGETS: TargetType[] = ['deploy', 'config'];

export default function PolicyAndDefaultsTab({ orgSlug }: Props) {
  const [policies, setPolicies] = useState<RolloutPolicy[]>([]);
  const [defaults, setDefaults] = useState<StrategyDefault[]>([]);
  const [strategies, setStrategies] = useState<EffectiveStrategy[]>([]);

  // Simple top-level policy row editor.
  const [topPolicy, setTopPolicy] = useState<PolicyKind>('off');
  const [topEnabled, setTopEnabled] = useState(false);

  async function load() {
    const [pol, def, strat] = await Promise.all([
      rolloutPolicyApi.list(orgSlug),
      strategyDefaultsApi.list(orgSlug),
      strategiesApi.list(orgSlug),
    ]);
    setPolicies(pol.items);
    setDefaults(def.items);
    setStrategies(strat.items);
    // Use the org-wide policy row (no env, no target_type) as the top-level pick.
    const top = pol.items.find((p) => !p.environment && !p.target_type);
    if (top) {
      setTopPolicy(top.policy);
      setTopEnabled(top.enabled);
    }
  }

  useEffect(() => { load(); }, [orgSlug]);

  async function saveTopPolicy() {
    await rolloutPolicyApi.set(orgSlug, { enabled: topEnabled, policy: topPolicy });
    await load();
  }

  return (
    <div>
      <h3>Rollout Policy (org-wide)</h3>
      <label>
        <input type="checkbox" checked={topEnabled} onChange={(e) => setTopEnabled(e.target.checked)} />
        Enable rollout control
      </label>
      <label>Policy
        <select value={topPolicy} onChange={(e) => setTopPolicy(e.target.value as PolicyKind)}>
          {POLICIES.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>
      </label>
      <button onClick={saveTopPolicy}>Save</button>

      <h4>Per-scope Overrides ({policies.length})</h4>
      {policies.length === 0 && <p className="empty-state">No overrides.</p>}
      {policies.length > 0 && (
        <table className="data-table">
          <thead><tr><th>Env</th><th>Target</th><th>Enabled</th><th>Policy</th></tr></thead>
          <tbody>
            {policies.map((p) => (
              <tr key={p.id}>
                <td>{p.environment || '*'}</td>
                <td>{p.target_type || '*'}</td>
                <td>{p.enabled ? 'yes' : 'no'}</td>
                <td>{p.policy}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>Strategy Defaults</h3>
      {defaults.length === 0 && <p className="empty-state">No defaults set.</p>}
      {defaults.length > 0 && (
        <table className="data-table">
          <thead><tr><th>Env</th><th>Target</th><th>Strategy</th></tr></thead>
          <tbody>
            {defaults.map((d) => {
              const strat = strategies.find((s) => s.strategy.id === d.strategy_id);
              return (
                <tr key={d.id}>
                  <td>{d.environment || '*'}</td>
                  <td>{d.target_type || '*'}</td>
                  <td>{strat ? strat.strategy.name : d.strategy_id.slice(0, 8)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      <DefaultRowEditor orgSlug={orgSlug} strategies={strategies} onSaved={load} />
    </div>
  );
}

function DefaultRowEditor({ orgSlug, strategies, onSaved }: { orgSlug: string; strategies: EffectiveStrategy[]; onSaved: () => void }) {
  const [env, setEnv] = useState('');
  const [target, setTarget] = useState<TargetType | ''>('');
  const [strategyName, setStrategyName] = useState('');

  async function save() {
    if (!strategyName) return;
    await strategyDefaultsApi.set(orgSlug, {
      environment: env || undefined,
      target_type: (target || undefined) as TargetType | undefined,
      strategy_name: strategyName,
    });
    setEnv(''); setTarget(''); setStrategyName('');
    onSaved();
  }

  return (
    <div className="card" style={{ marginTop: 8 }}>
      <h4>Add Default</h4>
      <label>Env (optional)<input value={env} onChange={(e) => setEnv(e.target.value)} /></label>
      <label>Target
        <select value={target} onChange={(e) => setTarget(e.target.value as TargetType | '')}>
          <option value="">(any)</option>
          {TARGETS.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </label>
      <label>Strategy
        <select value={strategyName} onChange={(e) => setStrategyName(e.target.value)}>
          <option value="">— pick —</option>
          {strategies.map((s) => (
            <option key={s.strategy.id} value={s.strategy.name}>{s.strategy.name}</option>
          ))}
        </select>
      </label>
      <button onClick={save} disabled={!strategyName}>Save</button>
    </div>
  );
}
```

- [ ] **Step 2: Embed in SettingsPage**

Read `web/src/pages/SettingsPage.tsx` first to understand how tabs are composed. It takes a `level` prop (`'org' | 'project' | 'app'`). Add a new tab when `level === 'org'`:

```tsx
// Inside the tab list / switch, add:
{level === 'org' && <TabButton active={tab === 'rollout'} onClick={() => setTab('rollout')}>Rollout Policy</TabButton>}

// In the tab content switch:
{tab === 'rollout' && orgSlug && <PolicyAndDefaultsTab orgSlug={orgSlug} />}
```

Adapt to the actual tab pattern in the existing file — could use a string enum, could use React children, etc. Follow the existing code's pattern.

- [ ] **Step 3: Build + commit**

```bash
cd web
npx tsc --noEmit && npm run lint && npm run build
git add web/src/pages/PolicyAndDefaultsTab.tsx web/src/pages/SettingsPage.tsx
git commit -m "feat(web): Policy & Defaults settings tab"
```

---

## Task 8: Integration — DeploymentsPage strategy picker

**Files:**
- Modify: `web/src/pages/DeploymentsPage.tsx`

Check first whether `DeploymentsPage.tsx` has a "create deployment" form, or whether it's only a list. If there's no create form in the UI today, skip the UI extension and document the gap; the CLI covers that flow. If there IS a deploy-create form:

- [ ] **Step 1: Read the current DeploymentsPage**

```bash
grep -n "create\|POST\|onSubmit" web/src/pages/DeploymentsPage.tsx | head -20
```

- [ ] **Step 2 (conditional): Add StrategyPicker to the create form**

If a create form exists, inside its JSX near the existing strategy select, add:

```tsx
import { StrategyPicker } from '@/components/rollout/StrategyPicker';

// Inside the form state:
const [strategyName, setStrategyName] = useState('');
const [applyImmediately, setApplyImmediately] = useState(false);

// Inside the JSX (near the existing strategy select):
<StrategyPicker
  orgSlug={orgSlug}
  targetType="deploy"
  value={strategyName}
  onChange={setStrategyName}
  allowImmediate
  immediate={applyImmediately}
  onImmediateChange={setApplyImmediately}
/>

// In the submit handler, include rollout intent if set:
const body: any = { /* existing fields */ };
if (strategyName && !applyImmediately) {
  body.rollout = { strategy_name: strategyName };
} else if (applyImmediately) {
  body.rollout = { apply_immediately: true };
}
```

If the deploy-create flow is CLI-only, skip. Note in the commit message: "deploy create UI not present; CLI ds deploy --strategy covers this flow."

- [ ] **Step 3: Build + commit**

```bash
cd web
npx tsc --noEmit && npm run lint && npm run build
git add web/src/pages/DeploymentsPage.tsx
git commit -m "feat(web): strategy picker on deploy create (or note if UI absent)"
```

---

## Task 9: Integration — FlagDetailPage rule edit

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Read FlagDetailPage to locate rule edit**

```bash
grep -n "updateRule\|PUT.*rules\|editRule\|rule.*form\|Percentage" web/src/pages/FlagDetailPage.tsx | head -15
```

Identify the rule-edit UI and the PUT request that updates a rule.

- [ ] **Step 2: Add optional strategy attachment**

Near the rule-edit form, add:

```tsx
import { StrategyPicker } from '@/components/rollout/StrategyPicker';
// ... state
const [ruleStrategyName, setRuleStrategyName] = useState('');
const [ruleImmediate, setRuleImmediate] = useState(true);
```

Inside the JSX where the rule is edited:

```tsx
<StrategyPicker
  orgSlug={orgSlug}
  targetType="config"
  value={ruleStrategyName}
  onChange={setRuleStrategyName}
  allowImmediate
  immediate={ruleImmediate}
  onImmediateChange={setRuleImmediate}
/>
```

In the save handler, if `ruleStrategyName && !ruleImmediate`, include `rollout: { strategy_name: ruleStrategyName }` in the request body. The backend (Plan 3) already accepts this and returns 202 with rollout created.

Handle 202 response: show a message "Rollout started — track in Rollouts page" instead of an immediate "Rule saved" notification.

- [ ] **Step 3: Build + commit**

```bash
cd web
npx tsc --noEmit && npm run lint && npm run build
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): optional strategy attachment on flag rule edit"
```

---

## Task 10: Docs + initiative update + manual smoke

**Files:**
- Modify: `docs/Rollout_Strategies.md`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Append web UI section**

Append to `docs/Rollout_Strategies.md`:

```markdown
## Web UI

The dashboard at `http://localhost:3000` (or deployed URL) exposes the rollout system under each organization:

| Path | Purpose |
|---|---|
| `/orgs/:slug/strategies` | List, create, edit, import/export strategy templates |
| `/orgs/:slug/rollouts` | List rollouts with status filter, auto-refresh every 5s |
| `/orgs/:slug/rollouts/:id` | Phase timeline, runtime actions (6 buttons), event log |
| `/orgs/:slug/rollout-groups` | List and create groups |
| `/orgs/:slug/rollout-groups/:id` | View group members, edit coordination policy |
| Settings → Rollout Policy | Configure mandate/prompt policy and strategy defaults |

### Runtime actions

The six control actions map to the same HTTP endpoints the CLI uses. `Rollback` and `Force Promote` require a reason — the UI shows a modal prompt and rejects empty input.

### Integration points

- **Deployment create form**: gains a strategy picker with an "Apply immediately" opt-out.
- **Flag rule edit form**: gains a strategy picker. When a strategy is attached, the rule edit returns 202 and creates a rollout; the user is redirected to the rollouts page to watch progress.

### Limitations (Plan 5)

- Strategies/policies/defaults are only manageable at the **org** level in the UI. Project- and app-scoped equivalents remain reachable via API + CLI.
- Rollout progress uses **polling** (3s for detail, 5s for list); SSE upgrade deferred.
- No timeline graph of health signals — just textual phase state and status badges.
```

- [ ] **Step 2: Update initiatives**

In `docs/Current_Initiatives.md`:
- Configurable Rollout Strategies row: Phase → `Complete`, add `[Plan 5](./superpowers/plans/2026-04-20-rollout-web-ui.md)`, Notes → `All 5 plans merged: templates, engine+deploy, config rollouts, rollout groups+coordination, web UI. Initiative complete.`
- Consider moving the row to the Archived section if the format permits — otherwise leave in Active with Phase=Complete until next review.
- Bump `> Last updated:`.

- [ ] **Step 3: Manual smoke test**

This is a required verification step — don't skip.

```bash
# Terminal 1: ensure backend is up
make dev-up
make migrate-up
make run-api

# Terminal 2: frontend
cd web
npm run dev
```

Open `http://localhost:3000`, sign in to a seed org.

Checklist:
- [ ] Navigate to `/orgs/<slug>/strategies`. Seeded `system-canary`, `system-blue-green`, `system-rolling` are listed.
- [ ] Click "New Strategy", create a simple one with two steps, save. It appears in the list.
- [ ] Click a strategy name, edit the description, save. Version increments.
- [ ] Export the strategy as YAML. File downloads. Import it back under a new name via the file picker.
- [ ] Navigate to `/orgs/<slug>/rollouts`. Empty or shows whatever rollouts exist.
- [ ] Via CLI, create a deploy with `--strategy system-canary`. Refresh the rollouts page — the new rollout appears within 5s.
- [ ] Click into the rollout. Phase timeline shows current phase highlighted. Pause button works. Resume button works. Rollback opens modal, rejects empty reason, succeeds with reason.
- [ ] Navigate to `/orgs/<slug>/rollout-groups`. Create a group. Attach a rollout to it via CLI. Refresh — member appears in the group's detail page.
- [ ] Open Settings → Rollout Policy tab. Toggle enabled, set to `prompt`, save. Confirm value persists on reload.

If any check fails, note which and either fix or create a follow-up plan.

- [ ] **Step 4: Commit docs**

```bash
git add docs/Rollout_Strategies.md docs/Current_Initiatives.md
git commit -m "docs: Plan 5 web UI section + initiative complete"
```

---

## Definition of Done

- All 10 tasks committed individually on branch `feature/rollout-web-ui`.
- `npm run lint`, `npx tsc --noEmit`, `npm run build` all pass cleanly in `web/`.
- Manual smoke test checklist (Task 10 Step 3) completed with all items green, or failures documented as follow-ups.
- 5 new pages accessible under `/orgs/:slug/{strategies,rollouts,rollouts/:id,rollout-groups,rollout-groups/:id}`.
- Settings page has a Rollout Policy tab at the org level.
- Docs updated; initiative marked complete.

## Not in scope (for potential follow-ups)

- Playwright E2E coverage of new pages.
- SSE push for rollout events (currently polling).
- Project/app-scoped StrategiesPage/PolicyAndDefaultsTab.
- Health signal timeline chart.
- Rollout `amend` UI.
- Dashboard "Active rollouts" summary card (mentioned in spec but not implemented here; easy follow-up).
- Visual step-builder UX (the editor uses a simple table; no drag-reorder or rich step-type fields yet).

## Self-Review Notes

- **Spec coverage**: implements spec Section 5 "Web UI" for the core surface (Strategies, Rollouts + Detail, Rollout Groups + Detail, Policy & Defaults tab). Dashboard summary card mentioned in spec is explicitly deferred.
- **Type consistency**: `Strategy`, `Step`, `Rollout`, `RolloutGroup`, `CoordinationPolicy`, `RolloutStatus`, `PhaseStatus`, `TargetType`, `ScopeType`, `PolicyKind` defined in T1 and used consistently across T2–T9. Nanosecond durations on `Step` match Go's `time.Duration` JSON encoding from Plans 1–2. The rollout column `release_id` is exposed on `Rollout.release_id` (not renamed to `group_id` — consistent with Plan 4).
- **Placeholder scan**: No "TBD"/"TODO"/"fill in". Task 8 Step 2 is genuinely conditional ("if the deploy-create form exists") — that's not a placeholder, it's a pragmatic branch. Task 9 relies on the implementer reading `FlagDetailPage` to locate the rule-edit form — that's an inspection instruction, not a spec gap.
- **Pre-existing UI library assumption**: plan uses plain `<table>`, `<button>`, `<select>` without a component library — matches the existing DeploymentsPage/FlagDetailPage style. If the repo adopts Radix/shadcn/etc. before execution, adapt but keep the same information hierarchy.
