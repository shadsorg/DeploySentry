import { useState } from 'react';
import type { ApiKey } from '@/types';
import { MOCK_API_KEYS, MOCK_ENVIRONMENTS, getEnvironmentName } from '@/mocks/hierarchy';

const AVAILABLE_SCOPES = ['flags:read', 'flags:write', 'deploys:read', 'deploys:write', 'admin'];

function scopeBadgeClass(scope: string): string {
  if (scope === 'admin') return 'badge badge-experiment';
  if (scope.startsWith('flags')) return 'badge badge-feature';
  if (scope.startsWith('deploys')) return 'badge badge-release';
  return 'badge badge-ops';
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export default function APIKeysPage() {
  const [keys, setKeys] = useState<ApiKey[]>([...MOCK_API_KEYS]);
  const [showCreate, setShowCreate] = useState(false);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  // Create form
  const [newName, setNewName] = useState('');
  const [newScopes, setNewScopes] = useState<string[]>([]);
  const [newEnvTargets, setNewEnvTargets] = useState<string[]>([]);
  // Revoke confirm
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null);

  function handleCreate() {
    if (!newName.trim() || newScopes.length === 0) return;

    const fullKey = 'ds_' + Date.now() + '_' + Math.random().toString(36).slice(2);
    const prefix = fullKey.slice(0, 12) + '****';

    const newKey: ApiKey = {
      id: 'key-' + Date.now(),
      name: newName.trim(),
      prefix,
      scopes: [...newScopes],
      environment_targets: [...newEnvTargets],
      created_at: new Date().toISOString(),
      last_used_at: null,
      expires_at: null,
    };

    setKeys((prev) => [newKey, ...prev]);
    setRevealedKey(fullKey);
    setNewName('');
    setNewScopes([]);
    setNewEnvTargets([]);
    setShowCreate(false);
  }

  function toggleScope(scope: string) {
    setNewScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  }

  function toggleEnvTarget(envId: string) {
    setNewEnvTargets((prev) =>
      prev.includes(envId) ? prev.filter((e) => e !== envId) : [...prev, envId]
    );
  }

  function handleRevoke(keyId: string) {
    setKeys((prev) => prev.filter((k) => k.id !== keyId));
    setConfirmRevoke(null);
  }

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">API Keys</h1>
        <button className="btn btn-primary" onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? 'Cancel' : 'Create API Key'}
        </button>
      </div>

      {showCreate && (
        <div className="inline-form">
          <div className="form-group">
            <label>Name</label>
            <input
              type="text"
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
            <label>Environment Restrictions (leave unchecked for all)</label>
            <div className="checkbox-group">
              {MOCK_ENVIRONMENTS.map((env) => (
                <label key={env.id}>
                  <input
                    type="checkbox"
                    checked={newEnvTargets.includes(env.id)}
                    onChange={() => toggleEnvTarget(env.id)}
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

      {keys.length === 0 ? (
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
                <td><strong>{key.name}</strong></td>
                <td><code>{key.prefix}</code></td>
                <td>
                  {key.scopes.map((scope) => (
                    <span key={scope} className={scopeBadgeClass(scope)}>{scope}</span>
                  ))}
                </td>
                <td>
                  {key.environment_targets.length === 0 ? (
                    <span className="badge badge-ops">All</span>
                  ) : (
                    key.environment_targets.map((envId) => (
                      <span key={envId} className="badge badge-feature">
                        {getEnvironmentName(envId)}
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
                      <button className="btn btn-sm btn-danger" onClick={() => handleRevoke(key.id)}>
                        Yes
                      </button>{' '}
                      <button className="btn btn-sm btn-secondary" onClick={() => setConfirmRevoke(null)}>
                        No
                      </button>
                    </span>
                  ) : (
                    <button className="btn btn-sm btn-danger" onClick={() => setConfirmRevoke(key.id)}>
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
