import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import type { Flag, TargetingRule } from '@/types';

const MOCK_FLAG: Flag = {
  id: 'flag-001',
  project_id: 'proj-1',
  environment_id: 'env-prod',
  key: 'enable-dark-mode',
  name: 'Dark Mode',
  description: 'Enable dark mode across the entire application UI. Supports system preference detection and manual override.',
  flag_type: 'boolean',
  category: 'feature',
  purpose: 'Allow users to switch to a dark color scheme for reduced eye strain and improved accessibility.',
  owners: ['frontend-team', 'design-team'],
  is_permanent: true,
  expires_at: null,
  enabled: true,
  default_value: 'false',
  archived: false,
  tags: ['ui', 'theme', 'accessibility'],
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
      return `Users: ${rule.target_values?.join(', ') ?? '—'}`;
    case 'attribute':
      return `${rule.attribute} ${rule.operator} ${rule.target_values?.join(', ') ?? '—'}`;
    case 'segment':
      return `Segment: ${rule.segment_id ?? '—'}`;
    case 'schedule':
      return `${rule.start_time ? formatDateTime(rule.start_time) : '—'} to ${rule.end_time ? formatDateTime(rule.end_time) : '—'}`;
    default:
      return '—';
  }
}

export default function FlagDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [flag, setFlag] = useState<Flag>(MOCK_FLAG);
  const rules = MOCK_RULES;

  // Use id to look up the flag in a real app
  void id;

  const handleToggle = () => {
    setFlag((prev) => ({ ...prev, enabled: !prev.enabled }));
  };

  const handleArchive = () => {
    setFlag((prev) => ({ ...prev, archived: true }));
  };

  return (
    <div>
      <div className="page-header-row">
        <div>
          <h1 className="page-header">{flag.name}</h1>
          <span className="font-mono text-muted">{flag.key}</span>
        </div>
        <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
          <button className="btn btn-secondary">Edit</button>
          <label className="toggle">
            <input
              type="checkbox"
              checked={flag.enabled}
              onChange={handleToggle}
            />
            <span>{flag.enabled ? 'Enabled' : 'Disabled'}</span>
          </label>
        </div>
      </div>

      <div className="card">
        <h2>Details</h2>
        <table>
          <tbody>
            <tr>
              <th>Category</th>
              <td>
                <span className={`badge badge-${flag.category}`}>
                  {flag.category}
                </span>
              </td>
            </tr>
            <tr>
              <th>Type</th>
              <td>{flag.flag_type}</td>
            </tr>
            <tr>
              <th>Description</th>
              <td>{flag.description}</td>
            </tr>
            <tr>
              <th>Purpose</th>
              <td>{flag.purpose}</td>
            </tr>
            <tr>
              <th>Owners</th>
              <td>{flag.owners.join(', ')}</td>
            </tr>
            <tr>
              <th>Default Value</th>
              <td className="font-mono">{flag.default_value}</td>
            </tr>
            <tr>
              <th>Tags</th>
              <td>{flag.tags.map((tag) => (
                <span key={tag} className="badge" style={{ marginRight: '0.25rem' }}>{tag}</span>
              ))}</td>
            </tr>
            <tr>
              <th>Expires</th>
              <td>{flag.is_permanent ? 'Permanent' : flag.expires_at ? formatDate(flag.expires_at) : '\u2014'}</td>
            </tr>
            <tr>
              <th>Created</th>
              <td>{formatDateTime(flag.created_at)} by {flag.created_by}</td>
            </tr>
            <tr>
              <th>Updated</th>
              <td>{formatDateTime(flag.updated_at)}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="card">
        <h2>Targeting Rules</h2>
        <table>
          <thead>
            <tr>
              <th>Priority</th>
              <th>Type</th>
              <th>Value</th>
              <th>Conditions</th>
              <th>Enabled</th>
            </tr>
          </thead>
          <tbody>
            {rules.map((rule) => (
              <tr key={rule.id}>
                <td>{rule.priority}</td>
                <td>{rule.rule_type}</td>
                <td className="font-mono">{rule.value}</td>
                <td>{describeConditions(rule)}</td>
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

      <div className="card">
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

      <div style={{ marginTop: '1rem' }}>
        <Link to="/flags" className="btn btn-secondary">
          &larr; Back to Flags
        </Link>
      </div>
    </div>
  );
}
