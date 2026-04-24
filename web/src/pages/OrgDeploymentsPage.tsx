import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { entitiesApi, orgStatusApi } from '@/api';
import type {
  Application,
  DeployStatus,
  OrgDeploymentRow,
  OrgEnvironment,
  Project,
} from '@/types';

const STATUS_OPTIONS: (DeployStatus | '')[] = [
  '',
  'pending',
  'running',
  'promoting',
  'paused',
  'completed',
  'failed',
  'rolled_back',
  'cancelled',
];

const MODE_OPTIONS: ('' | 'orchestrate' | 'record')[] = ['', 'orchestrate', 'record'];
const PAGE_SIZE = 50;

/**
 * Filter values that serialize into the URL so the current view is shareable.
 */
interface Filters {
  project_id: string;
  application_id: string;
  environment_id: string;
  status: string;
  mode: string;
  from: string;
  to: string;
}

const EMPTY: Filters = {
  project_id: '',
  application_id: '',
  environment_id: '',
  status: '',
  mode: '',
  from: '',
  to: '',
};

export default function OrgDeploymentsPage() {
  const { orgSlug } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();

  const filters: Filters = useMemo(() => {
    const f = { ...EMPTY };
    for (const k of Object.keys(EMPTY) as (keyof Filters)[]) {
      f[k] = searchParams.get(k) ?? '';
    }
    return f;
  }, [searchParams]);

  const [projects, setProjects] = useState<Project[]>([]);
  const [apps, setApps] = useState<Application[]>([]);
  const [envs, setEnvs] = useState<OrgEnvironment[]>([]);

  const [rows, setRows] = useState<OrgDeploymentRow[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load projects + envs once the org slug is known.
  useEffect(() => {
    if (!orgSlug) return;
    entitiesApi
      .listProjects(orgSlug)
      .then((r) => setProjects(r.projects))
      .catch(() => {});
    entitiesApi
      .listOrgEnvironments(orgSlug)
      .then((r) => setEnvs(r.environments))
      .catch(() => {});
  }, [orgSlug]);

  // Cascade: when a project is picked, narrow the app list by fetching its apps.
  useEffect(() => {
    if (!orgSlug) return;
    if (!filters.project_id) {
      setApps([]);
      return;
    }
    const project = projects.find((p) => p.id === filters.project_id);
    if (!project) return;
    entitiesApi
      .listApps(orgSlug, project.slug)
      .then((r) => setApps(r.applications))
      .catch(() => setApps([]));
  }, [orgSlug, filters.project_id, projects]);

  const load = useCallback(
    async (append = false, nextCursor?: string) => {
      if (!orgSlug) return;
      setError(null);
      if (append) setLoadingMore(true);
      else setLoading(true);

      try {
        const resp = await orgStatusApi.listDeployments(orgSlug, {
          project_id: filters.project_id || undefined,
          application_id: filters.application_id || undefined,
          environment_id: filters.environment_id || undefined,
          status: filters.status || undefined,
          mode: filters.mode || undefined,
          from: filters.from ? new Date(filters.from).toISOString() : undefined,
          to: filters.to ? new Date(filters.to).toISOString() : undefined,
          cursor: nextCursor,
          limit: PAGE_SIZE,
        });
        setRows((prev) =>
          append ? [...prev, ...(resp.deployments ?? [])] : resp.deployments ?? [],
        );
        setCursor(resp.next_cursor || undefined);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [orgSlug, filters],
  );

  // Re-fetch from scratch whenever filters change.
  useEffect(() => {
    setRows([]);
    setCursor(undefined);
    load(false);
  }, [load]);

  function setFilter<K extends keyof Filters>(key: K, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    if (key === 'project_id') next.delete('application_id'); // cascade reset
    setSearchParams(next, { replace: true });
  }

  function resetFilters() {
    setSearchParams(new URLSearchParams(), { replace: true });
  }

  if (!orgSlug) return null;

  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  return (
    <div className="org-deployments-page">
      <div className="page-header">
        <h1>Deploy History</h1>
        <p>Every deployment across this org, newest first.</p>
      </div>

      <div className="org-deployments-layout">
        <aside className="org-deployments-filters">
          <div className="org-deployments-filters-head">
            <span style={{ fontWeight: 600 }}>Filters</span>
            {activeFilterCount > 0 && (
              <button className="btn" type="button" onClick={resetFilters}>
                Reset
              </button>
            )}
          </div>

          <label className="form-label">Project</label>
          <select
            className="form-input"
            value={filters.project_id}
            onChange={(e) => setFilter('project_id', e.target.value)}
          >
            <option value="">All projects</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>

          <label className="form-label">Application</label>
          <select
            className="form-input"
            value={filters.application_id}
            onChange={(e) => setFilter('application_id', e.target.value)}
            disabled={!filters.project_id}
          >
            <option value="">
              {filters.project_id ? 'All applications' : 'Select a project first'}
            </option>
            {apps.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>

          <label className="form-label">Environment</label>
          <select
            className="form-input"
            value={filters.environment_id}
            onChange={(e) => setFilter('environment_id', e.target.value)}
          >
            <option value="">All environments</option>
            {envs.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>

          <label className="form-label">Status</label>
          <select
            className="form-input"
            value={filters.status}
            onChange={(e) => setFilter('status', e.target.value)}
          >
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s || 'Any'}
              </option>
            ))}
          </select>

          <label className="form-label">Mode</label>
          <select
            className="form-input"
            value={filters.mode}
            onChange={(e) => setFilter('mode', e.target.value)}
          >
            {MODE_OPTIONS.map((m) => (
              <option key={m} value={m}>
                {m || 'Any'}
              </option>
            ))}
          </select>

          <label className="form-label">From</label>
          <input
            type="datetime-local"
            className="form-input"
            value={filters.from}
            onChange={(e) => setFilter('from', e.target.value)}
          />

          <label className="form-label">To</label>
          <input
            type="datetime-local"
            className="form-input"
            value={filters.to}
            onChange={(e) => setFilter('to', e.target.value)}
          />
        </aside>

        <main className="org-deployments-main">
          {error && <div className="page-error">Error: {error}</div>}
          <div className="org-deployments-table">
            <div className="org-deployments-row org-deployments-head">
              <div>When</div>
              <div>Where</div>
              <div>Version</div>
              <div>Status</div>
              <div>Mode</div>
              <div>Source</div>
            </div>
            {loading && rows.length === 0 && (
              <div className="org-deployments-empty">Loading…</div>
            )}
            {!loading && rows.length === 0 && (
              <div className="org-deployments-empty">
                No deployments match the current filters.
              </div>
            )}
            {rows.map((row) => (
              <DeploymentRow key={row.id} orgSlug={orgSlug} row={row} />
            ))}
          </div>

          {cursor && (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 16 }}>
              <button
                type="button"
                className="btn"
                onClick={() => load(true, cursor)}
                disabled={loadingMore}
              >
                {loadingMore ? 'Loading…' : 'Load older'}
              </button>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

// -----------------------------------------------------------------------------

function DeploymentRow({ orgSlug, row }: { orgSlug: string; row: OrgDeploymentRow }) {
  const target = `/orgs/${orgSlug}/projects/${row.project.slug}/apps/${row.application.slug}/deployments/${row.id}`;
  return (
    <Link className="org-deployments-row" to={target}>
      <div className="org-deployments-when" title={new Date(row.created_at).toLocaleString()}>
        {relativeTime(row.created_at)}
      </div>
      <div className="org-deployments-where">
        <span className="dim">{row.project.name}</span>
        <span className="sep">›</span>
        <strong>{row.application.name}</strong>
        <span className="sep">›</span>
        <span className="env-chip env-chip-inline">
          {row.environment.slug ?? row.environment.name ?? '?'}
        </span>
      </div>
      <div className="org-deployments-version">
        <code title={row.commit_sha}>{shortVersion(row.version)}</code>
      </div>
      <div>
        <span className={`status-pill status-${row.status}`}>{row.status}</span>
      </div>
      <div>
        <span className={`mode-badge mode-${row.mode}`}>{row.mode}</span>
      </div>
      <div className="dim">{row.source ?? '—'}</div>
    </Link>
  );
}

function relativeTime(ts: string | null | undefined): string {
  if (!ts) return '—';
  const then = new Date(ts).getTime();
  const diff = Date.now() - then;
  if (diff < 0) return 'just now';
  const s = Math.floor(diff / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d}d ago`;
  return new Date(then).toLocaleDateString();
}

function shortVersion(v: string): string {
  if (!v) return '—';
  if (/^[0-9a-f]{40}$/i.test(v)) return v.slice(0, 7);
  return v;
}
