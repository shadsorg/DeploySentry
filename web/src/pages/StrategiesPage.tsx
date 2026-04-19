import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
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
      <header className="page-header">
        <h1>Rollout Strategies</h1>
        <div className="header-actions">
          <label className="btn">
            Import YAML
            <input
              type="file"
              accept=".yaml,.yml"
              style={{ display: 'none' }}
              onChange={(e) => e.target.files?.[0] && handleImportYAML(e.target.files[0])}
            />
          </label>
          <button className="btn btn-primary" onClick={() => setEditing('new')}>
            New Strategy
          </button>
        </div>
      </header>

      {loading && <p>Loading…</p>}
      {error && <p className="error">{error}</p>}

      {!loading && items.length === 0 && (
        <p className="empty-state">No strategies yet. Click "New Strategy" to create one.</p>
      )}

      {items.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Target</th>
              <th>Version</th>
              <th>Origin</th>
              <th>System</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((eff) => (
              <tr key={eff.strategy.id}>
                <td>
                  <Link to="#" onClick={(e) => { e.preventDefault(); setEditing(eff.strategy.name); }}>
                    {eff.strategy.name}
                  </Link>
                </td>
                <td>{eff.strategy.target_type}</td>
                <td>v{eff.strategy.version}</td>
                <td>
                  {eff.origin_scope.type}
                  {eff.is_inherited && ' (inh)'}
                </td>
                <td>{eff.strategy.is_system ? 'yes' : 'no'}</td>
                <td>
                  <button
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
                    <button onClick={() => handleDelete(eff.strategy.name)}>Delete</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
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
