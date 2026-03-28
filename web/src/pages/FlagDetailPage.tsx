import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Flag, TargetingRule, FlagEnvState } from '@/types';
import { MOCK_FLAG_ENV_STATE, MOCK_APPLICATIONS } from '@/mocks/hierarchy';

const MOCK_FLAG: Flag = {
  id: 'flag-001',
  project_id: 'proj-1',
  application_id: 'app-1',
  environment_id: 'env-prod',
  key: 'checkout-v2-rollout',
  name: 'Checkout V2 Rollout',
  description: 'Gradually roll out the new checkout flow to all users. Includes updated payment form, address validation, and order summary redesign.',
  flag_type: 'boolean',
  category: 'release',
  purpose: 'Migrate all users from legacy checkout to the redesigned V2 checkout experience.',
  owners: ['checkout-team', 'payments-team'],
  is_permanent: false,
  expires_at: '2026-06-01T00:00:00Z',
  enabled: true,
  default_value: 'false',
  archived: false,
  tags: ['checkout', 'migration', 'revenue'],
  created_by: 'alice',
  created_at: '2025-11-01T10:00:00Z',
  updated_at: '2026-03-18T14:30:00Z',
};

const MOCK_RULES: TargetingRule[] = [
  {
    id: 'rule-001',
    flag_id: 'flag-001',
    rule_type: 'percentage',
    priority: 1,
    value: 'true',
    percentage: 25,
    enabled: true,
    created_at: '2025-12-01T10:00:00Z',
    updated_at: '2026-03-10T08:00:00Z',
  },
  {
    id: 'rule-002',
    flag_id: 'flag-001',
    rule_type: 'user_target',
    priority: 2,
    value: 'true',
    target_values: ['user-101', 'user-202', 'user-303'],
    enabled: true,
    created_at: '2026-01-15T09:00:00Z',
    updated_at: '2026-02-20T11:00:00Z',
  },
  {
    id: 'rule-003',
    flag_id: 'flag-001',
    rule_type: 'attribute',
    priority: 3,
    value: 'true',
    attribute: 'plan',
    operator: 'equals',
    target_values: ['premium', 'enterprise'],
    enabled: false,
    created_at: '2026-02-10T14:00:00Z',
    updated_at: '2026-03-01T16:00:00Z',
  },
];

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

function describeConditions(rule: TargetingRule): string {
  switch (rule.rule_type) {
    case 'percentage':
      return `${rule.percentage}% of users`;
    case 'user_target':
      return `Users: ${rule.target_values?.join(', ') ?? '\u2014'}`;
    case 'attribute':
      return `${rule.attribute} ${rule.operator} ${rule.target_values?.join(', ') ?? '\u2014'}`;
    case 'segment':
      return `Segment: ${rule.segment_id ?? '\u2014'}`;
    case 'schedule':
      return `${rule.start_time ? formatDateTime(rule.start_time) : '\u2014'} to ${rule.end_time ? formatDateTime(rule.end_time) : '\u2014'}`;
    default:
      return '\u2014';
  }
}

function getAppNameById(appId: string): string {
  return MOCK_APPLICATIONS.find((a) => a.id === appId)?.name ?? appId;
}

export default function FlagDetailPage() {
  const { id, orgSlug, projectSlug, appSlug } = useParams();
  const backPath = appSlug
    ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/flags`
    : `/orgs/${orgSlug}/projects/${projectSlug}/flags`;

  const [flag, setFlag] = useState<Flag>(MOCK_FLAG);
  const [activeTab, setActiveTab] = useState<'rules' | 'environments'>('rules');
  const [envState, setEnvState] = useState<FlagEnvState[]>([...MOCK_FLAG_ENV_STATE]);
  const [editingEnvId, setEditingEnvId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const rules = MOCK_RULES;

  // Use id to look up the flag in a real app
  void id;

  const handleToggle = () => {
    setFlag((prev) => ({ ...prev, enabled: !prev.enabled }));
  };

  const handleArchive = () => {
    setFlag((prev) => ({ ...prev, archived: true }));
  };

  const handleEnvToggle = (envId: string) => {
    setEnvState((prev) =>
      prev.map((e) =>
        e.environment_id === envId ? { ...e, enabled: !e.enabled } : e
      )
    );
  };

  const startEditValue = (envId: string, currentValue: string) => {
    setEditingEnvId(envId);
    setEditValue(currentValue);
  };

  const saveEditValue = () => {
    if (editingEnvId) {
      setEnvState((prev) =>
        prev.map((e) =>
          e.environment_id === editingEnvId ? { ...e, value: editValue } : e
        )
      );
      setEditingEnvId(null);
      setEditValue('');
    }
  };

  const cancelEditValue = () => {
    setEditingEnvId(null);
    setEditValue('');
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
            <label className="toggle">
              <input
                type="checkbox"
                checked={flag.enabled}
                onChange={handleToggle}
              />
              <span>{flag.enabled ? 'Enabled' : 'Disabled'}</span>
            </label>
            <span className={`badge badge-${flag.category}`}>{flag.category}</span>
            <button className="btn btn-secondary">Edit</button>
          </div>
        </div>

        <div className="detail-chips">
          <span>Type: {flag.flag_type}</span>
          <span>Owners: {flag.owners.join(', ')}</span>
          <span>Expires: {flag.is_permanent ? 'Permanent' : flag.expires_at ? formatDate(flag.expires_at) : '\u2014'}</span>
          <span>Default Value: <span className="font-mono">{flag.default_value}</span></span>
          <span>Scope: {flag.application_id ? getAppNameById(flag.application_id) : 'Project-wide'}</span>
          {flag.purpose && <span>Purpose: {flag.purpose}</span>}
          {flag.tags.length > 0 && <span>Tags: {flag.tags.join(', ')}</span>}
        </div>

        <div className="detail-secondary">
          <span>Created by {flag.created_by}</span>
          <span>Created {formatDateTime(flag.created_at)}</span>
          <span>Updated {formatDateTime(flag.updated_at)}</span>
        </div>

        {flag.description && (
          <div className="detail-description">{flag.description}</div>
        )}
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
            <button className="btn btn-secondary">Add Rule</button>
          </div>
          <table>
            <thead>
              <tr>
                <th>Priority</th>
                <th>Type</th>
                <th>Condition</th>
                <th>Value</th>
                <th>Enabled</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td>{rule.priority}</td>
                  <td>{rule.rule_type}</td>
                  <td>{describeConditions(rule)}</td>
                  <td className="font-mono">{rule.value}</td>
                  <td>
                    <span className={`badge ${rule.enabled ? 'badge-enabled' : 'badge-disabled'}`}>
                      {rule.enabled ? 'enabled' : 'disabled'}
                    </span>
                  </td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ textAlign: 'center' }}>
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
          <table>
            <thead>
              <tr>
                <th>Environment</th>
                <th>Enabled</th>
                <th>Value</th>
                <th>Last Updated</th>
                <th>Updated By</th>
              </tr>
            </thead>
            <tbody>
              {envState.map((env) => (
                <tr key={env.environment_id}>
                  <td>{env.environment_name}</td>
                  <td>
                    <input
                      type="checkbox"
                      className="env-toggle"
                      checked={env.enabled}
                      onChange={() => handleEnvToggle(env.environment_id)}
                    />
                  </td>
                  <td
                    className="env-value-cell"
                    onClick={() => {
                      if (editingEnvId !== env.environment_id) {
                        startEditValue(env.environment_id, env.value);
                      }
                    }}
                  >
                    {editingEnvId === env.environment_id ? (
                      <input
                        type="text"
                        className="env-value-input"
                        value={editValue}
                        onChange={(e) => setEditValue(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') saveEditValue();
                          if (e.key === 'Escape') cancelEditValue();
                        }}
                        onBlur={saveEditValue}
                        autoFocus
                      />
                    ) : (
                      <span className="font-mono">{env.value}</span>
                    )}
                  </td>
                  <td>{formatDateTime(env.updated_at)}</td>
                  <td>{env.updated_by}</td>
                </tr>
              ))}
              {envState.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ textAlign: 'center' }}>
                    No environment overrides configured.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Danger Zone */}
      <div className="danger-zone">
        <h2>Danger Zone</h2>
        <p className="text-muted">
          Archiving a flag disables it permanently and removes it from active use.
          This action cannot be easily undone.
        </p>
        <button
          className="btn btn-danger"
          onClick={handleArchive}
          disabled={flag.archived}
        >
          {flag.archived ? 'Archived' : 'Archive Flag'}
        </button>
      </div>
    </div>
  );
}
