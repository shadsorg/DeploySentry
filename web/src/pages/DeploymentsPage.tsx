import React, { useState, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { getAppName } from '@/mocks/hierarchy';
import type { Deployment, DeployStrategy, DeployStatus } from '@/types';

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

const MOCK_DEPLOYMENTS: Deployment[] = [
  {
    id: 'dep-1',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    version: 'v2.4.1',
    strategy: 'canary',
    status: 'running',
    traffic_percentage: 25,
    health_score: 99.8,
    created_by: 'alice@example.com',
    created_at: '2026-03-21T09:15:00Z',
    updated_at: '2026-03-21T10:30:00Z',
    completed_at: null,
  },
  {
    id: 'dep-2',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    version: 'v2.4.0',
    strategy: 'blue-green',
    status: 'completed',
    traffic_percentage: 100,
    health_score: 98.5,
    created_by: 'ci/deploy-bot',
    created_at: '2026-03-21T06:00:00Z',
    updated_at: '2026-03-21T06:45:00Z',
    completed_at: '2026-03-21T06:45:00Z',
  },
  {
    id: 'dep-3',
    project_id: 'proj-1',
    environment_id: 'env-staging',
    version: 'v2.5.0-rc1',
    strategy: 'rolling',
    status: 'running',
    traffic_percentage: 60,
    health_score: 97.2,
    created_by: 'bob@example.com',
    created_at: '2026-03-21T11:00:00Z',
    updated_at: '2026-03-21T11:20:00Z',
    completed_at: null,
  },
  {
    id: 'dep-4',
    project_id: 'proj-2',
    environment_id: 'env-prod',
    version: 'v1.12.3',
    strategy: 'canary',
    status: 'failed',
    traffic_percentage: 10,
    health_score: 72.1,
    created_by: 'ci/deploy-bot',
    created_at: '2026-03-21T04:30:00Z',
    updated_at: '2026-03-21T04:55:00Z',
    completed_at: '2026-03-21T04:55:00Z',
  },
  {
    id: 'dep-5',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    version: 'v2.3.9',
    strategy: 'blue-green',
    status: 'completed',
    traffic_percentage: 100,
    health_score: 99.9,
    created_by: 'alice@example.com',
    created_at: '2026-03-20T14:00:00Z',
    updated_at: '2026-03-20T14:30:00Z',
    completed_at: '2026-03-20T14:30:00Z',
  },
  {
    id: 'dep-6',
    project_id: 'proj-2',
    environment_id: 'env-prod',
    version: 'v1.12.2',
    strategy: 'rolling',
    status: 'rolled_back',
    traffic_percentage: 0,
    health_score: 65.3,
    created_by: 'bob@example.com',
    created_at: '2026-03-21T01:00:00Z',
    updated_at: '2026-03-21T01:40:00Z',
    completed_at: '2026-03-21T01:40:00Z',
  },
  {
    id: 'dep-7',
    project_id: 'proj-1',
    environment_id: 'env-prod',
    version: 'v2.4.1-hotfix',
    strategy: 'canary',
    status: 'running',
    traffic_percentage: 5,
    health_score: 100,
    created_by: 'ci/deploy-bot',
    created_at: '2026-03-21T12:00:00Z',
    updated_at: '2026-03-21T12:05:00Z',
    completed_at: null,
  },
  {
    id: 'dep-8',
    project_id: 'proj-3',
    environment_id: 'env-prod',
    version: 'v3.0.0',
    strategy: 'blue-green',
    status: 'completed',
    traffic_percentage: 100,
    health_score: 96.4,
    created_by: 'alice@example.com',
    created_at: '2026-03-21T08:00:00Z',
    updated_at: '2026-03-21T08:25:00Z',
    completed_at: '2026-03-21T08:25:00Z',
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function strategyBadgeClass(strategy: DeployStrategy): string {
  switch (strategy) {
    case 'canary':
      return 'badge badge-experiment';
    case 'blue-green':
      return 'badge badge-release';
    case 'rolling':
      return 'badge badge-ops';
  }
}

function statusBadgeClass(status: DeployStatus): string {
  switch (status) {
    case 'running':
      return 'badge badge-active';
    case 'completed':
      return 'badge badge-completed';
    case 'failed':
      return 'badge badge-failed';
    case 'rolled_back':
      return 'badge badge-rolling-back';
    case 'paused':
      return 'badge badge-pending';
    case 'pending':
      return 'badge badge-pending';
    default:
      return 'badge';
  }
}

function statusLabel(status: DeployStatus): string {
  switch (status) {
    case 'rolled_back':
      return 'Rolled Back';
    default:
      return status.charAt(0).toUpperCase() + status.slice(1);
  }
}

function healthColor(score: number): string {
  if (score >= 95) return 'text-success';
  if (score >= 80) return 'text-warning';
  return 'text-danger';
}

function trafficBarColor(score: number): string {
  if (score >= 95) return 'var(--color-success)';
  if (score >= 80) return 'var(--color-warning)';
  return 'var(--color-danger)';
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function computeDuration(start: string, end: string | null): string {
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const mins = Math.round((e - s) / 60000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  const rem = mins % 60;
  return `${hours}h ${rem}m`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const DeploymentsPage: React.FC = () => {
  const { appSlug } = useParams();
  const appName = appSlug ? getAppName(appSlug) : '';

  const [search, setSearch] = useState('');
  const [strategyFilter, setStrategyFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  const filtered = useMemo(() => {
    return MOCK_DEPLOYMENTS.filter((d) => {
      if (search && !d.version.toLowerCase().includes(search.toLowerCase())) {
        return false;
      }
      if (strategyFilter !== 'all' && d.strategy !== strategyFilter) {
        return false;
      }
      if (statusFilter !== 'all' && d.status !== statusFilter) {
        return false;
      }
      return true;
    });
  }, [search, strategyFilter, statusFilter]);

  return (
    <div>
      {/* Page header */}
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>{appName ? `${appName} — Deployments` : 'Deployments'}</h1>
          <p>Monitor and manage application deployments across environments</p>
        </div>
        <button className="btn btn-primary">+ New Deployment</button>
      </div>

      {/* Stat cards */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">Active Deployments</div>
          <div className="stat-value">3</div>
          <div className="stat-meta">currently running</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Completed Today</div>
          <div className="stat-value">7</div>
          <div className="stat-meta">across all environments</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Rollbacks</div>
          <div className="stat-value text-warning">1</div>
          <div className="stat-meta">in the last 24 hours</div>
        </div>
      </div>

      {/* Filter bar */}
      <div className="filter-bar">
        <input
          className="form-input"
          type="text"
          placeholder="Search by version..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          className="form-select"
          value={strategyFilter}
          onChange={(e) => setStrategyFilter(e.target.value)}
        >
          <option value="all">All Strategies</option>
          <option value="canary">Canary</option>
          <option value="blue-green">Blue/Green</option>
          <option value="rolling">Rolling</option>
        </select>
        <select
          className="form-select"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="all">All Statuses</option>
          <option value="running">Running</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="rolled_back">Rolled Back</option>
        </select>
      </div>

      {/* Table */}
      <div className="card">
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Version</th>
                <th>Strategy</th>
                <th>Status</th>
                <th>Traffic %</th>
                <th>Health</th>
                <th>Started</th>
                <th>Duration</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <div className="empty-state">
                      <h3>No deployments found</h3>
                      <p>Try adjusting your filters or search term.</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((dep) => (
                  <tr key={dep.id}>
                    <td style={{ fontWeight: 500 }}>
                      <code className="font-mono text-sm">{dep.version}</code>
                    </td>
                    <td>
                      <span className={strategyBadgeClass(dep.strategy)}>
                        {dep.strategy}
                      </span>
                    </td>
                    <td>
                      <span className={statusBadgeClass(dep.status)}>
                        {statusLabel(dep.status)}
                      </span>
                    </td>
                    <td>
                      <div className="flex items-center gap-2">
                        <div
                          style={{
                            width: 60,
                            height: 6,
                            borderRadius: 3,
                            background: 'var(--color-bg)',
                            overflow: 'hidden',
                          }}
                        >
                          <div
                            style={{
                              width: `${dep.traffic_percentage}%`,
                              height: '100%',
                              borderRadius: 3,
                              background: trafficBarColor(dep.health_score),
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </div>
                        <span className="text-sm">{dep.traffic_percentage}%</span>
                      </div>
                    </td>
                    <td>
                      <span className={healthColor(dep.health_score)}>
                        {dep.health_score.toFixed(1)}%
                      </span>
                    </td>
                    <td className="text-sm text-secondary" style={{ whiteSpace: 'nowrap' }}>
                      {formatTime(dep.created_at)}
                    </td>
                    <td className="text-sm text-muted" style={{ whiteSpace: 'nowrap' }}>
                      {computeDuration(dep.created_at, dep.completed_at)}
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

export default DeploymentsPage;
