import { useCallback, useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutGroup, Rollout, CoordinationPolicy } from '@/types';
import { rolloutGroupsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

const POLICIES: CoordinationPolicy[] = ['independent', 'pause_on_sibling_abort', 'cascade_abort'];

export default function RolloutGroupDetailPage() {
  const { orgSlug = '', id = '' } = useParams();
  const [group, setGroup] = useState<RolloutGroup | null>(null);
  const [members, setMembers] = useState<Rollout[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await rolloutGroupsApi.get(orgSlug, id);
      setGroup(r.group);
      setMembers(r.members || []);
    } finally {
      setLoading(false);
    }
  }, [orgSlug, id]);

  useEffect(() => { load(); }, [load]);

  async function savePolicy(next: CoordinationPolicy) {
    if (!group) return;
    try {
      await rolloutGroupsApi.update(orgSlug, group.id, {
        name: group.name,
        description: group.description,
        coordination_policy: next,
      });
      setEditing(false);
      await load();
    } catch (e) {
      alert(`Update failed: ${e}`);
    }
  }

  if (loading) return <div className="page"><p>Loading…</p></div>;
  if (!group) return <div className="page"><p>Group not found.</p></div>;

  return (
    <div className="page">
      <header className="page-header">
        <h1>{group.name}</h1>
      </header>

      <section className="card">
        <p>{group.description || <em>No description.</em>}</p>
        <p>
          Coordination: <strong>{group.coordination_policy}</strong>
          {' '}<button onClick={() => setEditing(!editing)}>Edit</button>
        </p>
        {editing && (
          <select
            defaultValue={group.coordination_policy}
            onChange={(e) => savePolicy(e.target.value as CoordinationPolicy)}
          >
            {POLICIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        )}
      </section>

      <section className="card">
        <h3>Member Rollouts ({members.length})</h3>
        {members.length === 0 ? (
          <p className="empty-state">No rollouts attached.</p>
        ) : (
          <table className="data-table">
            <thead>
              <tr><th>ID</th><th>Target</th><th>Status</th></tr>
            </thead>
            <tbody>
              {members.map((r) => (
                <tr key={r.id}>
                  <td><Link to={`/orgs/${orgSlug}/rollouts/${r.id}`}>{r.id.slice(0, 8)}</Link></td>
                  <td>{r.target_type}</td>
                  <td><RolloutStatusBadge status={r.status} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
