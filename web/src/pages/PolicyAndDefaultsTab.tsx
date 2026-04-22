import { useEffect, useState } from 'react';
import type {
  RolloutPolicy,
  StrategyDefault,
  PolicyKind,
  TargetType,
  EffectiveStrategy,
} from '@/types';
import { rolloutPolicyApi, strategyDefaultsApi, strategiesApi } from '@/api';

interface Props {
  orgSlug: string;
}

const POLICIES: PolicyKind[] = ['off', 'prompt', 'mandate'];
const TARGETS: TargetType[] = ['deploy', 'config'];

export default function PolicyAndDefaultsTab({ orgSlug }: Props) {
  const [policies, setPolicies] = useState<RolloutPolicy[]>([]);
  const [defaults, setDefaults] = useState<StrategyDefault[]>([]);
  const [strategies, setStrategies] = useState<EffectiveStrategy[]>([]);

  // Simple top-level policy row editor.
  const [topPolicy, setTopPolicy] = useState<PolicyKind>('off');
  const [topEnabled, setTopEnabled] = useState(false);

  async function load() {
    const [pol, def, strat] = await Promise.all([
      rolloutPolicyApi.list(orgSlug),
      strategyDefaultsApi.list(orgSlug),
      strategiesApi.list(orgSlug),
    ]);
    setPolicies(pol.items);
    setDefaults(def.items);
    setStrategies(strat.items);
    // Use the org-wide policy row (no env, no target_type) as the top-level pick.
    const top = pol.items.find((p) => !p.environment && !p.target_type);
    if (top) {
      setTopPolicy(top.policy);
      setTopEnabled(top.enabled);
    }
  }

  useEffect(() => {
    load();
  }, [orgSlug]);

  async function saveTopPolicy() {
    await rolloutPolicyApi.set(orgSlug, { enabled: topEnabled, policy: topPolicy });
    await load();
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div className="card">
        <div className="card-header">
          <span className="card-title">Rollout Policy (org-wide)</span>
        </div>
        <div className="form-group">
          <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={topEnabled}
              onChange={(e) => setTopEnabled(e.target.checked)}
            />
            <span className="text-sm">Enable rollout control</span>
          </label>
        </div>
        <div className="form-group">
          <label className="form-label">Policy</label>
          <select
            className="form-select"
            value={topPolicy}
            onChange={(e) => setTopPolicy(e.target.value as PolicyKind)}
          >
            {POLICIES.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </div>
        <button className="btn btn-primary btn-sm" onClick={saveTopPolicy}>
          Save
        </button>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">Per-scope Overrides ({policies.length})</span>
        </div>
        {policies.length === 0 ? (
          <p className="text-muted">No overrides.</p>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Env</th>
                  <th>Target</th>
                  <th>Enabled</th>
                  <th>Policy</th>
                </tr>
              </thead>
              <tbody>
                {policies.map((p) => (
                  <tr key={p.id}>
                    <td>{p.environment || '*'}</td>
                    <td>{p.target_type || '*'}</td>
                    <td>{p.enabled ? 'yes' : 'no'}</td>
                    <td>{p.policy}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">Strategy Defaults</span>
        </div>
        {defaults.length === 0 ? (
          <p className="text-muted">No defaults set.</p>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Env</th>
                  <th>Target</th>
                  <th>Strategy</th>
                </tr>
              </thead>
              <tbody>
                {defaults.map((d) => {
                  const strat = strategies.find((s) => s.strategy.id === d.strategy_id);
                  return (
                    <tr key={d.id}>
                      <td>{d.environment || '*'}</td>
                      <td>{d.target_type || '*'}</td>
                      <td>{strat ? strat.strategy.name : d.strategy_id.slice(0, 8)}</td>
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

function DefaultRowEditor({
  orgSlug,
  strategies,
  onSaved,
}: {
  orgSlug: string;
  strategies: EffectiveStrategy[];
  onSaved: () => void;
}) {
  const [env, setEnv] = useState('');
  const [target, setTarget] = useState<TargetType | ''>('');
  const [strategyName, setStrategyName] = useState('');

  async function save() {
    if (!strategyName) return;
    await strategyDefaultsApi.set(orgSlug, {
      environment: env || undefined,
      target_type: (target || undefined) as TargetType | undefined,
      strategy_name: strategyName,
    });
    setEnv('');
    setTarget('');
    setStrategyName('');
    onSaved();
  }

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">Add Default</span>
      </div>
      <div className="form-group">
        <label className="form-label">Env (optional)</label>
        <input
          className="form-input"
          value={env}
          onChange={(e) => setEnv(e.target.value)}
          placeholder="e.g. production"
        />
      </div>
      <div className="form-group">
        <label className="form-label">Target</label>
        <select
          className="form-select"
          value={target}
          onChange={(e) => setTarget(e.target.value as TargetType | '')}
        >
          <option value="">(any)</option>
          {TARGETS.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>
      <div className="form-group">
        <label className="form-label">Strategy</label>
        <select
          className="form-select"
          value={strategyName}
          onChange={(e) => setStrategyName(e.target.value)}
        >
          <option value="">— pick —</option>
          {strategies.map((s) => (
            <option key={s.strategy.id} value={s.strategy.name}>
              {s.strategy.name}
            </option>
          ))}
        </select>
      </div>
      <button className="btn btn-primary btn-sm" onClick={save} disabled={!strategyName}>
        Save
      </button>
    </div>
  );
}
