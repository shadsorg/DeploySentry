import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutGroup, CoordinationPolicy } from '@/types';
import { rolloutGroupsApi } from '@/api';

const POLICIES: CoordinationPolicy[] = ['independent', 'pause_on_sibling_abort', 'cascade_abort'];

export default function RolloutGroupsPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<RolloutGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [policy, setPolicy] = useState<CoordinationPolicy>('independent');

  async function load() {
    setLoading(true);
    try {
      const r = await rolloutGroupsApi.list(orgSlug);
      setItems(r.items || []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, [orgSlug]);

  async function handleCreate() {
    if (!name.trim()) return;
    try {
      await rolloutGroupsApi.create(orgSlug, { name, description, coordination_policy: policy });
      setCreating(false);
      setName(''); setDescription(''); setPolicy('independent');
      await load();
    } catch (e) {
      alert(`Create failed: ${e}`);
    }
  }

  return (
    <div className="page">
      <header className="page-header">
        <h1>Rollout Groups</h1>
        <div className="header-actions">
          <button className="btn btn-primary" onClick={() => setCreating(true)}>New Group</button>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {!loading && items.length === 0 && <p className="empty-state">No rollout groups yet.</p>}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr><th>Name</th><th>Policy</th><th>Created</th></tr>
          </thead>
          <tbody>
            {items.map((g) => (
              <tr key={g.id}>
                <td><Link to={`/orgs/${orgSlug}/rollout-groups/${g.id}`}>{g.name}</Link></td>
                <td>{g.coordination_policy}</td>
                <td>{new Date(g.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {creating && (
        <div className="modal-backdrop" onClick={() => setCreating(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>New Rollout Group</h3>
            <label>Name<input value={name} onChange={(e) => setName(e.target.value)} /></label>
            <label>Description<input value={description} onChange={(e) => setDescription(e.target.value)} /></label>
            <label>Coordination Policy
              <select value={policy} onChange={(e) => setPolicy(e.target.value as CoordinationPolicy)}>
                {POLICIES.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </label>
            <div className="modal-actions">
              <button onClick={() => setCreating(false)}>Cancel</button>
              <button className="btn-primary" onClick={handleCreate}>Create</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
