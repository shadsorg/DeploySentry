import { useCallback, useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutStatus, RolloutWithTarget } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

const STATUS_FILTERS: RolloutStatus[] = [
  'active',
  'paused',
  'awaiting_approval',
  'pending',
  'succeeded',
  'rolled_back',
  'aborted',
];

export default function RolloutsPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<RolloutWithTarget[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<RolloutStatus | ''>('');
  const [includeTerminal, setIncludeTerminal] = useState(false);
  const [includeStale, setIncludeStale] = useState(false);
  const [staleCutoffHours, setStaleCutoffHours] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await rolloutsApi.list(orgSlug, {
        status: filter || undefined,
        limit: 100,
        include_terminal: includeTerminal,
        include_stale: includeStale,
      });
      setItems(r.items || []);
      setStaleCutoffHours(r.filter?.stale_cutoff_hours ?? null);
    } finally {
      setLoading(false);
    }
  }, [orgSlug, filter, includeTerminal, includeStale]);

  useEffect(() => { load(); }, [load]);

  // Auto-refresh active rollouts.
  useEffect(() => {
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollouts</h1>
        <div className="header-actions" style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
          <select value={filter} onChange={(e) => setFilter(e.target.value as RolloutStatus | '')}>
            <option value="">In progress only</option>
            {STATUS_FILTERS.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
          <label style={{ fontSize: 12 }}>
            <input
              type="checkbox"
              checked={includeTerminal}
              onChange={(e) => setIncludeTerminal(e.target.checked)}
              style={{ marginRight: 4 }}
            />
            Include completed
          </label>
          <label style={{ fontSize: 12 }} title={staleCutoffHours ? `Pending for more than ${staleCutoffHours}h` : ''}>
            <input
              type="checkbox"
              checked={includeStale}
              onChange={(e) => setIncludeStale(e.target.checked)}
              style={{ marginRight: 4 }}
            />
            Include stale pending
          </label>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {!loading && items.length === 0 && (
        <p className="empty-state">
          No rollouts match the filter.
          {!includeTerminal && !includeStale && filter === '' && (
            <>
              {' '}Try enabling "Include completed" or "Include stale pending" to see historical rows.
            </>
          )}
        </p>
      )}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>Target</th>
              <th>Strategy</th>
              <th>Phase</th>
              <th>Status</th>
              <th>Age</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {items.map((r) => {
              const steps = r.strategy_snapshot.steps ?? [];
              const currentPct = steps[r.current_phase_index]?.percent;
              return (
                <tr key={r.id}>
                  <td>
                    <div style={{ fontWeight: 500 }}>
                      {r.target?.summary || `${r.target_type} · ${r.id.slice(0, 8)}`}
                    </div>
                    {r.target?.kind === 'deploy' && r.target.version && (
                      <div style={{ fontSize: 11, color: '#778' }}>
                        version {r.target.version.slice(0, 12)}
                        {r.target.environment_slug ? ` · ${r.target.environment_slug}` : ''}
                      </div>
                    )}
                    {r.target?.kind === 'config' && r.target.flag_key && (
                      <div style={{ fontSize: 11, color: '#778' }}>
                        flag {r.target.flag_key}
                        {r.target.environment_slug ? ` · ${r.target.environment_slug}` : ''}
                      </div>
                    )}
                  </td>
                  <td>{r.strategy_snapshot.name}</td>
                  <td>
                    {r.current_phase_index + 1}/{steps.length || '—'}
                    {currentPct !== undefined && <> · {currentPct}%</>}
                  </td>
                  <td><RolloutStatusBadge status={r.status} /></td>
                  <td title={new Date(r.created_at).toLocaleString()}>
                    {formatAge(r.age_seconds)}
                  </td>
                  <td>
                    <Link to={`/orgs/${orgSlug}/rollouts/${r.id}`} className="btn">Open</Link>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </div>
  );
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
