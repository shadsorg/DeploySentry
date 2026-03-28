import React, { useState, useEffect, useCallback } from 'react';
import DashboardService, { type DashboardData } from '../services/dashboard';
import { useAutoRefresh, useRealtimeUpdates } from '../services/realtime';
import type { Deployment } from '../types';

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

function strategyBadgeClass(strategy: string): string {
  switch (strategy) {
    case 'canary':
      return 'badge badge-experiment';
    case 'blue-green':
      return 'badge badge-release';
    case 'rolling':
      return 'badge badge-ops';
    default:
      return 'badge badge-ops';
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'active':
      return 'badge badge-active';
    case 'pending':
      return 'badge badge-pending';
    case 'rolling-back':
      return 'badge badge-rolling-back';
    case 'failed':
      return 'badge badge-danger';
    default:
      return 'badge badge-secondary';
  }
}

function healthColor(score: number): string {
  if (score >= 99) return 'text-success';
  if (score >= 95) return 'text-warning';
  return 'text-danger';
}

function daysRemainingColor(days: number): string {
  if (days <= 3) return 'text-danger';
  if (days <= 7) return 'text-warning';
  return 'text-secondary';
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const DashboardPage: React.FC = () => {
  const [dashboardData, setDashboardData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const dashboardService = DashboardService.getInstance();

  // Initialize with default project context (would normally come from auth/routing)
  useEffect(() => {
    const projectId = localStorage.getItem('ds_project_id') || 'default-project';
    const environmentId = localStorage.getItem('ds_environment_id') || 'production';
    dashboardService.setContext(projectId, environmentId);
  }, [dashboardService]);

  const fetchDashboardData = useCallback(async () => {
    try {
      setError(null);
      const data = await dashboardService.getDashboardData();
      setDashboardData(data);
    } catch (err) {
      console.error('[DashboardPage] Error fetching data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load dashboard data');
    } finally {
      setLoading(false);
    }
  }, [dashboardService]);

  // Set up real-time updates and auto-refresh
  const { connected } = useAutoRefresh(fetchDashboardData, {
    interval: 30000, // Refresh every 30 seconds
    events: ['refresh', 'flag_updated', 'deployment_status_changed', 'release_promoted'],
    enabled: true
  });

  if (loading) {
    return (
      <div className="page-loading">
        <div className="page-header">
          <h1>Dashboard</h1>
          <p>Loading your deployment and feature flag activity...</p>
        </div>
        <div className="stat-grid">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="stat-card loading">
              <div className="stat-label">Loading...</div>
              <div className="stat-value">--</div>
              <div className="stat-meta">--</div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="page-error">
        <div className="page-header">
          <h1>Dashboard</h1>
          <p className="text-danger">Error loading dashboard data</p>
        </div>
        <div className="card">
          <div className="card-body">
            <p className="text-danger">{error}</p>
            <button
              className="btn btn-primary mt-3"
              onClick={fetchDashboardData}
            >
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (!dashboardData) {
    return null;
  }

  const { stats, flagsByCategory, expiringFlags, activeDeployments, recentActivity } = dashboardData;

  return (
    <div>
      {/* Page header with connection status */}
      <div className="page-header">
        <div className="flex items-center gap-3">
          <div>
            <h1>Dashboard</h1>
            <p>Overview of your deployment and feature flag activity</p>
          </div>
          <div className="ml-auto">
            <span className={`badge ${connected ? 'badge-success' : 'badge-warning'}`}>
              {connected ? '🟢 Live' : '🟡 Offline'}
            </span>
          </div>
        </div>
      </div>

      {/* Stat cards */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">Total Flags</div>
          <div className="stat-value">{stats.totalFlags}</div>
          <div className="stat-meta">
            {flagsByCategory.length > 0
              ? flagsByCategory.slice(0, 3).map(cat =>
                  `${cat.count} ${cat.category}`
                ).join(', ')
              : 'No flags yet'
            }
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Active Deployments</div>
          <div className="stat-value">{stats.activeDeployments}</div>
          <div className="stat-meta">
            {activeDeployments.length > 1
              ? `across ${new Set(activeDeployments.map(d => d.environment_id)).size} environments`
              : activeDeployments.length === 1 ? 'in 1 environment' : 'none active'
            }
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Expired Flags</div>
          <div className={`stat-value ${stats.expiredFlags > 0 ? 'text-warning' : ''}`}>
            {stats.expiredFlags}
          </div>
          <div className="stat-meta">
            {stats.expiredFlags > 0 ? 'require cleanup' : 'all current'}
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-label">System Health</div>
          <div className={`stat-value ${healthColor(stats.healthScore)}`}>
            {stats.healthScore.toFixed(1)}%
          </div>
          <div className="stat-meta">
            {stats.healthScore >= 99 ? 'excellent' : stats.healthScore >= 95 ? 'good' : 'needs attention'}
          </div>
        </div>
      </div>

      {/* Two-column layout: Flags by Category + Expiring Soon */}
      <div className="grid-2 mb-4">
        {/* Flags by Category */}
        <div className="card">
          <div className="card-header">
            <span className="card-title">Flags by Category</span>
            <span className="text-xs text-muted">{stats.totalFlags} total</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {flagsByCategory.length > 0 ? (
              flagsByCategory.map((cat) => (
                <div key={cat.category} className="flex items-center gap-3">
                  <span
                    className={`badge badge-${cat.category}`}
                    style={{ minWidth: 90, justifyContent: 'center' }}
                  >
                    {cat.category}
                  </span>
                  <div
                    style={{
                      flex: 1,
                      height: 8,
                      borderRadius: 4,
                      background: 'var(--color-bg)',
                      overflow: 'hidden',
                    }}
                  >
                    <div
                      style={{
                        width: `${cat.percentage}%`,
                        height: '100%',
                        borderRadius: 4,
                        background: `var(--color-${
                          cat.category === 'release'
                            ? 'info'
                            : cat.category === 'feature'
                            ? 'purple'
                            : cat.category === 'experiment'
                            ? 'warning'
                            : cat.category === 'ops'
                            ? 'cyan'
                            : 'success'
                        })`,
                        transition: 'width 0.4s ease',
                      }}
                    />
                  </div>
                  <span className="text-sm" style={{ minWidth: 24, textAlign: 'right' }}>
                    {cat.count}
                  </span>
                </div>
              ))
            ) : (
              <div className="text-center py-4 text-muted">
                <p>No flags found</p>
                <p className="text-xs">Create your first flag to get started</p>
              </div>
            )}
          </div>
        </div>

        {/* Expiring Soon */}
        <div className="card">
          <div className="card-header">
            <span className="card-title">Expiring Soon</span>
            <span className="text-xs text-muted">{expiringFlags.length} flags</span>
          </div>
          <div className="table-container">
            {expiringFlags.length > 0 ? (
              <table>
                <thead>
                  <tr>
                    <th>Flag</th>
                    <th>Owner</th>
                    <th style={{ textAlign: 'right' }}>Days Left</th>
                  </tr>
                </thead>
                <tbody>
                  {expiringFlags.slice(0, 5).map((flag) => (
                    <tr key={flag.id}>
                      <td>
                        <div className="flex items-center gap-2">
                          <code className="font-mono text-sm">{flag.name}</code>
                          <span className={`badge badge-${flag.category}`}>{flag.category}</span>
                        </div>
                      </td>
                      <td className="text-secondary text-sm">{flag.owner}</td>
                      <td style={{ textAlign: 'right' }}>
                        <span className={daysRemainingColor(flag.daysRemaining)}>
                          {flag.daysRemaining}d
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="text-center py-4 text-muted">
                <p>No flags expiring soon</p>
                <p className="text-xs">All flags are current or permanent</p>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Recent Activity */}
      <div className="card mb-4">
        <div className="card-header">
          <span className="card-title">Recent Activity</span>
          <span className="text-xs text-muted">
            {connected ? 'Live updates' : 'Last updated: refresh to see latest'}
          </span>
        </div>
        <div className="table-container">
          {recentActivity.length > 0 ? (
            <table>
              <thead>
                <tr>
                  <th>Event</th>
                  <th>Actor</th>
                  <th style={{ textAlign: 'right' }}>Time</th>
                </tr>
              </thead>
              <tbody>
                {recentActivity.map((event) => (
                  <tr key={event.id}>
                    <td className={event.warning ? 'text-warning' : ''}>{event.description}</td>
                    <td className="text-secondary text-sm">{event.actor}</td>
                    <td
                      className={`text-sm ${event.warning ? 'text-warning' : 'text-muted'}`}
                      style={{ textAlign: 'right', whiteSpace: 'nowrap' }}
                    >
                      {event.timestamp}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="text-center py-4 text-muted">
              <p>No recent activity</p>
              <p className="text-xs">Activity will appear here as flags and deployments change</p>
            </div>
          )}
        </div>
      </div>

      {/* Active Deployments */}
      <div className="card">
        <div className="card-header">
          <span className="card-title">Active Deployments</span>
          <span className="text-xs text-muted">{activeDeployments.length} running</span>
        </div>
        <div className="table-container">
          {activeDeployments.length > 0 ? (
            <table>
              <thead>
                <tr>
                  <th>Environment</th>
                  <th>Version</th>
                  <th>Strategy</th>
                  <th>Status</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {activeDeployments.slice(0, 10).map((dep) => (
                  <tr key={dep.id}>
                    <td style={{ fontWeight: 500 }}>{dep.environment_id}</td>
                    <td>
                      <code className="font-mono text-sm">{dep.version}</code>
                    </td>
                    <td>
                      <span className={strategyBadgeClass(dep.strategy)}>{dep.strategy}</span>
                    </td>
                    <td>
                      <span className={statusBadgeClass(dep.status)}>{dep.status}</span>
                    </td>
                    <td className="text-sm text-muted">
                      {new Date(dep.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="text-center py-4 text-muted">
              <p>No active deployments</p>
              <p className="text-xs">Create your first deployment to see it here</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default DashboardPage;
