import { useState } from 'react';
import { Link } from 'react-router-dom';
import type { Flag, FlagCategory } from '@/types';

const MOCK_FLAGS: Flag[] = [
  {
    id: 'flag-001',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'enable-dark-mode',
    name: 'Dark Mode',
    description: 'Enable dark mode across the application',
    flag_type: 'boolean',
    category: 'feature',
    purpose: 'Allow users to switch to dark theme',
    owners: ['frontend-team', 'design-team'],
    is_permanent: true,
    expires_at: null,
    enabled: true,
    default_value: 'false',
    archived: false,
    tags: ['ui', 'theme'],
    created_by: 'alice',
    created_at: '2025-11-01T10:00:00Z',
    updated_at: '2026-03-18T14:30:00Z',
  },
  {
    id: 'flag-002',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'checkout-v2-rollout',
    name: 'Checkout V2 Rollout',
    description: 'Gradual rollout of new checkout flow',
    flag_type: 'boolean',
    category: 'release',
    purpose: 'Progressive delivery of checkout redesign',
    owners: ['payments-team'],
    is_permanent: false,
    expires_at: '2026-06-01T00:00:00Z',
    enabled: true,
    default_value: 'false',
    archived: false,
    tags: ['checkout', 'rollout'],
    created_by: 'bob',
    created_at: '2026-01-15T09:00:00Z',
    updated_at: '2026-03-20T11:00:00Z',
  },
  {
    id: 'flag-003',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'search-ranking-experiment',
    name: 'Search Ranking A/B Test',
    description: 'Test new ML-based search ranking algorithm',
    flag_type: 'string',
    category: 'experiment',
    purpose: 'Measure impact of ML ranking on conversion',
    owners: ['search-team', 'data-science'],
    is_permanent: false,
    expires_at: '2026-04-15T00:00:00Z',
    enabled: true,
    default_value: 'control',
    archived: false,
    tags: ['search', 'ml', 'ab-test'],
    created_by: 'carol',
    created_at: '2026-02-01T08:00:00Z',
    updated_at: '2026-03-19T16:45:00Z',
  },
  {
    id: 'flag-004',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'rate-limit-override',
    name: 'Rate Limit Override',
    description: 'Override default rate limits for specific endpoints',
    flag_type: 'integer',
    category: 'ops',
    purpose: 'Operational control for API rate limiting',
    owners: ['platform-team'],
    is_permanent: true,
    expires_at: null,
    enabled: true,
    default_value: '1000',
    archived: false,
    tags: ['ops', 'rate-limit'],
    created_by: 'dave',
    created_at: '2025-09-10T12:00:00Z',
    updated_at: '2026-03-15T09:20:00Z',
  },
  {
    id: 'flag-005',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'admin-billing-access',
    name: 'Admin Billing Access',
    description: 'Grant billing dashboard access to admin users',
    flag_type: 'boolean',
    category: 'permission',
    purpose: 'Control access to billing management features',
    owners: ['security-team'],
    is_permanent: true,
    expires_at: null,
    enabled: true,
    default_value: 'false',
    archived: false,
    tags: ['rbac', 'billing'],
    created_by: 'eve',
    created_at: '2025-10-20T14:00:00Z',
    updated_at: '2026-02-28T10:00:00Z',
  },
  {
    id: 'flag-006',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'notification-center-v3',
    name: 'Notification Center V3',
    description: 'New notification center with grouped notifications',
    flag_type: 'boolean',
    category: 'release',
    purpose: 'Ship redesigned notification center',
    owners: ['frontend-team'],
    is_permanent: false,
    expires_at: '2026-05-01T00:00:00Z',
    enabled: false,
    default_value: 'false',
    archived: false,
    tags: ['notifications', 'ui'],
    created_by: 'frank',
    created_at: '2026-02-20T11:00:00Z',
    updated_at: '2026-03-10T08:30:00Z',
  },
  {
    id: 'flag-007',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'onboarding-wizard-experiment',
    name: 'Onboarding Wizard Experiment',
    description: 'Test simplified onboarding flow',
    flag_type: 'string',
    category: 'experiment',
    purpose: 'Improve new user activation rate',
    owners: ['growth-team', 'product-team'],
    is_permanent: false,
    expires_at: '2026-04-30T00:00:00Z',
    enabled: false,
    default_value: 'control',
    archived: false,
    tags: ['onboarding', 'growth'],
    created_by: 'grace',
    created_at: '2026-03-01T10:00:00Z',
    updated_at: '2026-03-05T14:00:00Z',
  },
  {
    id: 'flag-008',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'legacy-api-deprecation',
    name: 'Legacy API Deprecation',
    description: 'Disable deprecated v1 API endpoints',
    flag_type: 'boolean',
    category: 'ops',
    purpose: 'Gradually sunset legacy API',
    owners: ['platform-team', 'api-team'],
    is_permanent: false,
    expires_at: null,
    enabled: true,
    default_value: 'true',
    archived: true,
    tags: ['deprecation', 'api'],
    created_by: 'hank',
    created_at: '2025-06-15T09:00:00Z',
    updated_at: '2026-01-10T17:00:00Z',
  },
  {
    id: 'flag-009',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'premium-analytics-dashboard',
    name: 'Premium Analytics Dashboard',
    description: 'Advanced analytics for premium tier users',
    flag_type: 'boolean',
    category: 'permission',
    purpose: 'Gate advanced analytics behind premium plan',
    owners: ['product-team', 'analytics-team'],
    is_permanent: true,
    expires_at: null,
    enabled: true,
    default_value: 'false',
    archived: false,
    tags: ['premium', 'analytics'],
    created_by: 'iris',
    created_at: '2025-12-01T08:00:00Z',
    updated_at: '2026-03-17T12:15:00Z',
  },
  {
    id: 'flag-010',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    key: 'real-time-collab',
    name: 'Real-Time Collaboration',
    description: 'Enable WebSocket-based real-time editing',
    flag_type: 'boolean',
    category: 'feature',
    purpose: 'Ship collaborative editing capabilities',
    owners: ['backend-team', 'frontend-team'],
    is_permanent: false,
    expires_at: '2026-07-01T00:00:00Z',
    enabled: false,
    default_value: 'false',
    archived: false,
    tags: ['collab', 'websocket'],
    created_by: 'jack',
    created_at: '2026-03-10T10:00:00Z',
    updated_at: '2026-03-20T09:00:00Z',
  },
];

type StatusFilter = 'all' | 'enabled' | 'disabled' | 'archived';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export default function FlagListPage() {
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<'all' | FlagCategory>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  const filtered = MOCK_FLAGS.filter((flag) => {
    if (search) {
      const q = search.toLowerCase();
      if (
        !flag.name.toLowerCase().includes(q) &&
        !flag.key.toLowerCase().includes(q)
      ) {
        return false;
      }
    }
    if (categoryFilter !== 'all' && flag.category !== categoryFilter) {
      return false;
    }
    if (statusFilter === 'enabled' && (!flag.enabled || flag.archived)) return false;
    if (statusFilter === 'disabled' && (flag.enabled || flag.archived)) return false;
    if (statusFilter === 'archived' && !flag.archived) return false;
    return true;
  });

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">Feature Flags</h1>
        <Link to="/flags/new" className="btn btn-primary">
          Create Flag
        </Link>
      </div>

      <div className="filter-bar">
        <input
          type="text"
          className="form-input"
          placeholder="Search flags..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="form-select"
          value={categoryFilter}
          onChange={(e) => setCategoryFilter(e.target.value as 'all' | FlagCategory)}
        >
          <option value="all">All Categories</option>
          <option value="release">Release</option>
          <option value="feature">Feature</option>
          <option value="experiment">Experiment</option>
          <option value="ops">Ops</option>
          <option value="permission">Permission</option>
        </select>
        <select
          className="form-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
        >
          <option value="all">All Statuses</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
          <option value="archived">Archived</option>
        </select>
      </div>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th>Name / Key</th>
              <th>Category</th>
              <th>Status</th>
              <th>Owners</th>
              <th>Expires</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((flag) => (
              <tr key={flag.id}>
                <td>
                  <Link to={`/flags/${flag.id}`}>{flag.name}</Link>
                  <div className="font-mono text-muted">{flag.key}</div>
                </td>
                <td>
                  <span className={`badge badge-${flag.category}`}>
                    {flag.category}
                  </span>
                </td>
                <td>
                  {flag.archived ? (
                    <span className="badge badge-archived">archived</span>
                  ) : flag.enabled ? (
                    <span className="badge badge-enabled">enabled</span>
                  ) : (
                    <span className="badge badge-disabled">disabled</span>
                  )}
                </td>
                <td>{flag.owners.join(', ')}</td>
                <td>
                  {flag.is_permanent
                    ? 'Permanent'
                    : flag.expires_at
                      ? formatDate(flag.expires_at)
                      : '\u2014'}
                </td>
                <td>{formatDate(flag.updated_at)}</td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={6} style={{ textAlign: 'center' }}>
                  No flags match your filters.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
