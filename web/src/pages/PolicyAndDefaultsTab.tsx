import { useEffect, useState } from 'react';
import type { RolloutPolicy, StrategyDefault, PolicyKind, TargetType, EffectiveStrategy } from '@/types';
import { rolloutPolicyApi, strategyDefaultsApi, strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
}

const POLICIES: PolicyKind[] = ['off', 'prompt', 'mandate'];
const TARGETS: TargetType[] = ['deploy', 'config'];

function policyStyle(policy: PolicyKind): React.CSSProperties {
  switch (policy) {
    case 'mandate': return { background: 'var(--color-primary-bg)', color: 'var(--color-primary)' };
    case 'prompt': return { background: 'var(--color-warning-bg)', color: 'var(--color-warning)' };
    default: return { background: 'var(--color-bg-elevated)', color: 'var(--color-text-muted)' };
  }
}

export default function PolicyAndDefaultsTab({ orgSlug }: Props) {
  const [policies, setPolicies] = useState<RolloutPolicy[]>([]);
  const [defaults, setDefaults] = useState<StrategyDefault[]>([]);
  const [strategies, setStrategies] = useState<EffectiveStrategy[]>([]);

  const [topPolicy, setTopPolicy] = useState<PolicyKind>('off');
  const [topEnabled, setTopEnabled] = useState(false);
  const [saving, setSaving] = useState(false);

  async function load() {
    const [pol, def, strat] = await Promise.all([
      rolloutPolicyApi.list(orgSlug),
      strategyDefaultsApi.list(orgSlug),
      strategiesApi.list(orgSlug),
    ]);
    setPolicies(pol.items);
    setDefaults(def.items);
    setStrategies(strat.items);
    const top = pol.items.find((p) => !p.environment && !p.target_type);
    if (top) {
      setTopPolicy(top.policy);
      setTopEnabled(top.enabled);
    }
  }

  useEffect(() => { load(); }, [orgSlug]);

  async function saveTopPolicy() {
    setSaving(true);
    try {
      await rolloutPolicyApi.set(orgSlug, { enabled: topEnabled, policy: topPolicy });
      await load();
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* Org-wide policy */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{
          padding: '12px 20px', borderBottom: '1px solid var(--color-border)',
          display: 'flex', alignItems: 'center', gap: 8,
        }}>
          <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>policy</span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Rollout Policy (org-wide)</span>
        </div>
        <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 16 }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
            <div
              onClick={() => setTopEnabled(!topEnabled)}
              style={{
                position: 'relative', width: 36, height: 20, borderRadius: 10, cursor: 'pointer',
                background: topEnabled ? 'var(--color-primary)' : 'var(--color-bg-elevated)',
                boxShadow: topEnabled ? '0 0 8px rgba(99,102,241,0.4)' : 'none',
                transition: 'background 0.2s',
              }}
            >
              <div style={{
                position: 'absolute', top: 3,
                [topEnabled ? 'right' : 'left']: 3,
                width: 14, height: 14, borderRadius: '50%',
                background: topEnabled ? '#fff' : 'var(--color-text-muted)',
                transition: 'left 0.2s, right 0.2s',
              }} />
            </div>
            <span style={{ fontSize: 14, color: 'var(--color-text-secondary)' }}>Enable rollout control</span>
          </label>

          <div className="form-group" style={{ marginBottom: 0 }}>
            <label className="form-label">Policy</label>
            <select
              className="form-select"
              value={topPolicy}
              onChange={(e) => setTopPolicy(e.target.value as PolicyKind)}
              style={{ maxWidth: 240 }}
            >
              {POLICIES.map((p) => <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>)}
            </select>
          </div>

          <div>
            <button
              className="btn btn-primary btn-sm"
              onClick={saveTopPolicy}
              disabled={saving}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
            >
              <span className="ms" style={{ fontSize: 15 }}>{saving ? 'hourglass_empty' : 'save'}</span>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>
      </div>

      {/* Per-scope overrides */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{
          padding: '12px 20px', borderBottom: '1px solid var(--color-border)',
          display: 'flex', alignItems: 'center', gap: 8,
        }}>
          <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>tune</span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Per-scope Overrides</span>
          <span style={{
            marginLeft: 4, padding: '1px 8px', borderRadius: 20, fontSize: 11, fontWeight: 700,
            background: 'var(--color-bg-elevated)', color: 'var(--color-text-muted)',
          }}>{policies.length}</span>
        </div>

        {policies.length === 0 ? (
          <div className="empty-state" style={{ padding: '32px 24px' }}>
            <span className="ms" style={{ fontSize: 32, color: 'var(--color-text-muted)' }}>tune</span>
            <p style={{ color: 'var(--color-text-muted)', marginTop: 8, fontSize: 14 }}>No overrides configured.</p>
          </div>
        ) : (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Environment</th>
                  <th>Target</th>
                  <th>Enabled</th>
                  <th>Policy</th>
                </tr>
              </thead>
              <tbody>
                {policies.map((p) => (
                  <tr key={p.id}>
                    <td style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>{p.environment || <span style={{ color: 'var(--color-text-muted)' }}>*</span>}</td>
                    <td style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>{p.target_type || <span style={{ color: 'var(--color-text-muted)' }}>*</span>}</td>
                    <td>
                      <span style={{
                        padding: '2px 8px', borderRadius: 20, fontSize: 11, fontWeight: 700,
                        background: p.enabled ? 'var(--color-success-bg)' : 'var(--color-bg-elevated)',
                        color: p.enabled ? 'var(--color-success)' : 'var(--color-text-muted)',
                      }}>
                        {p.enabled ? 'yes' : 'no'}
                      </span>
                    </td>
                    <td>
                      <span style={{ padding: '2px 8px', borderRadius: 20, fontSize: 11, fontWeight: 700, ...policyStyle(p.policy) }}>
                        {p.policy}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Strategy defaults */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{
          padding: '12px 20px', borderBottom: '1px solid var(--color-border)',
          display: 'flex', alignItems: 'center', gap: 8,
        }}>
          <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>architecture</span>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Strategy Defaults</span>
        </div>

        {defaults.length === 0 ? (
          <div className="empty-state" style={{ padding: '32px 24px' }}>
            <span className="ms" style={{ fontSize: 32, color: 'var(--color-text-muted)' }}>architecture</span>
            <p style={{ color: 'var(--color-text-muted)', marginTop: 8, fontSize: 14 }}>No defaults set.</p>
          </div>
        ) : (
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Environment</th>
                  <th>Target</th>
                  <th>Strategy</th>
                </tr>
              </thead>
              <tbody>
                {defaults.map((d) => {
                  const strat = strategies.find((s) => s.strategy.id === d.strategy_id);
                  return (
                    <tr key={d.id}>
                      <td style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>{d.environment || <span style={{ color: 'var(--color-text-muted)' }}>*</span>}</td>
                      <td style={{ color: 'var(--color-text-secondary)', fontSize: 13 }}>{d.target_type || <span style={{ color: 'var(--color-text-muted)' }}>*</span>}</td>
                      <td style={{ fontSize: 13 }}>{strat ? strat.strategy.name : d.strategy_id.slice(0, 8)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <DefaultRowEditor orgSlug={orgSlug} strategies={strategies} onSaved={load} />
    </div>
  );
}

function DefaultRowEditor({ orgSlug, strategies, onSaved }: { orgSlug: string; strategies: EffectiveStrategy[]; onSaved: () => void }) {
  const [env, setEnv] = useState('');
  const [target, setTarget] = useState<TargetType | ''>('');
  const [strategyName, setStrategyName] = useState('');
  const [saving, setSaving] = useState(false);

  async function save() {
    if (!strategyName) return;
    setSaving(true);
    try {
      await strategyDefaultsApi.set(orgSlug, {
        environment: env || undefined,
        target_type: (target || undefined) as TargetType | undefined,
        strategy_name: strategyName,
      });
      setEnv(''); setTarget(''); setStrategyName('');
      onSaved();
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
      <div style={{
        padding: '12px 20px', borderBottom: '1px solid var(--color-border)',
        display: 'flex', alignItems: 'center', gap: 8,
      }}>
        <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>add_circle</span>
        <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Add Default</span>
      </div>
      <div style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 14 }}>
        <div className="form-group" style={{ marginBottom: 0 }}>
          <label className="form-label">Environment (optional)</label>
          <input
            className="form-input"
            value={env}
            onChange={(e) => setEnv(e.target.value)}
            placeholder="e.g. production"
            style={{ maxWidth: 300 }}
          />
        </div>
        <div className="form-group" style={{ marginBottom: 0 }}>
          <label className="form-label">Target</label>
          <select
            className="form-select"
            value={target}
            onChange={(e) => setTarget(e.target.value as TargetType | '')}
            style={{ maxWidth: 300 }}
          >
            <option value="">(any)</option>
            {TARGETS.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>
        <div className="form-group" style={{ marginBottom: 0 }}>
          <label className="form-label">Strategy</label>
          <select
            className="form-select"
            value={strategyName}
            onChange={(e) => setStrategyName(e.target.value)}
            style={{ maxWidth: 300 }}
          >
            <option value="">— pick —</option>
            {strategies.map((s) => (
              <option key={s.strategy.id} value={s.strategy.name}>{s.strategy.name}</option>
            ))}
          </select>
        </div>
        <div>
          <button
            className="btn btn-primary btn-sm"
            onClick={save}
            disabled={!strategyName || saving}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
          >
            <span className="ms" style={{ fontSize: 15 }}>{saving ? 'hourglass_empty' : 'save'}</span>
            {saving ? 'Saving…' : 'Save Default'}
          </button>
        </div>
      </div>
    </div>
  );
}
