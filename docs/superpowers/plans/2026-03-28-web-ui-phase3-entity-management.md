# Web UI Phase 3: Entity Management — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CRUD pages for org members (with group-based access control), API keys (with environment targeting), org-level environments, and applications.

**Architecture:** Create dedicated MembersPage (tabbed: Members + Groups) and APIKeysPage to replace SettingsPage overloads. Add Environments tab to org-level SettingsPage. Add CreateAppPage and update app-level SettingsPage with General + Danger Zone tabs. Update types and mock data to support groups, org environments, and env-targeted API keys.

**Tech Stack:** React 18, React Router 6, TypeScript 5.5, Vite 5.4, custom CSS (dark theme)

**Spec:** `docs/superpowers/specs/2026-03-28-web-ui-phase3-entity-management-design.md`

---

## File Structure

### New Files
- `web/src/pages/MembersPage.tsx` — org members + groups tabbed page
- `web/src/pages/APIKeysPage.tsx` — API keys list + create form with env targeting
- `web/src/pages/CreateAppPage.tsx` — application creation form

### Modified Files
- `web/src/types.ts` — add Member, Group, GroupRole, OrgEnvironment; update ApiKey with environment_targets
- `web/src/mocks/hierarchy.ts` — add MOCK_MEMBERS, MOCK_GROUPS, MOCK_ENVIRONMENTS, MOCK_API_KEYS
- `web/src/pages/SettingsPage.tsx` — restructure tabs: add Environments for org, update app to General + Danger Zone, remove Members/API Keys tabs
- `web/src/components/AppAccordion.tsx` — add "+ Add App" link
- `web/src/App.tsx` — update members/api-keys routes, add apps/new route
- `web/src/styles/globals.css` — styles for multi-select, inline forms, key reveal, group badges

---

## Task 1: Update Types

**Files:**
- Modify: `web/src/types.ts`

- [ ] **Step 1: Add GroupRole type and Member, Group, OrgEnvironment interfaces**

Append to end of `web/src/types.ts`:

```typescript
export type GroupRole = 'viewer' | 'editor' | 'admin';

export interface Member {
  id: string;
  name: string;
  email: string;
  role: 'owner' | 'member';
  group_ids: string[];
  joined_at: string;
}

export interface Group {
  id: string;
  name: string;
  role: GroupRole;
  environment_ids: string[];
  application_ids: string[];
  member_ids: string[];
  created_at: string;
}

export interface OrgEnvironment {
  id: string;
  name: string;
  slug: string;
  is_production: boolean;
  created_at: string;
}
```

- [ ] **Step 2: Update ApiKey interface**

Add `environment_targets` field to the existing `ApiKey` interface:

```typescript
export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  environment_targets: string[];  // env IDs, empty = all
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: May have errors in SettingsPage.tsx where local `APIKey` interface conflicts. That's fixed in Task 4.

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts
git commit -m "feat(web): add Member, Group, OrgEnvironment types; update ApiKey with environment_targets"
```

---

## Task 2: Add Mock Data

**Files:**
- Modify: `web/src/mocks/hierarchy.ts`

- [ ] **Step 1: Update imports**

Update the import line to include new types:

```typescript
import type { Organization, Application, Project, FlagEnvState, DeploymentEvent, Deployment, Release, ReleaseFlagChange, Member, Group, OrgEnvironment, ApiKey } from '@/types';
```

- [ ] **Step 2: Add MOCK_ENVIRONMENTS**

```typescript
export const MOCK_ENVIRONMENTS: OrgEnvironment[] = [
  { id: 'env-dev', name: 'Development', slug: 'development', is_production: false, created_at: '2025-06-01T00:00:00Z' },
  { id: 'env-staging', name: 'Staging', slug: 'staging', is_production: false, created_at: '2025-06-01T00:00:00Z' },
  { id: 'env-prod', name: 'Production', slug: 'production', is_production: true, created_at: '2025-06-01T00:00:00Z' },
];
```

- [ ] **Step 3: Add MOCK_MEMBERS**

```typescript
export const MOCK_MEMBERS: Member[] = [
  { id: 'user-1', name: 'Alice Chen', email: 'alice@acme.com', role: 'owner', group_ids: ['grp-1'], joined_at: '2025-06-01T00:00:00Z' },
  { id: 'user-2', name: 'Bob Smith', email: 'bob@acme.com', role: 'member', group_ids: ['grp-1', 'grp-2'], joined_at: '2025-07-15T00:00:00Z' },
  { id: 'user-3', name: 'Carol Davis', email: 'carol@acme.com', role: 'member', group_ids: ['grp-3'], joined_at: '2025-09-01T00:00:00Z' },
  { id: 'user-4', name: 'Dave Wilson', email: 'dave@acme.com', role: 'member', group_ids: ['grp-4'], joined_at: '2026-01-10T00:00:00Z' },
];
```

- [ ] **Step 4: Add MOCK_GROUPS**

```typescript
export const MOCK_GROUPS: Group[] = [
  { id: 'grp-1', name: 'Platform Admins', role: 'admin', environment_ids: [], application_ids: [], member_ids: ['user-1', 'user-2'], created_at: '2025-06-01T00:00:00Z' },
  { id: 'grp-2', name: 'Production Ops', role: 'editor', environment_ids: ['env-prod'], application_ids: [], member_ids: ['user-2'], created_at: '2025-08-01T00:00:00Z' },
  { id: 'grp-3', name: 'API Team', role: 'editor', environment_ids: [], application_ids: ['app-1', 'app-3'], member_ids: ['user-3'], created_at: '2025-09-15T00:00:00Z' },
  { id: 'grp-4', name: 'Junior Devs', role: 'viewer', environment_ids: ['env-dev', 'env-staging'], application_ids: [], member_ids: ['user-4'], created_at: '2026-01-15T00:00:00Z' },
];
```

- [ ] **Step 5: Add MOCK_API_KEYS**

```typescript
export const MOCK_API_KEYS: ApiKey[] = [
  {
    id: 'key-1',
    name: 'Production Backend',
    prefix: 'ds_prod_abc1****',
    scopes: ['flags:read'],
    environment_targets: ['env-prod'],
    created_at: '2025-11-15T00:00:00Z',
    last_used_at: '2026-03-28T10:00:00Z',
    expires_at: null,
  },
  {
    id: 'key-2',
    name: 'CI/CD Pipeline',
    prefix: 'ds_ci_def2****',
    scopes: ['deploys:read', 'deploys:write'],
    environment_targets: [],
    created_at: '2025-12-01T00:00:00Z',
    last_used_at: '2026-03-28T09:00:00Z',
    expires_at: null,
  },
  {
    id: 'key-3',
    name: 'Admin Dashboard',
    prefix: 'ds_admin_ghi3****',
    scopes: ['admin'],
    environment_targets: [],
    created_at: '2026-01-10T00:00:00Z',
    last_used_at: '2026-03-25T14:00:00Z',
    expires_at: null,
  },
];
```

- [ ] **Step 6: Add helper functions**

```typescript
export function getMockEnvironments(): OrgEnvironment[] {
  return MOCK_ENVIRONMENTS;
}

export function getEnvironmentName(envId: string): string {
  return MOCK_ENVIRONMENTS.find((e) => e.id === envId)?.name ?? envId;
}
```

- [ ] **Step 7: Commit**

```bash
git add web/src/mocks/hierarchy.ts
git commit -m "feat(web): add mock data for members, groups, environments, and API keys"
```

---

## Task 3: CSS for Phase 3 Components

**Files:**
- Modify: `web/src/styles/globals.css`

- [ ] **Step 1: Append Phase 3 CSS**

Append to end of `web/src/styles/globals.css`:

```css
/* ------------------------------------------------------------------ */
/* Multi-select checkbox group                                         */
/* ------------------------------------------------------------------ */
.checkbox-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.checkbox-group label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  transition: background 0.1s;
}

.checkbox-group label:hover {
  background: var(--color-bg-hover);
}

/* ------------------------------------------------------------------ */
/* Inline form (expandable create forms)                               */
/* ------------------------------------------------------------------ */
.inline-form {
  padding: 16px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  margin-bottom: 16px;
}

.inline-form-row {
  display: flex;
  gap: 12px;
  align-items: flex-end;
}

.inline-form-row .form-group {
  flex: 1;
  margin-bottom: 0;
}

/* ------------------------------------------------------------------ */
/* Key reveal box                                                      */
/* ------------------------------------------------------------------ */
.key-reveal {
  padding: 12px 16px;
  background: var(--color-success-bg);
  border: 1px solid rgba(34, 197, 94, 0.3);
  border-radius: var(--radius-md);
  margin: 12px 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.key-reveal code {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--color-success);
  word-break: break-all;
}

.key-reveal .btn {
  flex-shrink: 0;
}

/* ------------------------------------------------------------------ */
/* Group/role badges                                                   */
/* ------------------------------------------------------------------ */
.badge-viewer {
  background: var(--color-info-bg);
  color: var(--color-info);
}

.badge-editor {
  background: var(--color-warning-bg);
  color: var(--color-warning);
}

.badge-admin {
  background: var(--color-purple-bg);
  color: var(--color-purple);
}

.badge-owner {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

.badge-member {
  background: var(--color-info-bg);
  color: var(--color-info);
}

.badge-production {
  background: var(--color-danger-bg);
  color: var(--color-danger);
}

/* ------------------------------------------------------------------ */
/* Inline confirm                                                      */
/* ------------------------------------------------------------------ */
.inline-confirm {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 13px;
}

.inline-confirm span {
  color: var(--color-danger);
}

/* ------------------------------------------------------------------ */
/* Group manage popover                                                */
/* ------------------------------------------------------------------ */
.group-manage {
  position: relative;
}

.group-manage-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  min-width: 220px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 8px;
  z-index: 200;
  box-shadow: var(--shadow-md);
}

.group-manage-dropdown label {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  font-size: 13px;
  cursor: pointer;
  border-radius: var(--radius-sm);
}

.group-manage-dropdown label:hover {
  background: var(--color-bg-hover);
}

/* ------------------------------------------------------------------ */
/* Add app link in sidebar                                             */
/* ------------------------------------------------------------------ */
.add-app-link {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 12px;
  color: var(--color-primary);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: all 0.15s;
  text-decoration: none;
}

.add-app-link:hover {
  background: var(--color-primary-bg);
  color: var(--color-primary-hover);
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/styles/globals.css
git commit -m "feat(web): add CSS for members, groups, API keys, environments, and app forms"
```

---

## Task 4: Update SettingsPage

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

**Note:** Removing `members` and `api-keys` tabs will cause TypeScript errors in App.tsx (which still passes `tab="members"` and `tab="api-keys"` to SettingsPage) until Task 9 updates the routes. The `defaultTab` function should gracefully handle unknown tab values by falling back to the level's default. This prevents runtime crashes during the transition.

- [ ] **Step 1: Restructure SettingsPage tabs and content**

Major changes:
1. Update `SettingsTab` type to: `'environments' | 'webhooks' | 'notifications' | 'general' | 'danger'`
2. Remove the local `APIKey` interface (now imported from types)
3. Remove `MOCK_API_KEYS` (now in hierarchy.ts)
4. Remove `members` and `api-keys` from `getTabsForLevel('org')`; add `environments`
5. Update `getTabsForLevel('app')` to return `general` and `danger` tabs
6. Update `defaultTab`: org → `'environments'`, project → `'general'`, app → `'general'`
7. Remove the `members` and `api-keys` tab content sections
8. Add `environments` tab content for org level
9. Update `project` tab content key to `general`; for app level add `danger` tab

For the **Environments tab** (org level):
- Import `MOCK_ENVIRONMENTS, getMockEnvironments` from `@/mocks/hierarchy` and `OrgEnvironment` from `@/types`
- Use `useState` for local environment list
- Show table: Name, Slug (monospace), Production (badge), Created, Actions (delete with inline confirm)
- Add Environment form above table: Name input, slug (auto-generated), "Is Production" toggle, "Add" button
- Empty state: "No environments defined. Add one to get started."

For the **Project-level General tab** (was `project` tab):
- Keep the existing content: Project Name, Default Environment dropdown, Stale Flag Threshold. Save button.
- Just rename the tab key from `'project'` to `'general'`

For the **App-level General tab**:
- Different content than project level: Name, Slug (read-only input), Description (textarea), Repo URL. Save button.
- Detect `level === 'app'` to show app-specific fields instead of project fields.

For the **App-level Danger Zone tab**:
- Warning-styled section (`.danger-zone` class)
- "Delete Application" button with inline confirmation text
- On delete: no-op (mock) — just shows "Deleted" state

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat(web): restructure SettingsPage — add Environments tab, update app tabs to General + Danger Zone"
```

---

## Task 5: Members Page

**Files:**
- Create: `web/src/pages/MembersPage.tsx`

- [ ] **Step 1: Create MembersPage**

Create `web/src/pages/MembersPage.tsx` with:

**Imports:**
```tsx
import { useState, useRef, useEffect } from 'react';
import type { Member, Group, GroupRole } from '@/types';
import { MOCK_MEMBERS, MOCK_GROUPS, MOCK_ENVIRONMENTS, MOCK_APPLICATIONS, getEnvironmentName, getAppName } from '@/mocks/hierarchy';
```

**State:**
```tsx
const [activeTab, setActiveTab] = useState<'members' | 'groups'>('members');
const [members, setMembers] = useState<Member[]>([...MOCK_MEMBERS]);
const [groups, setGroups] = useState<Group[]>([...MOCK_GROUPS]);
// Add member form
const [newEmail, setNewEmail] = useState('');
const [newRole, setNewRole] = useState<'owner' | 'member'>('member');
// Create group form
const [showCreateGroup, setShowCreateGroup] = useState(false);
const [newGroupName, setNewGroupName] = useState('');
const [newGroupRole, setNewGroupRole] = useState<GroupRole>('viewer');
const [newGroupEnvIds, setNewGroupEnvIds] = useState<string[]>([]);
const [newGroupAppIds, setNewGroupAppIds] = useState<string[]>([]);
// Group management dropdown
const [managingMemberId, setManagingMemberId] = useState<string | null>(null);
```

**Members tab:**
- Add member form: email input + role dropdown + "Add" button (inline form row)
- Table: Name, Email, Org Role (badge), Groups (badge list with group names), Joined (formatted), Actions
- Actions column: role dropdown (changes immediately), "Manage Groups" button (opens dropdown), "Remove" button (inline confirm)
- "Manage Groups" dropdown: checkbox list of all groups. Toggle membership by updating both `members` and `groups` state.

**Groups tab:**
- "Create Group" button toggles the create form
- Create form (`.inline-form`): Name, Role dropdown, Environment checkboxes (from MOCK_ENVIRONMENTS), Application checkboxes (from MOCK_APPLICATIONS filtered to current org), "Create" button
- Unchecked environments/apps = all (show "All" badge in table)
- Table: Name, Role (badge with `.badge-viewer`/`.badge-editor`/`.badge-admin`), Environments (badge list using `getEnvironmentName()`), Applications (badge list using `getAppName()`, or "All" if empty), Members (count), Actions (Edit, Delete with inline confirm)
- **Edit Group**: clicking Edit on a row sets `editingGroupId` state. The row expands to show an inline edit form (same fields as create: Name, Role dropdown, Environment checkboxes, Application checkboxes). Pre-populated with current values. Save/Cancel buttons. Save updates the group in local state and clears `editingGroupId`.
- Add state: `const [editingGroupId, setEditingGroupId] = useState<string | null>(null);`
- Empty state: "No groups defined. Create one to manage access."

**Helpers:**
```tsx
function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/MembersPage.tsx
git commit -m "feat(web): add MembersPage with members list, groups management, and access control"
```

---

## Task 6: API Keys Page

**Files:**
- Create: `web/src/pages/APIKeysPage.tsx`

- [ ] **Step 1: Create APIKeysPage**

Create `web/src/pages/APIKeysPage.tsx` with:

**Imports:**
```tsx
import { useState } from 'react';
import type { ApiKey } from '@/types';
import { MOCK_API_KEYS, MOCK_ENVIRONMENTS, getEnvironmentName } from '@/mocks/hierarchy';
```

**State:**
```tsx
const [keys, setKeys] = useState<ApiKey[]>([...MOCK_API_KEYS]);
const [showCreate, setShowCreate] = useState(false);
const [revealedKey, setRevealedKey] = useState<string | null>(null); // full key shown once after create
// Create form
const [newName, setNewName] = useState('');
const [newScopes, setNewScopes] = useState<string[]>([]);
const [newEnvTargets, setNewEnvTargets] = useState<string[]>([]);
// Revoke confirm
const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null);
```

**Available scopes:**
```tsx
const AVAILABLE_SCOPES = ['flags:read', 'flags:write', 'deploys:read', 'deploys:write', 'admin'];
```

**Key list table:**
- Columns: Name, Key Prefix (monospace), Scopes (badges using existing `scopeBadgeClass`), Environments (badge list or "All"), Created (formatted), Last Used (formatted or "Never"), Actions
- Actions: Revoke button → inline confirm ("Are you sure? Yes / No")

**Create form** (toggled by "Create API Key" button):
- `.inline-form` container
- Name input (required)
- Scopes: `.checkbox-group` with checkboxes for each scope
- Environment targets: `.checkbox-group` with checkboxes for each org environment. Label: "Environment Restrictions (leave unchecked for all)"
- "Create Key" button
- On create: generate a fake key (`ds_${Date.now()}_${Math.random().toString(36).slice(2)}`), show in `.key-reveal` box with copy button. Add to list.

**Copy to clipboard:**
```tsx
function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
}
```

**Revoke:** removes key from local state.

Helper `scopeBadgeClass` — same as in current SettingsPage (copy it here or extract to shared location).

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/APIKeysPage.tsx
git commit -m "feat(web): add APIKeysPage with environment targeting and key reveal"
```

---

## Task 7: Create Application Page

**Files:**
- Create: `web/src/pages/CreateAppPage.tsx`

- [ ] **Step 1: Create CreateAppPage**

Create `web/src/pages/CreateAppPage.tsx`:

```tsx
import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { MOCK_ENVIRONMENTS } from '@/mocks/hierarchy';

export default function CreateAppPage() {
  const { orgSlug, projectSlug } = useParams();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [repoUrl, setRepoUrl] = useState('');

  function handleNameChange(value: string) {
    setName(value);
    setSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !slug) return;
    // Stub — would call applicationsApi.create() in the future
    navigate(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${slug}/deployments`);
  }

  const backPath = `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">Create Application</h1>
      </div>

      <div className="card" style={{ maxWidth: 600 }}>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Application Name</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              placeholder="API Server"
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label">Slug</label>
            <input
              type="text"
              className="form-input"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="api-server"
              required
            />
            <div className="form-hint">URL-safe identifier. Auto-generated from name.</div>
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea
              className="form-input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Core REST API for the platform"
              rows={3}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Repository URL (optional)</label>
            <input
              type="text"
              className="form-input"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="https://github.com/acme/api-server"
            />
          </div>
          <div className="form-group">
            <label className="form-label">Environments</label>
            <div className="form-hint" style={{ marginBottom: 8 }}>
              This application will inherit all org-level environments:
            </div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {MOCK_ENVIRONMENTS.map((env) => (
                <span key={env.id} className={`badge ${env.is_production ? 'badge-production' : 'badge-ops'}`}>
                  {env.name}
                </span>
              ))}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button type="submit" className="btn btn-primary">Create Application</button>
            <button type="button" className="btn btn-secondary" onClick={() => navigate(backPath)}>
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/CreateAppPage.tsx
git commit -m "feat(web): add CreateAppPage for application creation"
```

---

## Task 8: Update AppAccordion with "+ Add App" Link

**Files:**
- Modify: `web/src/components/AppAccordion.tsx`

- [ ] **Step 1: Add "+ Add App" link**

In `web/src/components/AppAccordion.tsx`, add a `Link` import:

```tsx
import { NavLink, Link, useParams } from 'react-router-dom';
```

After the `apps.map(...)` closing, before the final `</div>`, add:

```tsx
<Link
  to={`/orgs/${orgSlug}/projects/${projectSlug}/apps/new`}
  className="add-app-link"
>
  + Add App
</Link>
```

This appears below the accordion list, inside the `.app-accordion` container.

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/src/components/AppAccordion.tsx
git commit -m "feat(web): add '+ Add App' link to sidebar accordion"
```

---

## Task 9: Update Routes in App.tsx

**Files:**
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Add imports and update routes**

Add imports:
```tsx
import MembersPage from './pages/MembersPage';
import APIKeysPage from './pages/APIKeysPage';
import CreateAppPage from './pages/CreateAppPage';
```

Replace the members and api-keys routes:
```tsx
// Old:
<Route path="members" element={<SettingsPage level="org" tab="members" />} />
<Route path="api-keys" element={<SettingsPage level="org" tab="api-keys" />} />

// New:
<Route path="members" element={<MembersPage />} />
<Route path="api-keys" element={<APIKeysPage />} />
```

Add the apps/new route inside the project-level section, BEFORE the `apps/:appSlug` route:
```tsx
<Route path="apps/new" element={<CreateAppPage />} />
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): wire up MembersPage, APIKeysPage, and CreateAppPage routes"
```

---

## Task 10: Full Build and Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Full TypeScript check**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: Zero errors.

- [ ] **Step 2: Vite build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx vite build`
Expected: Build succeeds.

- [ ] **Step 3: Verify key routes**

Start dev server: `cd /Users/sgamel/git/DeploySentry/web && npx vite --port 3001 &`

Verify:
- `http://localhost:3001/orgs/acme-corp/members` → Members page with Members + Groups tabs
- `http://localhost:3001/orgs/acme-corp/api-keys` → API Keys page with create form
- `http://localhost:3001/orgs/acme-corp/settings` → Settings with Environments tab (default)
- `http://localhost:3001/orgs/acme-corp/projects/platform/apps/new` → Create Application form
- `http://localhost:3001/orgs/acme-corp/projects/platform/apps/api-server/settings` → App settings with General + Danger Zone tabs

- [ ] **Step 4: Stop dev server**

```bash
kill %1 2>/dev/null || true
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Add Phase 3 to initiatives and mark Phase 2 complete**

Update the table:
- Mark "Web UI Phase 2 — Page Redesigns" as Complete
- Add: `| Web UI Phase 3 — Entity Management | Implementation | [Link](./superpowers/plans/2026-03-28-web-ui-phase3-entity-management.md) |`

- [ ] **Step 2: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: mark Phase 2 complete, add Phase 3 entity management to initiatives"
```
