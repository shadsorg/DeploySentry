import React, { useState, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { getAppName } from '@/mocks/hierarchy';
import type { Release, ReleaseStatus } from '@/types';

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

const MOCK_RELEASES: Release[] = [
  {
    id: 'rel-1',
    application_id: 'app-1',
    name: 'Enable Checkout V2',
    description: 'Gradual rollout of checkout v2 flags',
    session_sticky: true,
    sticky_header: 'X-Session-ID',
    traffic_percent: 25,
    status: 'rolling_out',
    created_by: 'alice@example.com',
    started_at: '2026-03-21T10:00:00Z',
    created_at: '2026-03-20T14:00:00Z',
    updated_at: '2026-03-21T10:30:00Z',
  },
  {
    id: 'rel-2',
    application_id: 'app-1',
    name: 'Dark Mode Feature Flags',
    description: 'Enable dark mode across all environments',
    session_sticky: false,
    traffic_percent: 100,
    status: 'completed',
    created_by: 'bob@example.com',
    started_at: '2026-03-18T08:00:00Z',
    completed_at: '2026-03-19T11:30:00Z',
    created_at: '2026-03-17T10:00:00Z',
    updated_at: '2026-03-19T11:30:00Z',
  },
  {
    id: 'rel-3',
    application_id: 'app-1',
    name: 'Search Experiment Rollout',
    description: 'ML search ranking A/B test activation',
    session_sticky: true,
    sticky_header: 'X-User-ID',
    traffic_percent: 0,
    status: 'draft',
    created_by: 'team-platform',
    created_at: '2026-03-21T07:30:00Z',
    updated_at: '2026-03-21T07:30:00Z',
  },
  {
    id: 'rel-4',
    application_id: 'app-1',
    name: 'Payment Gateway Migration',
    description: 'Switch payment flags to new gateway',
    session_sticky: false,
    traffic_percent: 50,
    status: 'paused',
    created_by: 'alice@example.com',
    started_at: '2026-03-20T16:00:00Z',
    created_at: '2026-03-20T12:00:00Z',
    updated_at: '2026-03-21T06:00:00Z',
  },
  {
    id: 'rel-5',
    application_id: 'app-1',
    name: 'Legacy API Sunset',
    description: 'Disable legacy API endpoint flags',
    session_sticky: false,
    traffic_percent: 100,
    status: 'rolled_back',
    created_by: 'ci/deploy-bot',
    started_at: '2026-03-19T10:00:00Z',
    created_at: '2026-03-19T08:00:00Z',
    updated_at: '2026-03-19T14:00:00Z',
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const TABS = ['all', 'draft', 'rolling_out', 'paused', 'completed', 'rolled_back'] as const;

type TabKey = (typeof TABS)[number];

function tabLabel(tab: TabKey): string {
  switch (tab) {
    case 'all':
      return 'All';
    case 'draft':
      return 'Draft';
    case 'rolling_out':
      return 'Rolling Out';
    case 'paused':
      return 'Paused';
    case 'completed':
      return 'Completed';
    case 'rolled_back':
      return 'Rolled Back';
  }
}

function statusBadgeClass(status: ReleaseStatus): string {
  switch (status) {
    case 'draft':
      return 'badge badge-pending';
    case 'rolling_out':
      return 'badge badge-active';
    case 'paused':
      return 'badge badge-ops';
    case 'completed':
      return 'badge badge-completed';
    case 'rolled_back':
      return 'badge badge-danger';
  }
}

function statusLabel(status: ReleaseStatus): string {
  switch (status) {
    case 'rolling_out':
      return 'Rolling Out';
    case 'rolled_back':
      return 'Rolled Back';
    default:
      return status.charAt(0).toUpperCase() + status.slice(1);
  }
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
  const { appSlug } = useParams();
  const appName = appSlug ? getAppName(appSlug) : '';

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
          <h1>{appName ? `${appName} — Releases` : 'Releases'}</h1>
          <p>Coordinate flag changes across environments with managed releases</p>
        </div>
        <button className="btn btn-primary">+ Create Release</button>
      </div>

      {/* Tabs */}
      <div className="tabs">
        {TABS.map((tab) => (
          <button
            key={tab}
            className={`tab${activeTab === tab ? ' active' : ''}`}
            onClick={() => setActiveTab(tab)}
          >
            {tabLabel(tab)}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="card">
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Traffic %</th>
                <th>Description</th>
                <th>Created By</th>
                <th>Created</th>
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
                    <td style={{ fontWeight: 500 }}>{rel.name}</td>
                    <td>
                      <span className={statusBadgeClass(rel.status)}>
                        {statusLabel(rel.status)}
                      </span>
                    </td>
                    <td>
                      <span className="text-sm">{rel.traffic_percent}%</span>
                    </td>
                    <td className="text-sm" style={{ maxWidth: 300 }}>
                      <span className="truncate" style={{ display: 'block' }}>
                        {rel.description}
                      </span>
                    </td>
                    <td className="text-sm text-secondary">{rel.created_by}</td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatDate(rel.created_at)}
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
