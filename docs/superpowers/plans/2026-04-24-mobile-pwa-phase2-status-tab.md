# Mobile PWA — Phase 2: Status Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox syntax for tracking.

**Goal:** Replace the Phase 1 StatusPage placeholder with a real mobile-optimized fan-in status view: collapsible project cards, app rows with env-chip strips, monitoring-link icons, and 15-second auto-polling that pauses when the page is hidden.

**Architecture:** Polls `GET /api/v1/orgs/:slug/status` every 15 s (paused via Page Visibility API). Data arrives as `OrgStatusResponse` and renders into project → app → env-chip structure. No new backend — reuses the endpoint already consumed by `web/src/pages/OrgStatusPage.tsx`.

**Tech Stack:** Same as Phase 1 (Vite + React + TS + vitest + @testing-library/react). No new dependencies.

**Scope reminder:** Read-only status surface. No edits, no rollback buttons, no drill-down yet. Tapping an env chip will use `window.alert("Deployment detail coming in Phase 3")` as a temporary landing target — Phase 3 (History tab) wires up the real `history/:deploymentId` route.

---

## File Structure

```
mobile-pwa/
└── src/
    ├── types.ts                          # MODIFY — append OrgStatus types
    ├── api.ts                            # MODIFY — add orgStatusApi.get
    ├── hooks/
    │   └── useAutoPoll.ts                # CREATE — 15 s poll + visibility pause
    │   └── useAutoPoll.test.ts
    ├── components/
    │   ├── EnvChip.tsx                   # CREATE
    │   ├── EnvChip.test.tsx
    │   ├── HealthPill.tsx                # CREATE — aggregate project health
    │   ├── HealthPill.test.tsx
    │   ├── MonitoringLinkIcon.tsx        # CREATE
    │   └── ProjectCard.tsx               # CREATE — collapsible, embeds apps
    │       ProjectCard.test.tsx
    ├── pages/
    │   ├── StatusPage.tsx                # REPLACE — real page
    │   └── StatusPage.test.tsx           # CREATE — renders fan-in response
    └── styles/
        └── mobile.css                    # MODIFY — append status-specific classes
```

`App.tsx` and `main.tsx` need no changes — the Phase 1 route `/orgs/:orgSlug/status` already mounts `StatusPage`.

---

## Task 1: Extend types.ts with OrgStatus subset

**Files:**
- Modify: `mobile-pwa/src/types.ts`

- [ ] **Step 1:** Append the following to `mobile-pwa/src/types.ts` (after the existing `Organization` interface). This is a direct subset of `web/src/types.ts:220-271`.

```ts
export interface MonitoringLink {
  label: string;
  url: string;
  icon?: string;
}

export type HealthState = 'healthy' | 'degraded' | 'unhealthy' | 'unknown';
export type HealthStaleness = 'fresh' | 'stale' | 'missing';

export interface OrgStatusHealthBlock {
  state: HealthState;
  score?: number | null;
  reason?: string;
  source: string;
  last_reported_at?: string | null;
  staleness: HealthStaleness;
}

export interface OrgStatusDeploymentMini {
  id: string;
  version: string;
  commit_sha?: string;
  status: string;
  mode: string;
  source?: string | null;
  completed_at?: string | null;
}

export interface OrgStatusEnvCell {
  environment: { id: string; slug?: string; name?: string };
  current_deployment?: OrgStatusDeploymentMini | null;
  health: OrgStatusHealthBlock;
  never_deployed: boolean;
}

export interface OrgStatusApplicationNode {
  application: {
    id: string;
    slug: string;
    name: string;
    monitoring_links?: MonitoringLink[] | null;
  };
  environments: OrgStatusEnvCell[];
}

export interface OrgStatusProjectNode {
  project: { id: string; slug: string; name: string };
  aggregate_health: HealthState;
  applications: OrgStatusApplicationNode[];
}

export interface OrgStatusResponse {
  org: { id: string; slug: string; name: string };
  generated_at: string;
  projects: OrgStatusProjectNode[];
}
```

- [ ] **Step 2:** Typecheck.

```bash
cd /Users/sgamel/git/DeploySentry/.worktrees/mobile-pwa-phase2/mobile-pwa && npx tsc --noEmit
```

Expected: exit 0.

- [ ] **Step 3:** Commit.

```bash
git add mobile-pwa/src/types.ts
git commit -m "feat(mobile-pwa): add OrgStatus types"
```

---

## Task 2: Add orgStatusApi.get (TDD)

**Files:**
- Modify: `mobile-pwa/src/api.ts`
- Modify: `mobile-pwa/src/api.test.ts` — add one new test

- [ ] **Step 1:** Write the failing test. Append a new `it(...)` block inside the existing `describe('api', ...)` in `mobile-pwa/src/api.test.ts`:

```ts
it('orgStatusApi.get fetches /orgs/:slug/status with Bearer token', async () => {
  localStorage.setItem('ds_token', 'header.payload.sig');
  const fixture = {
    org: { id: '1', slug: 'acme', name: 'Acme' },
    generated_at: '2026-04-24T00:00:00Z',
    projects: [],
  };
  fetchMock.mockResolvedValue(new Response(JSON.stringify(fixture), { status: 200 }));
  const { orgStatusApi } = await import('./api');
  const res = await orgStatusApi.get('acme');
  expect(res.org.slug).toBe('acme');
  expect(fetchMock).toHaveBeenCalledWith(
    '/api/v1/orgs/acme/status',
    expect.objectContaining({ method: undefined }),
  );
});
```

Actually, `request()` calls `fetchImpl(path, { ...init, headers })` — `method` is `undefined` by default for GET (no `init.method` passed). The dynamic `import` is a workaround so the already-imported `authApi`/`orgsApi` bindings aren't shadowed; you can instead hoist `orgStatusApi` into the existing top-of-file import once it exists.

**Simpler version** (recommended): Replace the existing top `import { authApi, orgsApi, setFetch } from './api'` with `import { authApi, orgsApi, orgStatusApi, setFetch } from './api'` — TypeScript will fail until `orgStatusApi` is exported. Then your test reads:

```ts
it('orgStatusApi.get fetches /orgs/:slug/status with Bearer token', async () => {
  localStorage.setItem('ds_token', 'header.payload.sig');
  fetchMock.mockResolvedValue(
    new Response(
      JSON.stringify({
        org: { id: '1', slug: 'acme', name: 'Acme' },
        generated_at: '2026-04-24T00:00:00Z',
        projects: [],
      }),
      { status: 200 },
    ),
  );
  const res = await orgStatusApi.get('acme');
  expect(res.org.slug).toBe('acme');
  expect(fetchMock).toHaveBeenCalledWith(
    '/api/v1/orgs/acme/status',
    expect.objectContaining({}),
  );
  const init = fetchMock.mock.calls[0][1] as RequestInit;
  expect((init.headers as Record<string, string>).Authorization).toBe('Bearer header.payload.sig');
});
```

- [ ] **Step 2:** Run — fails (orgStatusApi not exported).

```bash
cd mobile-pwa && npx vitest run src/api.test.ts
```

Expected: compile error on import OR "orgStatusApi is not a function".

- [ ] **Step 3:** Add the export to `mobile-pwa/src/api.ts`. Append at the end (after `orgsApi`):

```ts
import type { OrgStatusResponse } from './types';

// (Keep the import at the top of the file with the existing type imports —
// don't create a second import block. Just add `OrgStatusResponse` to the
// existing `import type { AuthUser, Organization } from './types';` line so
// it becomes:
//   import type { AuthUser, Organization, OrgStatusResponse } from './types';
// )

export const orgStatusApi = {
  get: (orgSlug: string) => request<OrgStatusResponse>(`/orgs/${orgSlug}/status`),
};
```

- [ ] **Step 4:** Run — 5 api tests pass (4 prior + 1 new).

- [ ] **Step 5:** Full suite + typecheck.

```bash
cd mobile-pwa && npx vitest run && npx tsc --noEmit
```

Expected: 28 tests pass across 9 files. tsc exit 0.

- [ ] **Step 6:** Commit.

```bash
git add mobile-pwa/src/api.ts mobile-pwa/src/api.test.ts
git commit -m "feat(mobile-pwa): add orgStatusApi.get"
```

---

## Task 3: useAutoPoll hook (TDD)

**Files:**
- Create: `mobile-pwa/src/hooks/useAutoPoll.ts`
- Test: `mobile-pwa/src/hooks/useAutoPoll.test.ts`

Polls a function every N ms, pauses when `document.visibilityState === 'hidden'`, restarts on visibilitychange.

- [ ] **Step 1:** Write the failing test.

`mobile-pwa/src/hooks/useAutoPoll.test.ts`:

```ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAutoPoll } from './useAutoPoll';

describe('useAutoPoll', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
  });

  it('calls the tick function immediately on mount', () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    expect(tick).toHaveBeenCalledTimes(1);
  });

  it('calls tick on the interval while visible', async () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    expect(tick).toHaveBeenCalledTimes(1);
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    expect(tick).toHaveBeenCalledTimes(2);
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    expect(tick).toHaveBeenCalledTimes(3);
  });

  it('pauses ticking when the page becomes hidden', async () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    expect(tick).toHaveBeenCalledTimes(1);
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(tick).toHaveBeenCalledTimes(1);
  });

  it('resumes + immediately ticks when visibility returns', async () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    tick.mockClear();
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    expect(tick).toHaveBeenCalledTimes(1);
  });
});
```

- [ ] **Step 2:** Run — fails (module not found).

- [ ] **Step 3:** Implement.

`mobile-pwa/src/hooks/useAutoPoll.ts`:

```ts
import { useEffect, useRef } from 'react';

/**
 * Calls tick() immediately, then every intervalMs while the page is visible.
 * Pauses on document.visibilitychange → hidden; resumes (with an immediate
 * tick) on → visible. Cleans up on unmount.
 */
export function useAutoPoll(tick: () => void, intervalMs: number): void {
  const tickRef = useRef(tick);
  tickRef.current = tick;

  useEffect(() => {
    let timer: number | null = null;

    const schedule = () => {
      if (timer !== null) return;
      timer = window.setInterval(() => tickRef.current(), intervalMs);
    };
    const stop = () => {
      if (timer === null) return;
      window.clearInterval(timer);
      timer = null;
    };

    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        stop();
      } else {
        tickRef.current();
        schedule();
      }
    };

    tickRef.current();
    if (document.visibilityState !== 'hidden') {
      schedule();
    }
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      stop();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [intervalMs]);
}
```

- [ ] **Step 4:** Run — 4 tests pass.

- [ ] **Step 5:** Full suite + typecheck.

Expected: 32 tests passing. tsc exit 0.

- [ ] **Step 6:** Commit.

```bash
git add mobile-pwa/src/hooks/
git commit -m "feat(mobile-pwa): add useAutoPoll hook with visibility-pause"
```

---

## Task 4: HealthPill + EnvChip components (TDD)

**Files:**
- Create: `mobile-pwa/src/components/HealthPill.tsx`
- Create: `mobile-pwa/src/components/HealthPill.test.tsx`
- Create: `mobile-pwa/src/components/EnvChip.tsx`
- Create: `mobile-pwa/src/components/EnvChip.test.tsx`

- [ ] **Step 1:** Write failing `HealthPill.test.tsx`.

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { HealthPill } from './HealthPill';

describe('HealthPill', () => {
  it('renders HEALTHY for healthy state', () => {
    render(<HealthPill state="healthy" />);
    expect(screen.getByText(/healthy/i)).toBeInTheDocument();
  });
  it('renders UNHEALTHY for unhealthy state', () => {
    render(<HealthPill state="unhealthy" />);
    expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
  });
  it('renders UNKNOWN for unknown state', () => {
    render(<HealthPill state="unknown" />);
    expect(screen.getByText(/unknown/i)).toBeInTheDocument();
  });
  it('applies data-state attribute for CSS hooks', () => {
    render(<HealthPill state="degraded" />);
    expect(screen.getByText(/degraded/i)).toHaveAttribute('data-state', 'degraded');
  });
});
```

- [ ] **Step 2:** Implement.

```tsx
import type { HealthState } from '../types';

const LABELS: Record<HealthState, string> = {
  healthy: 'HEALTHY',
  degraded: 'DEGRADED',
  unhealthy: 'UNHEALTHY',
  unknown: 'UNKNOWN',
};

export function HealthPill({ state }: { state: HealthState }) {
  return (
    <span className={`m-health-pill m-health-pill-${state}`} data-state={state}>
      {LABELS[state]}
    </span>
  );
}
```

- [ ] **Step 3:** Write failing `EnvChip.test.tsx`.

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EnvChip } from './EnvChip';
import type { OrgStatusEnvCell } from '../types';

function cell(partial: Partial<OrgStatusEnvCell> = {}): OrgStatusEnvCell {
  return {
    environment: { id: 'e1', slug: 'prod', name: 'Production' },
    current_deployment: null,
    health: { state: 'healthy', source: 'agent', staleness: 'fresh' },
    never_deployed: false,
    ...partial,
  };
}

describe('EnvChip', () => {
  it('renders the env slug', () => {
    render(<EnvChip cell={cell()} onTap={() => {}} />);
    expect(screen.getByText('prod')).toBeInTheDocument();
  });
  it('shows the never-deployed state when applicable', () => {
    render(<EnvChip cell={cell({ never_deployed: true })} onTap={() => {}} />);
    const chip = screen.getByText(/prod/);
    expect(chip).toHaveAttribute('data-state', 'never');
  });
  it('encodes the health state into data-state', () => {
    render(<EnvChip cell={cell({ health: { state: 'unhealthy', source: 'agent', staleness: 'fresh' } })} onTap={() => {}} />);
    expect(screen.getByText('prod')).toHaveAttribute('data-state', 'unhealthy');
  });
  it('calls onTap when clicked', async () => {
    const onTap = vi.fn();
    render(<EnvChip cell={cell()} onTap={onTap} />);
    await userEvent.click(screen.getByText('prod'));
    expect(onTap).toHaveBeenCalledTimes(1);
  });
});
```

- [ ] **Step 4:** Implement `EnvChip.tsx`.

```tsx
import type { OrgStatusEnvCell } from '../types';

export function EnvChip({ cell, onTap }: { cell: OrgStatusEnvCell; onTap: (cell: OrgStatusEnvCell) => void }) {
  const slug = cell.environment.slug ?? '?';
  const dataState = cell.never_deployed ? 'never' : cell.health.state;
  const stale = !cell.never_deployed && cell.health.staleness === 'stale';
  return (
    <button
      type="button"
      className="m-env-chip"
      data-state={dataState}
      data-stale={stale || undefined}
      onClick={() => onTap(cell)}
    >
      {slug}
    </button>
  );
}
```

- [ ] **Step 5:** Run tests — 4 HealthPill + 4 EnvChip pass.

- [ ] **Step 6:** Append these styles to `mobile-pwa/src/styles/mobile.css`:

```css
/* Status tab -- pills and chips */
.m-health-pill {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  background: var(--color-bg-elevated, #1b2339);
  color: var(--color-text-muted, #64748b);
}
.m-health-pill-healthy { background: var(--color-success-bg, rgba(16,185,129,0.12)); color: var(--color-success, #10b981); }
.m-health-pill-degraded { background: var(--color-warning-bg, rgba(245,158,11,0.12)); color: var(--color-warning, #f59e0b); }
.m-health-pill-unhealthy { background: var(--color-danger-bg, rgba(239,68,68,0.12)); color: var(--color-danger, #ef4444); }

.m-env-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: lowercase;
  border: 1px solid var(--color-border, #1e293b);
  background: var(--color-bg-elevated, #1b2339);
  color: var(--color-text, #e2e8f0);
  cursor: pointer;
  min-height: 28px;
}
.m-env-chip[data-state='healthy'] { color: var(--color-success, #10b981); border-color: var(--color-success, #10b981); }
.m-env-chip[data-state='degraded'] { color: var(--color-warning, #f59e0b); border-color: var(--color-warning, #f59e0b); }
.m-env-chip[data-state='unhealthy'] { color: var(--color-danger, #ef4444); border-color: var(--color-danger, #ef4444); }
.m-env-chip[data-state='never'] { color: var(--color-text-muted, #64748b); opacity: 0.6; }
.m-env-chip[data-stale='true'] { opacity: 0.65; text-decoration: line-through dotted; }

.m-env-chip-row { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
```

- [ ] **Step 7:** Full suite + typecheck. 40 tests passing. tsc clean.

- [ ] **Step 8:** Commit.

```bash
git add mobile-pwa/src/components/ mobile-pwa/src/styles/mobile.css
git commit -m "feat(mobile-pwa): add HealthPill and EnvChip components"
```

---

## Task 5: MonitoringLinkIcon + ProjectCard components (TDD on ProjectCard)

**Files:**
- Create: `mobile-pwa/src/components/MonitoringLinkIcon.tsx`
- Create: `mobile-pwa/src/components/ProjectCard.tsx`
- Create: `mobile-pwa/src/components/ProjectCard.test.tsx`
- Modify: `mobile-pwa/src/styles/mobile.css` (append)

- [ ] **Step 1:** Write `MonitoringLinkIcon.tsx` (no test — it's a single tiny leaf).

```tsx
import type { MonitoringLink } from '../types';

function glyph(icon?: string): string {
  switch (icon) {
    case 'grafana':
      return '📊';
    case 'sentry':
      return '🛡';
    case 'datadog':
      return '🐶';
    case 'pagerduty':
      return '🚨';
    case 'slack':
      return '💬';
    default:
      return '↗';
  }
}

export function MonitoringLinkIcon({ link }: { link: MonitoringLink }) {
  return (
    <a
      href={link.url}
      target="_blank"
      rel="noopener noreferrer"
      className="m-monitor-link"
      title={link.label}
      aria-label={link.label}
      onClick={(e) => e.stopPropagation()}
    >
      {glyph(link.icon)}
    </a>
  );
}
```

- [ ] **Step 2:** Write failing `ProjectCard.test.tsx`.

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProjectCard } from './ProjectCard';
import type { OrgStatusProjectNode, OrgStatusApplicationNode } from '../types';

function app(slug: string, envs: string[] = ['prod']): OrgStatusApplicationNode {
  return {
    application: { id: `a-${slug}`, slug, name: slug.toUpperCase(), monitoring_links: null },
    environments: envs.map((e) => ({
      environment: { id: `e-${e}`, slug: e, name: e },
      current_deployment: null,
      health: { state: 'healthy', source: 'agent', staleness: 'fresh' },
      never_deployed: false,
    })),
  };
}

function project(partial: Partial<OrgStatusProjectNode> = {}): OrgStatusProjectNode {
  return {
    project: { id: 'p1', slug: 'payments', name: 'Payments' },
    aggregate_health: 'healthy',
    applications: [app('api'), app('web')],
    ...partial,
  };
}

describe('ProjectCard', () => {
  it('renders project name + aggregate health + app count when collapsed', () => {
    render(<ProjectCard project={project()} onEnvTap={() => {}} />);
    expect(screen.getByText('Payments')).toBeInTheDocument();
    expect(screen.getByText(/HEALTHY/i)).toBeInTheDocument();
    expect(screen.getByText(/2 apps/i)).toBeInTheDocument();
    expect(screen.queryByText('API')).not.toBeInTheDocument();
  });

  it('expands to show apps when tapped', async () => {
    render(<ProjectCard project={project()} onEnvTap={() => {}} />);
    await userEvent.click(screen.getByRole('button', { name: /payments/i }));
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('WEB')).toBeInTheDocument();
  });

  it('calls onEnvTap with the cell when an env chip is clicked', async () => {
    const onEnvTap = vi.fn();
    render(<ProjectCard project={project({ applications: [app('api', ['prod', 'staging'])] })} onEnvTap={onEnvTap} />);
    await userEvent.click(screen.getByRole('button', { name: /payments/i }));
    await userEvent.click(screen.getByRole('button', { name: 'prod' }));
    expect(onEnvTap).toHaveBeenCalledTimes(1);
    expect(onEnvTap.mock.calls[0][0].environment.slug).toBe('prod');
  });
});
```

- [ ] **Step 3:** Implement `ProjectCard.tsx`.

```tsx
import { useState } from 'react';
import type { OrgStatusEnvCell, OrgStatusProjectNode } from '../types';
import { HealthPill } from './HealthPill';
import { EnvChip } from './EnvChip';
import { MonitoringLinkIcon } from './MonitoringLinkIcon';

export function ProjectCard({
  project,
  onEnvTap,
}: {
  project: OrgStatusProjectNode;
  onEnvTap: (cell: OrgStatusEnvCell) => void;
}) {
  const [open, setOpen] = useState(false);
  const appCount = project.applications.length;

  return (
    <div className="m-card m-project-card">
      <button
        type="button"
        className="m-project-card-header"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <div className="m-project-card-title">
          <span className="m-project-name">{project.project.name}</span>
          <span className="m-project-apps">{appCount} apps</span>
        </div>
        <HealthPill state={project.aggregate_health} />
      </button>

      {open && (
        <ul className="m-app-list">
          {project.applications.map((a) => (
            <li key={a.application.id} className="m-app-row">
              <div className="m-app-row-header">
                <span className="m-app-name">{a.application.name}</span>
                <span className="m-monitor-row">
                  {(a.application.monitoring_links ?? []).map((link) => (
                    <MonitoringLinkIcon key={link.url} link={link} />
                  ))}
                </span>
              </div>
              <div className="m-env-chip-row">
                {a.environments.map((cell) => (
                  <EnvChip key={cell.environment.id} cell={cell} onTap={onEnvTap} />
                ))}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

- [ ] **Step 4:** Append styles to `mobile-pwa/src/styles/mobile.css`:

```css
.m-project-card { padding: 0; overflow: hidden; }
.m-project-card-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  background: transparent;
  border: none;
  color: inherit;
  text-align: left;
  cursor: pointer;
  min-height: 52px;
}
.m-project-card-title { display: flex; flex-direction: column; gap: 2px; }
.m-project-name { font-weight: 600; font-size: 15px; }
.m-project-apps { color: var(--color-text-muted, #64748b); font-size: 12px; }

.m-app-list { list-style: none; margin: 0; padding: 0 14px 14px; display: flex; flex-direction: column; gap: 12px; }
.m-app-row { border-top: 1px solid var(--color-border, #1e293b); padding-top: 12px; display: flex; flex-direction: column; gap: 8px; }
.m-app-row-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.m-app-name { font-weight: 500; font-size: 13px; }
.m-monitor-row { display: inline-flex; gap: 6px; }

.m-monitor-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  min-height: 28px;
  padding: 0 6px;
  border-radius: 6px;
  font-size: 14px;
  text-decoration: none;
  background: var(--color-bg-surface, #131b2e);
  border: 1px solid var(--color-border, #1e293b);
}
```

- [ ] **Step 5:** 3 ProjectCard tests pass. Full suite: 43 passing. tsc clean.

- [ ] **Step 6:** Commit.

```bash
git add mobile-pwa/src/components/ mobile-pwa/src/styles/mobile.css
git commit -m "feat(mobile-pwa): add ProjectCard + MonitoringLinkIcon"
```

---

## Task 6: StatusPage (TDD) — replace Phase 1 placeholder

**Files:**
- Replace: `mobile-pwa/src/pages/StatusPage.tsx`
- Create: `mobile-pwa/src/pages/StatusPage.test.tsx`

- [ ] **Step 1:** Write failing `StatusPage.test.tsx`.

```tsx
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { StatusPage } from './StatusPage';
import { setFetch } from '../api';

const FIXTURE = {
  org: { id: '1', slug: 'acme', name: 'Acme' },
  generated_at: '2026-04-24T12:00:00Z',
  projects: [
    {
      project: { id: 'p1', slug: 'payments', name: 'Payments' },
      aggregate_health: 'healthy',
      applications: [
        {
          application: { id: 'a1', slug: 'api', name: 'API', monitoring_links: null },
          environments: [
            {
              environment: { id: 'e1', slug: 'prod', name: 'Production' },
              current_deployment: null,
              health: { state: 'healthy', source: 'agent', staleness: 'fresh' },
              never_deployed: false,
            },
          ],
        },
      ],
    },
  ],
};

describe('StatusPage', () => {
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
          <Route path="/orgs/:orgSlug/status" element={<StatusPage />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('renders project cards when the fetch succeeds', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify(FIXTURE), { status: 200 }));
    renderAt('/orgs/acme/status');
    expect(await screen.findByText('Payments')).toBeInTheDocument();
  });

  it('renders an error message when the fetch fails', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ error: 'boom' }), { status: 500 }));
    renderAt('/orgs/acme/status');
    expect(await screen.findByText(/boom|failed/i)).toBeInTheDocument();
  });

  it('renders an empty state when the org has zero projects', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ ...FIXTURE, projects: [] }), { status: 200 }),
    );
    renderAt('/orgs/acme/status');
    expect(await screen.findByText(/no projects/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2:** Replace `mobile-pwa/src/pages/StatusPage.tsx` (it currently has placeholder content).

```tsx
import { useCallback, useState } from 'react';
import { useParams } from 'react-router-dom';
import { orgStatusApi } from '../api';
import type { OrgStatusEnvCell, OrgStatusResponse } from '../types';
import { useAutoPoll } from '../hooks/useAutoPoll';
import { ProjectCard } from '../components/ProjectCard';

const POLL_MS = 15_000;

export function StatusPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const [data, setData] = useState<OrgStatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    if (!orgSlug) return;
    orgStatusApi
      .get(orgSlug)
      .then((r) => {
        setData(r);
        setError(null);
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load status'));
  }, [orgSlug]);

  useAutoPoll(load, POLL_MS);

  const onEnvTap = (cell: OrgStatusEnvCell) => {
    const dep = cell.current_deployment;
    window.alert(
      dep
        ? `Deployment ${dep.version} (${dep.status})\nDetail screen ships in Phase 3.`
        : 'No deployment yet.',
    );
  };

  if (error && !data) {
    return (
      <section>
        <h2>Status</h2>
        <p style={{ color: 'var(--color-danger, #ef4444)' }}>{error}</p>
      </section>
    );
  }
  if (!data) {
    return <div className="m-page-loading">Loading…</div>;
  }
  if (data.projects.length === 0) {
    return (
      <section>
        <h2>Status</h2>
        <p style={{ color: 'var(--color-text-muted, #64748b)' }}>No projects in this organization yet.</p>
      </section>
    );
  }

  return (
    <section>
      <h2 style={{ margin: '4px 0 12px' }}>Status</h2>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {data.projects.map((p) => (
          <ProjectCard key={p.project.id} project={p} onEnvTap={onEnvTap} />
        ))}
      </div>
    </section>
  );
}
```

- [ ] **Step 3:** Run — 3 StatusPage tests pass.

- [ ] **Step 4:** Full suite + typecheck. 46 tests passing. tsc clean.

- [ ] **Step 5:** Commit.

```bash
git add mobile-pwa/src/pages/StatusPage.tsx mobile-pwa/src/pages/StatusPage.test.tsx
git commit -m "feat(mobile-pwa): implement StatusPage with fan-in polling"
```

---

## Task 7: Update Current_Initiatives + Playwright smoke

**Files:**
- Modify: `docs/Current_Initiatives.md`
- Modify: `mobile-pwa/e2e/smoke.spec.ts`

- [ ] **Step 1:** Update the Mobile PWA row in `docs/Current_Initiatives.md`. Append `/ [Phase 2 Plan](./superpowers/plans/2026-04-24-mobile-pwa-phase2-status-tab.md)` to the Plan/Spec column, and update the Notes column to mention Phase 2 in flight:

Before:
```
| Mobile PWA | Implementation | [Spec](./superpowers/specs/2026-04-24-mobile-pwa-design.md) / [Phase 1 Plan](./superpowers/plans/2026-04-24-mobile-pwa-phase1-scaffolding.md) | Phase 1 (scaffolding) implemented ...
```

After:
```
| Mobile PWA | Implementation | [Spec](./superpowers/specs/2026-04-24-mobile-pwa-design.md) / [Phase 1 Plan](./superpowers/plans/2026-04-24-mobile-pwa-phase1-scaffolding.md) / [Phase 2 Plan](./superpowers/plans/2026-04-24-mobile-pwa-phase2-status-tab.md) | Phase 1 merged (PR #50). Phase 2 (Status tab) in flight on `feature/mobile-pwa-phase2`: real org fan-in, 15s auto-poll with visibility-pause, collapsible project cards with env-chip strip + monitoring-link icons. Phases 3-6 pending.
```

Also update `Last updated:` to the current date (`2026-04-24`).

- [ ] **Step 2:** Extend the Playwright smoke. Append to `mobile-pwa/e2e/smoke.spec.ts`:

```ts
test('authenticated status page smoke (API mocked at browser level)', async ({ page, context }) => {
  // Fake a logged-in session by pre-setting the JWT + mocking the
  // /api/v1/users/me, /api/v1/orgs, /api/v1/orgs/:slug/status calls.
  await context.addInitScript(() => {
    const toB64u = (s: string) =>
      btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = { exp: Math.floor(Date.now() / 1000) + 3600 };
    const token = `${toB64u('{"alg":"HS256"}')}.${toB64u(JSON.stringify(payload))}.sig`;
    localStorage.setItem('ds_token', token);
  });
  await context.route('**/api/v1/users/me', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: '1', email: 'a@b.c', name: 'A' }) }),
  );
  await context.route('**/api/v1/orgs', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ organizations: [{ id: '1', slug: 'acme', name: 'Acme', created_at: '', updated_at: '' }] }),
    }),
  );
  await context.route('**/api/v1/orgs/acme/status', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        org: { id: '1', slug: 'acme', name: 'Acme' },
        generated_at: '2026-04-24T00:00:00Z',
        projects: [
          {
            project: { id: 'p1', slug: 'payments', name: 'Payments' },
            aggregate_health: 'healthy',
            applications: [],
          },
        ],
      }),
    }),
  );

  await page.goto('/m/orgs/acme/status');
  await page.getByText('Payments').waitFor({ state: 'visible' });
  await page.getByRole('button', { name: /flags/i }).waitFor({ state: 'visible' });
});
```

- [ ] **Step 3:** Run smoke — 3 passed + 1 skipped (the new test brings it to 3 passing).

```bash
cd mobile-pwa && npm run test:e2e
```

- [ ] **Step 4:** Commit.

```bash
git add docs/Current_Initiatives.md mobile-pwa/e2e/smoke.spec.ts
git commit -m "docs,test(mobile-pwa): track phase 2 + add status smoke"
```

---

## Success criteria

- `npm run lint` — zero warnings.
- `npx tsc --noEmit` — exit 0.
- `npm run test` — **46 tests passing across 12 files** (Phase 1's 27 + 19 new: 1 api + 4 useAutoPoll + 4 HealthPill + 4 EnvChip + 3 ProjectCard + 3 StatusPage).
- `npm run build` — `dist/` identical to Phase 1 plus the StatusPage bundle.
- `npm run test:e2e` — 3 passed + 1 skipped (Phase 1's 2 + the new status smoke).
- `make run-api && make run-mobile` — visiting `http://localhost:3002/m/orgs/<slug>/status` renders the real project fan-in.

## Out of scope (explicitly)

- Build pill (the `latest_build` field from the Build-Status initiative isn't on the mobile subset yet; defer until Phase 5+ adds flag-edit endpoints alongside it).
- Drill-down to deployment detail (Phase 3).
- Stale badge for SW-cached data (Phase 6).
- Pull-to-refresh gesture.
- Skeleton-loading placeholders — just the existing `m-page-loading` spinner.
