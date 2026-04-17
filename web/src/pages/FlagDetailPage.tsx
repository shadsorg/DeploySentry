import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Flag, TargetingRule, OrgEnvironment, FlagEnvironmentState, RuleEnvironmentState, FlagCategory, AuditLogEntry } from '@/types';
import { flagsApi, entitiesApi, flagEnvStateApi, auditApi } from '@/api';
import type { Application } from '@/types';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function getAppNameById(appId: string, apps: Application[]): string {
  return apps.find((a) => a.id === appId)?.name ?? appId;
}

function describeAction(entry: AuditLogEntry): string {
  switch (entry.action) {
    case 'flag.created': return 'Created flag';
    case 'flag.updated': return 'Updated flag settings';
    case 'flag.toggled': return 'Toggled flag';
    case 'flag.archived': return 'Archived flag';
    case 'flag.env_state.updated': return 'Updated environment state';
    case 'flag.rule.created': return 'Added targeting rule';
    case 'flag.rule.deleted': return 'Deleted targeting rule';
    case 'flag.rule.env_state.updated': return 'Updated rule environment state';
    default: return entry.action;
  }
}

function generateFlagYaml(
  flag: Flag,
  rules: TargetingRule[],
  environments: OrgEnvironment[],
  envStates: FlagEnvironmentState[],
  ruleEnvStates: RuleEnvironmentState[],
): string {
  const lines: string[] = [];
  lines.push(`- key: ${flag.key}`);
  lines.push(`  name: ${flag.name}`);
  lines.push(`  flag_type: ${flag.flag_type}`);
  lines.push(`  category: ${flag.category}`);
  lines.push(`  default_value: "${flag.default_value}"`);
  lines.push(`  is_permanent: ${flag.is_permanent}`);
  if (flag.expires_at) lines.push(`  expires_at: "${flag.expires_at}"`);
  lines.push(`  environments:`);
  for (const env of environments) {
    const state = envStates.find((s) => s.environment_id === env.id);
    lines.push(`    ${env.name}:`);
    lines.push(`      enabled: ${state?.enabled ?? false}`);
    lines.push(`      value: "${state?.value != null ? String(state.value) : flag.default_value}"`);
  }
  if (rules.length > 0) {
    lines.push(`  rules:`);
    for (const rule of rules) {
      lines.push(`    - attribute: ${rule.attribute}`);
      lines.push(`      operator: ${rule.operator}`);
      lines.push(`      target_values: [${(rule.target_values ?? []).map((v) => `"${v}"`).join(', ')}]`);
      lines.push(`      value: "${rule.value}"`);
      lines.push(`      priority: ${rule.priority}`);
      lines.push(`      environments:`);
      for (const env of environments) {
        const res = ruleEnvStates.find((s) => s.rule_id === rule.id && s.environment_id === env.id);
        lines.push(`        ${env.name}: ${res?.enabled ?? false}`);
      }
    }
  }
  return lines.join('\n');
}

export default function FlagDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  const [flag, setFlag] = useState<Flag | null>(null);
  const [rules, setRules] = useState<TargetingRule[]>([]);
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'environments' | 'rules' | 'yaml' | 'settings' | 'history'>('environments');
  const [environments, setEnvironments] = useState<OrgEnvironment[]>([]);
  const [envStates, setEnvStates] = useState<FlagEnvironmentState[]>([]);
  const [ruleEnvStates, setRuleEnvStates] = useState<RuleEnvironmentState[]>([]);
  const [expandedEnvs, setExpandedEnvs] = useState<Set<string>>(new Set());
  const [showAddRule, setShowAddRule] = useState(false);
  const [newRule, setNewRule] = useState({
    attribute: '',
    operator: 'equals',
    target_values: '',
    value: '',
    priority: 0,
  });
  const [settingsForm, setSettingsForm] = useState<{
    name: string;
    description: string;
    category: string;
    purpose: string;
    owners: string;
    is_permanent: boolean;
    expires_at: string;
    default_value: string;
    tags: string;
  } | null>(null);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSuccess, setSettingsSuccess] = useState(false);
  const [history, setHistory] = useState<AuditLogEntry[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);

    const fetchApps =
      orgSlug && projectSlug
        ? entitiesApi.listApps(orgSlug, projectSlug).then((r) => r.applications)
        : Promise.resolve([]);

    Promise.all([flagsApi.get(id), flagsApi.listRules(id).then((r) => r.rules), fetchApps])
      .then(([flagData, rulesData, appsData]) => {
        setFlag(flagData);
        setRules(rulesData ?? []);
        setApps(appsData);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id, orgSlug, projectSlug]);

  useEffect(() => {
    if (!orgSlug || !id) return;
    entitiesApi
      .listOrgEnvironments(orgSlug)
      .then((res) => setEnvironments(res.environments ?? []))
      .catch(() => {});
    flagEnvStateApi
      .list(id)
      .then((res) => setEnvStates(res.environment_states ?? []))
      .catch(() => {});
    flagsApi.listRuleEnvStates(id)
      .then((res) => setRuleEnvStates(res.rule_environment_states ?? []))
      .catch(() => {});
  }, [orgSlug, id]);

  useEffect(() => {
    setNewRule((prev) => ({ ...prev, priority: rules.length + 1 }));
  }, [rules]);

  useEffect(() => {
    if (activeTab !== 'history' || !id) return;
    setHistoryLoading(true);
    auditApi
      .query({ resource_type: 'flag', resource_id: id, limit: 50 })
      .then((res) => setHistory(res.entries ?? []))
      .catch(() => setHistory([]))
      .finally(() => setHistoryLoading(false));
  }, [activeTab, id]);

  useEffect(() => {
    if (flag) {
      setSettingsForm({
        name: flag.name,
        description: flag.description ?? '',
        category: flag.category,
        purpose: flag.purpose ?? '',
        owners: (flag.owners ?? []).join(', '),
        is_permanent: flag.is_permanent,
        expires_at: flag.expires_at ? flag.expires_at.slice(0, 16) : '',
        default_value: flag.default_value,
        tags: (flag.tags ?? []).join(', '),
      });
    }
  }, [flag]);

  const handleAddRule = async () => {
    if (!id || !newRule.attribute || !newRule.value) return;
    try {
      const targetValues = ['in', 'not_in'].includes(newRule.operator)
        ? newRule.target_values.split(',').map((s) => s.trim()).filter(Boolean)
        : [newRule.target_values.trim()];
      await flagsApi.addRule(id, {
        rule_type: 'attribute',
        attribute: newRule.attribute,
        operator: newRule.operator,
        target_values: targetValues,
        value: newRule.value,
        priority: newRule.priority,
      });
      const res = await flagsApi.listRules(id);
      setRules(res.rules ?? []);
      setShowAddRule(false);
      setNewRule({ attribute: '', operator: 'equals', target_values: '', value: '', priority: (res.rules?.length ?? 0) + 1 });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add rule');
    }
  };

  const handleDeleteRule = async (ruleId: string) => {
    if (!id) return;
    if (!window.confirm('Delete this targeting rule?')) return;
    try {
      await flagsApi.deleteRule(id, ruleId);
      setRules((prev) => prev.filter((r) => r.id !== ruleId));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete rule');
    }
  };

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!flag) return <div>Flag not found.</div>;

  const handleRuleEnvToggle = async (ruleId: string, envId: string, currentEnabled: boolean) => {
    if (!id) return;
    const nextEnabled = !currentEnabled;
    setRuleEnvStates((prev) => {
      const existing = prev.find((s) => s.rule_id === ruleId && s.environment_id === envId);
      if (existing) {
        return prev.map((s) => s.rule_id === ruleId && s.environment_id === envId ? { ...s, enabled: nextEnabled } : s);
      }
      return [...prev, { id: '', rule_id: ruleId, environment_id: envId, enabled: nextEnabled, created_at: '', updated_at: '' }];
    });
    try {
      await flagsApi.setRuleEnvState(id, ruleId, envId, { enabled: nextEnabled });
    } catch (err) {
      setRuleEnvStates((prev) =>
        prev.map((s) => s.rule_id === ruleId && s.environment_id === envId ? { ...s, enabled: currentEnabled } : s),
      );
      setError(err instanceof Error ? err.message : 'Failed to toggle rule');
    }
  };

  const toggleEnvAccordion = (envId: string) => {
    setExpandedEnvs((prev) => {
      const next = new Set(prev);
      if (next.has(envId)) next.delete(envId);
      else next.add(envId);
      return next;
    });
  };

  const saveEnvState = async (envId: string, updates: { enabled?: boolean; value?: unknown }) => {
    if (!flag) return;
    const prev = envStates.find((s) => s.environment_id === envId);
    const current = { enabled: prev?.enabled ?? false, value: prev?.value ?? null };
    const next = { ...current, ...updates };

    // Optimistic update
    setEnvStates((states) => {
      const existing = states.find((s) => s.environment_id === envId);
      if (existing) {
        return states.map((s) => (s.environment_id === envId ? { ...s, ...next } : s));
      }
      return [...states, { id: '', flag_id: flag.id, environment_id: envId, enabled: next.enabled, value: next.value, updated_by: '', updated_at: '' } as FlagEnvironmentState];
    });
    try {
      await flagEnvStateApi.set(flag.id, envId, next);
    } catch (err) {
      // Revert on failure
      setEnvStates((states) =>
        states.map((s) => (s.environment_id === envId ? { ...s, ...current } : s)),
      );
      setError(err instanceof Error ? err.message : 'Failed to update environment state');
    }
  };

  const handleEnvToggle = (envId: string, currentEnabled: boolean) => {
    saveEnvState(envId, { enabled: !currentEnabled });
  };

  const handleEnvValueChange = (envId: string, newValue: string) => {
    saveEnvState(envId, { value: newValue });
  };

  const handleArchive = () => {
    setFlag((prev) => (prev ? { ...prev, archived: true } : prev));
  };

  const handleSettingsSave = async () => {
    if (!flag || !settingsForm || !id) return;
    setSettingsSaving(true);
    setSettingsSuccess(false);
    setError(null);
    try {
      const updated = await flagsApi.update(id, {
        name: settingsForm.name,
        description: settingsForm.description,
        category: settingsForm.category as FlagCategory,
        purpose: settingsForm.purpose,
        owners: settingsForm.owners.split(',').map((s) => s.trim()).filter(Boolean),
        is_permanent: settingsForm.is_permanent,
        expires_at: settingsForm.is_permanent ? undefined : settingsForm.expires_at ? settingsForm.expires_at + ':00Z' : undefined,
        default_value: settingsForm.default_value,
      });
      setFlag(updated);
      setSettingsSuccess(true);
      setTimeout(() => setSettingsSuccess(false), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update flag');
    } finally {
      setSettingsSaving(false);
    }
  };

  return (
    <div>
      {/* Header Section */}
      <div className="detail-header">
        <Link to={backPath}>&larr; Back to Flags</Link>

        <div className="detail-header-top">
          <div>
            <h1 className="detail-header-title">{flag.name}</h1>
            <span className="detail-header-subtitle">{flag.key}</span>
          </div>
          <div className="detail-header-badges">
            <span className={`badge badge-${flag.category}`}>{flag.category}</span>
            {flag.archived && <span className="badge badge-disabled">Archived</span>}
          </div>
        </div>

        <div className="detail-chips">
          <span>Type: {flag.flag_type}</span>
          <span>Owners: {(flag.owners ?? []).join(', ')}</span>
          <span>
            Expires:{' '}
            {flag.is_permanent
              ? 'Permanent'
              : flag.expires_at
                ? formatDate(flag.expires_at)
                : '\u2014'}
          </span>
          <span>
            Default Value: <span className="font-mono">{flag.default_value}</span>
          </span>
          <span>
            Scope:{' '}
            {flag.application_id ? getAppNameById(flag.application_id, apps) : 'Project-wide'}
          </span>
          {flag.purpose && <span>Purpose: {flag.purpose}</span>}
          {(flag.tags ?? []).length > 0 && <span>Tags: {(flag.tags ?? []).join(', ')}</span>}
        </div>

        <div className="detail-secondary">
          <span>Created by {flag.created_by_name || flag.created_by}</span>
          <span>Created {formatDateTime(flag.created_at)}</span>
          <span>Updated {formatDateTime(flag.updated_at)}</span>
        </div>

        {flag.description && <div className="detail-description">{flag.description}</div>}
      </div>

      {/* Tabs */}
      <div className="detail-tabs">
        <button
          className={`detail-tab${activeTab === 'environments' ? ' active' : ''}`}
          onClick={() => setActiveTab('environments')}
        >
          Environments
        </button>
        <button
          className={`detail-tab${activeTab === 'rules' ? ' active' : ''}`}
          onClick={() => setActiveTab('rules')}
        >
          Targeting Rules
        </button>
        <button
          className={`detail-tab${activeTab === 'yaml' ? ' active' : ''}`}
          onClick={() => setActiveTab('yaml')}
        >
          YAML
        </button>
        <button
          className={`detail-tab${activeTab === 'settings' ? ' active' : ''}`}
          onClick={() => setActiveTab('settings')}
        >
          Settings
        </button>
        <button
          className={`detail-tab${activeTab === 'history' ? ' active' : ''}`}
          onClick={() => setActiveTab('history')}
        >
          History
        </button>
      </div>

      {/* Tab: Targeting Rules */}
      {activeTab === 'rules' && (
        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <span>{rules.length} rule{rules.length !== 1 ? 's' : ''}</span>
            <button className="btn btn-secondary" onClick={() => setShowAddRule(!showAddRule)}>
              {showAddRule ? 'Cancel' : 'Add Rule'}
            </button>
          </div>

          {showAddRule && (
            <div style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)', borderRadius: 6, padding: 16, marginBottom: 16 }}>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Attribute</label>
                  <input className="form-input" type="text" placeholder="e.g. userType" value={newRule.attribute} onChange={(e) => setNewRule({ ...newRule, attribute: e.target.value })} />
                </div>
                <div className="form-group">
                  <label className="form-label">Operator</label>
                  <select className="form-select" value={newRule.operator} onChange={(e) => setNewRule({ ...newRule, operator: e.target.value })}>
                    <option value="equals">equals</option>
                    <option value="not_equals">not equals</option>
                    <option value="in">in</option>
                    <option value="not_in">not in</option>
                    <option value="contains">contains</option>
                    <option value="starts_with">starts with</option>
                    <option value="ends_with">ends with</option>
                    <option value="greater_than">greater than</option>
                    <option value="less_than">less than</option>
                  </select>
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Target Value(s)</label>
                  <input className="form-input" type="text" placeholder={['in', 'not_in'].includes(newRule.operator) ? 'comma-separated values' : 'value'} value={newRule.target_values} onChange={(e) => setNewRule({ ...newRule, target_values: e.target.value })} />
                </div>
                <div className="form-group">
                  <label className="form-label">Serve Value</label>
                  <input className="form-input" type="text" placeholder="e.g. true" value={newRule.value} onChange={(e) => setNewRule({ ...newRule, value: e.target.value })} />
                </div>
                <div className="form-group">
                  <label className="form-label">Priority</label>
                  <input className="form-input" type="number" min={1} value={newRule.priority} onChange={(e) => setNewRule({ ...newRule, priority: parseInt(e.target.value) || 1 })} />
                </div>
              </div>
              <button className="btn btn-primary" onClick={handleAddRule} disabled={!newRule.attribute || !newRule.value}>
                Create Rule
              </button>
            </div>
          )}

          <table>
            <thead>
              <tr>
                <th>Priority</th>
                <th>Condition</th>
                <th>Serve Value</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td>{rule.priority}</td>
                  <td>{rule.attribute} {rule.operator} {(rule.target_values ?? []).join(', ')}</td>
                  <td className="font-mono">{rule.value}</td>
                  <td>
                    <button className="btn btn-sm btn-danger" onClick={() => handleDeleteRule(rule.id)}>Delete</button>
                  </td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr><td colSpan={4} style={{ textAlign: 'center' }}>No targeting rules defined.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Tab: Environments */}
      {activeTab === 'environments' && (
        <div className="card">
          {environments.length === 0 ? (
            <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
              No environments configured. Add environments in org settings.
            </p>
          ) : (
            <div>
              {environments.map((env) => {
                const state = envStates.find((s) => s.environment_id === env.id);
                const isEnabled = state?.enabled ?? false;
                const isExpanded = expandedEnvs.has(env.id);
                return (
                  <div key={env.id} style={{ borderBottom: '1px solid var(--color-border)', paddingBottom: 12, marginBottom: 12 }}>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <button
                          onClick={() => toggleEnvAccordion(env.id)}
                          style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text)', fontSize: 14 }}
                        >
                          {isExpanded ? '\u25be' : '\u25b8'}
                        </button>
                        <strong>{env.name}</strong>
                        {env.is_production && <span className="badge badge-enabled" style={{ fontSize: 11 }}>production</span>}
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
                        {flag.flag_type === 'boolean' ? (
                          <select
                            className="form-input"
                            style={{ width: 90, fontSize: 13, padding: '2px 6px' }}
                            value={state?.value != null ? String(state.value) : flag.default_value}
                            onChange={(e) => handleEnvValueChange(env.id, e.target.value)}
                          >
                            <option value="true">true</option>
                            <option value="false">false</option>
                          </select>
                        ) : (
                          <input
                            className="form-input font-mono"
                            style={{ width: 120, fontSize: 13, padding: '2px 6px' }}
                            value={state?.value != null ? String(state.value) : flag.default_value}
                            onBlur={(e) => handleEnvValueChange(env.id, e.target.value)}
                            onChange={(e) => {
                              const v = e.target.value;
                              setEnvStates((s) => s.map((st) => st.environment_id === env.id ? { ...st, value: v } : st));
                            }}
                          />
                        )}
                        <label className="toggle-switch">
                          <input type="checkbox" checked={isEnabled} onChange={() => handleEnvToggle(env.id, isEnabled)} />
                          <span className="toggle-track"></span>
                        </label>
                      </div>
                    </div>

                    {isExpanded && (
                      <div style={{ marginLeft: 28, marginTop: 8 }}>
                        {rules.length === 0 ? (
                          <p className="text-muted" style={{ fontSize: 13 }}>No targeting rules defined.</p>
                        ) : (
                          <table style={{ fontSize: 13 }}>
                            <thead>
                              <tr>
                                <th>Rule</th>
                                <th>Serve Value</th>
                                <th>Enabled</th>
                              </tr>
                            </thead>
                            <tbody>
                              {rules.map((rule) => {
                                const ruleState = ruleEnvStates.find(
                                  (s) => s.rule_id === rule.id && s.environment_id === env.id,
                                );
                                const ruleEnabled = ruleState?.enabled ?? false;
                                return (
                                  <tr key={rule.id}>
                                    <td>{rule.attribute} {rule.operator} {(rule.target_values ?? []).join(', ')}</td>
                                    <td className="font-mono">{rule.value}</td>
                                    <td>
                                      <label className="toggle-switch">
                                        <input
                                          type="checkbox"
                                          checked={ruleEnabled}
                                          onChange={() => handleRuleEnvToggle(rule.id, env.id, ruleEnabled)}
                                        />
                                        <span className="toggle-track"></span>
                                      </label>
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Tab: YAML Preview */}
      {activeTab === 'yaml' && flag && (
        <div className="card">
          <p className="text-muted" style={{ marginBottom: 12, fontSize: 13 }}>
            Read-only preview of this flag's configuration. Use the project-level export for the full file.
          </p>
          <pre style={{
            background: 'var(--color-bg-secondary)',
            border: '1px solid var(--color-border)',
            borderRadius: 6,
            padding: 16,
            fontSize: 13,
            overflow: 'auto',
            maxHeight: 500,
          }}>
            {generateFlagYaml(flag, rules, environments, envStates, ruleEnvStates)}
          </pre>
        </div>
      )}

      {/* Tab: Settings */}
      {activeTab === 'settings' && settingsForm && (
        <div className="card">
          <div className="form-group">
            <label className="form-label">Name</label>
            <input className="form-input" value={settingsForm.name}
              onChange={(e) => setSettingsForm({ ...settingsForm, name: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea className="form-input" rows={3} value={settingsForm.description}
              onChange={(e) => setSettingsForm({ ...settingsForm, description: e.target.value })} />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Category</label>
              <select className="form-select" value={settingsForm.category}
                onChange={(e) => setSettingsForm({ ...settingsForm, category: e.target.value })}>
                <option value="release">Release</option>
                <option value="feature">Feature</option>
                <option value="experiment">Experiment</option>
                <option value="ops">Ops</option>
                <option value="permission">Permission</option>
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">Default Value</label>
              {flag.flag_type === 'boolean' ? (
                <select className="form-select" value={settingsForm.default_value}
                  onChange={(e) => setSettingsForm({ ...settingsForm, default_value: e.target.value })}>
                  <option value="true">true</option>
                  <option value="false">false</option>
                </select>
              ) : (
                <input className="form-input" value={settingsForm.default_value}
                  onChange={(e) => setSettingsForm({ ...settingsForm, default_value: e.target.value })} />
              )}
            </div>
          </div>
          <div className="form-group">
            <label className="form-label">Purpose</label>
            <input className="form-input" value={settingsForm.purpose}
              onChange={(e) => setSettingsForm({ ...settingsForm, purpose: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">Owners (comma-separated)</label>
            <input className="form-input" value={settingsForm.owners}
              onChange={(e) => setSettingsForm({ ...settingsForm, owners: e.target.value })} />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={settingsForm.is_permanent}
                  onChange={(e) => setSettingsForm({ ...settingsForm, is_permanent: e.target.checked })} />
                Permanent flag
              </label>
            </div>
            {!settingsForm.is_permanent && (
              <div className="form-group">
                <label className="form-label">Expires At</label>
                <input className="form-input" type="datetime-local" value={settingsForm.expires_at}
                  onChange={(e) => setSettingsForm({ ...settingsForm, expires_at: e.target.value })} />
              </div>
            )}
          </div>
          <div className="form-group">
            <label className="form-label">Tags (comma-separated)</label>
            <input className="form-input" value={settingsForm.tags}
              onChange={(e) => setSettingsForm({ ...settingsForm, tags: e.target.value })} />
          </div>
          <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
            <button className="btn btn-primary" onClick={handleSettingsSave} disabled={settingsSaving}>
              {settingsSaving ? 'Saving...' : 'Save Changes'}
            </button>
            {settingsSuccess && <span style={{ color: 'var(--color-success)', fontSize: 13 }}>Saved successfully</span>}
          </div>
        </div>
      )}

      {/* Tab: History */}
      {activeTab === 'history' && (
        <div className="card">
          {historyLoading ? (
            <p style={{ textAlign: 'center', padding: '2rem 0' }}>Loading history...</p>
          ) : history.length === 0 ? (
            <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
              No history recorded yet.
            </p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>User</th>
                  <th>Action</th>
                  <th>Details</th>
                </tr>
              </thead>
              <tbody>
                {history.map((entry) => (
                  <tr key={entry.id}>
                    <td style={{ whiteSpace: 'nowrap', fontSize: 12 }}>{formatDateTime(entry.created_at)}</td>
                    <td>{entry.actor_name || 'System'}</td>
                    <td>{describeAction(entry)}</td>
                    <td style={{ fontSize: 12 }}>
                      {entry.old_value && (
                        <div>
                          <span className="text-muted">Before: </span>
                          <code style={{ fontSize: 11 }}>{entry.old_value}</code>
                        </div>
                      )}
                      {entry.new_value && (
                        <div>
                          <span className="text-muted">After: </span>
                          <code style={{ fontSize: 11 }}>{entry.new_value}</code>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Danger Zone */}
      <div className="danger-zone">
        <h2>Danger Zone</h2>
        <p className="text-muted">
          Archiving a flag disables it permanently and removes it from active use. This action
          cannot be easily undone.
        </p>
        <button className="btn btn-danger" onClick={handleArchive} disabled={flag.archived}>
          {flag.archived ? 'Archived' : 'Archive Flag'}
        </button>
      </div>
    </div>
  );
}
