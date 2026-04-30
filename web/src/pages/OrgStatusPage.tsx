import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { orgStatusApi } from '@/api';
import type {
  HealthState,
  MonitoringLink,
  OrgStatusApplicationNode,
  OrgStatusEnvCell,
  OrgStatusProjectNode,
  OrgStatusResponse,
} from '@/types';

const POLL_MS = 15_000;
const EXPANSION_KEY = 'ds_org_status_collapsed_projects';

export default function OrgStatusPage() {
  const { orgSlug } = useParams();
  const [data, setData] = useState<OrgStatusResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastFetched, setLastFetched] = useState<Date | null>(null);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() => {
    try {
      const raw = localStorage.getItem(EXPANSION_KEY);
      return raw ? JSON.parse(raw) : {};
    } catch {
      return {};
    }
  });

  const toggle = useCallback((id: string) => {
    setCollapsed((prev) => {
      const next = { ...prev, [id]: !prev[id] };
      try {
        localStorage.setItem(EXPANSION_KEY, JSON.stringify(next));
      } catch {
        /* ignore quota / privacy errors */
      }
      return next;
    });
  }, []);

  const load = useCallback(async () => {
    if (!orgSlug) return;
    setError(null);
    try {
      const resp = await orgStatusApi.get(orgSlug);
      setData(resp);
      setLastFetched(new Date());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [orgSlug]);

  useEffect(() => {
    load();
    const id = window.setInterval(load, POLL_MS);
    return () => window.clearInterval(id);
  }, [load]);

  const globalCounts = useMemo(() => countStates(data), [data]);

  if (!orgSlug) return null;
  if (loading && !data) return <div className="page-loading">Loading status…</div>;
  if (error && !data) return <div className="page-error">Error: {error}</div>;

  return (
    <div className="org-status-page">
      <div className="page-header">
        <h1>Status</h1>
        <p>Compact health + version view across every project and application in this org.</p>
      </div>

      <div className="stat-grid" style={{ marginBottom: 24 }}>
        <StatCard
          label="Healthy"
          value={globalCounts.healthy}
          color="var(--color-success)"
          icon="check_circle"
        />
        <StatCard
          label="Degraded"
          value={globalCounts.degraded}
          color="var(--color-warning)"
          icon="warning"
        />
        <StatCard
          label="Unhealthy"
          value={globalCounts.unhealthy}
          color="var(--color-danger)"
          icon="error"
        />
        <StatCard
          label="Unknown"
          value={globalCounts.unknown}
          color="var(--color-text-muted)"
          icon="help"
        />
      </div>

      <div className="org-status-summary">
        <div className="org-status-refresh" style={{ marginLeft: 0 }}>
          {lastFetched && (
            <span className="org-status-timestamp">Updated {relativeTime(lastFetched)}</span>
          )}
          <button className="btn btn-secondary btn-sm" type="button" onClick={load}>
            <span className="ms" style={{ fontSize: 14 }}>
              refresh
            </span>
            Refresh
          </button>
        </div>
      </div>

      {data?.projects.length === 0 && (
        <div className="empty-state">
          <p>No applications yet.</p>
          <Link to={`/orgs/${orgSlug}/projects`} className="btn btn-primary">
            Go to projects
          </Link>
        </div>
      )}

      {data?.projects.map((p) => {
        const isCollapsed = !!collapsed[p.project.id];
        return (
          <section key={p.project.id} className="org-status-project">
            <button
              type="button"
              className={`org-status-project-bar health-${p.aggregate_health}`}
              onClick={() => toggle(p.project.id)}
            >
              <span className="ms org-status-caret" style={{ fontSize: 16 }}>
                {isCollapsed ? 'chevron_right' : 'expand_more'}
              </span>
              <span className="org-status-project-name">{p.project.name}</span>
              <span className="org-status-project-count">
                {p.applications.length} app{p.applications.length === 1 ? '' : 's'}
              </span>
              <span className={`health-pill health-${p.aggregate_health}`}>
                {p.aggregate_health}
              </span>
            </button>
            {!isCollapsed && (
              <div className="org-status-app-list">
                {p.applications.length === 0 ? (
                  <div className="org-status-empty">No applications in this project.</div>
                ) : (
                  p.applications.map((a) => (
                    <AppRow key={a.application.id} orgSlug={orgSlug} project={p} app={a} />
                  ))
                )}
              </div>
            )}
          </section>
        );
      })}
    </div>
  );
}

// -----------------------------------------------------------------------------
// Sub-components
// -----------------------------------------------------------------------------

function AppRow({
  orgSlug,
  project,
  app,
}: {
  orgSlug: string;
  project: OrgStatusProjectNode;
  app: OrgStatusApplicationNode;
}) {
  const latest = mostRecentDeploy(app);
  return (
    <div className="org-status-app-row">
      <div className="org-status-app-ident">
        <Link
          to={`/orgs/${orgSlug}/projects/${project.project.slug}/apps/${app.application.slug}`}
          className="org-status-app-name"
        >
          {app.application.name}
        </Link>
        <div className="org-status-app-meta">
          {latest ? (
            <>
              <span title={latest.commit_sha}>{shortVersion(latest.version)}</span>
              <span className="dim">• {relativeTime(latest.completed_at)}</span>
            </>
          ) : (
            <span className="dim">no deployments yet</span>
          )}
        </div>
      </div>

      <div className="org-status-chips">
        {app.environments.map((cell) => (
          <EnvChip key={cell.environment.id} cell={cell} />
        ))}
      </div>

      <div className="org-status-links">
        {(app.application.monitoring_links ?? []).map((link, i) => (
          <MonitoringLinkIcon key={i} link={link} />
        ))}
      </div>

      <Link
        to={`/orgs/${orgSlug}/deployments?application_id=${app.application.id}`}
        className="org-status-history-link"
      >
        History →
      </Link>
    </div>
  );
}

function EnvChip({ cell }: { cell: OrgStatusEnvCell }) {
  const label = cell.environment.slug ?? '??';
  const tooltip = tooltipFor(cell);
  const classes = ['env-chip'];
  if (cell.never_deployed) {
    classes.push('env-chip-faded');
  } else {
    classes.push(`health-${cell.health.state}`);
    if (cell.health.staleness === 'stale') classes.push('stale');
    if (cell.health.staleness === 'missing' && cell.health.state !== 'unknown')
      classes.push('missing');
  }
  return (
    <span className={classes.join(' ')} title={tooltip}>
      {label}
    </span>
  );
}

function MonitoringLinkIcon({ link }: { link: MonitoringLink }) {
  const glyph = iconFor(link.icon);
  return (
    <a
      href={link.url}
      target="_blank"
      rel="noopener noreferrer"
      className="org-status-link-icon"
      title={link.label}
    >
      {link.icon === 'custom' ? (
        <Favicon url={link.url} label={link.label} />
      ) : (
        (glyph ?? link.label)
      )}
    </a>
  );
}

function Favicon({ url, label }: { url: string; label: string }) {
  const host = useMemo(() => {
    try {
      return new URL(url).host;
    } catch {
      return '';
    }
  }, [url]);
  const [broken, setBroken] = useState(false);
  if (!host) return <span>{label}</span>;
  if (broken) return <span>{label[0]?.toUpperCase() ?? '?'}</span>;
  return (
    <img
      alt={label}
      src={`https://${host}/favicon.ico`}
      width={16}
      height={16}
      onError={() => setBroken(true)}
      style={{ verticalAlign: 'middle' }}
    />
  );
}

function StatCard({
  label,
  value,
  color,
  icon,
}: {
  label: string;
  value: number;
  color: string;
  icon?: string;
}) {
  return (
    <div className="stat-card stat-card-with-icon">
      {icon && (
        <span className="ms stat-card-icon" style={{ color }} aria-hidden="true">
          {icon}
        </span>
      )}
      <div className="stat-label">{label}</div>
      <div className="stat-value" style={{ color, fontFamily: 'var(--font-display)' }}>
        {value}
      </div>
    </div>
  );
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

function countStates(data: OrgStatusResponse | null): Record<HealthState, number> {
  const out: Record<HealthState, number> = { healthy: 0, degraded: 0, unhealthy: 0, unknown: 0 };
  if (!data) return out;
  for (const p of data.projects) {
    for (const a of p.applications) {
      for (const e of a.environments) {
        if (e.never_deployed) continue;
        out[e.health.state] += 1;
      }
    }
  }
  return out;
}

function mostRecentDeploy(app: OrgStatusApplicationNode) {
  let latest: { version: string; commit_sha?: string; completed_at?: string | null } | null = null;
  for (const e of app.environments) {
    const d = e.current_deployment;
    if (!d || !d.completed_at) continue;
    if (!latest || new Date(d.completed_at) > new Date(latest.completed_at ?? 0)) {
      latest = { version: d.version, commit_sha: d.commit_sha, completed_at: d.completed_at };
    }
  }
  return latest;
}

function shortVersion(v: string): string {
  if (!v) return '—';
  if (/^[0-9a-f]{40}$/i.test(v)) return v.slice(0, 7);
  return v;
}

function relativeTime(ts: string | number | Date | null | undefined): string {
  if (!ts) return '—';
  const then =
    typeof ts === 'string' || typeof ts === 'number' ? new Date(ts).getTime() : ts.getTime();
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

function tooltipFor(cell: OrgStatusEnvCell): string {
  if (cell.never_deployed)
    return `${cell.environment.name ?? cell.environment.slug}: never deployed`;
  const envName = cell.environment.name ?? cell.environment.slug ?? '';
  const version = cell.current_deployment?.version ?? '—';
  const health = cell.health.state;
  const staleness = cell.health.staleness;
  const source = cell.health.source;
  return `${envName} • ${version} • ${health} (${staleness}, ${source})`;
}

function iconFor(icon?: string): string | null {
  switch (icon) {
    case 'github':
      return '⌘';
    case 'datadog':
      return '🐶';
    case 'newrelic':
      return 'NR';
    case 'grafana':
      return '⌬';
    case 'pagerduty':
      return '🔔';
    case 'sentry':
      return '◈';
    case 'slack':
      return '#';
    case 'loki':
      return 'L';
    case 'prometheus':
      return 'P';
    case 'cloudwatch':
      return '☁';
    default:
      return null;
  }
}
