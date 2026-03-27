import React, { useState, useMemo } from 'react';
import type { Release, ReleaseStatus } from '@/types';

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

const MOCK_RELEASES: Release[] = [
  {
    id: 'rel-1',
    project_id: 'proj-1',
    version: 'v2.5.0',
    status: 'draft',
    commit_sha: 'a3f8c1d9e27b04556f91ae3cdd8e4b7012c6f5a8',
    description: 'New checkout flow with improved cart validation',
    created_by: 'alice@example.com',
    created_at: '2026-03-21T10:00:00Z',
    updated_at: '2026-03-21T10:00:00Z',
    promoted_at: null,
  },
  {
    id: 'rel-2',
    project_id: 'proj-1',
    version: 'v2.4.1',
    status: 'production',
    commit_sha: 'b7e2f4a01c93d685e8ab12df0947c3e6a15b8d20',
    description: 'Hotfix for payment processing timeout under high load',
    created_by: 'ci/deploy-bot',
    created_at: '2026-03-20T14:00:00Z',
    updated_at: '2026-03-21T09:15:00Z',
    promoted_at: '2026-03-21T09:15:00Z',
  },
  {
    id: 'rel-3',
    project_id: 'proj-1',
    version: 'v2.4.0',
    status: 'archived',
    commit_sha: 'c4d91e5f738a206b1c4e97da50f823b6e19a0c47',
    description: 'Dashboard redesign with dark mode support',
    created_by: 'bob@example.com',
    created_at: '2026-03-18T08:00:00Z',
    updated_at: '2026-03-20T16:00:00Z',
    promoted_at: '2026-03-19T11:30:00Z',
  },
  {
    id: 'rel-4',
    project_id: 'proj-2',
    version: 'v1.13.0-rc1',
    status: 'staging',
    commit_sha: 'd82fa3b6c0154e9a7d31f68e52c0ab94d17e63f1',
    description: 'Add webhook retry logic and dead letter queue',
    created_by: 'alice@example.com',
    created_at: '2026-03-21T07:30:00Z',
    updated_at: '2026-03-21T08:00:00Z',
    promoted_at: '2026-03-21T08:00:00Z',
  },
  {
    id: 'rel-5',
    project_id: 'proj-1',
    version: 'v2.5.0-beta.2',
    status: 'canary',
    commit_sha: 'e5f07a2d19b8c43e6a0d71fc284b9e53f06d12a7',
    description: 'Feature flag evaluation engine v2 with segment support',
    created_by: 'team-platform',
    created_at: '2026-03-20T16:00:00Z',
    updated_at: '2026-03-21T06:00:00Z',
    promoted_at: '2026-03-21T06:00:00Z',
  },
  {
    id: 'rel-6',
    project_id: 'proj-3',
    version: 'v3.0.0',
    status: 'production',
    commit_sha: 'f19c8e4d3a7b52061e9d04ca83f6a72b5d0e18c9',
    description: 'Major version bump with breaking API changes',
    created_by: 'ci/deploy-bot',
    created_at: '2026-03-19T10:00:00Z',
    updated_at: '2026-03-21T08:25:00Z',
    promoted_at: '2026-03-21T08:25:00Z',
  },
  {
    id: 'rel-7',
    project_id: 'proj-2',
    version: 'v1.12.4',
    status: 'archived',
    commit_sha: '0ab3d7e8f1c94526b7e0da3f18c52e6a49d07b31',
    description: 'Security patch for JWT validation bypass',
    created_by: 'bob@example.com',
    created_at: '2026-03-15T09:00:00Z',
    updated_at: '2026-03-18T12:00:00Z',
    promoted_at: '2026-03-16T14:00:00Z',
  },
  {
    id: 'rel-8',
    project_id: 'proj-1',
    version: 'v2.5.1',
    status: 'draft',
    commit_sha: '1c4e82a5f93d076b2e8af10dc347b9e6a52f08d3',
    description: 'Performance improvements for flag evaluation cache',
    created_by: 'alice@example.com',
    created_at: '2026-03-21T11:30:00Z',
    updated_at: '2026-03-21T11:30:00Z',
    promoted_at: null,
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type TabKey = 'all' | ReleaseStatus;

const TABS: { key: TabKey; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'draft', label: 'Draft' },
  { key: 'staging', label: 'Staging' },
  { key: 'production', label: 'Production' },
  { key: 'archived', label: 'Archived' },
];

function statusBadgeClass(status: ReleaseStatus): string {
  switch (status) {
    case 'draft':
      return 'badge badge-pending';
    case 'staging':
      return 'badge badge-ops';
    case 'canary':
      return 'badge badge-experiment';
    case 'production':
      return 'badge badge-active';
    case 'archived':
      return 'badge badge-disabled';
  }
}

function statusLabel(status: ReleaseStatus): string {
  return status.charAt(0).toUpperCase() + status.slice(1);
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const ReleasesPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<TabKey>('all');

  const filtered = useMemo(() => {
    if (activeTab === 'all') return MOCK_RELEASES;
    return MOCK_RELEASES.filter((r) => r.status === activeTab);
  }, [activeTab]);

  return (
    <div>
      {/* Page header */}
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>Releases</h1>
          <p>Track release versions from draft through production</p>
        </div>
        <button className="btn btn-primary">+ Create Release</button>
      </div>

      {/* Tabs */}
      <div className="tabs">
        {TABS.map((tab) => (
          <button
            key={tab.key}
            className={`tab${activeTab === tab.key ? ' active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="card">
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Version</th>
                <th>Status</th>
                <th>Commit SHA</th>
                <th>Description</th>
                <th>Created</th>
                <th>Promoted</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <div className="empty-state">
                      <h3>No releases found</h3>
                      <p>There are no releases matching the selected filter.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((rel) => (
                  <tr key={rel.id}>
                    <td style={{ fontWeight: 500 }}>
                      <code className="font-mono text-sm">{rel.version}</code>
                    </td>
                    <td>
                      <span className={statusBadgeClass(rel.status)}>
                        {statusLabel(rel.status)}
                      </span>
                    </td>
                    <td>
                      <code className="font-mono text-sm text-secondary">
                        {rel.commit_sha.slice(0, 7)}
                      </code>
                    </td>
                    <td className="text-sm" style={{ maxWidth: 300 }}>
                      <span className="truncate" style={{ display: 'block' }}>
                        {rel.description}
                      </span>
                    </td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatDate(rel.created_at)}
                    </td>
                    <td className="text-sm text-muted" style={{ whiteSpace: 'nowrap' }}>
                      {rel.promoted_at ? formatDate(rel.promoted_at) : '\u2014'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default ReleasesPage;
