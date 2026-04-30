import { useCallback, useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { RolloutGroup, Rollout, CoordinationPolicy } from '@/types';
import { rolloutGroupsApi } from '@/api';
import { RolloutStatusBadge } from '@/components/rollout/RolloutStatusBadge';

const POLICIES: CoordinationPolicy[] = ['independent', 'pause_on_sibling_abort', 'cascade_abort'];

const POLICY_LABEL: Record<CoordinationPolicy, string> = {
  independent: 'Independent',
  pause_on_sibling_abort: 'Pause on sibling abort',
  cascade_abort: 'Cascade abort',
};

const POLICY_DESCRIPTION: Record<CoordinationPolicy, string> = {
  independent: 'Each member rollout proceeds on its own; sibling failures have no effect.',
  pause_on_sibling_abort: 'If any sibling rollout aborts, the others pause for manual review.',
  cascade_abort: 'Abort in one member cascades — all sibling rollouts are aborted together.',
};

export default function RolloutGroupDetailPage() {
  const { orgSlug = '', id = '' } = useParams();
  const [group, setGroup] = useState<RolloutGroup | null>(null);
  const [members, setMembers] = useState<Rollout[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const r = await rolloutGroupsApi.get(orgSlug, id);
      setGroup(r.group);
      setMembers(r.members || []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [orgSlug, id]);

  useEffect(() => {
    load();
  }, [load]);

  async function savePolicy(next: CoordinationPolicy) {
    if (!group) return;
    setSaving(true);
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
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="empty-state" style={{ padding: '40px 0' }}>
        <span
          className="ms"
          style={{
            fontSize: 32,
            color: 'var(--color-primary)',
            marginBottom: 12,
            display: 'block',
          }}
        >
          sync
        </span>
        Loading group…
      </div>
    );
  }
  if (error) return <div className="page-error">{error}</div>;
  if (!group) return <div className="page-error">Group not found.</div>;

  const activeCount = members.filter((m) =>
    ['active', 'paused', 'awaiting_approval', 'pending'].includes(m.status),
  ).length;
  const succeededCount = members.filter((m) => m.status === 'succeeded').length;
  const failedCount = members.filter((m) => ['aborted', 'rolled_back'].includes(m.status)).length;

  return (
    <div className="page">
      <div className="breadcrumb">
        <Link to={`/orgs/${orgSlug}/rollout-groups`} className="breadcrumb-link">
          Rollout Groups
        </Link>
        <span className="breadcrumb-sep">/</span>
        <span className="breadcrumb-current">{group.name}</span>
      </div>

      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              style={{
                width: 40,
                height: 40,
                borderRadius: 10,
                background: 'var(--color-primary-bg)',
                border: '1px solid rgba(99,102,241,0.25)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              <span className="ms" style={{ fontSize: 20, color: 'var(--color-primary)' }}>
                layers
              </span>
            </div>
            <div>
              <h1 style={{ margin: 0 }}>{group.name}</h1>
              {group.description ? (
                <p
                  style={{ margin: '4px 0 0', color: 'var(--color-text-secondary)', fontSize: 13 }}
                >
                  {group.description}
                </p>
              ) : (
                <p
                  style={{
                    margin: '4px 0 0',
                    color: 'var(--color-text-muted)',
                    fontSize: 13,
                    fontStyle: 'italic',
                  }}
                >
                  No description
                </p>
              )}
            </div>
          </div>
        </div>
        <span className="badge badge-ops">{POLICY_LABEL[group.coordination_policy]}</span>
      </div>

      <div className="info-cards" style={{ gridTemplateColumns: 'repeat(4, 1fr)', marginTop: 20 }}>
        <div className="info-card">
          <div className="info-card-label">Members</div>
          <div className="info-card-value">{members.length}</div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Active</div>
          <div
            className="info-card-value"
            style={{ color: activeCount > 0 ? 'var(--color-primary)' : 'var(--color-text)' }}
          >
            {activeCount}
          </div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Succeeded</div>
          <div
            className="info-card-value"
            style={{ color: succeededCount > 0 ? 'var(--color-success)' : 'var(--color-text)' }}
          >
            {succeededCount}
          </div>
        </div>
        <div className="info-card">
          <div className="info-card-label">Failed</div>
          <div
            className="info-card-value"
            style={{ color: failedCount > 0 ? 'var(--color-danger)' : 'var(--color-text)' }}
          >
            {failedCount}
          </div>
        </div>
      </div>

      <section className="card" style={{ marginBottom: 20 }}>
        <div className="card-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
              hub
            </span>
            <span
              className="card-title"
              style={{ fontFamily: 'var(--font-display)', fontWeight: 700 }}
            >
              Coordination Policy
            </span>
          </div>
          {!editing && (
            <button
              className="btn btn-sm btn-secondary"
              onClick={() => setEditing(true)}
              disabled={saving}
            >
              <span className="ms" style={{ fontSize: 14 }}>
                edit
              </span>
              Edit
            </button>
          )}
        </div>

        {!editing ? (
          <div>
            <div style={{ fontWeight: 600, marginBottom: 6 }}>
              {POLICY_LABEL[group.coordination_policy]}
            </div>
            <div style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
              {POLICY_DESCRIPTION[group.coordination_policy]}
            </div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {POLICIES.map((p) => (
              <label
                key={p}
                style={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: 10,
                  padding: 12,
                  border: `1px solid ${p === group.coordination_policy ? 'var(--color-primary)' : 'var(--color-border)'}`,
                  borderRadius: 'var(--radius-md)',
                  background:
                    p === group.coordination_policy ? 'var(--color-primary-bg)' : 'var(--color-bg)',
                  cursor: 'pointer',
                }}
              >
                <input
                  type="radio"
                  name="policy"
                  value={p}
                  defaultChecked={p === group.coordination_policy}
                  onChange={() => savePolicy(p)}
                  disabled={saving}
                  style={{ marginTop: 3 }}
                />
                <div>
                  <div style={{ fontWeight: 600, fontSize: 13 }}>{POLICY_LABEL[p]}</div>
                  <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 2 }}>
                    {POLICY_DESCRIPTION[p]}
                  </div>
                </div>
              </label>
            ))}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 4 }}>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => setEditing(false)}
                disabled={saving}
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </section>

      <section className="card" style={{ padding: 0, overflow: 'hidden' }}>
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
            dynamic_feed
          </span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>
            Member Rollouts
          </span>
          {members.length > 0 && (
            <span
              className="badge"
              style={{
                background: 'var(--color-primary-bg)',
                color: 'var(--color-primary)',
                marginLeft: 4,
              }}
            >
              {members.length}
            </span>
          )}
        </div>

        {members.length === 0 ? (
          <div className="empty-state" style={{ padding: '48px 24px' }}>
            <span
              className="ms"
              style={{
                fontSize: 40,
                color: 'var(--color-text-muted)',
                marginBottom: 12,
                display: 'block',
              }}
            >
              dynamic_feed
            </span>
            <h3>No rollouts attached</h3>
            <p>Attach rollouts to this group from the Active Rollouts page.</p>
          </div>
        ) : (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Rollout</th>
                  <th>Target</th>
                  <th>Status</th>
                  <th>Started</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {members.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <Link
                        to={`/orgs/${orgSlug}/rollouts/${r.id}`}
                        style={{
                          fontFamily: 'var(--font-mono)',
                          fontSize: 12,
                          color: 'var(--color-primary)',
                          fontWeight: 500,
                        }}
                      >
                        {r.id.slice(0, 8)}
                      </Link>
                    </td>
                    <td>
                      <span className="badge badge-ops">{r.target_type}</span>
                    </td>
                    <td>
                      <RolloutStatusBadge status={r.status} />
                    </td>
                    <td className="text-secondary" style={{ fontSize: 12 }}>
                      {r.created_at ? new Date(r.created_at).toLocaleString() : '—'}
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <Link
                        to={`/orgs/${orgSlug}/rollouts/${r.id}`}
                        className="btn btn-sm btn-secondary"
                      >
                        <span className="ms" style={{ fontSize: 14 }}>
                          open_in_new
                        </span>
                        Open
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
