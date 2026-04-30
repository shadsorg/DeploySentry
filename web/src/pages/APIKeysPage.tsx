import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { ApiKey } from '@/types';
import { apiKeysApi } from '@/api';
import { useEnvironments, useProjects, useApps } from '../hooks/useEntities';

// Must stay in sync with internal/models/api_key.go AllAPIKeyScopes().
const AVAILABLE_SCOPES = [
  'flags:read',
  'flags:write',
  'deploys:read',
  'deploys:write',
  'releases:read',
  'releases:write',
  'status:write',
  'apikey:manage',
  'admin',
];

function scopeBadgeClass(scope: string): string {
  if (scope === 'admin') return 'badge badge-experiment';
  if (scope === 'apikey:manage') return 'badge badge-permission';
  if (scope === 'status:write') return 'badge badge-ops';
  if (scope.startsWith('flags')) return 'badge badge-feature';
  if (scope.startsWith('deploys')) return 'badge badge-release';
  if (scope.startsWith('releases')) return 'badge badge-release';
  return 'badge badge-ops';
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export default function APIKeysPage() {
  const { orgSlug } = useParams();
  const { environments } = useEnvironments(orgSlug);
  const { projects } = useProjects(orgSlug);
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  // Create form
  const [newName, setNewName] = useState('');
  const [newScopes, setNewScopes] = useState<string[]>([]);
  const [selectedEnvIds, setSelectedEnvIds] = useState<string[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>('');
  const [selectedAppId, setSelectedAppId] = useState<string>('');
  const selectedProjectSlug = projects.find((p) => p.id === selectedProjectId)?.slug;
  const { apps } = useApps(orgSlug, selectedProjectSlug);
  // Revoke confirm
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null);

  // Reset selected app when project changes
  useEffect(() => {
    setSelectedAppId('');
  }, [selectedProjectId]);

  async function fetchKeys() {
    setLoading(true);
    setError(null);
    try {
      const result = await apiKeysApi.list();
      setKeys(result.api_keys);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load API keys');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchKeys();
  }, []);

  async function handleCreate() {
    if (!newName.trim() || newScopes.length === 0) return;
    try {
      const result = await apiKeysApi.create({
        name: newName.trim(),
        scopes: newScopes,
        environment_ids: selectedEnvIds.length > 0 ? selectedEnvIds : undefined,
        project_id: selectedProjectId || undefined,
        application_id: selectedAppId || undefined,
      });
      setRevealedKey(result.plaintext_key);
      setNewName('');
      setNewScopes([]);
      setSelectedEnvIds([]);
      setSelectedProjectId('');
      setSelectedAppId('');
      setShowCreate(false);
      await fetchKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    }
  }

  function toggleScope(scope: string) {
    setNewScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope],
    );
  }

  async function handleRevoke(keyId: string) {
    try {
      await apiKeysApi.revoke(keyId);
      setConfirmRevoke(null);
      await fetchKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke API key');
    }
  }

  return (
    <div>
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>API Keys</h1>
          <p>Authenticate programmatic access. Scope keys to projects, apps, or environments.</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(!showCreate)}>
          <span className="ms" style={{ fontSize: 16 }}>{showCreate ? 'close' : 'key'}</span>
          {showCreate ? 'Cancel' : 'Create API Key'}
        </button>
      </div>

      <div className="stat-grid" style={{ marginBottom: 24, marginTop: 20, gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))' }}>
        <div className="stat-card">
          <div className="stat-label">Active Keys</div>
          <div className="stat-value" style={{ fontFamily: 'var(--font-display)', color: 'var(--color-primary)' }}>{keys.length}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Scoped Keys</div>
          <div className="stat-value" style={{ fontFamily: 'var(--font-display)' }}>{keys.filter((k) => k.project_id || k.application_id).length}</div>
        </div>
      </div>

      <div className="code-block" style={{ marginBottom: 24 }}>
        <div className="code-header">
          <span className="code-lang">CLI</span>
          <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>Or use the form below</span>
        </div>
        <code style={{ display: 'block' }}>$ deploysentry apikeys create --name &lt;name&gt;</code>
      </div>

      {showCreate && (
        <div className="inline-form">
          <div className="form-group">
            <label>Name</label>
            <input
              type="text"
              className="form-input"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="e.g. Production Backend"
              required
            />
          </div>

          <div className="form-group">
            <label>Scopes</label>
            <div className="checkbox-group">
              {AVAILABLE_SCOPES.map((scope) => (
                <label key={scope}>
                  <input
                    type="checkbox"
                    checked={newScopes.includes(scope)}
                    onChange={() => toggleScope(scope)}
                  />
                  {scope}
                </label>
              ))}
            </div>
          </div>

          <div className="form-group">
            <label>Project Scope</label>
            <select
              className="form-input"
              value={selectedProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
            >
              <option value="">All Projects</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
            <p className="text-muted" style={{ fontSize: '0.85rem', marginTop: 4 }}>
              Select "All Projects" for org-wide access
            </p>
          </div>

          <div className="form-group">
            <label>Application Scope</label>
            <select
              className="form-input"
              value={selectedAppId}
              onChange={(e) => setSelectedAppId(e.target.value)}
              disabled={!selectedProjectId}
            >
              {!selectedProjectId ? (
                <option value="">Select a project first</option>
              ) : (
                <>
                  <option value="">All Applications</option>
                  {apps.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </>
              )}
            </select>
            {selectedProjectId && (
              <p className="text-muted" style={{ fontSize: '0.85rem', marginTop: 4 }}>
                Select "All Applications" for project-wide access
              </p>
            )}
          </div>

          <div style={{ marginBottom: 16 }}>
            <label className="form-label">Environment Restrictions</label>
            <p className="text-muted" style={{ fontSize: '0.85rem', marginBottom: 8 }}>
              Leave all unchecked for unrestricted access to all environments.
            </p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
              {environments.map((env) => (
                <label key={env.id} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <input
                    type="checkbox"
                    checked={selectedEnvIds.includes(env.id)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setSelectedEnvIds((prev) => [...prev, env.id]);
                      } else {
                        setSelectedEnvIds((prev) => prev.filter((id) => id !== env.id));
                      }
                    }}
                  />
                  {env.name}
                </label>
              ))}
            </div>
          </div>

          <button className="btn btn-primary" onClick={handleCreate}>
            Create Key
          </button>
        </div>
      )}

      {revealedKey && (
        <div className="key-reveal">
          <code>{revealedKey}</code>
          <button
            className="btn btn-sm btn-secondary"
            onClick={() => {
              navigator.clipboard.writeText(revealedKey);
            }}
          >
            Copy
          </button>
          <button className="btn btn-sm btn-secondary" onClick={() => setRevealedKey(null)}>
            Dismiss
          </button>
        </div>
      )}

      {error && (
        <p className="form-error" style={{ marginBottom: 8 }}>
          {error}
        </p>
      )}

      {loading ? (
        <div className="empty-state">Loading API keys…</div>
      ) : keys.length === 0 ? (
        <div className="empty-state card" style={{ padding: '48px 24px' }}>
          <span className="ms" style={{ fontSize: 40, color: 'var(--color-text-muted)', marginBottom: 12, display: 'block' }}>vpn_key</span>
          <h3>No API keys yet</h3>
          <p>Create one to integrate with DeploySentry.</p>
        </div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>shield</span>
            <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14 }}>Active Access Tokens</span>
            <span className="badge" style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', marginLeft: 4 }}>
              {keys.length}
            </span>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Prefix</th>
                  <th>Scopes</th>
                  <th>Environments</th>
                  <th>Created</th>
                  <th>Last Used</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {keys.map((key) => (
                  <tr key={key.id}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div style={{
                          width: 32, height: 32, borderRadius: 8,
                          background: 'var(--color-primary-bg)', border: '1px solid rgba(99,102,241,0.2)',
                          display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                        }}>
                          <span className="ms" style={{ fontSize: 16, color: 'var(--color-primary)' }}>vpn_key</span>
                        </div>
                        <span style={{ fontWeight: 600 }}>{key.name}</span>
                      </div>
                    </td>
                    <td>
                      <code style={{ background: 'var(--color-bg)', padding: '2px 8px', borderRadius: 4, fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        {key.prefix}
                      </code>
                    </td>
                    <td>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                        {key.scopes.map((scope) => (
                          <span key={scope} className={scopeBadgeClass(scope)}>{scope}</span>
                        ))}
                      </div>
                    </td>
                    <td>
                      {key.environment_ids.length === 0 ? (
                        <span className="badge badge-ops">All</span>
                      ) : (
                        key.environment_ids.map((eid: string) => {
                          const env = environments.find((e) => e.id === eid);
                          return <span key={eid} className="badge" style={{ marginRight: 4 }}>{env?.name || eid.slice(0, 8)}</span>;
                        })
                      )}
                    </td>
                    <td className="text-secondary">{formatDate(key.created_at)}</td>
                    <td className="text-secondary">{key.last_used_at ? formatDate(key.last_used_at) : <span className="text-muted">Never</span>}</td>
                    <td>
                      {confirmRevoke === key.id ? (
                        <span className="inline-confirm">
                          <button className="btn btn-sm btn-danger" onClick={() => handleRevoke(key.id)}>Yes</button>
                          <button className="btn btn-sm" onClick={() => setConfirmRevoke(null)}>No</button>
                        </span>
                      ) : (
                        <button className="btn-icon" title="Revoke" onClick={() => setConfirmRevoke(key.id)}>
                          <span className="ms" style={{ fontSize: 16, color: 'var(--color-danger)' }}>delete</span>
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
