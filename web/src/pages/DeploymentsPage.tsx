import React, { useState, useMemo, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Deployment, DeployStrategy, DeployStatus } from '@/types';
import { entitiesApi, deploymentsApi } from '@/api';

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
    case 'promoting':
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
    case 'cancelled':
      return 'badge badge-disabled';
    default:
      return 'badge';
  }
}

function statusLabel(status: DeployStatus): string {
  switch (status) {
    case 'rolled_back':
      return 'Rolled Back';
    case 'promoting':
      return 'Promoting';
    case 'cancelled':
      return 'Cancelled';
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
  const { orgSlug, projectSlug, appSlug } = useParams();
  const appName = appSlug ?? '';

  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [strategyFilter, setStrategyFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  useEffect(() => {
    if (!orgSlug || !projectSlug || !appSlug) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);

    entitiesApi
      .getApp(orgSlug, projectSlug, appSlug)
      .then((app) => deploymentsApi.list(app.id))
      .then((result) => setDeployments(result.deployments ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  const filtered = useMemo(() => {
    // Performance Optimization: Hoist search.toLowerCase() outside the loop
    // to avoid O(N) penalties per render. Also use optional chaining.
    const searchLower = search?.toLowerCase() ?? '';

    return deployments.filter((d) => {
      if (searchLower && !d.version?.toLowerCase().includes(searchLower)) {
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
  }, [deployments, search, strategyFilter, statusFilter]);

  if (!appSlug) {
    return (
      <div>
        <h1 className="page-header">Deployments</h1>
        <p>Select an application to view deployments</p>
      </div>
    );
  }

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

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
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="promoting">Promoting</option>
          <option value="paused">Paused</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="rolled_back">Rolled Back</option>
          <option value="cancelled">Cancelled</option>
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
                      <span className={strategyBadgeClass(dep.strategy)}>{dep.strategy}</span>
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
                              width: `${dep.traffic_percent}%`,
                              height: '100%',
                              borderRadius: 3,
                              background: trafficBarColor(dep.health_score),
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </div>
                        <span className="text-sm">{dep.traffic_percent}%</span>
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
