import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { auditApi, entitiesApi, membersApi } from '@/api';
import type { AuditLogEntry, Member, Project } from '@/types';
import AuditRow from '@/components/audit/AuditRow';

const ENTITY_TYPE_OPTIONS = ['', 'flag', 'rule'];
const PAGE_SIZE = 50;

interface Filters {
  action: string;
  entity_type: string;
  project_id: string;
  user_id: string;
  start_date: string;
  end_date: string;
}

const EMPTY: Filters = {
  action: '',
  entity_type: '',
  project_id: '',
  user_id: '',
  start_date: '',
  end_date: '',
};

export default function OrgAuditPage() {
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
  const [members, setMembers] = useState<Member[]>([]);

  const [rows, setRows] = useState<AuditLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load projects + members once.
  useEffect(() => {
    if (!orgSlug) return;
    entitiesApi.listProjects(orgSlug).then((r) => setProjects(r.projects)).catch(() => {});
    membersApi
      .listByOrg(orgSlug)
      .then((r) => setMembers(r.members ?? []))
      .catch(() => {});
  }, [orgSlug]);

  const load = useCallback(
    async (append = false, offset = 0) => {
      if (!orgSlug) return;
      setError(null);
      if (append) setLoadingMore(true);
      else setLoading(true);

      try {
        const resp = await auditApi.query({
          action: filters.action || undefined,
          entity_type: filters.entity_type || undefined,
          project_id: filters.project_id || undefined,
          user_id: filters.user_id || undefined,
          start_date: filters.start_date ? new Date(filters.start_date).toISOString() : undefined,
          end_date: filters.end_date ? new Date(filters.end_date).toISOString() : undefined,
          limit: PAGE_SIZE,
          offset,
        });
        setRows((prev) =>
          append ? [...prev, ...(resp.entries ?? [])] : (resp.entries ?? []),
        );
        setTotal(resp.total ?? 0);
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
    load(false, 0);
  }, [load]);

  function setFilter<K extends keyof Filters>(key: K, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next, { replace: true });
  }

  function resetFilters() {
    setSearchParams(new URLSearchParams(), { replace: true });
  }

  const handleRevert = (entry: AuditLogEntry) => {
    // Task 3.1 will replace this with the RevertConfirmDialog.
    if (!window.confirm(`Revert: ${entry.action}? This will be audit-logged.`)) return;
    auditApi
      .revert(entry.id, false)
      .then(() => load(false, 0))
      .catch((err) => {
        alert(`Revert failed: ${err.message}`);
      });
  };

  if (!orgSlug) return null;

  const projectName = (id: string): string | undefined =>
    projects.find((p) => p.id === id)?.name;

  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  return (
    <div className="org-audit-page">
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>Audit</h1>
          <p>Every change across this org, newest first.</p>
        </div>
      </div>

      <div
        className="org-audit-layout"
        style={{ display: 'grid', gridTemplateColumns: '260px 1fr', gap: 24 }}
      >
        <aside className="org-audit-filters">
          <div
            className="org-audit-filters-head"
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: 12,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>
                filter_list
              </span>
              <span style={{ fontWeight: 600 }}>Filters</span>
            </div>
            {activeFilterCount > 0 && (
              <button className="btn btn-sm" type="button" onClick={resetFilters}>
                Reset
              </button>
            )}
          </div>

          <label className="form-label">Action</label>
          <input
            type="text"
            className="form-input"
            placeholder="e.g. flag.archived"
            value={filters.action}
            onChange={(e) => setFilter('action', e.target.value)}
          />

          <label className="form-label">Entity type</label>
          <select
            className="form-input"
            value={filters.entity_type}
            onChange={(e) => setFilter('entity_type', e.target.value)}
          >
            {ENTITY_TYPE_OPTIONS.map((t) => (
              <option key={t} value={t}>
                {t || 'Any'}
              </option>
            ))}
          </select>

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

          {members.length > 0 && (
            <>
              <label className="form-label">User</label>
              <select
                className="form-input"
                value={filters.user_id}
                onChange={(e) => setFilter('user_id', e.target.value)}
              >
                <option value="">All users</option>
                {members.map((m) => (
                  <option key={m.user_id} value={m.user_id}>
                    {m.name || m.email}
                  </option>
                ))}
              </select>
            </>
          )}

          <label className="form-label">From</label>
          <input
            type="datetime-local"
            className="form-input"
            value={filters.start_date}
            onChange={(e) => setFilter('start_date', e.target.value)}
          />
          <label className="form-label">To</label>
          <input
            type="datetime-local"
            className="form-input"
            value={filters.end_date}
            onChange={(e) => setFilter('end_date', e.target.value)}
          />
        </aside>

        <main className="org-audit-main">
          {error && <div className="page-error">Error: {error}</div>}
          <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
            <div
              style={{
                padding: '12px 20px',
                borderBottom: '1px solid var(--color-border)',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
              }}
            >
              <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
                history_edu
              </span>
              <span
                style={{
                  fontFamily: 'var(--font-display)',
                  fontWeight: 700,
                  fontSize: 14,
                }}
              >
                Activity Stream
              </span>
              {rows.length > 0 && (
                <span
                  className="badge"
                  style={{
                    background: 'var(--color-primary-bg)',
                    color: 'var(--color-primary)',
                    marginLeft: 4,
                  }}
                >
                  {rows.length} / {total}
                </span>
              )}
              {loading && rows.length > 0 && (
                <span
                  className="ms"
                  style={{
                    fontSize: 16,
                    color: 'var(--color-text-muted)',
                    marginLeft: 'auto',
                    animation: 'spin 1s linear infinite',
                  }}
                >
                  sync
                </span>
              )}
            </div>

            <div className="audit-table">
              <div className="audit-row audit-row-head">
                <div>When</div>
                <div>Who</div>
                <div>What</div>
                <div>Where</div>
                <div></div>
              </div>

              {loading && rows.length === 0 && (
                <div className="audit-empty">Loading…</div>
              )}
              {!loading && rows.length === 0 && (
                <div className="audit-empty">No audit entries match these filters.</div>
              )}

              {rows.map((entry) => (
                <AuditRow
                  key={entry.id}
                  entry={entry}
                  where={projectName(entry.project_id)}
                  onRevert={handleRevert}
                />
              ))}
            </div>

            {rows.length < total && (
              <div
                style={{
                  padding: 16,
                  textAlign: 'center',
                  borderTop: '1px solid var(--color-border)',
                }}
              >
                <button
                  className="btn"
                  type="button"
                  disabled={loadingMore}
                  onClick={() => load(true, rows.length)}
                >
                  {loadingMore ? 'Loading…' : `Load more (${total - rows.length} remaining)`}
                </button>
              </div>
            )}
          </div>
        </main>
      </div>
    </div>
  );
}
