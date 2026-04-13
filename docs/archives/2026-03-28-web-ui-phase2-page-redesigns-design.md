# Web UI Phase 2: Page Redesigns

## Overview

Redesign the flag detail page with a split layout (inline header details + 2 tabs), create new deployment and release detail pages with vertical activity logs and context-aware action bars. This is Phase 2 of 3 for the full Web UI redesign.

**Parent spec:** `docs/superpowers/specs/2026-03-28-platform-redesign-design.md` (sections 4.4–4.6)
**Depends on:** Phase 1 Navigation Overhaul (complete)

## Goals

1. Redesign the flag detail page: key details always visible in header, two working tabs (Targeting Rules, Environments) below.
2. Create a deployment detail page with version/commit/artifact info, vertical activity log, and context-aware action bar.
3. Create a release detail page with flag changes displayed as inline diffs and rollout action bar.
4. Introduce a shared `ActionBar` component for state-machine-driven primary/secondary actions.

## Non-Goals

- Real API integration — pages continue using mock data.
- Production confirmation dialogs — deferred to API integration phase. (See Deferred section.)
- Change history/rollback from historical view — deferred. (See Deferred section.)
- Mobile responsiveness.
- Deployment or release creation forms — list pages already have stub "Create" buttons.
- Loading/skeleton states — mock data is synchronous, no loading states needed.

---

## 1. Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Flag detail layout | Header details + 2 tabs | Key info always visible while working with rules/environments. Reduces tabs from 3 to 2. More practical than pure 3-tab spec design. |
| Environment toggles | Simple on/off toggle | Mock data for now. Production confirmation dialogs deferred to API integration phase — noted as a hard requirement for that work. |
| Deployment timeline | Vertical activity log | Deployments don't follow a linear path (pause, rollback). Activity log shows exact timestamps for debugging. More useful than horizontal step indicator. |
| Release flag changes | Inline diff rows | Compact `old → new` notation in table rows. Scales to many flag changes. Universally understood. |
| Action buttons | Primary action + dropdown | Surfaces the most logical next action. Reduces cognitive load. Destructive actions behind a click. Matches GitHub/Vercel patterns. |

---

## 1a. TypeScript Type Updates

Phase 1 deferred type updates to Phase 2. The following types in `web/src/types.ts` must be updated to match the platform redesign data model:

**`DeployStatus`** — update from `'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'rolled_back' | 'active' | 'rolling-back'` to:
```typescript
export type DeployStatus = 'pending' | 'running' | 'promoting' | 'paused' | 'completed' | 'failed' | 'rolled_back' | 'cancelled';
```

**`ReleaseStatus`** — update from `'draft' | 'staging' | 'canary' | 'production' | 'archived'` to:
```typescript
export type ReleaseStatus = 'draft' | 'rolling_out' | 'paused' | 'completed' | 'rolled_back';
```

**`Deployment`** — add missing fields, rename `traffic_percentage` to `traffic_percent` for consistency with backend model:
```typescript
export interface Deployment {
  id: string;
  application_id: string;   // was project_id
  environment_id: string;
  version: string;
  commit_sha?: string;       // NEW
  artifact?: string;         // NEW
  strategy: DeployStrategy;
  status: DeployStatus;
  traffic_percent: number;   // was traffic_percentage
  health_score: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;       // NEW
  completed_at: string | null;
}
```

**`Release`** — complete rewrite for the new flag-change-bundle model:
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

**New types:**
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

**`Flag`** — add optional `application_id` field (parent spec section 2.7):
```typescript
// Add to existing Flag interface:
application_id?: string;  // Set for release flags (app-scoped), null for project-wide flags
```

**Note:** Mock data in existing pages (`DeploymentsPage`, `ReleasesPage`) must also be updated to match the new field names (e.g., `project_id` → `application_id`, `traffic_percentage` → `traffic_percent`). The old `Release` mock data fields (`version`, `commit_sha`, `promoted_at`) are removed since releases are now flag-change bundles, not code-shipping events.

---

## 2. Flag Detail Page Redesign

### 2.1 Current State

Single scrolling page with: Details card (table), Targeting Rules table, Danger Zone (archive button). 253 lines, single mock flag + 3 mock rules.

### 2.2 New Layout

```
┌──────────────────────────────────────────────────────────┐
│  ← Back to Flags                                         │
│                                                          │
│  Checkout V2 Rollout              [Enabled] [release]    │
│  checkout-v2-rollout                          [Edit]     │
│                                                          │
│  Type: boolean · Owners: payments-team · Expires: Jun 1  │
│  Default: false · Description: Gradual rollout of new... │
├──────────────────────────────────────────────────────────┤
│  [Targeting Rules]  [Environments]                       │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Tab content here                                        │
│                                                          │
├──────────────────────────────────────────────────────────┤
│  ⚠ Danger Zone                                           │
│  [Archive Flag]                                          │
└──────────────────────────────────────────────────────────┘
```

### 2.3 Header Section (always visible)

- **Back link**: `← Back to Flags` linking to the flag list (context-aware: project or app level)
- **Flag name**: large text
- **Flag key**: monospace, muted
- **Enabled toggle**: toggle switch showing current state
- **Category badge**: colored badge (release, feature, experiment, ops, permission)
- **Edit button**: navigates to edit form (stub for now)
- **Detail chips** (row below name): Type, Owners, Expires (or "Permanent"), Default Value, Scope ("Project-wide" or app name if `application_id` is set), Purpose (if set), Tags (if set)
- **Secondary details** (smaller row below chips): Created by, Created at, Updated at. Description shown as a full line below if present.

**Note:** The parent spec's Details tab included all of these fields. By inlining them in the header, we preserve the same information while eliminating a tab. This is a deliberate simplification from the parent spec's 3-tab design.

### 2.4 Tab: Targeting Rules

- **Header row**: rule count + "Add Rule" button
- **Table columns**: Priority, Type, Condition (human-readable description), Value, Enabled (dot indicator)
- **Row actions**: Edit, Delete (icon buttons on hover)
- **Empty state**: "No targeting rules. Add one to control flag evaluation."

Uses existing `MOCK_RULES` data.

### 2.5 Tab: Environments

- **Table columns**: Environment, Enabled (toggle), Value (editable text/JSON), Last Updated, Updated By
- **Toggle behavior**: clicking the toggle immediately changes local state (optimistic). No confirmation for now — this is mock data.
- **Value editing**: click the value cell to enter edit mode (shows a text input). Press Enter or blur to save (updates local mock state). Press Escape to cancel. For boolean flags, show a simple on/off toggle instead of text input. No JSON validation needed with mock data.
- **Empty state**: "No environments configured."

**New mock data** (`MOCK_FLAG_ENV_STATE`):
```typescript
interface FlagEnvState {
  flag_id: string;
  environment_id: string;
  environment_name: string;
  enabled: boolean;
  value: string;
  updated_by: string;
  updated_at: string;
}
```

Example data for the mock flag across 3 environments (development, staging, production).

**Note:** The existing `MOCK_FLAG` in `FlagDetailPage.tsx` should be changed to a release-category flag (e.g., `checkout-v2-rollout`) so the wireframe examples match the rendered output. This makes it easier to visually verify the redesign.

### 2.6 Danger Zone

Below the tabs (not inside a tab). Shows archive button with warning styling. Same as current implementation.

---

## 3. Deployment Detail Page (New)

### 3.1 Route

`/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments/:id`

Currently this route renders `DeploymentsPage` (the list page) as a placeholder. It will now render `DeploymentDetailPage`.

### 3.2 Layout

```
┌──────────────────────────────────────────────────────────┐
│  ← Back to Deployments                                   │
│                                                          │
│  v2.4.1                           [Promote ▾]            │
│  abc123f · canary · 25% traffic                          │
│  [running]                                               │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │
│  │Traffic  │ │Health   │ │Duration │ │Created  │       │
│  │  25%    │ │  99.8%  │ │  1h 15m │ │  alice  │       │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘       │
│                                                          │
│  Activity                                                │
│  ─────────                                               │
│  ● running      Mar 21, 10:30 AM   Traffic at 25%       │
│  ● pending      Mar 21, 09:15 AM   Deployment created   │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### 3.3 Header

- **Back link**: `← Back to Deployments` (context-aware)
- **Version**: large text (e.g., `v2.4.1`)
- **Commit SHA**: monospace, first 7 chars
- **Artifact**: link to artifact URL (if set), truncated to hostname
- **Strategy badge**: canary / blue-green / rolling
- **Status badge**: colored by status
- **Traffic %**: shown inline with version info

### 3.4 Action Bar

Top-right, next to the version. Shows a primary action button and a "More" dropdown. Actions change based on current deployment status:

| Status | Primary Action | More Actions |
|--------|---------------|-------------|
| `pending` | Start | Cancel |
| `running` | Promote | Pause, Rollback |
| `promoting` | — | Rollback |
| `paused` | Resume | Rollback, Cancel |
| `failed` | Rollback | — |
| `completed` | — | — |
| `rolled_back` | — | — |
| `cancelled` | — | — |

When no primary action exists, show status as a static label instead.

### 3.5 Info Cards

Four cards in a row:
- **Traffic**: current traffic percentage with a small progress bar
- **Health**: health score with color coding (green ≥99, yellow ≥95, red <95)
- **Duration**: time since deployment started (or total duration if completed)
- **Created by**: deployer name/email

### 3.6 Activity Log

Vertical timeline, most recent event at top.

Each entry:
- **Status dot**: colored by status (green for running/completed, yellow for paused/promoting, red for failed/rolled_back)
- **Status label**: the status name
- **Timestamp**: formatted datetime
- **Note**: optional description (e.g., "Traffic increased to 25%", "Deployment created")

**Mock data** (`MOCK_DEPLOYMENT_EVENTS`):
```typescript
interface DeploymentEvent {
  status: string;
  timestamp: string;
  note: string;
}
```

### 3.7 Mock Data

Single `MOCK_DEPLOYMENT` object matching the updated `Deployment` type, plus `MOCK_DEPLOYMENT_EVENTS` array (4-5 events showing a realistic deployment progression).

**ID lookup:** Detail pages always render the single mock object regardless of the `:id` URL param. No lookup function needed — the param is extracted but only used for navigation context. When real API integration happens, it will be used for the fetch call.

---

## 4. Release Detail Page (New)

### 4.1 Route

`/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases/:id`

Currently renders `ReleasesPage` (list) as a placeholder.

### 4.2 Layout

```
┌──────────────────────────────────────────────────────────┐
│  ← Back to Releases                                      │
│                                                          │
│  Enable Checkout V2               [Promote ▾]            │
│  Gradual rollout of checkout v2 flags                    │
│  [rolling_out] · 25% traffic                             │
│  🔒 Session sticky: X-Session-ID                         │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Flag Changes                                            │
│  ─────────────                                           │
│  Flag Key          Env         Previous      New         │
│  checkout-v2       production  disabled      enabled     │
│  checkout-v2       staging     false         true        │
│  checkout-theme    production  "v1"          "v2"        │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### 4.3 Header

- **Back link**: `← Back to Releases` (context-aware)
- **Release name**: large text
- **Description**: below the name, muted
- **Status badge**: colored by status (draft, rolling_out, paused, completed, rolled_back)
- **Traffic %**: shown inline
- **Session sticky config**: if `session_sticky` is true, show the sticky header name with a lock icon

### 4.4 Action Bar

Same `ActionBar` component as deployments. Actions based on release status:

| Status | Primary Action | More Actions |
|--------|---------------|-------------|
| `draft` | Start Rollout | Delete (danger variant) |
| `rolling_out` | Promote | Pause, Rollback |
| `paused` | Resume | Rollback |
| `completed` | — | — |
| `rolled_back` | — | — |

### 4.5 Flag Changes Table

**Columns**: Flag Key, Environment, Previous (enabled state + value), New (enabled state + value), Applied At

**Inline diff format**:
- Enabled state: `disabled → enabled` or `enabled → disabled` (colored green/red)
- Value: `"old" → "new"` in monospace
- If only enabled changed: show just the enabled diff
- If only value changed: show just the value diff
- If both changed: show both on the same row

**Applied At**: timestamp when the change was applied (null if release hasn't started).

**Empty state**: "No flag changes added to this release."

### 4.6 Mock Data

```typescript
interface MockReleaseFlagChange {
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

Single `MOCK_RELEASE` object (matching the new release model from the platform redesign: name, description, session_sticky, sticky_header, traffic_percent, status) plus `MOCK_RELEASE_FLAG_CHANGES` array (3-4 changes).

---

## 5. Shared ActionBar Component

### 5.1 Interface

```typescript
interface ActionBarProps {
  primaryAction?: {
    label: string;
    onClick: () => void;
    variant?: 'primary' | 'danger';
  };
  secondaryActions?: {
    label: string;
    onClick: () => void;
    variant?: 'default' | 'danger';
  }[];
}
```

### 5.2 Behavior

- If `primaryAction` is set, render it as a prominent button (styled by variant).
- If `secondaryActions` has items, render a "More" dropdown button next to the primary.
- If no `primaryAction` and no `secondaryActions`, render nothing.
- Dropdown closes on outside click or Escape key (same pattern as OrgSwitcher/ProjectSwitcher).

### 5.3 Usage

Both `DeploymentDetailPage` and `ReleaseDetailPage` compute their actions from the current status and pass them to `ActionBar`.

---

## 6. File Structure

### New Files
- `web/src/pages/DeploymentDetailPage.tsx` — deployment detail with activity log
- `web/src/pages/ReleaseDetailPage.tsx` — release detail with flag changes
- `web/src/components/ActionBar.tsx` — shared primary action + dropdown

### Modified Files
- `web/src/types.ts` — update DeployStatus, ReleaseStatus, Deployment, Release types; add FlagEnvState, DeploymentEvent, ReleaseFlagChange
- `web/src/pages/FlagDetailPage.tsx` — complete rewrite (header + 2 tabs)
- `web/src/pages/DeploymentsPage.tsx` — update mock data field names (project_id → application_id, traffic_percentage → traffic_percent)
- `web/src/pages/ReleasesPage.tsx` — update mock data to match new Release model (name instead of version, new statuses)
- `web/src/App.tsx` — import `DeploymentDetailPage` and `ReleaseDetailPage`, update `deployments/:id` and `releases/:id` routes to use them
- `web/src/styles/globals.css` — styles for activity log, action bar, env state table, detail header, inline diff
- `web/src/mocks/hierarchy.ts` — add mock data for flag env state, deployment events, release flag changes

---

## 7. Deferred Requirements

These are **hard requirements** for when real API integration is implemented. They are not in scope for Phase 2 (mock data) but must not be forgotten:

1. **Production confirmation dialogs**: All production environment modifications must display a clear confirmation dialog showing exactly what is being changed before applying. This applies to: flag environment toggles, deployment actions (promote, rollback), and release actions (start, promote).

2. **Change history and rollback**: Users must be able to view a history of changes to flag environment state and rollback to a prior point, or reverse a specific change from the historical view. This requires audit log infrastructure on the backend.
