import { useState, useEffect } from 'react';
import type { ApiKey } from '@/types';
import { apiKeysApi } from '@/api';

const AVAILABLE_SCOPES = ['flags:read', 'flags:write', 'deploys:read', 'deploys:write', 'admin'];

function scopeBadgeClass(scope: string): string {
  if (scope === 'admin') return 'badge badge-experiment';
  if (scope.startsWith('flags')) return 'badge badge-feature';
  if (scope.startsWith('deploys')) return 'badge badge-release';
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
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  // Create form
  const [newName, setNewName] = useState('');
  const [newScopes, setNewScopes] = useState<string[]>([]);
  // Revoke confirm
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null);

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
      const result = await apiKeysApi.create({ name: newName.trim(), scopes: newScopes });
      setRevealedKey(result.token);
      setNewName('');
      setNewScopes([]);
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
        <h1 className="page-header">API Keys</h1>
        <button className="btn btn-primary" onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? 'Cancel' : 'Create API Key'}
        </button>
      </div>

      <p className="text-muted" style={{ marginBottom: 24, maxWidth: 600 }}>
        API keys authenticate programmatic access to the DeploySentry API. Use them to integrate with CI/CD pipelines,
        automate flag management, or connect your SDKs. Keys are scoped to an organization and can be revoked at any time.
      </p>

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
            <label>Environment Restrictions</label>
            <p className="text-muted text-sm">Environment restrictions coming soon</p>
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
        <p className="text-muted">Loading API keys...</p>
      ) : keys.length === 0 ? (
        <p className="empty-state">No API keys. Create one to integrate with DeploySentry.</p>
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Key Prefix</th>
              <th>Scopes</th>
              <th>Environments</th>
              <th>Created</th>
              <th>Last Used</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {keys.map((key) => (
              <tr key={key.id}>
                <td>
                  <strong>{key.name}</strong>
                </td>
                <td>
                  <code>{key.prefix}</code>
                </td>
                <td>
                  {key.scopes.map((scope) => (
                    <span key={scope} className={scopeBadgeClass(scope)}>
                      {scope}
                    </span>
                  ))}
                </td>
                <td>
                  {key.environment_targets.length === 0 ? (
                    <span className="badge badge-ops">All</span>
                  ) : (
                    key.environment_targets.map((envId) => (
                      <span key={envId} className="badge badge-feature">
                        {envId}
                      </span>
                    ))
                  )}
                </td>
                <td>{formatDate(key.created_at)}</td>
                <td>{key.last_used_at ? formatDate(key.last_used_at) : 'Never'}</td>
                <td>
                  {confirmRevoke === key.id ? (
                    <span>
                      Are you sure?{' '}
                      <button
                        className="btn btn-sm btn-danger"
                        onClick={() => handleRevoke(key.id)}
                      >
                        Yes
                      </button>{' '}
                      <button
                        className="btn btn-sm btn-secondary"
                        onClick={() => setConfirmRevoke(null)}
                      >
                        No
                      </button>
                    </span>
                  ) : (
                    <button
                      className="btn btn-sm btn-danger"
                      onClick={() => setConfirmRevoke(key.id)}
                    >
                      Revoke
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
