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
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>Rollout Groups</h1>
          <p>Coordinate dependent rollouts across multiple applications.</p>
        </div>
        <button className="btn btn-primary" onClick={() => setCreating(true)}>
          <span className="ms" style={{ fontSize: 16 }}>add</span>
          New Group
        </button>
      </div>

      {loading && (
        <div className="empty-state" style={{ padding: '40px 0' }}>
          <span className="ms" style={{ fontSize: 32, color: 'var(--color-primary)', marginBottom: 12, display: 'block' }}>sync</span>
          Loading groups…
        </div>
      )}
      {!loading && items.length === 0 && (
        <>
          <div className="empty-state card" style={{ padding: '48px 24px' }}>
            <span className="ms" style={{ fontSize: 40, color: 'var(--color-text-muted)', marginBottom: 12, display: 'block' }}>layers</span>
            <h3>No rollout groups yet</h3>
            <p>Create a group to coordinate multi-app rollouts with a shared policy.</p>
            <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => setCreating(true)}>
              <span className="ms" style={{ fontSize: 16 }}>add</span>
              New Group
            </button>
          </div>
          <div className="bento-grid">
            <div className="bento-card">
              <span className="ms bento-icon">target</span>
              <h4>Smart Targeting</h4>
              <p>Define groups by rule sets — env, region, customer tier — so rollouts ride the same membership logic as your flags.</p>
            </div>
            <div className="bento-card">
              <span className="ms bento-icon">rocket_launch</span>
              <h4>Canary Releases</h4>
              <p>Coordinate phased rollouts across siblings. Pause or cascade-abort the whole group when one app trips a guardrail.</p>
            </div>
            <div className="bento-card">
              <span className="ms bento-icon">extension</span>
              <h4>SDK Integration</h4>
              <p>Group membership is delivered to clients via the same SDKs that already evaluate flags — no new wiring required.</p>
            </div>
          </div>
        </>
      )}

      {items.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>layers</span>
            <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Rollout Groups</span>
            <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', marginLeft: 4 }}>{items.length}</span>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr><th>Name</th><th>Policy</th><th>Created</th></tr>
              </thead>
              <tbody>
                {items.map((g) => (
                  <tr key={g.id}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{
                          width: 32, height: 32, borderRadius: 8,
                          background: 'var(--color-primary-bg)', border: '1px solid rgba(99,102,241,0.2)',
                          display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                        }}>
                          <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>layers</span>
                        </div>
                        <Link to={`/orgs/${orgSlug}/rollout-groups/${g.id}`} style={{ fontWeight: 600 }}>{g.name}</Link>
                      </div>
                    </td>
                    <td><span className="badge badge-ops">{g.coordination_policy}</span></td>
                    <td className="text-secondary">{new Date(g.created_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
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
