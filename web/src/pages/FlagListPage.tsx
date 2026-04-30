import { useState, useEffect, useMemo } from 'react';
import { Link, useParams } from 'react-router-dom';
import type { Flag, FlagCategory } from '@/types';
import { entitiesApi, flagsApi } from '@/api';

type StatusFilter = 'all' | 'enabled' | 'disabled' | 'archived';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const s = Math.floor(diff / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d}d ago`;
  return formatDate(iso);
}

function categoryBadgeStyle(cat: string): React.CSSProperties {
  switch (cat) {
    case 'release': return { background: 'var(--color-warning-bg)', color: 'var(--color-warning)' };
    case 'feature': return { background: 'var(--color-primary-bg)', color: 'var(--color-primary)' };
    case 'experiment': return { background: 'var(--color-purple-bg)', color: 'var(--color-purple)' };
    case 'ops': return { background: 'var(--color-success-bg)', color: 'var(--color-success)' };
    case 'permission': return { background: 'var(--color-danger-bg)', color: 'var(--color-danger)' };
    default: return { background: 'var(--color-bg-elevated)', color: 'var(--color-text-secondary)' };
  }
}

export default function FlagListPage() {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const contextName = appSlug ?? projectSlug ?? '';
  const heading = appSlug
    ? `${contextName} — Flags`
    : contextName
      ? `${contextName} — Feature Flags`
      : 'Feature Flags';

  const createPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/new`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags/new`;

  const flagDetailPath = (flagId: string) =>
    appSlug
      ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags/${flagId}`
      : `/orgs/${orgSlug}/projects/${projectSlug}/flags/${flagId}`;

  const [flags, setFlags] = useState<Flag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<'all' | FlagCategory>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug) return;
    setLoading(true);
    setError(null);

    entitiesApi
      .getProject(orgSlug, projectSlug)
      .then((project) =>
        flagsApi.list(project.id, appSlug ? { application: appSlug } : undefined),
      )
      .then((result) => setFlags(result.flags))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  const filtered = useMemo(() => {
    return flags.filter((flag) => {
      if (search) {
        const q = search.toLowerCase();
        if (!flag.name.toLowerCase().includes(q) && !flag.key.toLowerCase().includes(q)) {
          return false;
        }
      }
      if (categoryFilter !== 'all' && flag.category !== categoryFilter) return false;
      if (statusFilter === 'enabled' && (!flag.enabled || flag.archived)) return false;
      if (statusFilter === 'disabled' && (flag.enabled || flag.archived)) return false;
      if (statusFilter === 'archived' && !flag.archived) return false;
      return true;
    });
  }, [flags, search, categoryFilter, statusFilter]);

  const activeCount = flags.filter((f) => f.enabled && !f.archived).length;
  const staleCount = flags.filter((f) => {
    if (f.archived) return false;
    const daysSince = (Date.now() - new Date(f.updated_at).getTime()) / (1000 * 60 * 60 * 24);
    return daysSince >= 30;
  }).length;

  if (loading) return (
    <div className="empty-state" style={{ padding: '40px 0' }}>
      <span className="ms" style={{ fontSize: 32, color: 'var(--color-primary)', marginBottom: 12, display: 'block' }}>sync</span>
      Loading flags…
    </div>
  );
  if (error) return <div className="page-error">Error: {error}</div>;

  return (
    <div>
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>toggle_on</span>
            <span style={{ fontSize: 12, color: 'var(--color-primary)', background: 'var(--color-primary-bg)', padding: '2px 8px', borderRadius: 4, fontFamily: 'monospace' }}>
              {appSlug ?? projectSlug}
            </span>
          </div>
          <h1>{heading}</h1>
          <p>Manage feature rollouts and configuration toggles.</p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ position: 'relative' }}>
            <span className="ms" style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', fontSize: 16, color: 'var(--color-text-muted)' }}>search</span>
            <input
              type="text"
              className="form-input"
              placeholder="Filter flags..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{ paddingLeft: 34, width: 220 }}
            />
          </div>
          <Link to={createPath} className="btn btn-primary">
            <span className="ms" style={{ fontSize: 16 }}>add</span>
            Create Flag
          </Link>
        </div>
      </div>

      <div className="stat-grid" style={{ marginBottom: 24, marginTop: 20, gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))' }}>
        <div className="stat-card">
          <div className="stat-label">Total Flags</div>
          <div className="stat-value" style={{ fontFamily: 'var(--font-display)', color: 'var(--color-text)' }}>{flags.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Active (On)</div>
          <div className="stat-value" style={{ fontFamily: 'var(--font-display)', color: 'var(--color-primary)' }}>{activeCount}</div>
          <div style={{ marginTop: 8, height: 4, background: 'var(--color-bg-elevated)', borderRadius: 2, overflow: 'hidden' }}>
            <div style={{ height: '100%', width: `${flags.length ? Math.round((activeCount / flags.length) * 100) : 0}%`, background: 'var(--color-primary)', borderRadius: 2 }} />
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Stale (30d+)</div>
          <div className="stat-value" style={{ fontFamily: 'var(--font-display)', color: staleCount > 0 ? '#fb923c' : 'var(--color-text)' }}>{staleCount}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Archived</div>
          <div className="stat-value" style={{ fontFamily: 'var(--font-display)', color: 'var(--color-text-secondary)' }}>{flags.filter((f) => f.archived).length}</div>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
        <select
          className="form-input"
          value={categoryFilter}
          onChange={(e) => setCategoryFilter(e.target.value as 'all' | FlagCategory)}
          style={{ width: 'auto' }}
        >
          <option value="all">All Categories</option>
          <option value="release">Release</option>
          <option value="feature">Feature</option>
          <option value="experiment">Experiment</option>
          <option value="ops">Ops</option>
          <option value="permission">Permission</option>
        </select>
        <select
          className="form-input"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
          style={{ width: 'auto' }}
        >
          <option value="all">All Statuses</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
          <option value="archived">Archived</option>
        </select>
      </div>

      {flags.length === 0 ? (
        <div className="empty-state card" style={{ padding: '48px 24px' }}>
          <span className="ms" style={{ fontSize: 40, color: 'var(--color-text-muted)', marginBottom: 12, display: 'block' }}>toggle_on</span>
          <h3>No feature flags yet</h3>
          <p>Create your first flag to start controlling feature rollouts.</p>
          <Link to={createPath} className="btn btn-primary" style={{ marginTop: 16 }}>Create Flag</Link>
        </div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>toggle_on</span>
            <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Feature Flags</span>
            <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', marginLeft: 4 }}>
              {filtered.length}
            </span>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Name / Key</th>
                  <th>Category</th>
                  <th>Status</th>
                  <th>Owners</th>
                  <th>Updated</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((flag) => (
                  <tr key={flag.id}>
                    <td>
                      <Link to={flagDetailPath(flag.id)} style={{ fontWeight: 600, fontFamily: 'var(--font-display)', color: 'var(--color-text)', textDecoration: 'none' }}>
                        {flag.name}
                      </Link>
                      <div style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-muted)', marginTop: 2 }}>{flag.key}</div>
                    </td>
                    <td>
                      <span style={{
                        ...categoryBadgeStyle(flag.category),
                        fontSize: 10,
                        fontWeight: 700,
                        padding: '3px 8px',
                        borderRadius: 99,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                      }}>
                        {flag.category}
                      </span>
                    </td>
                    <td>
                      {flag.archived ? (
                        <span className="badge" style={{ background: 'var(--color-bg-elevated)', color: 'var(--color-text-muted)' }}>archived</span>
                      ) : (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <div style={{
                            position: 'relative',
                            width: 36,
                            height: 20,
                            borderRadius: 10,
                            background: flag.enabled ? 'var(--color-primary)' : 'var(--color-bg-elevated)',
                            boxShadow: flag.enabled ? '0 0 8px rgba(99,102,241,0.4)' : 'none',
                            flexShrink: 0,
                          }}>
                            <div style={{
                              position: 'absolute',
                              top: 3,
                              [flag.enabled ? 'right' : 'left']: 3,
                              width: 14,
                              height: 14,
                              borderRadius: '50%',
                              background: flag.enabled ? '#fff' : 'var(--color-text-muted)',
                            }} />
                          </div>
                          <span style={{ fontSize: 13, color: flag.enabled ? 'var(--color-text)' : 'var(--color-text-muted)' }}>
                            {flag.enabled ? 'On' : 'Off'}
                          </span>
                        </div>
                      )}
                    </td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        {(flag.owners ?? []).slice(0, 2).map((owner) => (
                          <div key={owner} style={{
                            width: 24, height: 24, borderRadius: '50%',
                            background: 'var(--color-primary-bg)',
                            border: '1px solid rgba(99,102,241,0.3)',
                            display: 'flex', alignItems: 'center', justifyContent: 'center',
                            fontSize: 9, fontWeight: 700, color: 'var(--color-primary)',
                          }}>
                            {owner.slice(0, 2).toUpperCase()}
                          </div>
                        ))}
                        {(flag.owners ?? []).length === 0 && (
                          <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>—</span>
                        )}
                        {(flag.owners ?? []).length > 0 && (
                          <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>{flag.owners![0]}</span>
                        )}
                      </div>
                    </td>
                    <td>
                      <div style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{relativeTime(flag.updated_at)}</div>
                    </td>
                    <td>
                      <Link to={flagDetailPath(flag.id)} className="btn btn-secondary btn-sm">View</Link>
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && (
                  <tr>
                    <td colSpan={6} style={{ textAlign: 'center', padding: '32px 0', color: 'var(--color-text-muted)' }}>
                      No flags match your filters.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <div style={{ padding: '12px 20px', borderTop: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
              Showing {filtered.length} of {flags.length} flags
            </span>
          </div>
        </div>
      )}
    </div>
  );
}
