import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import type { EffectiveStrategy } from '@/types';
import { strategiesApi } from '@/api';
import { StrategyEditor } from './StrategyEditor';

export default function StrategiesPage() {
  const { orgSlug = '' } = useParams();
  const [items, setItems] = useState<EffectiveStrategy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<'new' | string | null>(null); // 'new' or strategy name

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const r = await strategiesApi.list(orgSlug);
      setItems(r.items);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [orgSlug]);

  async function handleDelete(name: string) {
    if (!confirm(`Delete strategy "${name}"?`)) return;
    try {
      await strategiesApi.delete(orgSlug, name);
      await load();
    } catch (e) {
      alert(`Delete failed: ${e}`);
    }
  }

  async function handleImportYAML(file: File) {
    const yaml = await file.text();
    try {
      await strategiesApi.importYAML(orgSlug, yaml);
      await load();
    } catch (e) {
      alert(`Import failed: ${e}`);
    }
  }

  return (
    <div className="page">
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>Rollout Strategies</h1>
          <p>Configure phased rollout strategies for deployments and flag changes.</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <label className="btn btn-secondary">
            <span className="ms" style={{ fontSize: 16 }}>upload</span>
            Import YAML
            <input
              type="file"
              accept=".yaml,.yml"
              style={{ display: 'none' }}
              onChange={(e) => e.target.files?.[0] && handleImportYAML(e.target.files[0])}
            />
          </label>
          <button className="btn btn-primary" onClick={() => setEditing('new')}>
            <span className="ms" style={{ fontSize: 16 }}>add</span>
            New Strategy
          </button>
        </div>
      </div>

      {loading && (
        <div className="empty-state" style={{ padding: '40px 0' }}>
          <span className="ms" style={{ fontSize: 32, color: 'var(--color-primary)', marginBottom: 12, display: 'block' }}>sync</span>
          Loading strategies…
        </div>
      )}
      {error && <p className="form-error">{error}</p>}

      {!loading && items.length === 0 && (
        <div className="empty-state card" style={{ padding: '48px 24px' }}>
          <span className="ms" style={{ fontSize: 40, color: 'var(--color-text-muted)', marginBottom: 12, display: 'block' }}>architecture</span>
          <h3>No strategies yet</h3>
          <p>Create a strategy to control how deployments and flag changes roll out.</p>
          <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => setEditing('new')}>
            <span className="ms" style={{ fontSize: 16 }}>add</span>
            New Strategy
          </button>
        </div>
      )}

      {items.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>architecture</span>
            <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Strategies</span>
            <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', marginLeft: 4 }}>{items.length}</span>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Target</th>
                  <th>Version</th>
                  <th>Origin</th>
                  <th>System</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {items.map((eff) => (
                  <tr key={eff.strategy.id}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{
                          width: 32, height: 32, borderRadius: 8,
                          background: 'var(--color-primary-bg)', border: '1px solid rgba(99,102,241,0.2)',
                          display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                        }}>
                          <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>architecture</span>
                        </div>
                        <button
                          style={{ fontWeight: 600, background: 'none', border: 'none', color: 'var(--color-text)', cursor: 'pointer', padding: 0, textAlign: 'left' }}
                          onClick={() => setEditing(eff.strategy.name)}
                        >
                          {eff.strategy.name}
                        </button>
                      </div>
                    </td>
                    <td>
                      <span className="badge badge-ops">{eff.strategy.target_type}</span>
                    </td>
                    <td className="text-secondary">v{eff.strategy.version}</td>
                    <td>
                      <span className="badge">
                        {eff.origin_scope.type}
                        {eff.is_inherited && ' · inherited'}
                      </span>
                    </td>
                    <td>
                      {eff.strategy.is_system ? (
                        <span className="badge badge-permission">system</span>
                      ) : (
                        <span className="badge" style={{ color: 'var(--color-text-muted)' }}>custom</span>
                      )}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button
                          className="btn btn-sm btn-secondary"
                          onClick={async () => {
                            const yaml = await strategiesApi.exportYAML(orgSlug, eff.strategy.name);
                            const blob = new Blob([yaml], { type: 'application/yaml' });
                            const url = URL.createObjectURL(blob);
                            const a = document.createElement('a');
                            a.href = url;
                            a.download = `${eff.strategy.name}.yaml`;
                            a.click();
                            URL.revokeObjectURL(url);
                          }}
                        >
                          Export
                        </button>
                        {!eff.strategy.is_system && !eff.is_inherited && (
                          <button className="btn-icon" title="Delete" onClick={() => handleDelete(eff.strategy.name)}>
                            <span className="ms" style={{ fontSize: 16, color: 'var(--color-danger)' }}>delete</span>
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {editing && (
        <StrategyEditor
          orgSlug={orgSlug}
          strategyName={editing === 'new' ? null : editing}
          onClose={() => { setEditing(null); load(); }}
        />
      )}
    </div>
  );
}
