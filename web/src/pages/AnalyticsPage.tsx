import { useState, useEffect } from 'react';
import { useSearchParams, useParams } from 'react-router-dom';
import AnalyticsSummary from '../components/analytics/AnalyticsSummary';
import FlagAnalytics from '../components/analytics/FlagAnalytics';
import DeploymentAnalytics from '../components/analytics/DeploymentAnalytics';
import SystemHealthWidget from '../components/analytics/SystemHealthWidget';

type TimeRange = '24h' | '7d' | '30d';
type TabType = 'overview' | 'flags' | 'deployments' | 'system';

export default function AnalyticsPage() {
  const { projectSlug } = useParams();
  const projectName = projectSlug ?? '';

  const [searchParams, setSearchParams] = useSearchParams();
  const [timeRange, setTimeRange] = useState<TimeRange>(
    (searchParams.get('time_range') as TimeRange) || '24h',
  );
  const [activeTab, setActiveTab] = useState<TabType>(
    (searchParams.get('tab') as TabType) || 'overview',
  );

  // Mock project/environment IDs - in real app these would come from context
  const projectId = 'proj-123';
  const environmentId = 'env-prod';

  useEffect(() => {
    setSearchParams({
      tab: activeTab,
      time_range: timeRange,
    });
  }, [activeTab, timeRange, setSearchParams]);

  const handleTimeRangeChange = (range: TimeRange) => {
    setTimeRange(range);
  };

  const handleTabChange = (tab: TabType) => {
    setActiveTab(tab);
  };

  return (
    <div className="analytics-page">
      <div className="page-header-row">
        <div className="page-header" style={{ marginBottom: 0 }}>
          <h1>{projectName ? `${projectName} — Analytics` : 'Analytics'}</h1>
          <p>Aggregated insights for this project.</p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, background: 'var(--color-bg-elevated)', border: '1px solid var(--color-border)', borderRadius: 8, padding: 4 }}>
          {(['24h', '7d', '30d'] as TimeRange[]).map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => handleTimeRangeChange(r)}
              style={{
                padding: '6px 14px',
                fontSize: 12,
                fontWeight: 600,
                borderRadius: 6,
                border: timeRange === r ? '1px solid rgba(99,102,241,0.3)' : '1px solid transparent',
                background: timeRange === r ? 'var(--color-primary-bg)' : 'transparent',
                color: timeRange === r ? 'var(--color-primary)' : 'var(--color-text-muted)',
                cursor: 'pointer',
              }}
            >
              {r === '24h' ? 'Last 24h' : r === '7d' ? '7 Days' : '30 Days'}
            </button>
          ))}
        </div>
      </div>

      {/* Tab Navigation */}
      <div className="tabs">
        {([
          { key: 'overview', icon: 'insights', label: 'Overview' },
          { key: 'flags', icon: 'toggle_on', label: 'Flag Analytics' },
          { key: 'deployments', icon: 'rocket_launch', label: 'Deployments' },
          { key: 'system', icon: 'monitor_heart', label: 'System Health' },
        ] as { key: TabType; icon: string; label: string }[]).map(({ key, icon, label }) => (
          <button
            key={key}
            className={`tab${activeTab === key ? ' active' : ''}`}
            onClick={() => handleTabChange(key)}
          >
            <span className="ms" style={{ fontSize: 15, verticalAlign: 'middle', marginRight: 6 }}>{icon}</span>
            {label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="tab-content">
        {activeTab === 'overview' && (
          <div className="overview-tab">
            <AnalyticsSummary
              projectId={projectId}
              environmentId={environmentId}
              timeRange={timeRange}
            />
            <div className="overview-widgets">
              <div className="widget-row">
                <SystemHealthWidget />
                <div className="quick-stats-widget card">
                  <h3>Quick Stats</h3>
                  <div className="stats-grid">
                    <div className="stat-item">
                      <div className="stat-value">12.5k</div>
                      <div className="stat-label">Flag Evaluations</div>
                    </div>
                    <div className="stat-item">
                      <div className="stat-value">847</div>
                      <div className="stat-label">Unique Users</div>
                    </div>
                    <div className="stat-item">
                      <div className="stat-value">99.2%</div>
                      <div className="stat-label">Uptime</div>
                    </div>
                    <div className="stat-item">
                      <div className="stat-value">15ms</div>
                      <div className="stat-label">Avg Latency</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'flags' && (
          <FlagAnalytics
            projectId={projectId}
            environmentId={environmentId}
            timeRange={timeRange}
          />
        )}

        {activeTab === 'deployments' && (
          <DeploymentAnalytics
            projectId={projectId}
            environmentId={environmentId}
            timeRange={timeRange}
          />
        )}

        {activeTab === 'system' && (
          <div className="system-tab">
            <SystemHealthWidget detailed={true} />
            <div className="system-metrics-grid">
              <div className="metric-card card">
                <h3>API Performance</h3>
                <div className="metric-chart">
                  {/* Placeholder for API performance chart */}
                  <div className="chart-placeholder">API Performance Chart</div>
                </div>
              </div>
              <div className="metric-card card">
                <h3>Database Health</h3>
                <div className="metric-chart">
                  {/* Placeholder for database metrics chart */}
                  <div className="chart-placeholder">Database Metrics Chart</div>
                </div>
              </div>
              <div className="metric-card card">
                <h3>Cache Performance</h3>
                <div className="metric-chart">
                  {/* Placeholder for cache metrics chart */}
                  <div className="chart-placeholder">Cache Metrics Chart</div>
                </div>
              </div>
              <div className="metric-card card">
                <h3>Resource Usage</h3>
                <div className="metric-chart">
                  {/* Placeholder for resource usage chart */}
                  <div className="chart-placeholder">Resource Usage Chart</div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
