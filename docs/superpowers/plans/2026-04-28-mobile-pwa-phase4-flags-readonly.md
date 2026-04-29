# Mobile PWA — Phase 4: Flags Tab (Read-Only) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax. Each task ends with a verification step that includes `npm run lint` (eslint) — never commit a step without lint clean.

**Goal:** Replace the Phase 1 `FlagProjectPickerPage` placeholder with a real Flags tab. Three drill-down levels: project picker → flag list (with search + category filter) → flag detail (env states + rule listings + audit history). All read-only — no PUT/POST/DELETE in Phase 4.

**Architecture:** Reuse the same `request<T>` helper, the same JWT auth, the same env-chip vocabulary as the Status tab. The flag list maps the existing `GET /flags?project_id=...` response into rows showing the per-env on/off toggle state from `GET /flags/:id/environments`. Detail page fans in 4 endpoints in parallel. Audit history is offset-paginated (the existing endpoint uses limit/offset, not cursor).

**Tech Stack:** Same as Phase 2/3. No new dependencies.

**Scope reminder (per spec):** Read-only. No env enable toggle, no default-value edit, no rule enable/disable toggle, no rule reorder, no rule edit sheet, no flag create/archive — those are Phase 5. No "create rule" or "delete rule" in any form. Compound rules render but their summary just says "compound" — full rule introspection is desktop-only.

---

## File Structure

```
mobile-pwa/
└── src/
    ├── types.ts                                # MODIFY — append Flag + TargetingRule + FlagEnvironmentState + RuleEnvironmentState + OrgEnvironment + AuditLogEntry + Application
    ├── api.ts                                  # MODIFY — add flagsApi, flagEnvStateApi, envApi.listOrg, appsApi.list, auditApi.listForFlag
    ├── components/
    │   ├── CategoryBadge.tsx                   # CREATE — colored badge per flag.category
    │   ├── CategoryBadge.test.tsx
    │   ├── CategoryFilterChips.tsx             # CREATE — multi-select chip row (release/feature/experiment/ops/permission)
    │   ├── CategoryFilterChips.test.tsx
    │   ├── FlagEnvStrip.tsx                    # CREATE — env-chip strip showing enabled/disabled per env for a flag
    │   ├── FlagEnvStrip.test.tsx
    │   ├── FlagRow.tsx                         # CREATE — list row: key/name + CategoryBadge + FlagEnvStrip
    │   └── FlagRow.test.tsx
    ├── pages/
    │   ├── FlagProjectPickerPage.tsx           # REPLACE — real page (was placeholder)
    │   ├── FlagProjectPickerPage.test.tsx
    │   ├── FlagListPage.tsx                    # CREATE — search + category multi-select + rows
    │   ├── FlagListPage.test.tsx
    │   ├── FlagDetailPage.tsx                  # CREATE — env sections + rule listings + history
    │   └── FlagDetailPage.test.tsx
    ├── App.tsx                                 # MODIFY — add /flags routes
    ├── e2e/smoke.spec.ts                       # MODIFY — add flag drill-down case
    └── styles/
        └── mobile.css                          # MODIFY — append flag-specific classes
```

---

## Task 1: Extend types.ts with Flag domain types

**Files:**
- Modify: `mobile-pwa/src/types.ts`

- [ ] **Step 1:** Append after the existing `Project` interface:

```ts
export interface Application {
  id: string;
  slug: string;
  name: string;
  project_id: string;
}

export interface OrgEnvironment {
  id: string;
  slug: string;
  name: string;
  is_production?: boolean;
  sort_order?: number;
}

export type FlagCategory = 'release' | 'feature' | 'experiment' | 'ops' | 'permission';
export type FlagType = 'boolean' | 'string' | 'number' | 'json';

export interface Flag {
  id: string;
  project_id: string;
  application_id?: string | null;
  environment_id?: string;
  key: string;
  name: string;
  description?: string;
  flag_type: FlagType;
  category: FlagCategory;
  purpose?: string;
  owners?: string[];
  tags?: string[];
  is_permanent: boolean;
  expires_at?: string | null;
  default_value: string;
  enabled: boolean;
  archived: boolean;
  created_by?: string;
  created_by_name?: string;
  created_at: string;
  updated_at: string;
}

export type RuleType = 'percentage' | 'user_target' | 'attribute' | 'segment' | 'schedule' | 'compound';

export interface TargetingRule {
  id: string;
  flag_id: string;
  rule_type?: RuleType;
  attribute?: string;
  operator?: string;
  target_values?: string[];
  value: string;
  priority: number;
  percentage?: number | null;
  user_ids?: string[] | null;
  segment_id?: string | null;
  start_time?: string | null;
  end_time?: string | null;
  created_at: string;
  updated_at: string;
}

export interface FlagEnvironmentState {
  id?: string;
  flag_id: string;
  environment_id: string;
  enabled: boolean;
  value?: unknown;
  updated_by?: string;
  updated_at?: string;
}

export interface RuleEnvironmentState {
  id?: string;
  rule_id: string;
  environment_id: string;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface AuditLogEntry {
  id: string;
  resource_type: string;
  resource_id: string;
  action: string;
  actor_id?: string;
  actor_name?: string;
  old_value?: string | null;
  new_value?: string | null;
  metadata?: Record<string, unknown>;
  created_at: string;
}
```

- [ ] **Step 2:** Verify: `npx tsc --noEmit` clean.

---

## Task 2: Extend api.ts (TDD)

**Files:**
- Modify: `mobile-pwa/src/api.ts`
- Modify: `mobile-pwa/src/api.test.ts`

- [ ] **Step 1 (test first):** Append to `api.test.ts` covering:
  - `flagsApi.list(projectId)` builds `/flags?project_id=<id>` with optional `category` and `archived` query params, returns `{ flags }`.
  - `flagsApi.list(projectId, { applicationId })` includes `application_id` in the query string.
  - `flagsApi.get(id)` calls `/flags/<id>`.
  - `flagsApi.listRules(flagId)` calls `/flags/<flagId>/rules`.
  - `flagsApi.listRuleEnvStates(flagId)` calls `/flags/<flagId>/rules/environment-states`.
  - `flagEnvStateApi.list(flagId)` calls `/flags/<flagId>/environments`.
  - `envApi.listOrg(orgSlug)` calls `/orgs/<slug>/environments`.
  - `appsApi.list(orgSlug, projectSlug)` calls `/orgs/<slug>/projects/<proj>/applications`.
  - `auditApi.listForFlag(flagId, { limit, offset })` calls `/audit-log?resource_type=flag&resource_id=<id>&limit=<n>&offset=<m>`.
  - Each test uses the `setFetch` seam, asserts URL + Authorization header, returns canned JSON.

- [ ] **Step 2:** Add new sections to `api.ts`. Skeleton:

```ts
export const flagsApi = {
  list: (
    projectId: string,
    params: { category?: string; archived?: boolean; applicationId?: string } = {},
  ) => {
    const qs = new URLSearchParams({ project_id: projectId });
    if (params.category) qs.set('category', params.category);
    if (params.archived !== undefined) qs.set('archived', String(params.archived));
    if (params.applicationId) qs.set('application_id', params.applicationId);
    return request<{ flags: Flag[] }>(`/flags?${qs.toString()}`);
  },
  get: (id: string) => request<Flag>(`/flags/${id}`),
  listRules: (flagId: string) =>
    request<{ rules: TargetingRule[] }>(`/flags/${flagId}/rules`),
  listRuleEnvStates: (flagId: string) =>
    request<{ rule_environment_states: RuleEnvironmentState[] }>(
      `/flags/${flagId}/rules/environment-states`,
    ),
};

export const flagEnvStateApi = {
  list: (flagId: string) =>
    request<{ environment_states: FlagEnvironmentState[] }>(`/flags/${flagId}/environments`),
};

export const envApi = {
  listOrg: (orgSlug: string) =>
    request<{ environments: OrgEnvironment[] }>(`/orgs/${orgSlug}/environments`),
};

export const appsApi = {
  list: (orgSlug: string, projectSlug: string) =>
    request<{ applications: Application[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/applications`),
};

export const auditApi = {
  listForFlag: (flagId: string, opts: { limit?: number; offset?: number } = {}) => {
    const qs = new URLSearchParams({ resource_type: 'flag', resource_id: flagId });
    if (opts.limit) qs.set('limit', String(opts.limit));
    if (opts.offset) qs.set('offset', String(opts.offset));
    return request<{ entries: AuditLogEntry[]; total: number }>(`/audit-log?${qs.toString()}`);
  },
};
```

Import the new types at the top of `api.ts` and add to the existing `import type` block.

- [ ] **Step 3:** Verify: `npm run test --silent` (api tests pass), `npx tsc --noEmit` clean, `npm run lint` clean.

---

## Task 3: CategoryBadge component (TDD)

**Files:**
- Create: `mobile-pwa/src/components/CategoryBadge.tsx`
- Create: `mobile-pwa/src/components/CategoryBadge.test.tsx`

- [ ] **Step 1 (test first):** Render `<CategoryBadge category="release" />` for each of the 5 categories. Assert text matches the category, and `data-category="release"` (or whichever) is set on the root for CSS hooks. Snapshot or simple class assertion is fine — match the pattern used in `StatusPill.test.tsx`.

- [ ] **Step 2:** Implement:

```tsx
import type { FlagCategory } from '../types';

export function CategoryBadge({ category }: { category: FlagCategory }) {
  return (
    <span className="m-cat-badge" data-category={category}>
      {category}
    </span>
  );
}
```

- [ ] **Step 3:** Add styles to `mobile.css`:

```css
.m-cat-badge {
  display: inline-block;
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 99px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-family: var(--font-display, system-ui);
}
.m-cat-badge[data-category='release']    { background: rgba(251,146,60,0.12); color: #fb923c; }
.m-cat-badge[data-category='feature']    { background: rgba(99,102,241,0.12); color: #818cf8; }
.m-cat-badge[data-category='experiment'] { background: rgba(168,85,247,0.12); color: #c084fc; }
.m-cat-badge[data-category='ops']        { background: rgba(34,197,94,0.12);  color: #4ade80; }
.m-cat-badge[data-category='permission'] { background: rgba(239,68,68,0.12);  color: #f87171; }
```

- [ ] **Step 4:** Verify: vitest green, `npm run lint` clean, `npx tsc --noEmit` clean.

---

## Task 4: CategoryFilterChips (TDD)

**Files:**
- Create: `mobile-pwa/src/components/CategoryFilterChips.tsx`
- Create: `mobile-pwa/src/components/CategoryFilterChips.test.tsx`

Multi-select chip row mirroring the Phase 3 `StatusFilterChips` API shape.

- [ ] **Step 1 (test first):** Tests:
  - Renders chips for each of the 5 categories plus an "All" chip.
  - With no `value` (or empty array), the "All" chip is `aria-pressed="true"`.
  - Tapping a category chip calls `onChange` with `['release']`.
  - Tapping a second chip extends the set: `['release', 'feature']`.
  - Tapping a chip already in the set removes it.
  - Tapping "All" calls `onChange([])` to clear.

- [ ] **Step 2:** Component shape:

```tsx
import type { FlagCategory } from '../types';

const CATEGORIES: FlagCategory[] = ['release', 'feature', 'experiment', 'ops', 'permission'];

export function CategoryFilterChips({
  value,
  onChange,
}: {
  value: FlagCategory[];
  onChange: (next: FlagCategory[]) => void;
}) {
  const allActive = value.length === 0;
  return (
    <div className="m-chip-row" role="group" aria-label="Category filter">
      <button
        type="button"
        className="m-filter-chip"
        aria-pressed={allActive}
        onClick={() => onChange([])}
      >
        All
      </button>
      {CATEGORIES.map((cat) => {
        const active = value.includes(cat);
        return (
          <button
            key={cat}
            type="button"
            className="m-filter-chip"
            aria-pressed={active}
            onClick={() =>
              onChange(active ? value.filter((c) => c !== cat) : [...value, cat])
            }
          >
            {cat}
          </button>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 3:** Reuse the `.m-filter-chip` and `.m-chip-row` styles from Phase 3. If they don't already accommodate multi-select via `aria-pressed`, append:

```css
.m-filter-chip[aria-pressed='true'] {
  background: var(--color-primary, #6366f1);
  color: var(--color-primary-fg, #fff);
  border-color: var(--color-primary, #6366f1);
}
```

- [ ] **Step 4:** Verify: vitest green, `npm run lint` clean, `npx tsc --noEmit` clean.

---

## Task 5: FlagEnvStrip + FlagRow (TDD)

**Files:**
- Create: `mobile-pwa/src/components/FlagEnvStrip.tsx` + test
- Create: `mobile-pwa/src/components/FlagRow.tsx` + test

`FlagEnvStrip` is a presentational strip of small env chips, one per env, colored on/off based on the corresponding `FlagEnvironmentState.enabled`. Reuses the env-chip styling vocabulary from `EnvChip` but is non-interactive (purely visual on the list row).

- [ ] **Step 1 (test FlagEnvStrip):**
  - Renders one chip per environment passed in.
  - Chips with a matching `FlagEnvironmentState.enabled === true` render `data-on="true"`; otherwise `data-on="false"`.
  - When no state row exists for an env, default is `data-on="false"`.

- [ ] **Step 2:** Implement:

```tsx
import type { OrgEnvironment, FlagEnvironmentState } from '../types';

export function FlagEnvStrip({
  environments,
  states,
}: {
  environments: OrgEnvironment[];
  states: FlagEnvironmentState[];
}) {
  return (
    <div className="m-flag-env-strip" aria-label="Environment states">
      {environments.map((env) => {
        const state = states.find((s) => s.environment_id === env.id);
        const on = state?.enabled === true;
        return (
          <span
            key={env.id}
            className="m-flag-env-pip"
            data-on={on}
            title={`${env.slug}: ${on ? 'on' : 'off'}`}
          >
            {env.slug}
          </span>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 3 (test FlagRow):**
  - Renders flag.key (mono) and flag.name.
  - Renders `<CategoryBadge>` with the flag's category.
  - Renders `<FlagEnvStrip>` with the supplied environments and states for this flag.
  - Wraps in a `<Link to={detailHref}>` so `MemoryRouter` tests can assert href.

- [ ] **Step 4:** Implement using the same React Router import pattern as `DeploymentRow`. Pull pre-fetched env states from a parent-supplied prop — `FlagRow` does NOT call the API; the parent fetches once in bulk for all flags.

- [ ] **Step 5:** Append styles for `.m-flag-env-strip` and `.m-flag-env-pip` (small monospace pips with `data-on` driving color).

- [ ] **Step 6:** Verify: vitest green, `npm run lint` clean, `npx tsc --noEmit` clean.

---

## Task 6: FlagProjectPickerPage — replace Phase 1 placeholder (TDD)

**Files:**
- Modify: `mobile-pwa/src/pages/FlagProjectPickerPage.tsx` (replace placeholder body)
- Create: `mobile-pwa/src/pages/FlagProjectPickerPage.test.tsx`

Mirrors Phase 3's project filter sheet semantics, but as a full page. Single-project orgs auto-redirect to their flag list.

- [ ] **Step 1 (test):**
  - Mount under `MemoryRouter` at `/m/orgs/acme/flags`.
  - Mock `setFetch` to return `{ projects: [{ id, slug, name, ... }] }`.
  - **Single project case:** assert `useNavigate` is called with `/m/orgs/acme/flags/<slug>` (use a navigation spy or assert location after rerender).
  - **Multi-project case:** renders one row per project, each row is a `<Link>` whose `to` is the flag list path.
  - **Empty case:** renders an empty state ("No projects in this org").
  - **Error case:** renders the error message.

- [ ] **Step 2:** Implementation pattern:

```tsx
import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { projectsApi } from '../api';
import type { Project } from '../types';

export function FlagProjectPickerPage() {
  const { orgSlug } = useParams();
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!orgSlug) return;
    projectsApi
      .list(orgSlug)
      .then((res) => setProjects(res.projects))
      .catch((err) => setError(err.message));
  }, [orgSlug]);

  useEffect(() => {
    if (projects && projects.length === 1 && orgSlug) {
      navigate(`/m/orgs/${orgSlug}/flags/${projects[0].slug}`, { replace: true });
    }
  }, [projects, orgSlug, navigate]);

  if (error) return <section><p className="m-error">{error}</p></section>;
  if (projects === null) return <section><p>Loading projects…</p></section>;
  if (projects.length === 0) {
    return (
      <section>
        <h2>Flags</h2>
        <p className="m-muted">No projects in this org yet.</p>
      </section>
    );
  }

  return (
    <section>
      <h2>Flags</h2>
      <ul className="m-list">
        {projects.map((p) => (
          <li key={p.id}>
            <Link to={`/m/orgs/${orgSlug}/flags/${p.slug}`} className="m-list-row">
              {p.name}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
```

- [ ] **Step 3:** Verify: vitest green, `npm run lint` clean, `npx tsc --noEmit` clean.

---

## Task 7: FlagListPage (TDD)

**Files:**
- Create: `mobile-pwa/src/pages/FlagListPage.tsx`
- Create: `mobile-pwa/src/pages/FlagListPage.test.tsx`

Drilled-into page at `/m/orgs/:orgSlug/flags/:projectSlug` (and the app-scoped variant at `/m/orgs/:orgSlug/flags/:projectSlug/apps/:appSlug`).

**Behavior:**
1. On mount, fan in:
   - `entitiesApi.getProject(orgSlug, projectSlug)` to resolve project ID. (Or piggyback on `projectsApi.list` and find by slug — pick whichever already exists. The PWA api currently has `projectsApi.list`; use that to avoid adding `getProject` if we don't need it.)
   - `envApi.listOrg(orgSlug)` for env-strip rendering.
   - When `:appSlug` is present: `appsApi.list(orgSlug, projectSlug)` to resolve `application_id` for the filter.
2. Once project ID is known, call `flagsApi.list(projectId, { applicationId? })`.
3. For each flag in the list, fan in `flagEnvStateApi.list(flag.id)` in parallel and stash results in a `Map<flagId, FlagEnvironmentState[]>` for `FlagRow` to consume. (Cap concurrency at ~6 using a small in-component helper if the list is large; for MVP a `Promise.all` over the result is fine — typical project has < 50 flags.)
4. Render search input (filters by `flag.key` or `flag.name`, case-insensitive), `CategoryFilterChips`, then a `<ul>` of `FlagRow`.
5. Filter state is held in `useState`. Search query is also reflected in `?q=...&category=...` so deep-links work; use `useSearchParams`. `category` becomes a comma-separated list when multi-select.

- [ ] **Step 1 (test):** Tests should cover:
  - Renders flag rows from a mocked API response.
  - Search input filters by `key` (typing "checkout" hides flags whose key/name doesn't include it).
  - Tapping a category chip filters the list (only `release` flags visible after tapping `release`).
  - URL syncs: starting with `?q=foo&category=release` pre-fills the inputs and filters appropriately.
  - Error state surfaces a message.
  - Empty state renders ("No flags in this project").
  - App-scoped variant: when `:appSlug` is present, `flagsApi.list` is called with `application_id` set.

- [ ] **Step 2:** Implement. Heading shows `<projectName>` (or `<projectName> / <appName>` when in the app-scoped variant). Add a "Switch project →" link back to `/m/orgs/:orgSlug/flags`.

- [ ] **Step 3:** Verify: vitest green, `npm run lint` clean, `npx tsc --noEmit` clean.

---

## Task 8: FlagDetailPage (TDD)

**Files:**
- Create: `mobile-pwa/src/pages/FlagDetailPage.tsx`
- Create: `mobile-pwa/src/pages/FlagDetailPage.test.tsx`

Mounted at `/m/orgs/:orgSlug/flags/:projectSlug/:flagId` (and the app-scoped variant). All read-only.

**Header:**
- Back chevron → flag list.
- Flag key (mono), name, `<CategoryBadge>`, `is_permanent | expires_at` text.
- Owners list (truncated with "…" if more than 3).
- Default value shown in mono.
- "Edit on desktop →" link deep-linked to the web app (`https://dr-sentry.com/orgs/:orgSlug/projects/:projectSlug/flags/:flagId` — or build off `window.location.origin` if a desktop-base setting lands later; for MVP hard-code the host but read it from `import.meta.env.VITE_WEB_BASE_URL` falling back to `'/'`).

**Environment sections (collapsible, one per env):**
- Header row: env name + badge for `production` if applicable + read-only "On"/"Off" pill (NOT a toggle).
- Inside the section: current default value for this env (or "(uses flag default)" if no per-env override), then a numbered list of rules whose `RuleEnvironmentState.enabled === true` for this env. Each rule row shows:
  - Priority number.
  - Rule type pill.
  - One-line summary (per the spec):
    | Type | Summary |
    |------|---------|
    | `percentage` | `<n>% rollout` |
    | `user_target` | `<n> user IDs` |
    | `attribute` | `<attribute> <operator> <value>` (truncate >40 chars) |
    | `segment` | `segment: <segment_id>` |
    | `schedule` | `<start> – <end>` |
    | `compound` | `compound (edit on desktop)` |
- A footer note: "Rules are read-only on mobile. Edit on desktop." (Phase 5 will replace with controls.)

**History section (paginated):**
- Calls `auditApi.listForFlag(flagId, { limit: 20, offset })`.
- Renders rows: timestamp · actor_name · `describeAction(entry.action)`.
- "Load more" button at bottom that increments `offset` by 20 until `entries.length < limit`.

**Data fetching:** Fan in 5 endpoints in parallel: `flagsApi.get`, `flagsApi.listRules`, `flagsApi.listRuleEnvStates`, `flagEnvStateApi.list`, `envApi.listOrg`. Audit history is fetched lazily on first render of the History section (always rendered at the bottom — no tabs on mobile; the page is a single scroll).

- [ ] **Step 1 (test):** Tests:
  - Renders header with key/name/category.
  - Renders one env section per environment.
  - Tapping an env section header expands/collapses it.
  - Rules with `RuleEnvironmentState.enabled === true` for an env render inside that env's section; rules disabled for that env do NOT.
  - Rule summary string matches expected output for each type.
  - History section: loading → 20 rows → "Load more" → 20 more rows; when fewer than 20 are returned, the button hides.
  - Compound rule renders the `compound (edit on desktop)` summary, no other detail.

- [ ] **Step 2:** Implement. Use a single `useEffect` for the parallel fan-in. Keep optimistic-update logic absent — Phase 4 does no writes.

- [ ] **Step 3:** Verify: vitest green, `npm run lint` clean, `npx tsc --noEmit` clean.

---

## Task 9: Routes + Playwright smoke + initiatives + final verify

**Files:**
- Modify: `mobile-pwa/src/App.tsx`
- Modify: `mobile-pwa/e2e/smoke.spec.ts`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1:** Wire the new routes inside the existing `RequireAuth` shell:

```tsx
<Route path="/m/orgs/:orgSlug/flags" element={<FlagProjectPickerPage />} />
<Route path="/m/orgs/:orgSlug/flags/:projectSlug" element={<FlagListPage />} />
<Route path="/m/orgs/:orgSlug/flags/:projectSlug/apps/:appSlug" element={<FlagListPage />} />
<Route path="/m/orgs/:orgSlug/flags/:projectSlug/:flagId" element={<FlagDetailPage />} />
<Route path="/m/orgs/:orgSlug/flags/:projectSlug/apps/:appSlug/:flagId" element={<FlagDetailPage />} />
```

- [ ] **Step 2:** Add a Playwright case to `smoke.spec.ts`: log in, click the Flags tab, assert at least one flag row renders (or assert the "No flags" empty state). Don't drill into detail — that requires a known seed flag, which the existing smoke fixtures don't guarantee. Mirror the structure of the existing history smoke case.

- [ ] **Step 3:** Update `docs/Current_Initiatives.md`: bump the Mobile PWA row to reflect Phase 3 merged (PR #60) and Phase 4 in flight on `feature/mobile-pwa-phase4`.

- [ ] **Step 4:** From `mobile-pwa/`, run the full local check:
  - `npm run lint`
  - `npx tsc --noEmit`
  - `npm run test --silent`
  - `npm run build`
  - (Skip `npm run test:e2e` if port 3002 is held — note in the PR.)

- [ ] **Step 5:** Commit each task as it lands; final commit at the end is a `docs,test(mobile-pwa)` housekeeping commit. Then push and open PR.

---

## Success criteria

- All 9 tasks committed in order on `feature/mobile-pwa-phase4`.
- `npm run lint` clean **before every commit** (this was raised by the user — do not skip it).
- `npx tsc --noEmit` clean.
- `npm run test --silent` green; new tests must move the count up by at least the number of new components/pages (≥ ~30 added tests, baseline 69 → expect ~100+).
- `npm run build` succeeds; SW precache regenerates without warnings.
- Playwright smoke green when run in CI (local Playwright skipped if port 3002 is held).
- `docs/Current_Initiatives.md` updated.
- PR opened with detailed body matching the Phase 3 PR style.

## Out of scope (explicit Phase 5 deferrals)

- Env enable toggle, default value editing.
- Rule enable/disable toggle, rule reorder, rule edit sheet.
- Compound rule editing.
- Flag create / archive.
- Anything that issues a non-GET request.
