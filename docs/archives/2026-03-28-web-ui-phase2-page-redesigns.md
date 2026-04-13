# Web UI Phase 2: Page Redesigns — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the flag detail page (header + 2 tabs), create deployment and release detail pages with activity logs and context-aware action bars.

**Architecture:** Rewrite FlagDetailPage with inline header details and tabbed Targeting Rules / Environments sections. Create new DeploymentDetailPage and ReleaseDetailPage with vertical activity logs and a shared ActionBar component. Update TypeScript types to match the platform redesign data model.

**Tech Stack:** React 18, React Router 6, TypeScript 5.5, Vite 5.4, custom CSS (dark theme)

**Spec:** `docs/superpowers/specs/2026-03-28-web-ui-phase2-page-redesigns-design.md`

---

## File Structure

### New Files
- `web/src/components/ActionBar.tsx` — shared primary action + "More" dropdown component
- `web/src/pages/DeploymentDetailPage.tsx` — deployment detail with info cards and activity log
- `web/src/pages/ReleaseDetailPage.tsx` — release detail with flag changes table

### Modified Files
- `web/src/types.ts` — update DeployStatus, ReleaseStatus, Deployment, Release; add FlagEnvState, DeploymentEvent, ReleaseFlagChange; add application_id to Flag
- `web/src/mocks/hierarchy.ts` — add mock data for flag env state, deployment events, release flag changes
- `web/src/pages/FlagDetailPage.tsx` — complete rewrite (header details + 2 tabs)
- `web/src/pages/DeploymentsPage.tsx` — update mock data fields (project_id → application_id, traffic_percentage → traffic_percent)
- `web/src/pages/ReleasesPage.tsx` — rewrite mock data for new Release model (name, status, traffic_percent)
- `web/src/App.tsx` — import new detail pages, update deployments/:id and releases/:id routes
- `web/src/styles/globals.css` — styles for detail header, tabs, activity log, action bar, env state table, inline diff

---

## Task 1: Update TypeScript Types

**Files:**
- Modify: `web/src/types.ts`

- [ ] **Step 1: Update DeployStatus and ReleaseStatus**

In `web/src/types.ts`, replace:

```typescript
export type DeployStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'rolled_back' | 'active' | 'rolling-back';
export type ReleaseStatus = 'draft' | 'staging' | 'canary' | 'production' | 'archived';
```

With:

```typescript
export type DeployStatus = 'pending' | 'running' | 'promoting' | 'paused' | 'completed' | 'failed' | 'rolled_back' | 'cancelled';
export type ReleaseStatus = 'draft' | 'rolling_out' | 'paused' | 'completed' | 'rolled_back';
```

- [ ] **Step 2: Update Deployment interface**

Replace the existing `Deployment` interface with:

```typescript
export interface Deployment {
  id: string;
  application_id: string;
  environment_id: string;
  version: string;
  commit_sha?: string;
  artifact?: string;
  strategy: DeployStrategy;
  status: DeployStatus;
  traffic_percent: number;
  health_score: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at: string | null;
}
```

- [ ] **Step 3: Update Release interface**

Replace the existing `Release` interface with:

```typescript
export interface Release {
  id: string;
  application_id: string;
  name: string;
  description?: string;
  session_sticky: boolean;
  sticky_header?: string;
  traffic_percent: number;
  status: ReleaseStatus;
  created_by: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 4: Add application_id to Flag interface**

In the `Flag` interface, add after `project_id`:

```typescript
  application_id?: string;
```

- [ ] **Step 5: Add new types at end of file**

Append before the closing of the types file (after `Application` interface):

```typescript
export interface FlagEnvState {
  flag_id: string;
  environment_id: string;
  environment_name: string;
  enabled: boolean;
  value: string;
  updated_by: string;
  updated_at: string;
}

export interface DeploymentEvent {
  status: DeployStatus;
  timestamp: string;
  note: string;
}

export interface ReleaseFlagChange {
  id: string;
  release_id: string;
  flag_key: string;
  environment_name: string;
  previous_enabled: boolean;
  new_enabled: boolean;
  previous_value: string;
  new_value: string;
  applied_at: string | null;
}
```

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Errors in DeploymentsPage.tsx and ReleasesPage.tsx (mock data uses old field names). That's expected — fixed in Task 3.

- [ ] **Step 7: Commit**

```bash
git add web/src/types.ts
git commit -m "feat(web): update Deployment/Release/Flag types for platform redesign data model"
```

---

## Task 2: Add Mock Data for Detail Pages

**Files:**
- Modify: `web/src/mocks/hierarchy.ts`

- [ ] **Step 1: Add mock data imports and exports**

Add to the imports at top of `web/src/mocks/hierarchy.ts`:

```typescript
import type { Organization, Application, Project, FlagEnvState, DeploymentEvent, Deployment, Release, ReleaseFlagChange } from '@/types';
```

- [ ] **Step 2: Add MOCK_FLAG_ENV_STATE**

Append to `web/src/mocks/hierarchy.ts`:

```typescript
export const MOCK_FLAG_ENV_STATE: FlagEnvState[] = [
  {
    flag_id: 'flag-001',
    environment_id: 'env-dev',
    environment_name: 'Development',
    enabled: true,
    value: 'true',
    updated_by: 'alice',
    updated_at: '2026-03-18T14:30:00Z',
  },
  {
    flag_id: 'flag-001',
    environment_id: 'env-staging',
    environment_name: 'Staging',
    enabled: true,
    value: 'true',
    updated_by: 'bob',
    updated_at: '2026-03-19T10:00:00Z',
  },
  {
    flag_id: 'flag-001',
    environment_id: 'env-prod',
    environment_name: 'Production',
    enabled: false,
    value: 'false',
    updated_by: 'alice',
    updated_at: '2026-03-15T09:00:00Z',
  },
];
```

- [ ] **Step 3: Add MOCK_DEPLOYMENT_DETAIL and MOCK_DEPLOYMENT_EVENTS**

```typescript
export const MOCK_DEPLOYMENT_DETAIL: Deployment = {
  id: 'dep-1',
  application_id: 'app-1',
  environment_id: 'env-prod',
  version: 'v2.4.1',
  commit_sha: 'abc123f',
  artifact: 'https://registry.acme.com/api-server:v2.4.1',
  strategy: 'canary',
  status: 'running',
  traffic_percent: 25,
  health_score: 99.8,
  created_by: 'alice@example.com',
  created_at: '2026-03-21T09:15:00Z',
  updated_at: '2026-03-21T10:30:00Z',
  started_at: '2026-03-21T09:20:00Z',
  completed_at: null,
};

export const MOCK_DEPLOYMENT_EVENTS: DeploymentEvent[] = [
  { status: 'running', timestamp: '2026-03-21T10:30:00Z', note: 'Traffic increased to 25%' },
  { status: 'running', timestamp: '2026-03-21T09:45:00Z', note: 'Traffic increased to 10%' },
  { status: 'running', timestamp: '2026-03-21T09:20:00Z', note: 'Canary deployment started' },
  { status: 'pending', timestamp: '2026-03-21T09:15:00Z', note: 'Deployment created' },
];
```

- [ ] **Step 4: Add MOCK_RELEASE_DETAIL and MOCK_RELEASE_FLAG_CHANGES**

```typescript
export const MOCK_RELEASE_DETAIL: Release = {
  id: 'rel-1',
  application_id: 'app-1',
  name: 'Enable Checkout V2',
  description: 'Gradual rollout of checkout v2 flags across all environments',
  session_sticky: true,
  sticky_header: 'X-Session-ID',
  traffic_percent: 25,
  status: 'rolling_out',
  created_by: 'alice@example.com',
  started_at: '2026-03-21T10:00:00Z',
  created_at: '2026-03-20T14:00:00Z',
  updated_at: '2026-03-21T10:30:00Z',
};

export const MOCK_RELEASE_FLAG_CHANGES: ReleaseFlagChange[] = [
  {
    id: 'rfc-1',
    release_id: 'rel-1',
    flag_key: 'checkout-v2-rollout',
    environment_name: 'Production',
    previous_enabled: false,
    new_enabled: true,
    previous_value: 'false',
    new_value: 'true',
    applied_at: '2026-03-21T10:00:00Z',
  },
  {
    id: 'rfc-2',
    release_id: 'rel-1',
    flag_key: 'checkout-v2-rollout',
    environment_name: 'Staging',
    previous_enabled: true,
    new_enabled: true,
    previous_value: 'false',
    new_value: 'true',
    applied_at: '2026-03-21T10:00:00Z',
  },
  {
    id: 'rfc-3',
    release_id: 'rel-1',
    flag_key: 'checkout-theme',
    environment_name: 'Production',
    previous_enabled: true,
    new_enabled: true,
    previous_value: '"v1"',
    new_value: '"v2"',
    applied_at: null,
  },
  {
    id: 'rfc-4',
    release_id: 'rel-1',
    flag_key: 'legacy-checkout-disable',
    environment_name: 'Production',
    previous_enabled: true,
    new_enabled: false,
    previous_value: 'true',
    new_value: 'true',
    applied_at: null,
  },
];
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Still errors in DeploymentsPage/ReleasesPage (old mock data). That's fine.

- [ ] **Step 6: Commit**

```bash
git add web/src/mocks/hierarchy.ts
git commit -m "feat(web): add mock data for flag env state, deployment events, and release flag changes"
```

---

## Task 3: Update DeploymentsPage and ReleasesPage Mock Data

**Files:**
- Modify: `web/src/pages/DeploymentsPage.tsx`
- Modify: `web/src/pages/ReleasesPage.tsx`

- [ ] **Step 1: Update DeploymentsPage mock data**

In `web/src/pages/DeploymentsPage.tsx`, update ALL mock deployment objects:
- Rename `project_id` → `application_id` in every object
- Rename `traffic_percentage` → `traffic_percent` in every object
- Add `commit_sha` and `artifact` fields (optional, can add to first couple)

Also update all references in the component JSX from `traffic_percentage` to `traffic_percent` (search for `traffic_percentage` and replace all).

- [ ] **Step 2: Update ReleasesPage mock data and component**

In `web/src/pages/ReleasesPage.tsx`, this requires a bigger rewrite since the Release model changed completely. Replace the `MOCK_RELEASES` array with data matching the new model:

```typescript
const MOCK_RELEASES: Release[] = [
  {
    id: 'rel-1',
    application_id: 'app-1',
    name: 'Enable Checkout V2',
    description: 'Gradual rollout of checkout v2 flags',
    session_sticky: true,
    sticky_header: 'X-Session-ID',
    traffic_percent: 25,
    status: 'rolling_out',
    created_by: 'alice@example.com',
    started_at: '2026-03-21T10:00:00Z',
    created_at: '2026-03-20T14:00:00Z',
    updated_at: '2026-03-21T10:30:00Z',
  },
  {
    id: 'rel-2',
    application_id: 'app-1',
    name: 'Dark Mode Feature Flags',
    description: 'Enable dark mode across all environments',
    session_sticky: false,
    traffic_percent: 100,
    status: 'completed',
    created_by: 'bob@example.com',
    started_at: '2026-03-18T08:00:00Z',
    completed_at: '2026-03-19T11:30:00Z',
    created_at: '2026-03-17T10:00:00Z',
    updated_at: '2026-03-19T11:30:00Z',
  },
  {
    id: 'rel-3',
    application_id: 'app-1',
    name: 'Search Experiment Rollout',
    description: 'ML search ranking A/B test activation',
    session_sticky: true,
    sticky_header: 'X-User-ID',
    traffic_percent: 0,
    status: 'draft',
    created_by: 'team-platform',
    created_at: '2026-03-21T07:30:00Z',
    updated_at: '2026-03-21T07:30:00Z',
  },
  {
    id: 'rel-4',
    application_id: 'app-1',
    name: 'Payment Gateway Migration',
    description: 'Switch payment flags to new gateway',
    session_sticky: false,
    traffic_percent: 50,
    status: 'paused',
    created_by: 'alice@example.com',
    started_at: '2026-03-20T16:00:00Z',
    created_at: '2026-03-20T12:00:00Z',
    updated_at: '2026-03-21T06:00:00Z',
  },
  {
    id: 'rel-5',
    application_id: 'app-1',
    name: 'Legacy API Sunset',
    description: 'Disable legacy API endpoint flags',
    session_sticky: false,
    traffic_percent: 100,
    status: 'rolled_back',
    created_by: 'ci/deploy-bot',
    started_at: '2026-03-19T10:00:00Z',
    created_at: '2026-03-19T08:00:00Z',
    updated_at: '2026-03-19T14:00:00Z',
  },
];
```

Update the tab filters to match new statuses. Replace the old tab array with:

```typescript
const TABS = ['all', 'draft', 'rolling_out', 'paused', 'completed', 'rolled_back'] as const;
```

Update the table columns to show `name` instead of `version`, and `traffic_percent` instead of `commit_sha`. Remove `promoted_at` column.

Update status badge classes to map the new status values.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/DeploymentsPage.tsx web/src/pages/ReleasesPage.tsx
git commit -m "feat(web): update Deployments and Releases mock data for platform redesign model"
```

---

## Task 4: ActionBar Component

**Files:**
- Create: `web/src/components/ActionBar.tsx`

- [ ] **Step 1: Create ActionBar component**

Create `web/src/components/ActionBar.tsx`:

```tsx
import { useState, useRef, useEffect } from 'react';

interface ActionBarAction {
  label: string;
  onClick: () => void;
  variant?: 'default' | 'primary' | 'danger';
}

interface ActionBarProps {
  primaryAction?: ActionBarAction;
  secondaryActions?: ActionBarAction[];
}

export default function ActionBar({ primaryAction, secondaryActions }: ActionBarProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const hasSecondary = secondaryActions && secondaryActions.length > 0;

  if (!primaryAction && !hasSecondary) return null;

  return (
    <div className="action-bar" ref={ref}>
      {primaryAction && (
        <button
          className={`btn btn-${primaryAction.variant || 'primary'}`}
          onClick={primaryAction.onClick}
        >
          {primaryAction.label}
        </button>
      )}
      {hasSecondary && (
        <div className="action-bar-more">
          <button
            className="btn btn-secondary action-bar-more-btn"
            onClick={() => setOpen(!open)}
          >
            More ▾
          </button>
          {open && (
            <div className="action-bar-dropdown">
              {secondaryActions!.map((action) => (
                <button
                  key={action.label}
                  className={`action-bar-option${action.variant === 'danger' ? ' action-bar-option-danger' : ''}`}
                  onClick={() => { action.onClick(); setOpen(false); }}
                >
                  {action.label}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export type { ActionBarProps, ActionBarAction };
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/ActionBar.tsx
git commit -m "feat(web): add shared ActionBar component with primary action and dropdown"
```

---

## Task 5: CSS for Phase 2 Components

**Files:**
- Modify: `web/src/styles/globals.css`

- [ ] **Step 1: Append Phase 2 CSS to globals.css**

Append to the end of `web/src/styles/globals.css`:

```css
/* ------------------------------------------------------------------ */
/* Detail page header                                                  */
/* ------------------------------------------------------------------ */
.detail-header {
  padding-bottom: 20px;
  border-bottom: 1px solid var(--color-border);
  margin-bottom: 0;
}

.detail-header-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.detail-header-title {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 2px;
}

.detail-header-subtitle {
  font-size: 13px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.detail-header-badges {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 8px;
}

.detail-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--color-text-muted);
}

.detail-chips span > span {
  color: var(--color-text);
}

.detail-secondary {
  margin-top: 8px;
  font-size: 11px;
  color: var(--color-text-muted);
}

.detail-description {
  margin-top: 8px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* ------------------------------------------------------------------ */
/* Detail page tabs                                                    */
/* ------------------------------------------------------------------ */
.detail-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--color-border);
  margin-bottom: 24px;
}

.detail-tab {
  padding: 12px 16px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.detail-tab:hover {
  color: var(--color-text);
}

.detail-tab.active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
}

/* ------------------------------------------------------------------ */
/* Action bar                                                          */
/* ------------------------------------------------------------------ */
.action-bar {
  display: flex;
  gap: 8px;
  align-items: center;
}

.action-bar-more {
  position: relative;
}

.action-bar-more-btn {
  padding: 6px 12px;
  font-size: 13px;
}

.action-bar-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  min-width: 160px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 4px;
  z-index: 200;
  box-shadow: var(--shadow-md);
}

.action-bar-option {
  display: block;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: none;
  color: var(--color-text-secondary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  border-radius: var(--radius-sm);
}

.action-bar-option:hover {
  background: var(--color-bg-hover);
  color: var(--color-text);
}

.action-bar-option-danger {
  color: var(--color-danger);
}

.action-bar-option-danger:hover {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

/* ------------------------------------------------------------------ */
/* Info cards row                                                      */
/* ------------------------------------------------------------------ */
.info-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin: 24px 0;
}

.info-card {
  padding: 16px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

.info-card-label {
  font-size: 11px;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 4px;
}

.info-card-value {
  font-size: 20px;
  font-weight: 600;
}

.info-card-bar {
  height: 4px;
  background: var(--color-border);
  border-radius: 2px;
  margin-top: 8px;
  overflow: hidden;
}

.info-card-bar-fill {
  height: 100%;
  background: var(--color-primary);
  border-radius: 2px;
  transition: width 0.3s;
}

/* ------------------------------------------------------------------ */
/* Activity log                                                        */
/* ------------------------------------------------------------------ */
.activity-log {
  margin-top: 24px;
}

.activity-log-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 16px;
}

.activity-log-entry {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--color-border);
}

.activity-log-entry:last-child {
  border-bottom: none;
}

.activity-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
}

.activity-dot-green { background: var(--color-success); }
.activity-dot-yellow { background: var(--color-warning); }
.activity-dot-red { background: var(--color-danger); }
.activity-dot-gray { background: var(--color-text-muted); }

.activity-status {
  font-size: 13px;
  font-weight: 500;
  min-width: 100px;
}

.activity-time {
  font-size: 12px;
  color: var(--color-text-muted);
  min-width: 160px;
}

.activity-note {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* ------------------------------------------------------------------ */
/* Environment state table                                             */
/* ------------------------------------------------------------------ */
.env-toggle {
  appearance: none;
  width: 36px;
  height: 20px;
  background: var(--color-border);
  border-radius: 10px;
  position: relative;
  cursor: pointer;
  border: none;
  transition: background 0.2s;
}

.env-toggle:checked {
  background: var(--color-success);
}

.env-toggle::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  background: white;
  border-radius: 50%;
  transition: transform 0.2s;
}

.env-toggle:checked::after {
  transform: translateX(16px);
}

.env-value-cell {
  cursor: pointer;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  font-family: var(--font-mono);
  font-size: 13px;
}

.env-value-cell:hover {
  background: var(--color-bg-hover);
}

.env-value-input {
  font-family: var(--font-mono);
  font-size: 13px;
  padding: 4px 8px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-primary);
  border-radius: var(--radius-sm);
  color: var(--color-text);
  width: 200px;
}

/* ------------------------------------------------------------------ */
/* Inline diff                                                         */
/* ------------------------------------------------------------------ */
.diff-arrow {
  color: var(--color-text-muted);
  margin: 0 6px;
}

.diff-old {
  color: var(--color-danger);
  text-decoration: line-through;
  font-family: var(--font-mono);
  font-size: 13px;
}

.diff-new {
  color: var(--color-success);
  font-family: var(--font-mono);
  font-size: 13px;
}

.diff-enabled {
  color: var(--color-success);
}

.diff-disabled {
  color: var(--color-danger);
}

/* ------------------------------------------------------------------ */
/* Session sticky badge                                                */
/* ------------------------------------------------------------------ */
.sticky-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--color-warning);
  background: var(--color-warning-bg);
  padding: 3px 10px;
  border-radius: 4px;
}

/* ------------------------------------------------------------------ */
/* Danger zone                                                         */
/* ------------------------------------------------------------------ */
.danger-zone {
  margin-top: 32px;
  padding: 20px;
  border: 1px solid var(--color-danger-bg);
  border-radius: var(--radius-lg);
  background: rgba(239, 68, 68, 0.04);
}

.danger-zone h3 {
  color: var(--color-danger);
  font-size: 14px;
  margin-bottom: 8px;
}

.danger-zone p {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-bottom: 12px;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/globals.css
git commit -m "feat(web): add CSS for detail pages, action bar, activity log, env state, and inline diff"
```

---

## Task 6: Flag Detail Page Rewrite

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx` — complete rewrite

- [ ] **Step 1: Rewrite FlagDetailPage**

Replace the entire contents of `web/src/pages/FlagDetailPage.tsx`. The new page has:
- Header section with flag name, key, enabled toggle, category badge, detail chips, secondary details, description
- Two tabs: Targeting Rules and Environments
- Targeting Rules tab: table with priority, type, condition, value, enabled dot
- Environments tab: table with environment name, enabled toggle, editable value, updated info
- Danger zone below tabs

Key implementation details:
- Use `useState` for `activeTab` ('rules' | 'environments')
- Use `useState` for `envState` (copy of MOCK_FLAG_ENV_STATE, for toggle/edit functionality)
- Use `useState<string | null>` for `editingEnvId` (which environment value is being edited)
- Import `MOCK_FLAG_ENV_STATE` from `@/mocks/hierarchy`
- Import `FlagEnvState` from `@/types`
- Change MOCK_FLAG to a release-category flag (checkout-v2-rollout) per spec
- Keep existing `MOCK_RULES`, `formatDate`, `formatDateTime`, `describeConditions` helpers
- Back link uses context-aware `backPath` (already exists in current code)

The component should be structured as:
1. Header with inline details
2. Tab bar
3. Tab content (conditional render)
4. Danger zone

**Header must include ALL of these elements (per spec section 2.3):**
- Back link (`← Back to Flags`)
- Flag name (large text) + flag key (monospace, muted)
- Enabled toggle + category badge
- **Edit button** (stub — `<button className="btn btn-secondary">Edit</button>`, no-op for now)
- **Detail chips row**: Type, Owners, Expires (or "Permanent"), Default Value, **Scope** ("Project-wide" if no `application_id`, else app name), **Purpose** (if set), **Tags** (if set)
- **Secondary details row**: Created by, Created at, Updated at
- **Description** (full line below if present)

For the Environments tab value editing:
- Click value cell → set `editingEnvId` to that env's ID
- Show input with current value
- Enter or blur → save to local state, clear `editingEnvId`
- Escape → cancel, clear `editingEnvId`

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): rewrite FlagDetailPage with header details and tabbed Rules/Environments"
```

---

## Task 7: Deployment Detail Page

**Files:**
- Create: `web/src/pages/DeploymentDetailPage.tsx`

- [ ] **Step 1: Create DeploymentDetailPage**

Create `web/src/pages/DeploymentDetailPage.tsx`:

The page should:
- Import `useParams, Link` from react-router-dom
- Import `ActionBar` from `@/components/ActionBar`
- Import `MOCK_DEPLOYMENT_DETAIL, MOCK_DEPLOYMENT_EVENTS` from `@/mocks/hierarchy`
- Import `DeploymentEvent` from `@/types`
- Extract `orgSlug, projectSlug, appSlug` from useParams
- Build context-aware `backPath` to deployments list

Header section:
- Back link
- Version (large)
- Commit SHA (monospace, 7 chars), artifact link (if set), strategy badge, status badge, traffic %
- ActionBar in top-right based on current status

Info cards: Traffic (with progress bar), Health (color-coded), Duration (computed), Created by

Activity log: map MOCK_DEPLOYMENT_EVENTS, most recent first. Color dots:
- green: running, completed
- yellow: paused, promoting, pending
- red: failed, rolled_back, cancelled

Helper functions needed:
- `statusDotColor(status)` → CSS class
- `statusBadgeClass(status)` → badge CSS class
- `strategyBadgeClass(strategy)` → badge CSS class
- `formatDateTime(iso)` → formatted string
- `computeDuration(start, end?)` → "Xh Ym" string

Action bar logic:
- `pending` → primary: Start; more: Cancel
- `running` → primary: Promote; more: Pause, Rollback
- `promoting` → more: Rollback
- `paused` → primary: Resume; more: Rollback, Cancel
- `failed` → primary: Rollback
- others → no actions

All action handlers should be stubs (console.log or no-op).

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/DeploymentDetailPage.tsx
git commit -m "feat(web): add DeploymentDetailPage with info cards and activity log"
```

---

## Task 8: Release Detail Page

**Files:**
- Create: `web/src/pages/ReleaseDetailPage.tsx`

- [ ] **Step 1: Create ReleaseDetailPage**

Create `web/src/pages/ReleaseDetailPage.tsx`:

The page should:
- Import `useParams, Link` from react-router-dom
- Import `ActionBar` from `@/components/ActionBar`
- Import `MOCK_RELEASE_DETAIL, MOCK_RELEASE_FLAG_CHANGES` from `@/mocks/hierarchy`
- Extract route params for context-aware back link

Header section:
- Back link
- Release name (large)
- Description (muted)
- Status badge, traffic %, session sticky badge (if enabled)
- ActionBar based on status

Action bar logic:
- `draft` → primary: Start Rollout; more: Delete (danger variant)
- `rolling_out` → primary: Promote; more: Pause, Rollback
- `paused` → primary: Resume; more: Rollback
- others → no actions

Flag changes table:
- Columns: Flag Key, Environment, Previous, New, Applied At
- Previous/New columns show inline diff:
  - If enabled changed: `disabled → enabled` (colored)
  - If value changed: `"old" → "new"` (monospace, colored)
  - If both changed: show both diffs
- Applied At: formatted timestamp or "—" if null

Helper functions:
- `formatDateTime(iso)` for timestamps
- `renderDiff(change)` for inline diff rendering

All action handlers stubs.

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ReleaseDetailPage.tsx
git commit -m "feat(web): add ReleaseDetailPage with flag changes table and action bar"
```

---

## Task 9: Update App.tsx Routes

**Files:**
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Import new detail pages and update routes**

In `web/src/App.tsx`:

Add imports:
```tsx
import DeploymentDetailPage from './pages/DeploymentDetailPage';
import ReleaseDetailPage from './pages/ReleaseDetailPage';
```

Replace:
```tsx
<Route path="deployments/:id" element={<DeploymentsPage />} /> {/* Detail page in Phase 2 */}
```
With:
```tsx
<Route path="deployments/:id" element={<DeploymentDetailPage />} />
```

Replace:
```tsx
<Route path="releases/:id" element={<ReleasesPage />} /> {/* Detail page in Phase 2 */}
```
With:
```tsx
<Route path="releases/:id" element={<ReleaseDetailPage />} />
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): wire up DeploymentDetailPage and ReleaseDetailPage routes"
```

---

## Task 10: Full Build and Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Full TypeScript check**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 2: Vite build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx vite build`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Verify key routes load**

Start dev server: `cd /Users/sgamel/git/DeploySentry/web && npx vite --port 3001 &`

Verify:
- `http://localhost:3001/orgs/acme-corp/projects/platform/flags/flag-001` → redesigned flag detail with tabs
- `http://localhost:3001/orgs/acme-corp/projects/platform/apps/api-server/deployments/dep-1` → deployment detail with activity log
- `http://localhost:3001/orgs/acme-corp/projects/platform/apps/api-server/releases/rel-1` → release detail with flag changes

- [ ] **Step 4: Stop dev server**

```bash
kill %1 2>/dev/null || true
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Add Phase 2 to current initiatives**

Add row:
```markdown
| Web UI Phase 2 — Page Redesigns | Implementation | [Link](./superpowers/plans/2026-03-28-web-ui-phase2-page-redesigns.md) |
```

- [ ] **Step 2: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add Web UI Phase 2 page redesigns to current initiatives"
```
