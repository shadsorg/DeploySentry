import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Rollout, RolloutStatus } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

const STATUS_FILTERS: RolloutStatus[] = ['active', 'paused', 'awaiting_approval', 'succeeded', 'rolled_back'];

export default function RolloutsPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<Rollout[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<RolloutStatus | ''>('');

  async function load() {
    setLoading(true);
    try {
      const r = await rolloutsApi.list(orgSlug, { status: filter || undefined, limit: 100 });
      setItems(r.items || []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, [orgSlug, filter]);

  // Auto-refresh active rollouts.
  useEffect(() => {
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [orgSlug, filter]);

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollouts</h1>
        <div className="header-actions">
          <select value={filter} onChange={(e) => setFilter(e.target.value as RolloutStatus | '')}>
            <option value="">All statuses</option>
            {STATUS_FILTERS.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {!loading && items.length === 0 && <p className="empty-state">No rollouts match the filter.</p>}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Target</th>
              <th>Strategy</th>
              <th>Phase</th>
              <th>Status</th>
              <th>Started</th>
            </tr>
          </thead>
          <tbody>
            {items.map((r) => (
              <tr key={r.id}>
                <td>
                  <Link to={`/orgs/${orgSlug}/rollouts/${r.id}`}>{r.id.slice(0, 8)}</Link>
                </td>
                <td>
                  {r.target_type === 'deploy'
                    ? `deploy ${r.target_ref.deployment_id?.slice(0, 8) ?? ''}`
                    : `rule ${r.target_ref.rule_id?.slice(0, 8) ?? ''}`}
                </td>
                <td>{r.strategy_snapshot.name}</td>
                <td>
                  {r.current_phase_index + 1}/{r.strategy_snapshot.steps.length}
                  {' • '}
                  {r.strategy_snapshot.steps[r.current_phase_index]?.percent ?? '?'}%
                </td>
                <td><RolloutStatusBadge status={r.status} /></td>
                <td>{new Date(r.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
