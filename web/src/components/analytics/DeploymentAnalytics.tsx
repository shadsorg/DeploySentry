import { useState, useEffect } from 'react';

interface DeploymentAnalyticsProps {
  projectId: string;
  environmentId: string;
  timeRange: string;
}

interface DeploymentStrategyStats {
  strategy: string;
  total_deployments: number;
  successful_deployments: number;
  failed_deployments: number;
  success_rate: number;
  avg_duration_minutes: number;
}

export default function DeploymentAnalytics({ projectId, environmentId, timeRange }: DeploymentAnalyticsProps) {
  const [strategyStats, setStrategyStats] = useState<DeploymentStrategyStats[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchDeploymentStats = async () => {
      try {
        setLoading(true);

        // Mock data for development - replace with real API call
        const mockStats: DeploymentStrategyStats[] = [
          {
            strategy: 'canary',
            total_deployments: 15,
            successful_deployments: 14,
            failed_deployments: 1,
            success_rate: 93.3,
            avg_duration_minutes: 12.4,
          },
          {
            strategy: 'blue_green',
            total_deployments: 6,
            successful_deployments: 6,
            failed_deployments: 0,
            success_rate: 100.0,
            avg_duration_minutes: 8.7,
          },
          {
            strategy: 'rolling',
            total_deployments: 2,
            successful_deployments: 1,
            failed_deployments: 1,
            success_rate: 50.0,
            avg_duration_minutes: 15.2,
          },
        ];

        setStrategyStats(mockStats);
      } catch (err) {
        console.error('Failed to fetch deployment stats:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchDeploymentStats();
  }, [projectId, environmentId, timeRange]);

  if (loading) {
    return <div className="loading-spinner">Loading deployment analytics...</div>;
  }

  const totalDeployments = strategyStats.reduce((sum, stat) => sum + stat.total_deployments, 0);
  const totalSuccessful = strategyStats.reduce((sum, stat) => sum + stat.successful_deployments, 0);
  const overallSuccessRate = totalDeployments > 0 ? (totalSuccessful / totalDeployments) * 100 : 0;
  const avgDuration = strategyStats.length > 0
    ? strategyStats.reduce((sum, stat) => sum + stat.avg_duration_minutes, 0) / strategyStats.length
    : 0;

  return (
    <div className="deployment-analytics">
      <div className="deployment-overview">
        <h2>Deployment Performance ({timeRange})</h2>

        <div className="overview-cards">
          <div className="overview-card card">
            <h3>Total Deployments</h3>
            <div className="overview-value">{totalDeployments}</div>
            <div className="overview-subtitle">across all strategies</div>
          </div>
          <div className="overview-card card">
            <h3>Success Rate</h3>
            <div className={`overview-value ${overallSuccessRate > 90 ? 'good' : 'warning'}`}>
              {overallSuccessRate.toFixed(1)}%
            </div>
            <div className="overview-subtitle">{totalSuccessful} successful</div>
          </div>
          <div className="overview-card card">
            <h3>Avg Duration</h3>
            <div className="overview-value">{avgDuration.toFixed(1)}m</div>
            <div className="overview-subtitle">end to end</div>
          </div>
          <div className="overview-card card">
            <h3>Rollback Rate</h3>
            <div className="overview-value">4.3%</div>
            <div className="overview-subtitle">1 rollback</div>
          </div>
        </div>
      </div>

      <div className="strategy-performance">
        <h3>Performance by Strategy</h3>
        <div className="card">
          <table className="strategy-table">
            <thead>
              <tr>
                <th>Strategy</th>
                <th>Deployments</th>
                <th>Success Rate</th>
                <th>Avg Duration</th>
                <th>Health Score</th>
              </tr>
            </thead>
            <tbody>
              {strategyStats.map((stat) => (
                <tr key={stat.strategy}>
                  <td>
                    <div className="strategy-name">
                      <span className={`strategy-icon ${stat.strategy}`}>
                        {stat.strategy === 'canary' && '🐦'}
                        {stat.strategy === 'blue_green' && '🔄'}
                        {stat.strategy === 'rolling' && '📦'}
                      </span>
                      {stat.strategy.replace('_', ' ')}
                    </div>
                  </td>
                  <td>
                    <div className="deployment-count">
                      {stat.total_deployments}
                      <div className="count-breakdown">
                        {stat.successful_deployments} success, {stat.failed_deployments} failed
                      </div>
                    </div>
                  </td>
                  <td>
                    <div className={`success-rate ${stat.success_rate > 90 ? 'good' : 'warning'}`}>
                      {stat.success_rate.toFixed(1)}%
                    </div>
                  </td>
                  <td>
                    <div className="duration">
                      {stat.avg_duration_minutes.toFixed(1)}m
                    </div>
                  </td>
                  <td>
                    <div className="health-score">
                      <div className="score-bar">
                        <div
                          className="score-fill"
                          style={{ width: `${stat.success_rate}%` }}
                        ></div>
                      </div>
                      <span className="score-text">{stat.success_rate.toFixed(0)}/100</span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="deployment-timeline">
        <h3>Recent Deployments</h3>
        <div className="card">
          <div className="timeline">
            <div className="timeline-item success">
              <div className="timeline-icon">✅</div>
              <div className="timeline-content">
                <div className="timeline-title">v2.1.4 Canary Deployment</div>
                <div className="timeline-meta">Production • 2 hours ago • 12.3m duration</div>
                <div className="timeline-description">Successfully deployed with 0% traffic increase</div>
              </div>
            </div>
            <div className="timeline-item success">
              <div className="timeline-icon">✅</div>
              <div className="timeline-content">
                <div className="timeline-title">v2.1.3 Blue-Green Deployment</div>
                <div className="timeline-meta">Production • 1 day ago • 8.7m duration</div>
                <div className="timeline-description">Completed successfully with zero downtime</div>
              </div>
            </div>
            <div className="timeline-item warning">
              <div className="timeline-icon">⚠️</div>
              <div className="timeline-content">
                <div className="timeline-title">v2.1.2 Rolling Deployment</div>
                <div className="timeline-meta">Production • 3 days ago • 15.2m duration</div>
                <div className="timeline-description">Completed with warnings - slow health checks</div>
              </div>
            </div>
            <div className="timeline-item error">
              <div className="timeline-icon">❌</div>
              <div className="timeline-content">
                <div className="timeline-title">v2.1.1 Canary Deployment</div>
                <div className="timeline-meta">Production • 5 days ago • Failed after 8.1m</div>
                <div className="timeline-description">Failed health checks, automatically rolled back</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="deployment-insights">
        <h3>Insights & Recommendations</h3>
        <div className="insights-grid">
          <div className="insight-card card">
            <div className="insight-icon">🎯</div>
            <div className="insight-content">
              <div className="insight-title">Blue-Green Preferred</div>
              <div className="insight-description">
                Blue-green deployments have 100% success rate and fastest completion time
              </div>
            </div>
          </div>
          <div className="insight-card card">
            <div className="insight-icon">⚡</div>
            <div className="insight-content">
              <div className="insight-title">Optimize Rolling</div>
              <div className="insight-description">
                Rolling deployments take 74% longer than average - consider health check tuning
              </div>
            </div>
          </div>
          <div className="insight-card card">
            <div className="insight-icon">📈</div>
            <div className="insight-content">
              <div className="insight-title">Trend Improving</div>
              <div className="insight-description">
                Success rate increased from 85% to 93% over the past 30 days
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}