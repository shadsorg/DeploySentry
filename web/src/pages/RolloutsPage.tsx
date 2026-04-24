import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutStatus, RolloutWithTarget } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

// Special sentinel for the "All" option. Picking it flips the two
// include-* checkboxes on so every terminal / stale row is returned.
// Picking the empty-value "In progress only" option keeps the default
// filter behavior (server hides completed + stale-pending).
const STATUS_ALL = '__all__';

const STATUS_OPTIONS: { value: RolloutStatus | '' | typeof STATUS_ALL; label: string }[] = [
  { value: STATUS_ALL, label: 'All' },
  { value: '', label: 'In progress only' },
  { value: 'active', label: 'Active' },
  { value: 'paused', label: 'Paused' },
  { value: 'awaiting_approval', label: 'Awaiting approval' },
  { value: 'pending', label: 'Pending' },
  { value: 'succeeded', label: 'Succeeded' },
  { value: 'rolled_back', label: 'Rolled back' },
  { value: 'aborted', label: 'Aborted' },
  { value: 'superseded', label: 'Superseded' },
];

const TARGET_OPTIONS: { value: 'deploy' | 'config' | ''; label: string }[] = [
  { value: '', label: 'Any target' },
  { value: 'deploy', label: 'Deploys' },
  { value: 'config', label: 'Configs (flag rules)' },
];

const AGE_OPTIONS: { value: number; label: string }[] = [
  { value: 0, label: 'All time' },
  { value: 24, label: 'Last 24 hours' },
  { value: 24 * 7, label: 'Last 7 days' },
  { value: 24 * 30, label: 'Last 30 days' },
];

type SortKey = 'target' | 'strategy' | 'phase' | 'status' | 'age';
type SortDir = 'asc' | 'desc';

export default function RolloutsPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<RolloutWithTarget[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<RolloutStatus | '' | typeof STATUS_ALL>('');
  const [targetType, setTargetType] = useState<'deploy' | 'config' | ''>('');
  const [sinceHours, setSinceHours] = useState<number>(0);
  const [includeTerminal, setIncludeTerminal] = useState(false);
  const [includeStale, setIncludeStale] = useState(false);
  const [filterEcho, setFilterEcho] = useState<{
    hidden_terminal_count: number;
    hidden_stale_count: number;
    stale_cutoff_hours: number;
  } | null>(null);

  // Default sort: newest first. age_seconds ascending = most recent at top.
  const [sortKey, setSortKey] = useState<SortKey>('age');
  const [sortDir, setSortDir] = useState<SortDir>('asc');

  // "All" is a UI preset — treat it as no status filter + include_terminal +
  // include_stale, without permanently flipping the checkboxes (so the user
  // can still see the effective state and switch back).
  const isAll = filter === STATUS_ALL;
  const effectiveStatus = isAll ? undefined : (filter || undefined);
  const effectiveIncludeTerminal = includeTerminal || isAll;
  const effectiveIncludeStale = includeStale || isAll;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await rolloutsApi.list(orgSlug, {
        status: effectiveStatus as RolloutStatus | undefined,
        target_type: targetType || undefined,
        since_hours: sinceHours || undefined,
        include_terminal: effectiveIncludeTerminal,
        include_stale: effectiveIncludeStale,
        limit: 200,
      });
      setItems(r.items || []);
      setFilterEcho(r.filter);
    } finally {
      setLoading(false);
    }
  }, [orgSlug, effectiveStatus, targetType, sinceHours, effectiveIncludeTerminal, effectiveIncludeStale]);

  useEffect(() => { load(); }, [load]);

  // Auto-refresh to keep active rollouts current.
  useEffect(() => {
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
  }

  const sorted = useMemo(() => {
    const rows = [...items];
    rows.sort((a, b) => cmp(a, b, sortKey) * (sortDir === 'asc' ? 1 : -1));
    return rows;
  }, [items, sortKey, sortDir]);

  function resetFilters() {
    setFilter('');
    setTargetType('');
    setSinceHours(0);
    setIncludeTerminal(false);
    setIncludeStale(false);
  }

  // `filter !== ''` covers both explicit statuses and the "All" sentinel.
  const anyFilterActive =
    filter !== '' ||
    targetType !== '' ||
    sinceHours !== 0 ||
    includeTerminal ||
    includeStale;

  const totalHidden =
    (filterEcho?.hidden_terminal_count ?? 0) + (filterEcho?.hidden_stale_count ?? 0);

  return (
    <div className="page">
      <div className="page-header">
        <h1>Rollouts</h1>
        <p>Monitor active and historical rollouts across all projects.</p>
      </div>

      <section className="rollouts-filter-bar">
        <div className="filter-group">
          <label className="filter-label">Status</label>
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value as RolloutStatus | '' | typeof STATUS_ALL)}
          >
            {STATUS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Target</label>
          <select
            value={targetType}
            onChange={(e) => setTargetType(e.target.value as 'deploy' | 'config' | '')}
          >
            {TARGET_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Age</label>
          <select value={sinceHours} onChange={(e) => setSinceHours(Number(e.target.value))}>
            {AGE_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>
        <div className="filter-group filter-checks">
          <label>
            <input
              type="checkbox"
              checked={includeTerminal}
              onChange={(e) => setIncludeTerminal(e.target.checked)}
            />{' '}
            Include completed
          </label>
          <label
            title={
              filterEcho
                ? `Pending rollouts older than ${filterEcho.stale_cutoff_hours}h with no progress`
                : ''
            }
          >
            <input
              type="checkbox"
              checked={includeStale}
              onChange={(e) => setIncludeStale(e.target.checked)}
            />{' '}
            Include stale pending
          </label>
        </div>
        {anyFilterActive && (
          <button className="btn filter-reset" type="button" onClick={resetFilters}>
            Reset
          </button>
        )}
      </section>

      {loading && (
        <div className="empty-state" style={{ padding: '40px 0' }}>
          <span className="ms" style={{ fontSize: 32, color: 'var(--color-primary)', marginBottom: 12, display: 'block' }}>sync</span>
          Loading rollouts…
        </div>
      )}

      {!loading && items.length === 0 && (
        <div className="empty-state card" style={{ padding: '48px 24px' }}>
          <span className="ms" style={{ fontSize: 40, color: 'var(--color-text-muted)', marginBottom: 12, display: 'block' }}>dynamic_feed</span>
          <h3>No rollouts match the filter</h3>
          {!includeTerminal && !includeStale && filter === '' && (
            <p>Try "Include completed" or "Include stale pending" to see historical rows.</p>
          )}
        </div>
      )}

      {items.length > 0 && (
        <>
          <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', padding: '14px 20px', borderBottom: '1px solid var(--color-border)', gap: 8 }}>
              <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>monitoring</span>
              <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 15 }}>Active Rollouts</span>
              <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', marginLeft: 4 }}>
                {items.length}
              </span>
              {loading && <span className="ms" style={{ fontSize: 16, color: 'var(--color-text-muted)', marginLeft: 'auto', animation: 'spin 1s linear infinite' }}>sync</span>}
            </div>
            <div className="table-container">
              <table className="rollouts-table">
                <thead>
                  <tr>
                    <th className="sortable" onClick={() => toggleSort('target')}>
                      Target{sortIndicator(sortKey === 'target', sortDir)}
                    </th>
                    <th className="sortable" onClick={() => toggleSort('strategy')}>
                      Strategy{sortIndicator(sortKey === 'strategy', sortDir)}
                    </th>
                    <th className="sortable" onClick={() => toggleSort('phase')}>
                      Phase{sortIndicator(sortKey === 'phase', sortDir)}
                    </th>
                    <th className="sortable" onClick={() => toggleSort('status')}>
                      Status{sortIndicator(sortKey === 'status', sortDir)}
                    </th>
                    <th className="sortable" onClick={() => toggleSort('age')}>
                      Created{sortIndicator(sortKey === 'age', sortDir)}
                    </th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {sorted.map((r) => {
                    const steps = r.strategy_snapshot.steps ?? [];
                    const currentPct = steps[r.current_phase_index]?.percent;
                    return (
                      <tr key={r.id}>
                        <td>
                          <div style={{ fontWeight: 600, fontFamily: 'var(--font-display)' }}>
                            {r.target?.summary || `${r.target_type} · ${r.id.slice(0, 8)}`}
                          </div>
                          {r.target?.kind === 'deploy' && r.target.version && (
                            <div className="row-subtitle">
                              version {r.target.version.slice(0, 12)}
                              {r.target.environment_slug ? ` · ${r.target.environment_slug}` : ''}
                            </div>
                          )}
                          {r.target?.kind === 'config' && r.target.flag_key && (
                            <div className="row-subtitle">
                              flag {r.target.flag_key}
                              {r.target.environment_slug ? ` · ${r.target.environment_slug}` : ''}
                            </div>
                          )}
                        </td>
                        <td>
                          <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', borderRadius: 4 }}>
                            {r.strategy_snapshot.name}
                          </span>
                        </td>
                        <td>
                          {r.current_phase_index + 1}/{steps.length || '—'}
                          {currentPct !== undefined && <> · {currentPct}%</>}
                        </td>
                        <td><RolloutStatusBadge status={r.status} /></td>
                        <td>
                          <div>{new Date(r.created_at).toLocaleString()}</div>
                          <div className="row-subtitle">{formatAge(r.age_seconds)} ago</div>
                        </td>
                        <td>
                          <Link to={`/orgs/${orgSlug}/rollouts/${r.id}`} className="btn btn-secondary btn-sm">Details</Link>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {totalHidden > 0 && (
            <p className="rollouts-hidden-hint">
              {totalHidden} row{totalHidden === 1 ? '' : 's'} hidden by the default filter
              {filterEcho!.hidden_terminal_count > 0 && (
                <> · {filterEcho!.hidden_terminal_count} completed</>
              )}
              {filterEcho!.hidden_stale_count > 0 && (
                <> · {filterEcho!.hidden_stale_count} stale pending</>
              )}
              . Toggle the checkboxes above to include them.
            </p>
          )}
        </>
      )}
    </div>
  );
}

// -----------------------------------------------------------------------------

function sortIndicator(active: boolean, dir: SortDir): string {
  if (!active) return '';
  return dir === 'asc' ? ' ↑' : ' ↓';
}

function cmp(a: RolloutWithTarget, b: RolloutWithTarget, key: SortKey): number {
  switch (key) {
    case 'target':
      return (a.target?.summary ?? '').localeCompare(b.target?.summary ?? '');
    case 'strategy':
      return (a.strategy_snapshot.name ?? '').localeCompare(b.strategy_snapshot.name ?? '');
    case 'phase':
      return a.current_phase_index - b.current_phase_index;
    case 'status':
      return a.status.localeCompare(b.status);
    case 'age':
      // Smaller age_seconds = newer. Sort-asc shows newest first,
      // which matches the user's expectation when clicking "Created"
      // the first time.
      return a.age_seconds - b.age_seconds;
  }
}

function formatAge(seconds: number | undefined): string {
  if (!seconds || seconds < 0) return '—';
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  const m = Math.floor(seconds / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h`;
  const d = Math.floor(h / 24);
  return `${d}d`;
}
