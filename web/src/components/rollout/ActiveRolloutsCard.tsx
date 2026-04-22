import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import type { Rollout } from '@/types';
import { rolloutsApi } from '@/api';
import { RolloutStatusBadge } from './RolloutStatusBadge';

interface Props {
  orgSlug: string;
}

export function ActiveRolloutsCard({ orgSlug }: Props) {
  const [items, setItems] = useState<Rollout[]>([]);
  const [loading, setLoading] = useState(true);

  async function load() {
    try {
      const r = await rolloutsApi.list(orgSlug, { status: 'active', limit: 5 });
      setItems(r.items || []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const t = setInterval(load, 10000);
    return () => clearInterval(t);
  }, [orgSlug]);

  // Hide entirely if nothing active — avoids dashboard noise.
  if (!loading && items.length === 0) return null;

  return (
    <section className="card">
      <header className="card-header">
        <h3>Active Rollouts</h3>
        <Link to={`/orgs/${orgSlug}/rollouts`} className="card-link">
          View all →
        </Link>
      </header>
      {loading && <p>Loading…</p>}
      {items.length > 0 && (
        <ul className="active-rollouts-list">
          {items.map((r) => {
            const step = r.strategy_snapshot.steps[r.current_phase_index];
            return (
              <li key={r.id}>
                <Link to={`/orgs/${orgSlug}/rollouts/${r.id}`}>
                  <span className="rollout-target">
                    {r.target_type === 'deploy' ? '⬢' : '⚑'} {r.strategy_snapshot.name}
                  </span>
                  <span className="rollout-phase">
                    phase {r.current_phase_index + 1}/{r.strategy_snapshot.steps.length}
                    {step ? ` • ${step.percent}%` : ''}
                  </span>
                  <RolloutStatusBadge status={r.status} />
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
