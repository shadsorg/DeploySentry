# Mobile PWA — Phase 3: Deploy History Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax.

**Goal:** Replace the Phase 1 `HistoryPage` placeholder with a real deploy-history list. Status filter chip row, project picker, cursor-paginated rows, drill-down to deployment detail.

**Architecture:** Polls `GET /api/v1/orgs/:slug/deployments` once on mount and on each filter change; cursor-paginated "Load more" instead of infinite scroll (simpler, easier to test). Drill-down to `/orgs/:slug/history/:deploymentId` reads the row from `location.state` to avoid a per-row API call (Phase 1's read-only contract — no rollback/promote).

**Tech Stack:** Same as Phase 2. No new dependencies.

**Scope reminder:** Read-only. Status filter chip row + single project picker bottom-sheet. App/env/mode/date filters deferred to a later polish task.

---

## File Structure

```
mobile-pwa/
└── src/
    ├── types.ts                              # MODIFY — append Deployment + OrgDeploymentRow + filters
    ├── api.ts                                # MODIFY — add orgDeploymentsApi.list + projectsApi.list
    ├── components/
    │   ├── StatusPill.tsx                    # CREATE — colored pill for deploy status
    │   ├── StatusPill.test.tsx
    │   ├── DeploymentRow.tsx                 # CREATE — compact row for the list
    │   ├── DeploymentRow.test.tsx
    │   ├── StatusFilterChips.tsx             # CREATE — All / Pending / Running / Completed / Failed
    │   ├── StatusFilterChips.test.tsx
    │   ├── ProjectFilterSheet.tsx            # CREATE — bottom-sheet picker
    │   └── ProjectFilterSheet.test.tsx
    ├── pages/
    │   ├── HistoryPage.tsx                   # REPLACE — real page
    │   ├── HistoryPage.test.tsx
    │   ├── DeploymentDetailPage.tsx          # CREATE
    │   └── DeploymentDetailPage.test.tsx
    ├── App.tsx                               # MODIFY — add /orgs/:orgSlug/history/:deploymentId route
    └── styles/
        └── mobile.css                        # MODIFY — append history-specific classes
```

---

## Task 1: Extend types.ts with Deployment + OrgDeployment subset

**Files:**
- Modify: `mobile-pwa/src/types.ts`

- [ ] **Step 1:** Append the following block after the existing `OrgStatusResponse` interface in `mobile-pwa/src/types.ts`:

```ts
export type DeployStrategy = 'canary' | 'blue-green' | 'rolling';
export type DeployStatus =
  | 'pending'
  | 'running'
  | 'promoting'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'rolled_back'
  | 'cancelled';

export interface Deployment {
  id: string;
  application_id: string;
  environment_id: string;
  version: string;
  commit_sha?: string;
  artifact?: string;
  strategy: DeployStrategy;
  status: DeployStatus;
  mode?: 'orchestrate' | 'record';
  source?: string | null;
  traffic_percent: number;
  health_score?: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at: string | null;
}

export interface OrgDeploymentRow extends Deployment {
  application: { id: string; slug: string; name: string };
  environment: { id: string; slug?: string; name?: string };
  project: { id: string; slug: string; name: string };
}

export interface OrgDeploymentsResponse {
  deployments: OrgDeploymentRow[];
  next_cursor?: string;
}

export interface OrgDeploymentsFilters {
  project_id?: string;
  application_id?: string;
  environment_id?: string;
  status?: string;
  mode?: string;
  from?: string;
  to?: string;
  cursor?: string;
  limit?: number;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  org_id: string;
  description?: string;
}
```

- [ ] **Step 2:** Typecheck:

```bash
cd /Users/sgamel/git/DeploySentry/.worktrees/mobile-pwa-phase3/mobile-pwa && npx tsc --noEmit
```

- [ ] **Step 3:** Commit:

```bash
git add mobile-pwa/src/types.ts
git commit -m "feat(mobile-pwa): add Deployment + OrgDeploymentRow types"
```

---

## Task 2: Add orgDeploymentsApi.list and projectsApi.list (TDD)

**Files:**
- Modify: `mobile-pwa/src/api.ts`
- Modify: `mobile-pwa/src/api.test.ts` — add 2 new tests

- [ ] **Step 1:** Update the import line at the top of `mobile-pwa/src/api.test.ts` from:

```ts
import { authApi, orgsApi, orgStatusApi, setFetch } from './api';
```

to:

```ts
import { authApi, orgsApi, orgStatusApi, orgDeploymentsApi, projectsApi, setFetch } from './api';
```

Then append two new tests inside the existing `describe('api', ...)`:

```ts
it('orgDeploymentsApi.list builds query string from filters and uses Bearer token', async () => {
  localStorage.setItem('ds_token', 'header.payload.sig');
  fetchMock.mockResolvedValue(
    new Response(JSON.stringify({ deployments: [], next_cursor: 'abc' }), { status: 200 }),
  );
  const res = await orgDeploymentsApi.list('acme', { status: 'completed', limit: 25 });
  expect(res.next_cursor).toBe('abc');
  const url = fetchMock.mock.calls[0][0] as string;
  expect(url).toContain('/api/v1/orgs/acme/deployments');
  expect(url).toContain('status=completed');
  expect(url).toContain('limit=25');
  const init = fetchMock.mock.calls[0][1] as RequestInit;
  expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
});

it('orgDeploymentsApi.list omits undefined filter values from the URL', async () => {
  localStorage.setItem('ds_token', 'header.payload.sig');
  fetchMock.mockResolvedValue(new Response(JSON.stringify({ deployments: [] }), { status: 200 }));
  await orgDeploymentsApi.list('acme', { status: 'failed' });
  const url = fetchMock.mock.calls[0][0] as string;
  expect(url).toBe('/api/v1/orgs/acme/deployments?status=failed');
});

it('projectsApi.list fetches /orgs/:slug/projects', async () => {
  localStorage.setItem('ds_token', 'header.payload.sig');
  fetchMock.mockResolvedValue(
    new Response(JSON.stringify({ projects: [{ id: 'p1', slug: 'pay', name: 'Pay', org_id: 'o1' }] }), {
      status: 200,
    }),
  );
  const res = await projectsApi.list('acme');
  expect(res.projects[0].slug).toBe('pay');
  expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/orgs/acme/projects');
});
```

- [ ] **Step 2:** Run — fails (`orgDeploymentsApi`/`projectsApi` not exported).

```bash
cd mobile-pwa && npx vitest run src/api.test.ts
```

- [ ] **Step 3:** Implement in `mobile-pwa/src/api.ts`:

1. Extend the type import at the top:
   ```ts
   import type {
     AuthUser,
     Organization,
     OrgStatusResponse,
     OrgDeploymentsFilters,
     OrgDeploymentsResponse,
     Project,
   } from './types';
   ```

2. Add a small helper just before the `orgStatusApi` export (or at the bottom of the file):

   ```ts
   function buildQueryString(params: Record<string, string | number | undefined>): string {
     const parts = Object.entries(params)
       .filter(([, v]) => v !== undefined && v !== '' && v !== null)
       .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`);
     return parts.length ? `?${parts.join('&')}` : '';
   }
   ```

3. Append two new exports:

   ```ts
   export const orgDeploymentsApi = {
     list: (orgSlug: string, filters: OrgDeploymentsFilters = {}) =>
       request<OrgDeploymentsResponse>(
         `/orgs/${orgSlug}/deployments${buildQueryString(filters as Record<string, string | number | undefined>)}`,
       ),
   };

   export const projectsApi = {
     list: (orgSlug: string) =>
       request<{ projects: Project[] }>(`/orgs/${orgSlug}/projects`),
   };
   ```

- [ ] **Step 4:** Run — 8 api tests pass (5 prior + 3 new). Full suite: 49 tests across 14 files. tsc clean.

- [ ] **Step 5:** Commit:

```bash
git add mobile-pwa/src/api.ts mobile-pwa/src/api.test.ts
git commit -m "feat(mobile-pwa): add orgDeploymentsApi.list + projectsApi.list"
```

---

## Task 3: StatusPill component (TDD)

**Files:**
- Create: `mobile-pwa/src/components/StatusPill.tsx`
- Create: `mobile-pwa/src/components/StatusPill.test.tsx`
- Modify (append): `mobile-pwa/src/styles/mobile.css`

- [ ] **Step 1:** Failing test:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusPill } from './StatusPill';

describe('StatusPill', () => {
  it('renders the status label uppercased', () => {
    render(<StatusPill status="completed" />);
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
  });
  it('encodes status into data-state', () => {
    render(<StatusPill status="failed" />);
    expect(screen.getByText('FAILED')).toHaveAttribute('data-state', 'failed');
  });
  it('renders all DeployStatus values as their uppercase form', () => {
    const statuses = ['pending', 'running', 'promoting', 'paused', 'completed', 'failed', 'rolled_back', 'cancelled'] as const;
    statuses.forEach((s) => {
      const { unmount } = render(<StatusPill status={s} />);
      expect(screen.getByText(s.toUpperCase())).toBeInTheDocument();
      unmount();
    });
  });
});
```

- [ ] **Step 2:** Implement `StatusPill.tsx`:

```tsx
import type { DeployStatus } from '../types';

export function StatusPill({ status }: { status: DeployStatus }) {
  return (
    <span className="m-status-pill" data-state={status}>
      {status.toUpperCase()}
    </span>
  );
}
```

- [ ] **Step 3:** Append CSS to `mobile-pwa/src/styles/mobile.css`:

```css
/* History tab — status pill */
.m-status-pill {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  background: var(--color-bg-elevated, #1b2339);
  color: var(--color-text-muted, #64748b);
}
.m-status-pill[data-state='completed'] { background: var(--color-success-bg, rgba(16,185,129,0.12)); color: var(--color-success, #10b981); }
.m-status-pill[data-state='running'],
.m-status-pill[data-state='promoting'] { background: var(--color-info-bg, rgba(59,130,246,0.12)); color: var(--color-info, #3b82f6); }
.m-status-pill[data-state='pending'],
.m-status-pill[data-state='paused'] { background: var(--color-warning-bg, rgba(245,158,11,0.12)); color: var(--color-warning, #f59e0b); }
.m-status-pill[data-state='failed'],
.m-status-pill[data-state='rolled_back'] { background: var(--color-danger-bg, rgba(239,68,68,0.12)); color: var(--color-danger, #ef4444); }
.m-status-pill[data-state='cancelled'] { background: var(--color-bg-elevated, #1b2339); color: var(--color-text-muted, #64748b); }
```

- [ ] **Step 4:** Tests pass; full suite 52 across 15 files; tsc clean. Commit:

```bash
git add mobile-pwa/src/components/StatusPill.tsx mobile-pwa/src/components/StatusPill.test.tsx mobile-pwa/src/styles/mobile.css
git commit -m "feat(mobile-pwa): add StatusPill"
```

---

## Task 4: DeploymentRow + StatusFilterChips (TDD)

**Files:**
- Create: `mobile-pwa/src/components/DeploymentRow.tsx`
- Create: `mobile-pwa/src/components/DeploymentRow.test.tsx`
- Create: `mobile-pwa/src/components/StatusFilterChips.tsx`
- Create: `mobile-pwa/src/components/StatusFilterChips.test.tsx`
- Modify (append): `mobile-pwa/src/styles/mobile.css`

- [ ] **Step 1:** Failing `DeploymentRow.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DeploymentRow } from './DeploymentRow';
import type { OrgDeploymentRow } from '../types';

function row(partial: Partial<OrgDeploymentRow> = {}): OrgDeploymentRow {
  return {
    id: 'd1',
    application_id: 'a1',
    environment_id: 'e1',
    version: 'v2.1.0',
    strategy: 'canary',
    status: 'completed',
    traffic_percent: 100,
    created_by: 'u1',
    created_at: new Date(Date.now() - 5 * 60_000).toISOString(),
    updated_at: '',
    completed_at: null,
    application: { id: 'a1', slug: 'api', name: 'API' },
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    project: { id: 'p1', slug: 'payments', name: 'Payments' },
    ...partial,
  };
}

describe('DeploymentRow', () => {
  it('renders version, env slug, app name, status', () => {
    render(<DeploymentRow row={row()} onTap={() => {}} />);
    expect(screen.getByText('v2.1.0')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
  });

  it('renders relative age when created within the last hour', () => {
    render(<DeploymentRow row={row()} onTap={() => {}} />);
    expect(screen.getByText(/5m ago/i)).toBeInTheDocument();
  });

  it('calls onTap with the row when clicked', async () => {
    const onTap = vi.fn();
    render(<DeploymentRow row={row()} onTap={onTap} />);
    await userEvent.click(screen.getByRole('button'));
    expect(onTap).toHaveBeenCalledTimes(1);
    expect(onTap.mock.calls[0][0].id).toBe('d1');
  });
});
```

- [ ] **Step 2:** Implement:

```tsx
import type { OrgDeploymentRow } from '../types';
import { StatusPill } from './StatusPill';

function relativeAge(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms) || ms < 0) return '';
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  return `${day}d ago`;
}

export function DeploymentRow({
  row,
  onTap,
}: {
  row: OrgDeploymentRow;
  onTap: (row: OrgDeploymentRow) => void;
}) {
  return (
    <button type="button" className="m-card m-deploy-row" onClick={() => onTap(row)}>
      <div className="m-deploy-row-head">
        <span className="m-deploy-version">{row.version}</span>
        <StatusPill status={row.status} />
      </div>
      <div className="m-deploy-row-meta">
        <span className="m-app-name">{row.application.name}</span>
        <span className="m-env-chip" data-state="never" style={{ cursor: 'default' }}>
          {row.environment.slug ?? '?'}
        </span>
        <span className="m-deploy-row-age">{relativeAge(row.created_at)}</span>
      </div>
    </button>
  );
}
```

- [ ] **Step 3:** Failing `StatusFilterChips.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StatusFilterChips } from './StatusFilterChips';

describe('StatusFilterChips', () => {
  it('renders five chips: All / Pending / Running / Completed / Failed', () => {
    render(<StatusFilterChips value="" onChange={() => {}} />);
    ['All', 'Pending', 'Running', 'Completed', 'Failed'].forEach((label) => {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument();
    });
  });

  it('marks the active chip with aria-pressed=true', () => {
    render(<StatusFilterChips value="failed" onChange={() => {}} />);
    expect(screen.getByRole('button', { name: 'Failed' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('emits the canonical status string on click (empty for All)', async () => {
    const onChange = vi.fn();
    render(<StatusFilterChips value="" onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'Failed' }));
    expect(onChange).toHaveBeenCalledWith('failed');
    await userEvent.click(screen.getByRole('button', { name: 'All' }));
    expect(onChange).toHaveBeenCalledWith('');
  });
});
```

- [ ] **Step 4:** Implement:

```tsx
const CHIPS: { label: string; value: string }[] = [
  { label: 'All', value: '' },
  { label: 'Pending', value: 'pending' },
  { label: 'Running', value: 'running' },
  { label: 'Completed', value: 'completed' },
  { label: 'Failed', value: 'failed' },
];

export function StatusFilterChips({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="m-filter-chip-row" role="group" aria-label="Filter by status">
      {CHIPS.map((c) => (
        <button
          key={c.value || 'all'}
          type="button"
          className="m-filter-chip"
          aria-pressed={value === c.value}
          onClick={() => onChange(c.value)}
        >
          {c.label}
        </button>
      ))}
    </div>
  );
}
```

- [ ] **Step 5:** Append CSS:

```css
.m-deploy-row { width: 100%; padding: 12px 14px; text-align: left; cursor: pointer; display: flex; flex-direction: column; gap: 6px; color: inherit; }
.m-deploy-row-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.m-deploy-row-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 12px; color: var(--color-text-muted, #64748b); }
.m-deploy-version { font-family: var(--font-mono, monospace); font-weight: 600; }
.m-deploy-row-age { margin-left: auto; }

.m-filter-chip-row { display: flex; gap: 6px; padding: 4px 0 12px; overflow-x: auto; -webkit-overflow-scrolling: touch; }
.m-filter-chip {
  flex-shrink: 0;
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--color-border, #1e293b);
  background: var(--color-bg-elevated, #1b2339);
  color: var(--color-text-secondary, #94a3b8);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  min-height: 32px;
}
.m-filter-chip[aria-pressed='true'] {
  background: var(--color-primary, #6366f1);
  border-color: var(--color-primary, #6366f1);
  color: #fff;
}
```

- [ ] **Step 6:** Both test files pass (3 + 3 = 6 new). Full suite 58 across 17 files. tsc clean. Commit:

```bash
git add mobile-pwa/src/components/ mobile-pwa/src/styles/mobile.css
git commit -m "feat(mobile-pwa): add DeploymentRow and StatusFilterChips"
```

---

## Task 5: ProjectFilterSheet (TDD)

**Files:**
- Create: `mobile-pwa/src/components/ProjectFilterSheet.tsx`
- Create: `mobile-pwa/src/components/ProjectFilterSheet.test.tsx`

Bottom-sheet picker that lists projects + an "All projects" option. Props: `open`, `projects`, `value` (selected project id, `''` for all), `onSelect(id)`, `onClose`.

- [ ] **Step 1:** Failing test:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProjectFilterSheet } from './ProjectFilterSheet';
import type { Project } from '../types';

const PROJECTS: Project[] = [
  { id: 'p1', slug: 'pay', name: 'Payments', org_id: 'o1' },
  { id: 'p2', slug: 'web', name: 'Website', org_id: 'o1' },
];

describe('ProjectFilterSheet', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <ProjectFilterSheet open={false} projects={PROJECTS} value="" onSelect={() => {}} onClose={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders All projects + each project name when open', () => {
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="" onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByRole('button', { name: /all projects/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Payments' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Website' })).toBeInTheDocument();
  });

  it('marks the selected project with aria-pressed=true', () => {
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="p2" onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByRole('button', { name: 'Website' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('calls onSelect + onClose when a project is tapped', async () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="" onSelect={onSelect} onClose={onClose} />,
    );
    await userEvent.click(screen.getByRole('button', { name: 'Payments' }));
    expect(onSelect).toHaveBeenCalledWith('p1');
    expect(onClose).toHaveBeenCalled();
  });

  it('All projects emits empty string', async () => {
    const onSelect = vi.fn();
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="p1" onSelect={onSelect} onClose={() => {}} />,
    );
    await userEvent.click(screen.getByRole('button', { name: /all projects/i }));
    expect(onSelect).toHaveBeenCalledWith('');
  });
});
```

- [ ] **Step 2:** Implement:

```tsx
import type { Project } from '../types';

export function ProjectFilterSheet({
  open,
  projects,
  value,
  onSelect,
  onClose,
}: {
  open: boolean;
  projects: Project[];
  value: string;
  onSelect: (projectId: string) => void;
  onClose: () => void;
}) {
  if (!open) return null;

  const choose = (id: string) => {
    onSelect(id);
    onClose();
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Filter by project"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'flex-end',
        zIndex: 900,
      }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--color-bg-elevated, #1b2339)',
          border: '1px solid var(--color-border, #1e293b)',
          borderRadius: '16px 16px 0 0',
          padding: '16px 20px',
          width: '100%',
          maxHeight: '70vh',
          overflowY: 'auto',
          paddingBottom: 'calc(env(safe-area-inset-bottom) + 16px)',
        }}
      >
        <h3 style={{ margin: '0 0 12px' }}>Project</h3>
        <button
          type="button"
          className="m-button"
          aria-pressed={value === ''}
          onClick={() => choose('')}
          style={{ width: '100%', justifyContent: 'flex-start', marginBottom: 8 }}
        >
          All projects
        </button>
        {projects.map((p) => (
          <button
            key={p.id}
            type="button"
            className="m-button"
            aria-pressed={value === p.id}
            onClick={() => choose(p.id)}
            style={{ width: '100%', justifyContent: 'flex-start', marginBottom: 6 }}
          >
            {p.name}
          </button>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 3:** 5 tests pass. Full suite 63 across 18 files. Commit:

```bash
git add mobile-pwa/src/components/
git commit -m "feat(mobile-pwa): add ProjectFilterSheet"
```

---

## Task 6: HistoryPage (TDD) — replace Phase 1 placeholder

**Files:**
- Replace: `mobile-pwa/src/pages/HistoryPage.tsx`
- Create: `mobile-pwa/src/pages/HistoryPage.test.tsx`

- [ ] **Step 1:** Failing test:

```tsx
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { HistoryPage } from './HistoryPage';
import { setFetch } from '../api';

function rowFixture(partial: Record<string, unknown> = {}) {
  return {
    id: 'd1',
    application_id: 'a1',
    environment_id: 'e1',
    version: 'v2.1.0',
    strategy: 'canary',
    status: 'completed',
    traffic_percent: 100,
    created_by: 'u1',
    created_at: new Date(Date.now() - 5 * 60_000).toISOString(),
    updated_at: '',
    completed_at: null,
    application: { id: 'a1', slug: 'api', name: 'API' },
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    project: { id: 'p1', slug: 'pay', name: 'Payments' },
    ...partial,
  };
}

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

describe('HistoryPage', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.clear();
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  function renderAt(path: string) {
    return render(
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/orgs/:orgSlug/history" element={<HistoryPage />} />
          <Route path="/orgs/:orgSlug/history/:deploymentId" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('renders deployment rows from the response', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [rowFixture()] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    expect(await screen.findByText('v2.1.0')).toBeInTheDocument();
  });

  it('renders an empty state when zero deployments', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    expect(await screen.findByText(/no deployments/i)).toBeInTheDocument();
  });

  it('navigates to /history/:id when a row is tapped', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [rowFixture()] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    const row = await screen.findByText('v2.1.0');
    await userEvent.click(row);
    await waitFor(() =>
      expect(screen.getByTestId('loc').textContent).toBe('/orgs/acme/history/d1'),
    );
  });

  it('refetches with status=failed when the Failed chip is tapped', async () => {
    fetchMock.mockImplementation(async (url) => {
      const u = String(url);
      if (u.includes('/projects')) return new Response(JSON.stringify({ projects: [] }), { status: 200 });
      return new Response(JSON.stringify({ deployments: [] }), { status: 200 });
    });
    renderAt('/orgs/acme/history');
    await screen.findByText(/no deployments/i);
    await userEvent.click(screen.getByRole('button', { name: 'Failed' }));
    await waitFor(() => {
      const calls = fetchMock.mock.calls.map((c) => String(c[0]));
      expect(calls.some((u) => u.includes('/orgs/acme/deployments') && u.includes('status=failed'))).toBe(true);
    });
  });
});
```

- [ ] **Step 2:** Replace `mobile-pwa/src/pages/HistoryPage.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { orgDeploymentsApi, projectsApi } from '../api';
import type { OrgDeploymentRow, Project } from '../types';
import { DeploymentRow } from '../components/DeploymentRow';
import { StatusFilterChips } from '../components/StatusFilterChips';
import { ProjectFilterSheet } from '../components/ProjectFilterSheet';

const PAGE_SIZE = 25;

export function HistoryPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const nav = useNavigate();

  const [status, setStatus] = useState('');
  const [projectId, setProjectId] = useState('');
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectSheetOpen, setProjectSheetOpen] = useState(false);
  const [rows, setRows] = useState<OrgDeploymentRow[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load project list once.
  useEffect(() => {
    if (!orgSlug) return;
    projectsApi
      .list(orgSlug)
      .then((r) => setProjects(r.projects))
      .catch(() => {
        // non-fatal: filter just doesn't show projects
      });
  }, [orgSlug]);

  const fetchPage = useCallback(
    async (reset: boolean) => {
      if (!orgSlug) return;
      const isInitial = reset;
      if (isInitial) {
        setLoading(true);
      } else {
        setLoadingMore(true);
      }
      try {
        const res = await orgDeploymentsApi.list(orgSlug, {
          status: status || undefined,
          project_id: projectId || undefined,
          limit: PAGE_SIZE,
          cursor: isInitial ? undefined : cursor,
        });
        setRows((prev) => (isInitial ? res.deployments : [...prev, ...res.deployments]));
        setCursor(res.next_cursor);
        setError(null);
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load deployments');
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [orgSlug, status, projectId, cursor],
  );

  // Refetch from scratch whenever the filters change.
  useEffect(() => {
    void fetchPage(true);
    // intentionally exclude `cursor` so a Load more click doesn't loop
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgSlug, status, projectId]);

  const projectName =
    projectId === '' ? 'All projects' : projects.find((p) => p.id === projectId)?.name ?? 'Project';

  return (
    <section>
      <h2 style={{ margin: '4px 0 8px' }}>Deploy History</h2>

      <StatusFilterChips value={status} onChange={setStatus} />

      <button
        type="button"
        className="m-button"
        style={{ width: '100%', justifyContent: 'space-between', marginBottom: 12 }}
        onClick={() => setProjectSheetOpen(true)}
      >
        <span style={{ color: 'var(--color-text-muted, #64748b)' }}>Project</span>
        <span>{projectName}</span>
      </button>

      <ProjectFilterSheet
        open={projectSheetOpen}
        projects={projects}
        value={projectId}
        onSelect={setProjectId}
        onClose={() => setProjectSheetOpen(false)}
      />

      {error && (
        <p style={{ color: 'var(--color-danger, #ef4444)', fontSize: 13 }}>{error}</p>
      )}

      {loading && rows.length === 0 ? (
        <div className="m-page-loading" style={{ height: 'auto', padding: 24 }}>Loading…</div>
      ) : rows.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>No deployments match these filters.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {rows.map((r) => (
            <DeploymentRow
              key={r.id}
              row={r}
              onTap={(row) =>
                nav(`/orgs/${orgSlug}/history/${row.id}`, { state: { row } })
              }
            />
          ))}
          {cursor ? (
            <button
              type="button"
              className="m-button"
              style={{ width: '100%' }}
              disabled={loadingMore}
              onClick={() => void fetchPage(false)}
            >
              {loadingMore ? 'Loading…' : 'Load older'}
            </button>
          ) : null}
        </div>
      )}
    </section>
  );
}
```

- [ ] **Step 3:** Run — 4 HistoryPage tests pass. Full suite 67 across 19 files. tsc clean.

- [ ] **Step 4:** Commit:

```bash
git add mobile-pwa/src/pages/HistoryPage.tsx mobile-pwa/src/pages/HistoryPage.test.tsx
git commit -m "feat(mobile-pwa): implement HistoryPage with status + project filters"
```

---

## Task 7: DeploymentDetailPage (TDD) + route wiring

**Files:**
- Create: `mobile-pwa/src/pages/DeploymentDetailPage.tsx`
- Create: `mobile-pwa/src/pages/DeploymentDetailPage.test.tsx`
- Modify: `mobile-pwa/src/App.tsx` — add the new route

- [ ] **Step 1:** Failing test:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { DeploymentDetailPage } from './DeploymentDetailPage';
import type { OrgDeploymentRow } from '../types';

function row(): OrgDeploymentRow {
  return {
    id: 'd1',
    application_id: 'a1',
    environment_id: 'e1',
    version: 'v2.1.0',
    commit_sha: 'abc1234',
    strategy: 'canary',
    status: 'completed',
    mode: 'orchestrate',
    source: 'github-actions',
    traffic_percent: 100,
    created_by: 'u1',
    created_at: '2026-04-26T12:00:00Z',
    updated_at: '2026-04-26T12:05:00Z',
    started_at: '2026-04-26T12:01:00Z',
    completed_at: '2026-04-26T12:05:00Z',
    application: { id: 'a1', slug: 'api', name: 'API' },
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    project: { id: 'p1', slug: 'pay', name: 'Payments' },
  };
}

describe('DeploymentDetailPage', () => {
  it('renders all key fields when state is provided', () => {
    render(
      <MemoryRouter
        initialEntries={[{ pathname: '/orgs/acme/history/d1', state: { row: row() } }]}
      >
        <Routes>
          <Route path="/orgs/:orgSlug/history/:deploymentId" element={<DeploymentDetailPage />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText('v2.1.0')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED')).toBeInTheDocument();
    expect(screen.getByText(/abc1234/i)).toBeInTheDocument();
    expect(screen.getByText('Payments')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText(/orchestrate/i)).toBeInTheDocument();
    expect(screen.getByText(/github-actions/i)).toBeInTheDocument();
  });

  it('falls back gracefully when no row is in state', () => {
    render(
      <MemoryRouter initialEntries={['/orgs/acme/history/d1']}>
        <Routes>
          <Route path="/orgs/:orgSlug/history/:deploymentId" element={<DeploymentDetailPage />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText(/return to history/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2:** Implement:

```tsx
import { Link, useLocation, useParams } from 'react-router-dom';
import type { OrgDeploymentRow } from '../types';
import { StatusPill } from '../components/StatusPill';

interface LocationState {
  row?: OrgDeploymentRow;
}

function fmt(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString();
}

export function DeploymentDetailPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const loc = useLocation();
  const row = (loc.state as LocationState | null)?.row;

  if (!row) {
    return (
      <section>
        <h2>Deployment</h2>
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>
          Detail not available — open this deployment from the history list.
        </p>
        <Link to={`/orgs/${orgSlug}/history`} className="m-button" style={{ width: '100%' }}>
          Return to history
        </Link>
      </section>
    );
  }

  return (
    <section>
      <header style={{ marginBottom: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <h2 style={{ margin: 0, fontFamily: 'var(--font-mono, monospace)' }}>{row.version}</h2>
          <StatusPill status={row.status} />
        </div>
        {row.commit_sha ? (
          <p style={{ margin: '4px 0 0', color: 'var(--color-text-muted, #64748b)', fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}>
            {row.commit_sha.slice(0, 7)}
          </p>
        ) : null}
      </header>

      <div className="m-card" style={{ marginBottom: 12 }}>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Project</span>
          <span>{row.project.name}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Application</span>
          <span>{row.application.name}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Environment</span>
          <span>{row.environment.slug ?? row.environment.name ?? '—'}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Strategy</span>
          <span>{row.strategy}</span>
        </div>
        {row.mode ? (
          <div className="m-list-row">
            <span style={{ color: 'var(--color-text-muted)' }}>Mode</span>
            <span>{row.mode}</span>
          </div>
        ) : null}
        {row.source ? (
          <div className="m-list-row">
            <span style={{ color: 'var(--color-text-muted)' }}>Source</span>
            <span>{row.source}</span>
          </div>
        ) : null}
      </div>

      <div className="m-card">
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Created</span>
          <span>{fmt(row.created_at)}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Started</span>
          <span>{fmt(row.started_at)}</span>
        </div>
        <div className="m-list-row">
          <span style={{ color: 'var(--color-text-muted)' }}>Completed</span>
          <span>{fmt(row.completed_at)}</span>
        </div>
      </div>

      <p style={{ color: 'var(--color-text-muted, #64748b)', fontSize: 12, marginTop: 16 }}>
        Phase timeline + rollback / promote ship in a later phase. Open in the desktop dashboard
        for full controls.
      </p>
    </section>
  );
}
```

- [ ] **Step 3:** Update `mobile-pwa/src/App.tsx`. Inside the existing `<Route path="/orgs/:orgSlug" element={<MobileLayout />}>` block, **add** a new route AFTER the existing `<Route path="history" .../>` line:

Find:
```tsx
<Route path="history" element={<HistoryPage />} />
```

Add immediately after:
```tsx
<Route path="history/:deploymentId" element={<DeploymentDetailPage />} />
```

And add the import at the top:
```tsx
import { DeploymentDetailPage } from './pages/DeploymentDetailPage';
```

- [ ] **Step 4:** Run — 2 DeploymentDetailPage tests pass + existing 27 App tests still pass. Full suite 69 across 20 files. tsc clean.

- [ ] **Step 5:** Commit:

```bash
git add mobile-pwa/src/pages/DeploymentDetailPage.tsx mobile-pwa/src/pages/DeploymentDetailPage.test.tsx mobile-pwa/src/App.tsx
git commit -m "feat(mobile-pwa): add DeploymentDetailPage + /history/:id route"
```

---

## Task 8: Initiatives + Playwright + final verify + push

**Files:**
- Modify: `docs/Current_Initiatives.md`
- Modify: `mobile-pwa/e2e/smoke.spec.ts`

- [ ] **Step 1:** Update the Mobile PWA row in `docs/Current_Initiatives.md` to reference the Phase 3 plan and reflect Phase 3 in flight:

Append `/ [Phase 3 Plan](./superpowers/plans/2026-04-27-mobile-pwa-phase3-history-tab.md)` to the link column. Update the Notes column to note Phase 3 in flight, Phases 4-6 pending.

Also update `Last updated:` to `2026-04-27`.

- [ ] **Step 2:** Append a Playwright smoke for History to `mobile-pwa/e2e/smoke.spec.ts`:

```ts
test('authenticated history page smoke (API mocked at browser level)', async ({ page, context }) => {
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (r) =>
    r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }) }),
  );
  await context.route('**/api/v1/orgs/acme/projects', (r) =>
    r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ projects: [] }) }),
  );
  await context.route('**/api/v1/orgs/acme/deployments**', (r) =>
    r.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        deployments: [
          {
            id: 'd1',
            application_id: 'a1',
            environment_id: 'e1',
            version: 'v9.9.9',
            strategy: 'canary',
            status: 'completed',
            traffic_percent: 100,
            created_by: 'u1',
            created_at: new Date(Date.now() - 60_000).toISOString(),
            updated_at: '',
            completed_at: null,
            application: { id: 'a1', slug: 'api', name: 'API' },
            environment: { id: 'e1', slug: 'prod', name: 'Production' },
            project: { id: 'p1', slug: 'pay', name: 'Payments' },
          },
        ],
      }),
    }),
  );
  await page.goto('/m/orgs/acme/history');
  await page.getByText('v9.9.9').waitFor({ state: 'visible' });
  await page.getByRole('button', { name: 'Failed' }).click();
});
```

- [ ] **Step 3:** Final gates:

```bash
cd /Users/sgamel/git/DeploySentry/.worktrees/mobile-pwa-phase3/mobile-pwa
npm run lint
npx tsc --noEmit
npm run test
npm run build
npm run test:e2e
```

Expected: lint 0 warn; tsc 0; vitest 69 passing across 20 files; build OK; playwright 4 passed + 1 skipped.

- [ ] **Step 4:** Commit + push:

```bash
cd /Users/sgamel/git/DeploySentry/.worktrees/mobile-pwa-phase3
git add docs/Current_Initiatives.md mobile-pwa/e2e/smoke.spec.ts
git commit -m "docs,test(mobile-pwa): track phase 3 + add history smoke"
git push -u origin feature/mobile-pwa-phase3
```

---

## Success criteria

- 8 commits on `feature/mobile-pwa-phase3`.
- 69 unit tests passing (Phase 2 baseline 46 + 23 new: 3 api + 3 StatusPill + 3 DeploymentRow + 3 StatusFilterChips + 5 ProjectFilterSheet + 4 HistoryPage + 2 DeploymentDetailPage).
- 4 Playwright smoke tests + 1 skipped.
- All quality gates green.

## Out of scope

- App / environment / mode / date-range filters (deferred — status + project covers ~80% of mobile use cases).
- Phase timeline on detail page (would need a new endpoint).
- Rollback / promote action buttons.
- Pull-to-refresh.
- Infinite-scroll auto-trigger (manual "Load older" button is the v1 UX).
- Direct deep-links to `/history/:id` from outside the app (needs a single-deployment GET endpoint; today the page reads from `location.state` and falls back gracefully).
