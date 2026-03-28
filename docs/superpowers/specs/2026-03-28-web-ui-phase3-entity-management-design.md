# Web UI Phase 3: Entity Management Pages

## Overview

Add CRUD pages for org members (with group-based access control), API keys (org-scoped with environment targeting), org-level environments, and applications. Introduces a groups system where groups carry a role and are scoped to specific environments and/or applications.

**Parent spec:** `docs/superpowers/specs/2026-03-28-platform-redesign-design.md`
**Depends on:** Phase 1 Navigation Overhaul (complete), Phase 2 Page Redesigns (complete)

## Goals

1. Add org members management with a groups-based access control model.
2. Add group management — groups carry a role (viewer/editor/admin) and are scoped to environments and/or applications.
3. Add API key management with org-level keys that can target specific environments.
4. Add org-level environment management (environments are org-wide, not per-application).
5. Add application CRUD within projects.

## Non-Goals

- Real API integration — pages continue using mock data.
- Invitation/email flow for adding members — just adds to mock list.
- Granular per-action permissions — groups carry a single role, not fine-grained permission flags.
- Project CRUD — the "Create Project" button stub from Phase 1 remains a stub.

---

## 1. Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| API keys | Org-level with env targeting | Keys are org-scoped but can target one or more environments to segregate staging vs production access. |
| Application creation | With default environments | Create form offers org environments as defaults. Saves a round trip. |
| Org members | Simple list + add form | No invitation flow with mock data. Add/remove from list. |
| Environments | Org-level | Environments are org-wide. Apps inherit all org environments and may not use some. **Deliberate deviation from parent spec** which scoped environments to applications. |
| Groups | Role + scope model | Each group has one role (viewer/editor/admin) and optional scope constraints (environments, applications). Simplest model that solves large-org access management. |

---

## 2. Access Control Model

### 2.1 Concepts

- **Member**: a user belonging to an org. Has a base org role (owner or member) and belongs to zero or more groups.
- **Group**: a named collection of members with a role and optional scope. Groups are the unit of access control.
- **Role** (on groups): `viewer` (read-only), `editor` (read-write), `admin` (full control including destructive actions).
- **Scope** (on groups): optional environment and/or application restrictions. No scope = org-wide access.

### 2.2 Examples

| Group | Role | Environments | Applications | Effect |
|-------|------|-------------|-------------|--------|
| Platform Admins | admin | (all) | (all) | Full org-wide access |
| Production Ops | editor | Production | (all) | Can deploy/toggle flags in production across all apps |
| API Team | editor | (all) | API Server, Worker | Full access to API Server and Worker only |
| Junior Devs | viewer | Development, Staging | (all) | Read-only in dev and staging |
| QA | editor | Staging | (all) | Can deploy/toggle in staging only |

### 2.3 Effective Access

A member's effective access is: base org role (owner = full org admin, member = restricted) PLUS the union of all their group scopes and roles. The highest role wins for overlapping scopes.

With mock data, this is purely display — no enforcement. The model is designed for backend enforcement when API integration lands.

### 2.4 Types

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
  environment_ids: string[];   // empty = all environments
  application_ids: string[];   // empty = all applications
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

---

## 3. Members & Groups Page

### 3.1 Route

`/orgs/:orgSlug/members` — currently renders `SettingsPage level="org" tab="members"`. Replace with dedicated `MembersPage`.

### 3.2 Layout

Tabbed page with **Members** and **Groups** tabs.

### 3.3 Members Tab

**Member list table:**
- Columns: Name, Email, Org Role (owner/member badge), Groups (badge list), Joined, Actions
- Actions: Change org role (dropdown), Manage groups (opens inline group picker), Remove (with inline "Are you sure?" confirmation)

**Add Member form** (above table):
- Email input + org role dropdown (owner/member) + "Add" button
- Adds to local mock state immediately

### 3.4 Groups Tab

**Group list table:**
- Columns: Name, Role (viewer/editor/admin badge), Environments (badge list or "All"), Applications (badge list or "All"), Members (count), Actions
- Actions: Edit, Delete (with confirmation)

**Create Group form** (expandable inline section):
- Name (text input)
- Role (dropdown: viewer/editor/admin)
- Environments (multi-select checkboxes from org environments list). Unchecked = all environments.
- Applications (multi-select checkboxes from all org applications). Unchecked = all applications.
- "Create" button
- Adds to local mock state

**Edit Group:** clicking Edit on a row expands an inline edit form (same fields as create, pre-populated). Save/Cancel buttons.

**Manage Groups (from Members tab):** clicking "Manage Groups" on a member row shows a checkbox list of all groups. Toggle membership immediately (mock state).

### 3.5 Mock Data

```typescript
const MOCK_MEMBERS: Member[] = [
  { id: 'user-1', name: 'Alice Chen', email: 'alice@acme.com', role: 'owner', group_ids: ['grp-1'], joined_at: '2025-06-01T00:00:00Z' },
  { id: 'user-2', name: 'Bob Smith', email: 'bob@acme.com', role: 'member', group_ids: ['grp-1', 'grp-2'], joined_at: '2025-07-15T00:00:00Z' },
  { id: 'user-3', name: 'Carol Davis', email: 'carol@acme.com', role: 'member', group_ids: ['grp-3'], joined_at: '2025-09-01T00:00:00Z' },
  { id: 'user-4', name: 'Dave Wilson', email: 'dave@acme.com', role: 'member', group_ids: ['grp-4'], joined_at: '2026-01-10T00:00:00Z' },
];

const MOCK_GROUPS: Group[] = [
  { id: 'grp-1', name: 'Platform Admins', role: 'admin', environment_ids: [], application_ids: [], member_ids: ['user-1', 'user-2'], created_at: '2025-06-01T00:00:00Z' },
  { id: 'grp-2', name: 'Production Ops', role: 'editor', environment_ids: ['env-prod'], application_ids: [], member_ids: ['user-2'], created_at: '2025-08-01T00:00:00Z' },
  { id: 'grp-3', name: 'API Team', role: 'editor', environment_ids: [], application_ids: ['app-1', 'app-3'], member_ids: ['user-3'], created_at: '2025-09-15T00:00:00Z' },
  { id: 'grp-4', name: 'Junior Devs', role: 'viewer', environment_ids: ['env-dev', 'env-staging'], application_ids: [], member_ids: ['user-4'], created_at: '2026-01-15T00:00:00Z' },
];
```

---

## 4. API Keys Page

### 4.1 Route

`/orgs/:orgSlug/api-keys` — currently renders `SettingsPage level="org" tab="api-keys"`. Replace with dedicated `APIKeysPage`.

### 4.2 Layout

**Key list table:**
- Columns: Name, Key Prefix, Scopes (badge list), Environments (badge list or "All"), Created, Last Used, Actions
- Actions: Revoke (with confirmation)

**Create Key form** (expandable section):
- Name (text input)
- Scopes (multi-select checkboxes): `flags:read`, `flags:write`, `deploys:read`, `deploys:write`, `admin`
- Environment targets (multi-select checkboxes from org environments). Unchecked = all environments.
- "Create Key" button
- On create: show the full key in a highlighted box with a "Copy" button. Key is only shown once — after dismissing, only the prefix is visible.

**Revoke:** button swaps to "Are you sure? Yes / No" inline confirmation.

### 4.3 Updated ApiKey Type

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

### 4.4 Mock Data

Update existing `MOCK_API_KEYS` (currently in SettingsPage) to include `environment_targets`. Move to `hierarchy.ts` alongside other mock data. **Note:** The existing mock data in SettingsPage uses camelCase field names (`created`, `lastUsed`) while the type uses snake_case (`created_at`, `last_used_at`). When moving, update the mock data to match the `ApiKey` type field names.

---

## 5. Org Settings — Environments Tab

### 5.1 Route

`/orgs/:orgSlug/settings` — existing `SettingsPage level="org"`. Add an "Environments" tab.

### 5.2 Environments Tab

**Environment list table:**
- Columns: Name, Slug (monospace), Production (badge if true), Created, Actions
- Actions: Edit (inline name edit), Delete (with confirmation)

**Add Environment form** (inline, above table):
- Name input
- Slug (auto-generated from name, editable)
- "Is Production" toggle
- "Add" button

**Default mock environments:**
```typescript
const MOCK_ENVIRONMENTS: OrgEnvironment[] = [
  { id: 'env-dev', name: 'Development', slug: 'development', is_production: false, created_at: '2025-06-01T00:00:00Z' },
  { id: 'env-staging', name: 'Staging', slug: 'staging', is_production: false, created_at: '2025-06-01T00:00:00Z' },
  { id: 'env-prod', name: 'Production', slug: 'production', is_production: true, created_at: '2025-06-01T00:00:00Z' },
];
```

### 5.3 SettingsPage Tab Updates

**Org-level tabs change:** Remove `members` and `api-keys` tabs from `getTabsForLevel('org')` since those routes now render dedicated pages. Add `environments` tab. The updated org tabs are:
- `environments` (new — default tab)
- `webhooks` (existing)
- `notifications` (existing)

**App-level tabs change:** Replace the single `project` tab with:
- `general` (new — default tab): Name, Slug (read-only), Description, Repo URL, Save button
- `danger` (new): Delete Application button with confirmation

**Updated `SettingsTab` type:**
```typescript
type SettingsTab = 'environments' | 'webhooks' | 'notifications' | 'general' | 'danger';
```

The `defaultTab` function updates accordingly: org defaults to `'environments'`, project defaults to `'general'`, app defaults to `'general'`.

### 5.4 `OrgEnvironment` vs. Existing `Environment` Type

The existing `Environment` type (`{ id, name, project_id }`) was from the old model where environments were project-scoped. The new `OrgEnvironment` type replaces it conceptually. For Phase 3:
- Add `OrgEnvironment` as a new type (used by environment management pages, group scoping, API key targeting)
- Keep the existing `Environment` type for now — it's still referenced by mock data in other pages
- When real API integration lands, `Environment` will be removed and `OrgEnvironment` will become the canonical type

Both types coexist temporarily. New code should use `OrgEnvironment`.

---

## 6. Application CRUD

### 6.1 Create Application

**Route:** `/orgs/:orgSlug/projects/:projectSlug/apps/new` (NEW)

**Form fields:**
- Name (text input, required)
- Slug (auto-generated from name, editable, required)
- Description (textarea, optional)
- Repo URL (text input, optional)

On submit: adds to mock app list in local state, navigates to `/orgs/:orgSlug/projects/:projectSlug/apps/:newSlug/deployments`.

**Sidebar integration:** Add a "+ Add App" link below the app accordion list. Only visible when a project is selected.

### 6.2 Edit Application

**Route:** `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/settings`

Update the app-level `SettingsPage` tabs:
- **General tab**: Name (editable), Slug (read-only), Description (editable), Repo URL (editable). Save button.
- **Danger Zone tab**: "Delete Application" button with confirmation text. Deleting navigates back to project flags page.

### 6.3 No Standalone List Page

The sidebar `AppAccordion` is the application list. No separate list page needed.

---

## 7. File Structure

### New Files
- `web/src/pages/MembersPage.tsx` — members + groups tabbed page
- `web/src/pages/APIKeysPage.tsx` — API keys list + create form with env targeting
- `web/src/pages/CreateAppPage.tsx` — application creation form

### Modified Files
- `web/src/types.ts` — add Member, Group, GroupRole, OrgEnvironment; update ApiKey with environment_targets
- `web/src/mocks/hierarchy.ts` — add MOCK_MEMBERS, MOCK_GROUPS, MOCK_ENVIRONMENTS, MOCK_API_KEYS (moved from SettingsPage)
- `web/src/App.tsx` — update members/api-keys routes, add apps/new route
- `web/src/pages/SettingsPage.tsx` — add Environments tab for org level; update app level with General + Danger Zone tabs
- `web/src/components/AppAccordion.tsx` — add "+ Add App" link
- `web/src/styles/globals.css` — styles for multi-select, inline edit, key reveal, group badges

### Routes

```
/orgs/:orgSlug/members                                          → MembersPage (replaces SettingsPage)
/orgs/:orgSlug/api-keys                                         → APIKeysPage (replaces SettingsPage)
/orgs/:orgSlug/settings                                         → SettingsPage level="org" (add Environments tab)
/orgs/:orgSlug/projects/:projectSlug/apps/new                   → CreateAppPage (NEW)
/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/settings     → SettingsPage level="app" (updated tabs)
```

---

## 8. Form Validation and Empty States

**Validation:** All forms require basic client-side validation (required fields must be non-empty). Duplicate detection (e.g., duplicate email, duplicate group name, duplicate environment slug) is deferred to API integration — with mock data, duplicates may exist temporarily.

**Empty states:** All tables show an empty state message when the list is empty:
- Members: "No members yet. Add one above."
- Groups: "No groups defined. Create one to manage access."
- API Keys: "No API keys. Create one to integrate with DeploySentry."
- Environments: "No environments defined. Add one to get started."

---

## 9. Deferred

- **Real invitation flow** — email-based invites with pending state, resend, expire. Requires backend.
- **Granular permissions** — per-action permission flags on groups (can_deploy, can_toggle_flags, etc.). Current model uses role-based (viewer/editor/admin).
- **Project CRUD** — "Create Project" button on ProjectListPage remains a stub.
- **Group enforcement** — backend middleware to check group scopes on API requests. Currently display-only.
