import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Flag, TargetingRule } from '@/types';
import { flagsApi, entitiesApi } from '@/api';
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
        setRules(rulesData);
        setApps(appsData);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id, orgSlug, projectSlug]);

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!flag) return <div>Flag not found.</div>;

  const handleToggle = () => {
    setFlag((prev) => (prev ? { ...prev, enabled: !prev.enabled } : prev));
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
            <label className="toggle">
              <input type="checkbox" checked={flag.enabled} onChange={handleToggle} />
              <span>{flag.enabled ? 'Enabled' : 'Disabled'}</span>
            </label>
            <span className={`badge badge-${flag.category}`}>{flag.category}</span>
            <button className="btn btn-secondary">Edit</button>
          </div>
        </div>

        <div className="detail-chips">
          <span>Type: {flag.flag_type}</span>
          <span>Owners: {flag.owners.join(', ')}</span>
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
          {flag.tags.length > 0 && <span>Tags: {flag.tags.join(', ')}</span>}
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
          <p style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--color-text-muted)' }}>
            No environment data available
          </p>
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
