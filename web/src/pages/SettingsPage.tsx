import React, { useState } from 'react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type SettingsTab = 'api-keys' | 'webhooks' | 'notifications' | 'project';

interface APIKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  created: string;
  lastUsed: string;
}

interface Webhook {
  id: string;
  url: string;
  events: string[];
  status: 'Active' | 'Inactive';
  created: string;
}

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

const MOCK_API_KEYS: APIKey[] = [
  {
    id: 'key-1',
    name: 'Production Backend',
    prefix: 'ds_prod_abc1****',
    scopes: ['flags:read'],
    created: '2025-11-15',
    lastUsed: '2 minutes ago',
  },
  {
    id: 'key-2',
    name: 'CI/CD Pipeline',
    prefix: 'ds_ci_def2****',
    scopes: ['deploys:read', 'deploys:write'],
    created: '2025-12-01',
    lastUsed: '1 hour ago',
  },
  {
    id: 'key-3',
    name: 'Admin Dashboard',
    prefix: 'ds_admin_ghi3****',
    scopes: ['admin'],
    created: '2026-01-10',
    lastUsed: '3 days ago',
  },
];

const MOCK_WEBHOOKS: Webhook[] = [
  {
    id: 'wh-1',
    url: 'https://hooks.slack.com/services/T00/B00/xxxx',
    events: ['deploy.completed', 'deploy.failed', 'deploy.rolled_back'],
    status: 'Active',
    created: '2025-11-20',
  },
  {
    id: 'wh-2',
    url: 'https://api.pagerduty.com/webhooks/deploysentry',
    events: ['deploy.failed', 'flag.changed'],
    status: 'Inactive',
    created: '2026-01-05',
  },
];

const NOTIFICATION_EVENTS = [
  'deploy.started',
  'deploy.completed',
  'deploy.failed',
  'deploy.rolled_back',
  'flag.changed',
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function scopeBadgeClass(scope: string): string {
  if (scope === 'admin') return 'badge badge-experiment';
  if (scope.startsWith('flags')) return 'badge badge-feature';
  if (scope.startsWith('deploys')) return 'badge badge-release';
  return 'badge badge-ops';
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const SettingsPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<SettingsTab>('api-keys');

  // Notifications form state
  const [slackUrl, setSlackUrl] = useState('');
  const [enabledEvents, setEnabledEvents] = useState<Set<string>>(
    new Set(['deploy.completed', 'deploy.failed']),
  );

  // Project form state
  const [projectName, setProjectName] = useState('DeploySentry');
  const [defaultEnv, setDefaultEnv] = useState('production');
  const [staleThreshold, setStaleThreshold] = useState('30d');

  const toggleEvent = (event: string) => {
    setEnabledEvents((prev) => {
      const next = new Set(prev);
      if (next.has(event)) {
        next.delete(event);
      } else {
        next.add(event);
      }
      return next;
    });
  };

  const TABS: { key: SettingsTab; label: string }[] = [
    { key: 'api-keys', label: 'API Keys' },
    { key: 'webhooks', label: 'Webhooks' },
    { key: 'notifications', label: 'Notifications' },
    { key: 'project', label: 'Project' },
  ];

  return (
    <div>
      {/* Page header */}
      <div className="page-header">
        <h1>Settings</h1>
      </div>

      {/* Tabs */}
      <div className="tabs">
        {TABS.map((t) => (
          <button
            key={t.key}
            className={`tab ${activeTab === t.key ? 'active' : ''}`}
            onClick={() => setActiveTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* API Keys tab */}
      {activeTab === 'api-keys' && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">API Keys</span>
            <button className="btn btn-primary btn-sm">Create API Key</button>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Key Prefix</th>
                  <th>Scopes</th>
                  <th>Created</th>
                  <th>Last Used</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {MOCK_API_KEYS.map((key) => (
                  <tr key={key.id}>
                    <td style={{ fontWeight: 500 }}>{key.name}</td>
                    <td>
                      <code className="font-mono text-sm">{key.prefix}</code>
                    </td>
                    <td>
                      <div className="flex items-center gap-2">
                        {key.scopes.map((scope) => (
                          <span key={scope} className={scopeBadgeClass(scope)}>
                            {scope}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="text-secondary text-sm">{key.created}</td>
                    <td className="text-secondary text-sm">{key.lastUsed}</td>
                    <td>
                      <button className="btn btn-danger btn-sm">Revoke</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Webhooks tab */}
      {activeTab === 'webhooks' && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">Webhooks</span>
            <button className="btn btn-primary btn-sm">Add Webhook</button>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>URL</th>
                  <th>Events</th>
                  <th>Status</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {MOCK_WEBHOOKS.map((wh) => (
                  <tr key={wh.id}>
                    <td>
                      <code className="font-mono text-sm">{wh.url}</code>
                    </td>
                    <td>
                      <div className="flex items-center gap-2" style={{ flexWrap: 'wrap' }}>
                        {wh.events.map((evt) => (
                          <span key={evt} className="badge badge-ops">
                            {evt}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td>
                      <span
                        className={`badge ${
                          wh.status === 'Active' ? 'badge-active' : 'badge-disabled'
                        }`}
                      >
                        {wh.status}
                      </span>
                    </td>
                    <td className="text-secondary text-sm">{wh.created}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Notifications tab */}
      {activeTab === 'notifications' && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">Notification Preferences</span>
          </div>
          <div className="form-group">
            <label className="form-label">Slack Webhook URL</label>
            <input
              type="text"
              className="form-input"
              placeholder="https://hooks.slack.com/services/..."
              value={slackUrl}
              onChange={(e) => setSlackUrl(e.target.value)}
            />
            <div className="form-hint">
              Notifications will be sent to this Slack channel via incoming webhook.
            </div>
          </div>

          <div className="form-group">
            <label className="form-label">Notification Events</label>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {NOTIFICATION_EVENTS.map((event) => (
                <label
                  key={event}
                  className="flex items-center gap-3"
                  style={{ cursor: 'pointer' }}
                >
                  <input
                    type="checkbox"
                    checked={enabledEvents.has(event)}
                    onChange={() => toggleEvent(event)}
                  />
                  <code className="font-mono text-sm">{event}</code>
                </label>
              ))}
            </div>
          </div>

          <button className="btn btn-primary">Save</button>
        </div>
      )}

      {/* Project tab */}
      {activeTab === 'project' && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">Project Settings</span>
          </div>
          <div className="form-group">
            <label className="form-label">Project Name</label>
            <input
              type="text"
              className="form-input"
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Default Environment</label>
            <select
              className="form-select"
              value={defaultEnv}
              onChange={(e) => setDefaultEnv(e.target.value)}
            >
              <option value="development">Development</option>
              <option value="staging">Staging</option>
              <option value="production">Production</option>
            </select>
          </div>

          <div className="form-group">
            <label className="form-label">Stale Flag Threshold</label>
            <input
              type="text"
              className="form-input"
              value={staleThreshold}
              onChange={(e) => setStaleThreshold(e.target.value)}
            />
            <div className="form-hint">
              Flags with no evaluation activity beyond this threshold will be marked as stale.
              Examples: 30d, 2w, 90d.
            </div>
          </div>

          <button className="btn btn-primary">Save</button>
        </div>
      )}
    </div>
  );
};

export default SettingsPage;
