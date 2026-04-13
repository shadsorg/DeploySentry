import React, { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useEnvironments } from '../hooks/useEntities';
import { useWebhooks } from '../hooks/useWebhooks';
import { useNotifications } from '../hooks/useNotifications';
import { entitiesApi, webhooksApi, Webhook } from '../api';

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

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const SettingsPage: React.FC<SettingsPageProps> = ({ level = 'org', tab }) => {
  const { orgSlug, projectSlug, appSlug } = useParams();
  const [activeTab, setActiveTab] = useState<SettingsTab>(defaultTab(level, tab));

  // ---------------------------------------------------------------------------
  // Environments (org level) — Task 7
  // ---------------------------------------------------------------------------
  const {
    environments,
    loading: envsLoading,
    error: envsError,
    refresh: refreshEnvs,
  } = useEnvironments(orgSlug);
  const [newEnvName, setNewEnvName] = useState('');
  const [newEnvSlug, setNewEnvSlug] = useState('');
  const [newEnvIsProd, setNewEnvIsProd] = useState(false);
  const [confirmDeleteEnv, setConfirmDeleteEnv] = useState<string | null>(null);
  const [envSaving, setEnvSaving] = useState(false);

  // ---------------------------------------------------------------------------
  // Webhooks — Task 8
  // ---------------------------------------------------------------------------
  const {
    webhooks,
    loading: webhooksLoading,
    error: webhooksError,
    refresh: refreshWebhooks,
  } = useWebhooks();
  const [addingWebhook, setAddingWebhook] = useState(false);
  const [editingWebhookId, setEditingWebhookId] = useState<string | null>(null);
  const [webhookUrl, setWebhookUrl] = useState('');
  const [webhookEvents, setWebhookEvents] = useState<string[]>([]);
  const [webhookActive, setWebhookActive] = useState(true);
  const [webhookSaving, setWebhookSaving] = useState(false);
  const [testingWebhookId, setTestingWebhookId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ id: string; success: boolean } | null>(null);

  // ---------------------------------------------------------------------------
  // Notifications — Task 9
  // ---------------------------------------------------------------------------
  const {
    preferences: notifPrefs,
    loading: notifLoading,
    error: notifError,
    saving: notifSaving,
    save: saveNotifPrefs,
    reset: resetNotifPrefs,
  } = useNotifications();
  const [notifSuccess, setNotifSuccess] = useState(false);

  // ---------------------------------------------------------------------------
  // Project form state — Task 10
  // ---------------------------------------------------------------------------
  const [projectName, setProjectName] = useState('');
  const [defaultEnv, setDefaultEnv] = useState('production');
  const [staleThreshold, setStaleThreshold] = useState('30d');
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSuccess, setSettingsSuccess] = useState(false);

  // ---------------------------------------------------------------------------
  // App form state — Task 10
  // ---------------------------------------------------------------------------
  const [appName, setAppName] = useState('');
  const [appDescription, setAppDescription] = useState('');
  const [appRepoUrl, setAppRepoUrl] = useState('');

  // ---------------------------------------------------------------------------
  // Handlers — Environments
  // ---------------------------------------------------------------------------

  const handleEnvNameChange = (value: string) => {
    setNewEnvName(value);
    setNewEnvSlug(
      value
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-|-$/g, ''),
    );
  };

  const handleAddEnvironment = async () => {
    if (!newEnvName.trim() || !orgSlug) return;
    setEnvSaving(true);
    try {
      await entitiesApi.createEnvironment(orgSlug, {
        name: newEnvName,
        slug: newEnvSlug || newEnvName.toLowerCase().replace(/\s+/g, '-'),
        is_production: newEnvIsProd,
      });
      setNewEnvName('');
      setNewEnvSlug('');
      setNewEnvIsProd(false);
      refreshEnvs();
    } catch (err) {
      console.error('Failed to create environment:', err);
    } finally {
      setEnvSaving(false);
    }
  };

  const handleDeleteEnvironment = async (envSlug: string) => {
    if (!orgSlug) return;
    try {
      await entitiesApi.deleteEnvironment(orgSlug, envSlug);
      setConfirmDeleteEnv(null);
      refreshEnvs();
    } catch (err) {
      console.error('Failed to delete environment:', err);
    }
  };

  // ---------------------------------------------------------------------------
  // Handlers — Webhooks
  // ---------------------------------------------------------------------------

  const startEditWebhook = (wh: Webhook) => {
    setEditingWebhookId(wh.id);
    setWebhookUrl(wh.url);
    setWebhookEvents([...wh.events]);
    setWebhookActive(wh.is_active);
    setAddingWebhook(false);
  };

  const cancelWebhookForm = () => {
    setAddingWebhook(false);
    setEditingWebhookId(null);
    setWebhookUrl('');
    setWebhookEvents([]);
    setWebhookActive(true);
  };

  const handleSaveWebhook = async () => {
    if (!webhookUrl.trim()) return;
    setWebhookSaving(true);
    try {
      if (editingWebhookId) {
        await webhooksApi.update(editingWebhookId, {
          url: webhookUrl,
          events: webhookEvents,
          is_active: webhookActive,
        });
      } else {
        await webhooksApi.create({
          url: webhookUrl,
          events: webhookEvents,
          is_active: webhookActive,
        });
      }
      cancelWebhookForm();
      refreshWebhooks();
    } catch (err) {
      console.error('Failed to save webhook:', err);
    } finally {
      setWebhookSaving(false);
    }
  };

  const handleDeleteWebhook = async (id: string) => {
    try {
      await webhooksApi.delete(id);
      refreshWebhooks();
    } catch (err) {
      console.error('Failed to delete webhook:', err);
    }
  };

  const handleTestWebhook = async (id: string) => {
    setTestingWebhookId(id);
    setTestResult(null);
    try {
      const res = await webhooksApi.test(id);
      setTestResult({ id, success: res.success });
    } catch (err) {
      console.error('Failed to test webhook:', err);
      setTestResult({ id, success: false });
    } finally {
      setTestingWebhookId(null);
    }
  };

  const toggleWebhookEvent = (event: string) => {
    setWebhookEvents((prev) =>
      prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event],
    );
  };

  // ---------------------------------------------------------------------------
  // Handlers — Notifications
  // ---------------------------------------------------------------------------

  const handleSaveNotifications = async () => {
    if (!notifPrefs) return;
    try {
      await saveNotifPrefs({
        channels: notifPrefs.channels,
        event_routing: notifPrefs.event_routing,
      });
      setNotifSuccess(true);
      setTimeout(() => setNotifSuccess(false), 3000);
    } catch (err) {
      console.error('Failed to save notification settings:', err);
    }
  };

  // ---------------------------------------------------------------------------
  // Handlers — Project Settings (Task 10)
  // ---------------------------------------------------------------------------

  const handleSaveProjectSettings = async () => {
    if (!orgSlug || !projectSlug) return;
    setSettingsSaving(true);
    try {
      await entitiesApi.updateProject(orgSlug, projectSlug, { name: projectName });
      setSettingsSuccess(true);
      setTimeout(() => setSettingsSuccess(false), 3000);
    } catch (err) {
      console.error('Failed to save project settings:', err);
    } finally {
      setSettingsSaving(false);
    }
  };

  // ---------------------------------------------------------------------------
  // Handlers — App Settings (Task 10)
  // ---------------------------------------------------------------------------

  const handleSaveAppSettings = async () => {
    if (!orgSlug || !projectSlug || !appSlug) return;
    setSettingsSaving(true);
    try {
      await entitiesApi.updateApp(orgSlug, projectSlug, appSlug, {
        name: appName,
        description: appDescription,
      });
      setSettingsSuccess(true);
      setTimeout(() => setSettingsSuccess(false), 3000);
    } catch (err) {
      console.error('Failed to save app settings:', err);
    } finally {
      setSettingsSaving(false);
    }
  };

  const handleDeleteApp = async () => {
    if (!window.confirm('Are you sure you want to delete this application? This cannot be undone.'))
      return;
    if (!orgSlug || !projectSlug || !appSlug) return;
    try {
      await fetch(`/api/v1/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${localStorage.getItem('ds_token') ?? ''}` },
      });
      window.location.href = `/orgs/${orgSlug}/projects/${projectSlug}/apps`;
    } catch (err) {
      console.error('Failed to delete app:', err);
    }
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const headingMap: Record<string, string> = {
    org: 'Organization Settings',
    project: 'Project Settings',
    app: 'Application Settings',
  };

  const tabs = getTabsForLevel(level);

  const WEBHOOK_EVENT_OPTIONS = [
    'deploy.started',
    'deploy.completed',
    'deploy.failed',
    'deploy.rolled_back',
    'flag.changed',
  ];

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

      {/* ------------------------------------------------------------------ */}
      {/* Environments tab (org level) — Task 7                               */}
      {/* ------------------------------------------------------------------ */}
      {activeTab === 'environments' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {envsLoading && <p className="text-muted text-sm">Loading environments…</p>}
          {envsError && <p className="text-danger text-sm">Error: {envsError}</p>}
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
              disabled={!newEnvName.trim() || envSaving}
            >
              {envSaving ? 'Adding…' : 'Add Environment'}
            </button>
          </div>

          {/* Environments table */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Environments</span>
            </div>
            {!envsLoading && environments.length === 0 ? (
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
                          {confirmDeleteEnv === env.slug ? (
                            <span className="flex items-center gap-2">
                              <button
                                className="btn btn-danger btn-sm"
                                onClick={() => handleDeleteEnvironment(env.slug)}
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
                              onClick={() => setConfirmDeleteEnv(env.slug)}
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

      {/* ------------------------------------------------------------------ */}
      {/* Webhooks tab — Task 8                                               */}
      {/* ------------------------------------------------------------------ */}
      {activeTab === 'webhooks' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header">
              <span className="card-title">Webhooks</span>
              {!addingWebhook && !editingWebhookId && (
                <button
                  className="btn btn-primary btn-sm"
                  onClick={() => {
                    cancelWebhookForm();
                    setAddingWebhook(true);
                  }}
                >
                  Add Webhook
                </button>
              )}
            </div>

            {webhooksLoading && <p className="text-muted text-sm">Loading webhooks…</p>}
            {webhooksError && <p className="text-danger text-sm">Error: {webhooksError}</p>}

            {/* Inline add/edit form */}
            {(addingWebhook || editingWebhookId) && (
              <div
                style={{
                  background: 'var(--color-bg-secondary, #1e293b)',
                  border: '1px solid var(--color-border, #334155)',
                  borderRadius: 6,
                  padding: 16,
                  marginBottom: 16,
                }}
              >
                <div className="form-group">
                  <label className="form-label">Webhook URL</label>
                  <input
                    type="text"
                    className="form-input"
                    placeholder="https://hooks.example.com/..."
                    value={webhookUrl}
                    onChange={(e) => setWebhookUrl(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Events</label>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    {WEBHOOK_EVENT_OPTIONS.map((evt) => (
                      <label
                        key={evt}
                        className="flex items-center gap-2"
                        style={{ cursor: 'pointer' }}
                      >
                        <input
                          type="checkbox"
                          checked={webhookEvents.includes(evt)}
                          onChange={() => toggleWebhookEvent(evt)}
                        />
                        <code className="font-mono text-sm">{evt}</code>
                      </label>
                    ))}
                  </div>
                </div>
                <div className="form-group">
                  <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={webhookActive}
                      onChange={(e) => setWebhookActive(e.target.checked)}
                    />
                    <span className="text-sm">Active</span>
                  </label>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={handleSaveWebhook}
                    disabled={!webhookUrl.trim() || webhookSaving}
                  >
                    {webhookSaving
                      ? 'Saving…'
                      : editingWebhookId
                        ? 'Update Webhook'
                        : 'Create Webhook'}
                  </button>
                  <button className="btn btn-sm" onClick={cancelWebhookForm}>
                    Cancel
                  </button>
                </div>
              </div>
            )}

            {!webhooksLoading && webhooks.length === 0 && !addingWebhook ? (
              <p className="text-muted">No webhooks configured. Add one to get started.</p>
            ) : (
              <div className="table-container">
                <table>
                  <thead>
                    <tr>
                      <th>URL</th>
                      <th>Events</th>
                      <th>Status</th>
                      <th>Created</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {webhooks.map((wh) => (
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
                            className={`badge ${wh.is_active ? 'badge-active' : 'badge-disabled'}`}
                          >
                            {wh.is_active ? 'Active' : 'Inactive'}
                          </span>
                        </td>
                        <td className="text-secondary text-sm">
                          {new Date(wh.created_at).toLocaleDateString()}
                        </td>
                        <td>
                          <div className="flex items-center gap-2">
                            <button
                              className="btn btn-sm"
                              onClick={() => startEditWebhook(wh)}
                              disabled={!!editingWebhookId || addingWebhook}
                            >
                              Edit
                            </button>
                            <button
                              className="btn btn-sm"
                              onClick={() => handleTestWebhook(wh.id)}
                              disabled={testingWebhookId === wh.id}
                            >
                              {testingWebhookId === wh.id ? 'Testing…' : 'Test'}
                            </button>
                            <button
                              className="btn btn-danger btn-sm"
                              onClick={() => handleDeleteWebhook(wh.id)}
                            >
                              Delete
                            </button>
                          </div>
                          {testResult?.id === wh.id && (
                            <span
                              className={`text-sm ${testResult.success ? 'text-success' : 'text-danger'}`}
                              style={{ display: 'block', marginTop: 4 }}
                            >
                              {testResult.success
                                ? 'Test delivered successfully'
                                : 'Test delivery failed'}
                            </span>
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

      {/* ------------------------------------------------------------------ */}
      {/* Notifications tab — Task 9                                          */}
      {/* ------------------------------------------------------------------ */}
      {activeTab === 'notifications' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {notifLoading && <p className="text-muted text-sm">Loading notification settings…</p>}
          {notifError && <p className="text-danger text-sm">Error: {notifError}</p>}

          {notifPrefs && (
            <>
              {/* Channels */}
              {Object.entries(notifPrefs.channels).map(([channelName, config]) => (
                <div className="card" key={channelName}>
                  <div className="card-header">
                    <span className="card-title" style={{ textTransform: 'capitalize' }}>
                      {channelName}
                    </span>
                    <div className="flex items-center gap-2">
                      {config.source === 'config' && (
                        <span className="badge badge-ops" style={{ fontSize: 11 }}>
                          config-file
                        </span>
                      )}
                      <span
                        className={`badge ${config.enabled ? 'badge-active' : 'badge-disabled'}`}
                      >
                        {config.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                  </div>
                  {config.source === 'config' ? (
                    <p className="text-muted text-sm">
                      This channel is configured via the server config file and cannot be edited
                      here.
                    </p>
                  ) : (
                    <p className="text-muted text-sm">
                      Manage this channel's settings via the API or server configuration.
                    </p>
                  )}
                </div>
              ))}

              {/* Event routing */}
              {Object.keys(notifPrefs.event_routing).length > 0 && (
                <div className="card">
                  <div className="card-header">
                    <span className="card-title">Event Routing</span>
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    {Object.entries(notifPrefs.event_routing).map(([event, channels]) => (
                      <div key={event} className="flex items-center gap-3">
                        <code className="font-mono text-sm" style={{ minWidth: 200 }}>
                          {event}
                        </code>
                        <div className="flex items-center gap-2">
                          {(channels as string[]).map((ch) => (
                            <span key={ch} className="badge badge-ops">
                              {ch}
                            </span>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className="flex items-center gap-3" style={{ alignSelf: 'flex-start' }}>
                <button
                  className="btn btn-primary"
                  onClick={handleSaveNotifications}
                  disabled={notifSaving}
                >
                  {notifSaving ? 'Saving…' : 'Save Notification Settings'}
                </button>
                <button className="btn btn-sm" onClick={resetNotifPrefs} disabled={notifSaving}>
                  Reset to Defaults
                </button>
                {notifSuccess && <span className="text-sm text-success">Settings saved.</span>}
              </div>
            </>
          )}
        </div>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* General tab — project level — Task 10                               */}
      {/* ------------------------------------------------------------------ */}
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

          <div className="flex items-center gap-3">
            <button
              className="btn btn-primary"
              onClick={handleSaveProjectSettings}
              disabled={settingsSaving}
            >
              {settingsSaving ? 'Saving…' : 'Save'}
            </button>
            {settingsSuccess && <span className="text-sm text-success">Settings saved.</span>}
          </div>
        </div>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* General tab — app level — Task 10                                   */}
      {/* ------------------------------------------------------------------ */}
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
              value={appSlug ?? ''}
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

          <div className="flex items-center gap-3">
            <button
              className="btn btn-primary"
              onClick={handleSaveAppSettings}
              disabled={settingsSaving}
            >
              {settingsSaving ? 'Saving…' : 'Save'}
            </button>
            {settingsSuccess && <span className="text-sm text-success">Settings saved.</span>}
          </div>
        </div>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* Danger Zone tab (app level) — Task 10                               */}
      {/* ------------------------------------------------------------------ */}
      {activeTab === 'danger' && (
        <div className="danger-zone">
          <h3>Delete Application</h3>
          <p>
            Deleting this application will remove all its deployments, releases, and flag
            configurations. This action cannot be undone.
          </p>
          <button className="btn btn-danger" onClick={handleDeleteApp}>
            Delete Application
          </button>
        </div>
      )}
    </div>
  );
};

export default SettingsPage;
