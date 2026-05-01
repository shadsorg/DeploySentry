import { useState, useEffect, useMemo, Fragment } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import type {
  Flag,
  TargetingRule,
  OrgEnvironment,
  FlagEnvironmentState,
  RuleEnvironmentState,
  FlagCategory,
  AuditLogEntry,
  RolloutPolicy,
} from '@/types';
import { flagsApi, entitiesApi, flagEnvStateApi, auditApi, rolloutPolicyApi } from '@/api';
import type { Application } from '@/types';
import { StrategyPicker } from '@/components/rollout/StrategyPicker';
import ConfirmDialog from '@/components/ConfirmDialog';
import { GroupPicker } from '@/components/rollout/GroupPicker';
import { resolvePolicy } from '@/lib/policyResolver';
import { useStagingEnabled } from '@/hooks/useStagingEnabled';
import { stageOrCall } from '@/hooks/stageOrCall';

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
    case 'flag.created':
      return 'Created flag';
    case 'flag.updated':
      return 'Updated flag settings';
    case 'flag.toggled':
      return 'Toggled flag';
    case 'flag.archived':
      return 'Archived flag';
    case 'flag.env_state.updated':
      return 'Updated environment state';
    case 'flag.rule.created':
      return 'Added targeting rule';
    case 'flag.rule.deleted':
      return 'Deleted targeting rule';
    case 'flag.rule.env_state.updated':
      return 'Updated rule environment state';
    case 'flag.smoke_test.recorded':
      return 'Smoke test recorded';
    case 'flag.user_test.recorded':
      return 'User test recorded';
    case 'flag.scheduled_for_removal.set':
      return 'Scheduled for removal';
    case 'flag.scheduled_for_removal.cancelled':
      return 'Removal cancelled';
    case 'flag.iteration_exhausted':
      return 'Iteration exhausted';
    default:
      return entry.action;
  }
}

function formatCountdown(targetIso: string): string {
  const ms = new Date(targetIso).getTime() - Date.now();
  if (ms <= 0) return 'due now';
  const days = Math.floor(ms / (24 * 60 * 60 * 1000));
  const hours = Math.floor((ms / (60 * 60 * 1000)) % 24);
  if (days > 0) return `in ${days}d ${hours}h`;
  const minutes = Math.floor((ms / (60 * 1000)) % 60);
  return `in ${hours}h ${minutes}m`;
}

function statusPillClass(status?: string | null): string {
  switch (status) {
    case 'pass':
      return 'badge-success';
    case 'fail':
      return 'badge-error';
    case 'pending':
      return 'badge-warning';
    default:
      return 'badge-neutral';
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
      lines.push(
        `      target_values: [${(rule.target_values ?? []).map((v) => `"${v}"`).join(', ')}]`,
      );
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
  const navigate = useNavigate();
  const stagingEnabled = useStagingEnabled(orgSlug);
  const backPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  const [flag, setFlag] = useState<Flag | null>(null);
  const [rules, setRules] = useState<TargetingRule[]>([]);
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<
    'environments' | 'rules' | 'yaml' | 'settings' | 'history' | 'lifecycle'
  >('environments');
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
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [ruleStrategyName, setRuleStrategyName] = useState('');
  const [ruleImmediate, setRuleImmediate] = useState(true);
  const [ruleGroupID, setRuleGroupID] = useState('');
  const [ruleSaveMsg, setRuleSaveMsg] = useState<string | null>(null);
  const [policies, setPolicies] = useState<RolloutPolicy[]>([]);
  const [confirmArchiveOpen, setConfirmArchiveOpen] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [confirmQueueOpen, setConfirmQueueOpen] = useState(false);
  const [confirmHardDeleteOpen, setConfirmHardDeleteOpen] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [queueing, setQueueing] = useState(false);
  const [hardDeleting, setHardDeleting] = useState(false);

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
    flagsApi
      .listRuleEnvStates(id)
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
      .query({ entity_type: 'flag', resource_id: id, limit: 50 })
      .then((res) => setHistory(res.entries ?? []))
      .catch(() => setHistory([]))
      .finally(() => setHistoryLoading(false));
  }, [activeTab, id]);

  useEffect(() => {
    if (activeTab !== 'rules' || !orgSlug) return;
    rolloutPolicyApi
      .list(orgSlug)
      .then((r) => setPolicies(r.items ?? []))
      .catch(() => {});
  }, [activeTab, orgSlug]);

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
        ? newRule.target_values
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean)
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
      setNewRule({
        attribute: '',
        operator: 'equals',
        target_values: '',
        value: '',
        priority: (res.rules?.length ?? 0) + 1,
      });
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

  const handleUpdateRule = async (ruleId: string) => {
    if (!id) return;
    try {
      const body: Partial<TargetingRule> & {
        rollout?: { strategy_name?: string; apply_immediately?: boolean };
      } = {};
      if (ruleStrategyName && !ruleImmediate) {
        body.rollout = {
          strategy_name: ruleStrategyName,
          ...(ruleGroupID ? { release_id: ruleGroupID } : {}),
        };
      } else if (ruleImmediate) {
        body.rollout = { apply_immediately: true };
      }
      await flagsApi.updateRule(id, ruleId, body as Partial<TargetingRule>);
      const isRollout = ruleStrategyName && !ruleImmediate;
      setRuleSaveMsg(isRollout ? 'Rollout started' : 'Rule saved');
      setEditingRuleId(null);
      setRuleStrategyName('');
      setRuleImmediate(true);
      setRuleGroupID('');
      setTimeout(() => setRuleSaveMsg(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update rule');
    }
  };

  const lifecycleState = useMemo(() => {
    if (!flag) return 'active';
    if (!flag.archived_at) return 'active';
    const elapsedAt = new Date(flag.archived_at).getTime() + 30 * 24 * 3600 * 1000;
    return Date.now() >= elapsedAt ? 'elapsed' : 'within';
  }, [flag?.archived_at]);

  if (loading)
    return (
      <div className="empty-state" style={{ padding: '40px 0' }}>
        <span
          className="ms"
          style={{
            fontSize: 32,
            color: 'var(--color-primary)',
            marginBottom: 12,
            display: 'block',
          }}
        >
          sync
        </span>
        Loading flag…
      </div>
    );
  if (error) return <div className="page-error">Error: {error}</div>;
  if (!flag) return <div className="page-error">Flag not found.</div>;

  const handleRuleEnvToggle = async (ruleId: string, envId: string, currentEnabled: boolean) => {
    if (!id) return;
    const nextEnabled = !currentEnabled;
    setRuleEnvStates((prev) => {
      const existing = prev.find((s) => s.rule_id === ruleId && s.environment_id === envId);
      if (existing) {
        return prev.map((s) =>
          s.rule_id === ruleId && s.environment_id === envId ? { ...s, enabled: nextEnabled } : s,
        );
      }
      return [
        ...prev,
        {
          id: '',
          rule_id: ruleId,
          environment_id: envId,
          enabled: nextEnabled,
          created_at: '',
          updated_at: '',
        },
      ];
    });
    try {
      await flagsApi.setRuleEnvState(id, ruleId, envId, { enabled: nextEnabled });
    } catch (err) {
      setRuleEnvStates((prev) =>
        prev.map((s) =>
          s.rule_id === ruleId && s.environment_id === envId
            ? { ...s, enabled: currentEnabled }
            : s,
        ),
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
      return [
        ...states,
        {
          id: '',
          flag_id: flag.id,
          environment_id: envId,
          enabled: next.enabled,
          value: next.value,
          updated_by: '',
          updated_at: '',
        } as FlagEnvironmentState,
      ];
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

  const handleArchive = async () => {
    if (!id || !orgSlug) return;
    setArchiving(true);
    setError(null);
    try {
      const outcome = await stageOrCall({
        staged: stagingEnabled,
        orgSlug,
        stage: {
          resource_type: 'flag',
          resource_id: id,
          action: 'archive',
          old_value: flag ? { archived: flag.archived } : undefined,
          new_value: { archived: true },
        },
        direct: () => flagsApi.archive(id),
      });
      // Reflect the user's intent in the local view either way: with
      // staging on, the banner overlay is the source of truth for "what
      // I see" until Deploy commits — leave the optimistic update in
      // place so the user doesn't see two clashing states.
      if (outcome.mode === 'direct') {
        setFlag((prev) => (prev ? { ...prev, archived: true } : prev));
      } else {
        setFlag((prev) => (prev ? { ...prev, archived: true } : prev));
      }
      setConfirmArchiveOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to archive flag');
    } finally {
      setArchiving(false);
    }
  };

  const handleRestore = async () => {
    if (!id || !orgSlug) return;
    setRestoring(true);
    setError(null);
    try {
      const outcome = await stageOrCall({
        staged: stagingEnabled,
        orgSlug,
        stage: {
          resource_type: 'flag',
          resource_id: id,
          action: 'restore',
          old_value: flag ? { archived: flag.archived } : undefined,
          new_value: { archived: false },
        },
        direct: () => flagsApi.restore(id),
      });
      if (outcome.mode === 'direct') {
        // Direct path returns the canonical Flag — replace local state.
        setFlag(outcome.result);
      } else {
        // Staged path: optimistic local flip; Deploy will materialise it.
        setFlag((prev) => (prev ? { ...prev, archived: false } : prev));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to restore flag');
    } finally {
      setRestoring(false);
    }
  };

  const handleQueueDeletion = async () => {
    if (!id) return;
    setQueueing(true);
    setError(null);
    try {
      const updated = await flagsApi.queueDeletion(id);
      setFlag(updated);
      setConfirmQueueOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to queue for deletion');
    } finally {
      setQueueing(false);
    }
  };

  const handleCancelQueuedDeletion = async () => {
    if (!id) return;
    setQueueing(true);
    setError(null);
    try {
      const updated = await flagsApi.cancelQueuedDeletion(id);
      setFlag(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel queued deletion');
    } finally {
      setQueueing(false);
    }
  };

  const handleHardDelete = async () => {
    if (!id || !flag) return;
    setHardDeleting(true);
    setError(null);
    try {
      await flagsApi.hardDelete(id, flag.key);
      navigate(-1);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete flag');
      setHardDeleting(false);
    }
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
        owners: settingsForm.owners
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        is_permanent: settingsForm.is_permanent,
        expires_at: settingsForm.is_permanent
          ? undefined
          : settingsForm.expires_at
            ? settingsForm.expires_at + ':00Z'
            : undefined,
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
        <Link
          to={backPath}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            fontSize: 13,
            color: 'var(--color-text-muted)',
            textDecoration: 'none',
            marginBottom: 12,
          }}
        >
          <span className="ms" style={{ fontSize: 14 }}>
            arrow_back
          </span>
          Back to Flags
        </Link>

        <div className="detail-header-top">
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
              <div
                style={{
                  width: 36,
                  height: 36,
                  borderRadius: 10,
                  background: 'var(--color-primary-bg)',
                  border: '1px solid rgba(99,102,241,0.25)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  flexShrink: 0,
                }}
              >
                <span className="ms" style={{ fontSize: 18, color: 'var(--color-primary)' }}>
                  toggle_on
                </span>
              </div>
              <h1
                className="detail-header-title"
                style={{
                  fontFamily: 'var(--font-display)',
                  fontWeight: 800,
                  letterSpacing: '-0.02em',
                }}
              >
                {flag.name}
              </h1>
            </div>
            <span className="flag-key">{flag.key}</span>
          </div>
          <div className="detail-header-badges">
            <span className={`badge badge-${flag.category}`}>{flag.category}</span>
            {flag.archived && <span className="badge badge-disabled">Archived</span>}
            <span className="badge badge-ops">{flag.flag_type}</span>
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
        {[
          { key: 'environments', icon: 'lan', label: 'Environments' },
          { key: 'rules', icon: 'rule', label: 'Targeting Rules' },
          { key: 'yaml', icon: 'code', label: 'YAML' },
          { key: 'settings', icon: 'tune', label: 'Settings' },
          { key: 'lifecycle', icon: 'event_available', label: 'Lifecycle' },
        ].map(({ key, icon, label }) => (
          <button
            key={key}
            className={`detail-tab${activeTab === key ? ' active' : ''}`}
            onClick={() => setActiveTab(key as typeof activeTab)}
          >
            <span className="ms" style={{ fontSize: 15, verticalAlign: 'middle', marginRight: 5 }}>
              {icon}
            </span>
            {label}
          </button>
        ))}
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
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '1rem',
            }}
          >
            <span>
              {rules.length} rule{rules.length !== 1 ? 's' : ''}
            </span>
            <button className="btn btn-secondary" onClick={() => setShowAddRule(!showAddRule)}>
              {showAddRule ? 'Cancel' : 'Add Rule'}
            </button>
          </div>

          {showAddRule && (
            <div
              style={{
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
                borderRadius: 6,
                padding: 16,
                marginBottom: 16,
              }}
            >
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Attribute</label>
                  <input
                    className="form-input"
                    type="text"
                    placeholder="e.g. userType"
                    value={newRule.attribute}
                    onChange={(e) => setNewRule({ ...newRule, attribute: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Operator</label>
                  <select
                    className="form-select"
                    value={newRule.operator}
                    onChange={(e) => setNewRule({ ...newRule, operator: e.target.value })}
                  >
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
                  <input
                    className="form-input"
                    type="text"
                    placeholder={
                      ['in', 'not_in'].includes(newRule.operator)
                        ? 'comma-separated values'
                        : 'value'
                    }
                    value={newRule.target_values}
                    onChange={(e) => setNewRule({ ...newRule, target_values: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Serve Value</label>
                  <input
                    className="form-input"
                    type="text"
                    placeholder="e.g. true"
                    value={newRule.value}
                    onChange={(e) => setNewRule({ ...newRule, value: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Priority</label>
                  <input
                    className="form-input"
                    type="number"
                    min={1}
                    value={newRule.priority}
                    onChange={(e) =>
                      setNewRule({ ...newRule, priority: parseInt(e.target.value) || 1 })
                    }
                  />
                </div>
              </div>
              <button
                className="btn btn-primary"
                onClick={handleAddRule}
                disabled={!newRule.attribute || !newRule.value}
              >
                Create Rule
              </button>
            </div>
          )}

          {ruleSaveMsg && (
            <div
              style={{
                marginBottom: 12,
                padding: '8px 12px',
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
                borderRadius: 4,
                color: 'var(--color-success)',
              }}
            >
              {ruleSaveMsg}
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
                <Fragment key={rule.id}>
                  <tr>
                    <td>{rule.priority}</td>
                    <td>
                      {rule.attribute} {rule.operator} {(rule.target_values ?? []).join(', ')}
                    </td>
                    <td className="font-mono">{rule.value}</td>
                    <td style={{ display: 'flex', gap: 6 }}>
                      <button
                        className="btn btn-sm btn-secondary"
                        onClick={() => {
                          if (editingRuleId === rule.id) {
                            setEditingRuleId(null);
                          } else {
                            setEditingRuleId(rule.id);
                            setRuleStrategyName('');
                            setRuleImmediate(true);
                            setRuleGroupID('');
                          }
                        }}
                      >
                        {editingRuleId === rule.id ? 'Cancel' : 'Edit'}
                      </button>
                      <button
                        className="btn btn-sm btn-danger"
                        onClick={() => handleDeleteRule(rule.id)}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                  {editingRuleId === rule.id &&
                    orgSlug &&
                    (() => {
                      const flagEnvName = environments.find(
                        (e) => e.id === flag?.environment_id,
                      )?.name;
                      const eff = resolvePolicy(policies, flagEnvName, 'config');
                      const ruleBlocked =
                        eff.enabled &&
                        eff.policy === 'mandate' &&
                        !ruleStrategyName &&
                        !ruleImmediate;
                      return (
                        <tr>
                          <td colSpan={4}>
                            <div style={{ padding: '12px 0' }}>
                              {eff.enabled && eff.policy === 'mandate' && (
                                <p
                                  className="helper-text"
                                  style={{ color: 'var(--color-danger)', marginBottom: 6 }}
                                >
                                  Strategy required for this scope.
                                </p>
                              )}
                              {eff.enabled && eff.policy === 'prompt' && (
                                <p className="helper-text" style={{ marginBottom: 6 }}>
                                  Strategy recommended; uncheck &ldquo;apply immediately&rdquo; to
                                  attach one.
                                </p>
                              )}
                              <StrategyPicker
                                orgSlug={orgSlug}
                                targetType="config"
                                value={ruleStrategyName}
                                onChange={setRuleStrategyName}
                                allowImmediate={eff.policy !== 'mandate' || !eff.enabled}
                                immediate={ruleImmediate}
                                onImmediateChange={setRuleImmediate}
                              />
                              {!ruleImmediate && ruleStrategyName && (
                                <div style={{ marginTop: 8 }}>
                                  <label className="form-label" style={{ fontSize: 13 }}>
                                    Rollout Group (optional)
                                  </label>
                                  <GroupPicker
                                    orgSlug={orgSlug}
                                    value={ruleGroupID}
                                    onChange={setRuleGroupID}
                                  />
                                </div>
                              )}
                              <button
                                className="btn btn-primary btn-sm"
                                style={{ marginTop: 8 }}
                                onClick={() => handleUpdateRule(rule.id)}
                                disabled={ruleBlocked}
                              >
                                Save
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })()}
                </Fragment>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={4} style={{ textAlign: 'center' }}>
                    No targeting rules defined.
                  </td>
                </tr>
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
                  <div
                    key={env.id}
                    style={{
                      borderBottom: '1px solid var(--color-border)',
                      paddingBottom: 12,
                      marginBottom: 12,
                    }}
                  >
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <button
                          onClick={() => toggleEnvAccordion(env.id)}
                          style={{
                            background: 'none',
                            border: 'none',
                            cursor: 'pointer',
                            color: 'var(--color-text)',
                            fontSize: 14,
                          }}
                        >
                          {isExpanded ? '\u25be' : '\u25b8'}
                        </button>
                        <strong>{env.name}</strong>
                        {env.is_production && (
                          <span className="badge badge-enabled" style={{ fontSize: 11 }}>
                            production
                          </span>
                        )}
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
                              setEnvStates((s) =>
                                s.map((st) =>
                                  st.environment_id === env.id ? { ...st, value: v } : st,
                                ),
                              );
                            }}
                          />
                        )}
                        <label className="toggle-switch">
                          <input
                            type="checkbox"
                            checked={isEnabled}
                            onChange={() => handleEnvToggle(env.id, isEnabled)}
                          />
                          <span className="toggle-track"></span>
                        </label>
                      </div>
                    </div>

                    {isExpanded && (
                      <div style={{ marginLeft: 28, marginTop: 8 }}>
                        {rules.length === 0 ? (
                          <p className="text-muted" style={{ fontSize: 13 }}>
                            No targeting rules defined.
                          </p>
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
                                    <td>
                                      {rule.attribute} {rule.operator}{' '}
                                      {(rule.target_values ?? []).join(', ')}
                                    </td>
                                    <td className="font-mono">{rule.value}</td>
                                    <td>
                                      <label className="toggle-switch">
                                        <input
                                          type="checkbox"
                                          checked={ruleEnabled}
                                          onChange={() =>
                                            handleRuleEnvToggle(rule.id, env.id, ruleEnabled)
                                          }
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
            Read-only preview of this flag's configuration. Use the project-level export for the
            full file.
          </p>
          <pre
            style={{
              background: 'var(--color-bg-secondary)',
              border: '1px solid var(--color-border)',
              borderRadius: 6,
              padding: 16,
              fontSize: 13,
              overflow: 'auto',
              maxHeight: 500,
            }}
          >
            {generateFlagYaml(flag, rules, environments, envStates, ruleEnvStates)}
          </pre>
        </div>
      )}

      {/* Tab: Settings */}
      {activeTab === 'settings' && settingsForm && (
        <div className="card">
          <div className="form-group">
            <label className="form-label">Name</label>
            <input
              className="form-input"
              value={settingsForm.name}
              onChange={(e) => setSettingsForm({ ...settingsForm, name: e.target.value })}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea
              className="form-input"
              rows={3}
              value={settingsForm.description}
              onChange={(e) => setSettingsForm({ ...settingsForm, description: e.target.value })}
            />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Category</label>
              <select
                className="form-select"
                value={settingsForm.category}
                onChange={(e) => setSettingsForm({ ...settingsForm, category: e.target.value })}
              >
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
                <select
                  className="form-select"
                  value={settingsForm.default_value}
                  onChange={(e) =>
                    setSettingsForm({ ...settingsForm, default_value: e.target.value })
                  }
                >
                  <option value="true">true</option>
                  <option value="false">false</option>
                </select>
              ) : (
                <input
                  className="form-input"
                  value={settingsForm.default_value}
                  onChange={(e) =>
                    setSettingsForm({ ...settingsForm, default_value: e.target.value })
                  }
                />
              )}
            </div>
          </div>
          <div className="form-group">
            <label className="form-label">Purpose</label>
            <input
              className="form-input"
              value={settingsForm.purpose}
              onChange={(e) => setSettingsForm({ ...settingsForm, purpose: e.target.value })}
            />
          </div>
          <div className="form-group">
            <label className="form-label">Owners (comma-separated)</label>
            <input
              className="form-input"
              value={settingsForm.owners}
              onChange={(e) => setSettingsForm({ ...settingsForm, owners: e.target.value })}
            />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label
                className="form-label"
                style={{ display: 'flex', alignItems: 'center', gap: 8 }}
              >
                <input
                  type="checkbox"
                  checked={settingsForm.is_permanent}
                  onChange={(e) =>
                    setSettingsForm({ ...settingsForm, is_permanent: e.target.checked })
                  }
                />
                Permanent flag
              </label>
            </div>
            {!settingsForm.is_permanent && (
              <div className="form-group">
                <label className="form-label">Expires At</label>
                <input
                  className="form-input"
                  type="datetime-local"
                  value={settingsForm.expires_at}
                  onChange={(e) => setSettingsForm({ ...settingsForm, expires_at: e.target.value })}
                />
              </div>
            )}
          </div>
          <div className="form-group">
            <label className="form-label">Tags (comma-separated)</label>
            <input
              className="form-input"
              value={settingsForm.tags}
              onChange={(e) => setSettingsForm({ ...settingsForm, tags: e.target.value })}
            />
          </div>
          <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
            <button
              className="btn btn-primary"
              onClick={handleSettingsSave}
              disabled={settingsSaving}
            >
              {settingsSaving ? 'Saving...' : 'Save Changes'}
            </button>
            {settingsSuccess && (
              <span style={{ color: 'var(--color-success)', fontSize: 13 }}>
                Saved successfully
              </span>
            )}
          </div>

          {/* Lifecycle panel — state-driven */}
          {lifecycleState === 'active' && (
            <div className="danger-zone" style={{ marginTop: 32 }}>
              <h2>Danger Zone</h2>
              <p className="text-muted">
                Archiving a flag disables it for all environments and removes it from the active
                flag list. Archived flags can be restored at any time. After 30 days, they become
                eligible for permanent deletion.
              </p>
              <button className="btn btn-danger" onClick={() => setConfirmArchiveOpen(true)}>
                <span className="ms" style={{ fontSize: 16 }}>
                  archive
                </span>
                Archive Flag
              </button>
            </div>
          )}

          {lifecycleState === 'within' && (
            <div className="lifecycle-panel" style={{ marginTop: 32 }}>
              <h2>Archived</h2>
              <p>
                Archived on <strong>{new Date(flag.archived_at!).toLocaleDateString()}</strong> —
                eligible for permanent deletion on{' '}
                <strong>
                  {new Date(
                    new Date(flag.archived_at!).getTime() + 30 * 24 * 3600 * 1000,
                  ).toLocaleDateString()}
                </strong>
                .
              </p>
              {flag.delete_after && (
                <p style={{ color: 'var(--color-warning, #d97706)' }}>
                  ⚠ Queued for permanent deletion at{' '}
                  <strong>{new Date(flag.delete_after).toLocaleString()}</strong>. The retention
                  sweep will permanently remove this flag and its rules / ratings after that time.
                </p>
              )}
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <button className="btn btn-secondary" onClick={handleRestore} disabled={restoring}>
                  {restoring ? 'Restoring…' : 'Restore Flag'}
                </button>
                {!flag.delete_after && (
                  <button className="btn btn-danger" onClick={() => setConfirmQueueOpen(true)}>
                    Queue for Deletion
                  </button>
                )}
                {flag.delete_after && (
                  <button
                    className="btn btn-secondary"
                    onClick={handleCancelQueuedDeletion}
                    disabled={queueing}
                  >
                    {queueing ? 'Cancelling…' : 'Cancel Queued Deletion'}
                  </button>
                )}
              </div>
            </div>
          )}

          {lifecycleState === 'elapsed' && (
            <div className="lifecycle-panel danger-zone" style={{ marginTop: 32 }}>
              <h2>Eligible for permanent deletion</h2>
              <p>
                The 30-day retention window elapsed on{' '}
                <strong>
                  {new Date(
                    new Date(flag.archived_at!).getTime() + 30 * 24 * 3600 * 1000,
                  ).toLocaleDateString()}
                </strong>
                . The next retention sweep will permanently delete this flag, or you can do it now.
              </p>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <button className="btn btn-secondary" onClick={handleRestore} disabled={restoring}>
                  {restoring ? 'Restoring…' : 'Restore Flag'}
                </button>
                <button className="btn btn-danger" onClick={() => setConfirmHardDeleteOpen(true)}>
                  Permanently Delete
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirmArchiveOpen}
        title="Archive this flag?"
        message={`This will disable "${flag.name}" everywhere it is evaluated. SDKs will fall through to default values. The flag stays in the audit log and can be restored within 30 days.`}
        confirmLabel="Archive Flag"
        confirmVariant="danger"
        loading={archiving}
        requireTypedConfirm={flag.key}
        acknowledgement={`I confirm that "${flag.key}" is no longer in use or needed.`}
        onConfirm={handleArchive}
        onCancel={() => setConfirmArchiveOpen(false)}
      />

      <ConfirmDialog
        open={confirmQueueOpen}
        title="Queue this flag for permanent deletion?"
        message={`After the retention window (${flag.archived_at ? new Date(new Date(flag.archived_at).getTime() + 30 * 24 * 3600 * 1000).toLocaleDateString() : 'archived_at + 30 days'}), the flag will be permanently tombstoned by the retention sweep. You can revert this from the Audit page until the sweep fires.`}
        confirmLabel="Queue for Deletion"
        confirmVariant="danger"
        loading={queueing}
        requireTypedConfirm={flag.key}
        acknowledgement={`I understand "${flag.key}" will be tombstoned automatically after the retention window.`}
        onConfirm={handleQueueDeletion}
        onCancel={() => setConfirmQueueOpen(false)}
      />

      <ConfirmDialog
        open={confirmHardDeleteOpen}
        title="Permanently delete this flag?"
        message={`This is irreversible. "${flag.name}" and all its rules, ratings, and per-environment states will be removed. Audit history is preserved but the flag itself cannot be recovered.`}
        confirmLabel="Permanently Delete"
        confirmVariant="danger"
        loading={hardDeleting}
        requireTypedConfirm={flag.key}
        acknowledgement={`I have verified "${flag.key}" is no longer needed and accept that this action cannot be undone.`}
        onConfirm={handleHardDelete}
        onCancel={() => setConfirmHardDeleteOpen(false)}
      />

      {/* Tab: Lifecycle */}
      {activeTab === 'lifecycle' && (
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Feature Lifecycle</h3>
          <p className="text-muted" style={{ marginTop: 0 }}>
            Status reported by the CrowdSoft feature-agent (or any controller that calls the
            lifecycle API). All fields are optional — a flag with no lifecycle data behaves exactly
            like a flag in the traditional flow.
          </p>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr',
              gap: '1rem',
              marginBottom: '1rem',
            }}
          >
            <div>
              <label
                className="form-label"
                style={{
                  fontSize: 12,
                  textTransform: 'uppercase',
                  color: 'var(--color-text-muted)',
                }}
              >
                Smoke test
              </label>
              <div>
                <span className={`badge ${statusPillClass(flag.smoke_test_status)}`}>
                  {flag.smoke_test_status ?? 'not reported'}
                </span>
              </div>
              {flag.last_smoke_test_notes && (
                <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 4 }}>
                  {flag.last_smoke_test_notes}
                </p>
              )}
            </div>
            <div>
              <label
                className="form-label"
                style={{
                  fontSize: 12,
                  textTransform: 'uppercase',
                  color: 'var(--color-text-muted)',
                }}
              >
                User test
              </label>
              <div>
                <span className={`badge ${statusPillClass(flag.user_test_status)}`}>
                  {flag.user_test_status ?? 'not reported'}
                </span>
              </div>
              {flag.last_user_test_notes && (
                <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 4 }}>
                  {flag.last_user_test_notes}
                </p>
              )}
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <label
                className="form-label"
                style={{
                  fontSize: 12,
                  textTransform: 'uppercase',
                  color: 'var(--color-text-muted)',
                }}
              >
                Iteration
              </label>
              <div>
                <strong>{flag.iteration_count ?? 0}</strong>
                {flag.iteration_exhausted && (
                  <span className="badge badge-error" style={{ marginLeft: 8 }}>
                    exhausted
                  </span>
                )}
              </div>
            </div>
            <div>
              <label
                className="form-label"
                style={{
                  fontSize: 12,
                  textTransform: 'uppercase',
                  color: 'var(--color-text-muted)',
                }}
              >
                Scheduled removal
              </label>
              <div>
                {flag.scheduled_removal_at ? (
                  <>
                    <strong>{formatCountdown(flag.scheduled_removal_at)}</strong>{' '}
                    <span className="text-muted">
                      ({formatDateTime(flag.scheduled_removal_at)})
                    </span>
                  </>
                ) : (
                  <span className="text-muted">not scheduled</span>
                )}
              </div>
            </div>
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
                    <td style={{ whiteSpace: 'nowrap', fontSize: 12 }}>
                      {formatDateTime(entry.created_at)}
                    </td>
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
    </div>
  );
}
