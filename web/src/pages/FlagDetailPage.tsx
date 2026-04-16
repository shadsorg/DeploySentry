import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Flag, TargetingRule, OrgEnvironment, FlagEnvironmentState, RuleEnvironmentState } from '@/types';
import { flagsApi, entitiesApi, flagEnvStateApi } from '@/api';
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
  const [activeTab, setActiveTab] = useState<'rules' | 'environments'>('rules');
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

  const handleEnvToggle = async (envId: string, currentEnabled: boolean) => {
    if (!flag) return;
    const nextEnabled = !currentEnabled;
    // Optimistic update
    setEnvStates((prev) =>
      prev.map((s) => (s.environment_id === envId ? { ...s, enabled: nextEnabled } : s)),
    );
    try {
      await flagEnvStateApi.set(flag.id, envId, { enabled: nextEnabled });
    } catch (err) {
      // Revert on failure
      setEnvStates((prev) =>
        prev.map((s) => (s.environment_id === envId ? { ...s, enabled: currentEnabled } : s)),
      );
      setError(err instanceof Error ? err.message : 'Failed to toggle environment');
    }
  };

  const handleArchive = () => {
    setFlag((prev) => (prev ? { ...prev, archived: true } : prev));
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
          <span>Created by {flag.created_by}</span>
          <span>Created {formatDateTime(flag.created_at)}</span>
          <span>Updated {formatDateTime(flag.updated_at)}</span>
        </div>

        {flag.description && <div className="detail-description">{flag.description}</div>}
      </div>

      {/* Tabs */}
      <div className="detail-tabs">
        <button
          className={`detail-tab${activeTab === 'rules' ? ' active' : ''}`}
          onClick={() => setActiveTab('rules')}
        >
          Targeting Rules
        </button>
        <button
          className={`detail-tab${activeTab === 'environments' ? ' active' : ''}`}
          onClick={() => setActiveTab('environments')}
        >
          Environments
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
                        <span className="font-mono" style={{ fontSize: 13 }}>
                          {state?.value != null ? String(state.value) : '\u2014'}
                        </span>
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
