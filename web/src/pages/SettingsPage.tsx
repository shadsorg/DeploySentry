import React, { useState } from 'react';
import type { OrgEnvironment } from '@/types';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type SettingsTab = 'environments' | 'webhooks' | 'notifications' | 'general' | 'danger';

interface SettingsPageProps {
  level?: 'org' | 'project' | 'app';
  tab?: string;
}

function defaultTab(level: string, tab?: string): SettingsTab {
  const validTabs: Record<string, SettingsTab[]> = {
    org: ['environments', 'webhooks', 'notifications'],
    project: ['general'],
    app: ['general', 'danger'],
  };
  const levelTabs = validTabs[level] || [];
  if (tab && levelTabs.includes(tab as SettingsTab)) return tab as SettingsTab;
  switch (level) {
    case 'org':
      return 'environments';
    case 'project':
      return 'general';
    case 'app':
      return 'general';
    default:
      return 'environments';
  }
}

function getTabsForLevel(level: string): { key: SettingsTab; label: string }[] {
  switch (level) {
    case 'org':
      return [
        { key: 'environments', label: 'Environments' },
        { key: 'webhooks', label: 'Webhooks' },
        { key: 'notifications', label: 'Notifications' },
      ];
    case 'project':
      return [{ key: 'general', label: 'Project Settings' }];
    case 'app':
      return [
        { key: 'general', label: 'General' },
        { key: 'danger', label: 'Danger Zone' },
      ];
    default:
      return [];
  }
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
// Component
// ---------------------------------------------------------------------------

const SettingsPage: React.FC<SettingsPageProps> = ({ level = 'org', tab }) => {
  const [activeTab, setActiveTab] = useState<SettingsTab>(defaultTab(level, tab));

  // Environments state (org level)
  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [newEnvName, setNewEnvName] = useState('');
  const [newEnvSlug, setNewEnvSlug] = useState('');
  const [newEnvIsProd, setNewEnvIsProd] = useState(false);
  const [confirmDeleteEnv, setConfirmDeleteEnv] = useState<string | null>(null);

  // Notifications form state
  const [slackUrl, setSlackUrl] = useState('');
  const [slackChannel, setSlackChannel] = useState('');
  const [slackEnabled, setSlackEnabled] = useState(false);
  const [emailEnabled, setEmailEnabled] = useState(false);
  const [emailSmtpHost, setEmailSmtpHost] = useState('');
  const [emailSmtpPort, setEmailSmtpPort] = useState('587');
  const [emailUsername, setEmailUsername] = useState('');
  const [emailPassword, setEmailPassword] = useState('');
  const [emailFrom, setEmailFrom] = useState('');
  const [pagerdutyEnabled, setPagerdutyEnabled] = useState(false);
  const [pagerdutyKey, setPagerdutyKey] = useState('');
  const [enabledEvents, setEnabledEvents] = useState<Set<string>>(
    new Set(['deploy.completed', 'deploy.failed']),
  );

  // Project form state
  const [projectName, setProjectName] = useState('DeploySentry');
  const [defaultEnv, setDefaultEnv] = useState('production');
  const [staleThreshold, setStaleThreshold] = useState('30d');

  // App form state
  const [appName, setAppName] = useState('API Server');
  const [appDescription, setAppDescription] = useState('Core REST API');
  const [appRepoUrl, setAppRepoUrl] = useState('https://github.com/acme/api-server');

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

  const handleEnvNameChange = (value: string) => {
    setNewEnvName(value);
    setNewEnvSlug(
      value
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-|-$/g, ''),
    );
  };

  const handleAddEnvironment = () => {
    if (!newEnvName.trim() || !newEnvSlug.trim()) return;
    const env: OrgEnvironment = {
      id: `env-${Date.now()}`,
      name: newEnvName.trim(),
      slug: newEnvSlug.trim(),
      is_production: newEnvIsProd,
      created_at: new Date().toISOString(),
    };
    setEnvironments((prev) => [...prev, env]);
    setNewEnvName('');
    setNewEnvSlug('');
    setNewEnvIsProd(false);
  };

  const handleDeleteEnvironment = (envId: string) => {
    setEnvironments((prev) => prev.filter((e) => e.id !== envId));
    setConfirmDeleteEnv(null);
  };

  const headingMap: Record<string, string> = {
    org: 'Organization Settings',
    project: 'Project Settings',
    app: 'Application Settings',
  };

  const tabs = getTabsForLevel(level);

  return (
    <div>
      {/* Page header */}
      <div className="page-header">
        <h1>{headingMap[level]}</h1>
      </div>

      {/* Tabs */}
      <div className="tabs">
        {tabs.map((t) => (
          <button
            key={t.key}
            className={`tab${activeTab === t.key ? ' active' : ''}`}
            onClick={() => setActiveTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Environments tab (org level) */}
      {activeTab === 'environments' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <p className="text-muted text-sm">
            Note: Environment changes are local to this session. Backend persistence coming soon.
          </p>
          {/* Add environment form */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Add Environment</span>
            </div>
            <div className="grid-2">
              <div className="form-group">
                <label className="form-label">Name</label>
                <input
                  type="text"
                  className="form-input"
                  placeholder="e.g. QA"
                  value={newEnvName}
                  onChange={(e) => handleEnvNameChange(e.target.value)}
                />
              </div>
              <div className="form-group">
                <label className="form-label">Slug</label>
                <input
                  type="text"
                  className="form-input font-mono"
                  value={newEnvSlug}
                  readOnly
                  style={{ opacity: 0.7 }}
                />
              </div>
            </div>
            <div className="form-group">
              <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={newEnvIsProd}
                  onChange={(e) => setNewEnvIsProd(e.target.checked)}
                />
                <span className="text-sm">Production environment</span>
              </label>
            </div>
            <button
              className="btn btn-primary btn-sm"
              onClick={handleAddEnvironment}
              disabled={!newEnvName.trim()}
            >
              Add Environment
            </button>
          </div>

          {/* Environments table */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Environments</span>
            </div>
            {environments.length === 0 ? (
              <p className="text-muted">No environments defined. Add one to get started.</p>
            ) : (
              <div className="table-container">
                <table>
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Slug</th>
                      <th>Production</th>
                      <th>Created</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {environments.map((env) => (
                      <tr key={env.id}>
                        <td style={{ fontWeight: 500 }}>{env.name}</td>
                        <td>
                          <code className="font-mono text-sm">{env.slug}</code>
                        </td>
                        <td>
                          {env.is_production && (
                            <span className="badge badge-active">Production</span>
                          )}
                        </td>
                        <td className="text-secondary text-sm">
                          {new Date(env.created_at).toLocaleDateString()}
                        </td>
                        <td>
                          {confirmDeleteEnv === env.id ? (
                            <span className="flex items-center gap-2">
                              <button
                                className="btn btn-danger btn-sm"
                                onClick={() => handleDeleteEnvironment(env.id)}
                              >
                                Confirm
                              </button>
                              <button
                                className="btn btn-sm"
                                onClick={() => setConfirmDeleteEnv(null)}
                              >
                                Cancel
                              </button>
                            </span>
                          ) : (
                            <button
                              className="btn btn-danger btn-sm"
                              onClick={() => setConfirmDeleteEnv(env.id)}
                            >
                              Delete
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
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
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Slack */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Slack</span>
              <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={slackEnabled}
                  onChange={(e) => setSlackEnabled(e.target.checked)}
                />
                <span className="text-sm">Enabled</span>
              </label>
            </div>
            {slackEnabled && (
              <>
                <div className="form-group">
                  <label className="form-label">Webhook URL</label>
                  <input
                    type="text"
                    className="form-input"
                    placeholder="https://hooks.slack.com/services/..."
                    value={slackUrl}
                    onChange={(e) => setSlackUrl(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Channel (optional)</label>
                  <input
                    type="text"
                    className="form-input"
                    placeholder="#deployments"
                    value={slackChannel}
                    onChange={(e) => setSlackChannel(e.target.value)}
                  />
                </div>
              </>
            )}
          </div>

          {/* Email */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Email (SMTP)</span>
              <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={emailEnabled}
                  onChange={(e) => setEmailEnabled(e.target.checked)}
                />
                <span className="text-sm">Enabled</span>
              </label>
            </div>
            {emailEnabled && (
              <>
                <div className="grid-2">
                  <div className="form-group">
                    <label className="form-label">SMTP Host</label>
                    <input
                      type="text"
                      className="form-input"
                      placeholder="smtp.gmail.com"
                      value={emailSmtpHost}
                      onChange={(e) => setEmailSmtpHost(e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label">SMTP Port</label>
                    <input
                      type="number"
                      className="form-input"
                      placeholder="587"
                      value={emailSmtpPort}
                      onChange={(e) => setEmailSmtpPort(e.target.value)}
                    />
                  </div>
                </div>
                <div className="grid-2">
                  <div className="form-group">
                    <label className="form-label">Username</label>
                    <input
                      type="text"
                      className="form-input"
                      placeholder="user@example.com"
                      value={emailUsername}
                      onChange={(e) => setEmailUsername(e.target.value)}
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label">Password</label>
                    <input
                      type="password"
                      className="form-input"
                      placeholder="App password"
                      value={emailPassword}
                      onChange={(e) => setEmailPassword(e.target.value)}
                    />
                  </div>
                </div>
                <div className="form-group">
                  <label className="form-label">From Address</label>
                  <input
                    type="email"
                    className="form-input"
                    placeholder="noreply@deploysentry.com"
                    value={emailFrom}
                    onChange={(e) => setEmailFrom(e.target.value)}
                  />
                </div>
              </>
            )}
          </div>

          {/* PagerDuty */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">PagerDuty</span>
              <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={pagerdutyEnabled}
                  onChange={(e) => setPagerdutyEnabled(e.target.checked)}
                />
                <span className="text-sm">Enabled</span>
              </label>
            </div>
            {pagerdutyEnabled && (
              <div className="form-group">
                <label className="form-label">Integration/Routing Key</label>
                <input
                  type="text"
                  className="form-input"
                  placeholder="Events API v2 routing key"
                  value={pagerdutyKey}
                  onChange={(e) => setPagerdutyKey(e.target.value)}
                />
                <div className="form-hint">
                  PagerDuty incidents are auto-created for deployment failures and health alerts.
                </div>
              </div>
            )}
          </div>

          {/* Event Types */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Event Types</span>
            </div>
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

          <button className="btn btn-primary" style={{ alignSelf: 'flex-start' }}>
            Save Notification Settings
          </button>
        </div>
      )}

      {/* General tab — project level */}
      {activeTab === 'general' && level === 'project' && (
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

      {/* General tab — app level */}
      {activeTab === 'general' && level === 'app' && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">General</span>
          </div>
          <div className="form-group">
            <label className="form-label">Name</label>
            <input
              type="text"
              className="form-input"
              value={appName}
              onChange={(e) => setAppName(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Slug</label>
            <input
              type="text"
              className="form-input font-mono"
              value="api-server"
              readOnly
              style={{ opacity: 0.7 }}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea
              className="form-input"
              rows={3}
              value={appDescription}
              onChange={(e) => setAppDescription(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Repository URL</label>
            <input
              type="text"
              className="form-input"
              placeholder="https://github.com/org/repo"
              value={appRepoUrl}
              onChange={(e) => setAppRepoUrl(e.target.value)}
            />
          </div>

          <button className="btn btn-primary">Save</button>
        </div>
      )}

      {/* Danger Zone tab (app level) */}
      {activeTab === 'danger' && (
        <div className="danger-zone">
          <h3>Delete Application</h3>
          <p>
            Deleting this application will remove all its deployments, releases, and flag
            configurations. This action cannot be undone.
          </p>
          <button className="btn btn-danger">Delete Application</button>
        </div>
      )}
    </div>
  );
};

export default SettingsPage;
