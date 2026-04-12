import { useState, useEffect } from 'react';

interface AnalyticsSummaryProps {
  projectId: string;
  environmentId: string;
  timeRange: string;
}

interface AnalyticsSummaryData {
  project_id: string;
  environment_id: string;
  time_range: string;

  // Flag Analytics
  total_flags: number;
  active_flags: number;
  flag_evaluations: number;
  unique_users: number;
  avg_evaluation_latency_ms: number;
  cache_hit_rate: number;
  error_rate: number;

  // Deployment Analytics
  total_deployments: number;
  successful_deployments: number;
  failed_deployments: number;
  avg_deployment_time_minutes: number;
  rollback_rate: number;

  // API Health
  api_requests: number;
  api_errors: number;
  avg_api_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
}

export default function AnalyticsSummary({
  projectId,
  environmentId,
  timeRange,
}: AnalyticsSummaryProps) {
  const [data, setData] = useState<AnalyticsSummaryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchSummary = async () => {
      try {
        setLoading(true);
        setError(null);
        const response = await fetch(
          `/api/v1/analytics/summary?project_id=${projectId}&environment_id=${environmentId}&time_range=${timeRange}`,
        );

        if (!response.ok) {
          throw new Error('Failed to fetch analytics summary');
        }

        const summaryData = await response.json();
        setData(summaryData);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Unknown error');
        // Mock data for development
        setData({
          project_id: projectId,
          environment_id: environmentId,
          time_range: timeRange,
          total_flags: 24,
          active_flags: 18,
          flag_evaluations: 12543,
          unique_users: 847,
          avg_evaluation_latency_ms: 15.2,
          cache_hit_rate: 94.7,
          error_rate: 0.03,
          total_deployments: 23,
          successful_deployments: 21,
          failed_deployments: 1,
          avg_deployment_time_minutes: 12.4,
          rollback_rate: 4.3,
          api_requests: 45678,
          api_errors: 23,
          avg_api_latency_ms: 125.6,
          p95_latency_ms: 245.2,
          p99_latency_ms: 387.1,
        });
      } finally {
        setLoading(false);
      }
    };

    fetchSummary();
  }, [projectId, environmentId, timeRange]);

  if (loading) {
    return <div className="loading-spinner">Loading analytics summary...</div>;
  }

  if (error) {
    return <div className="error-message">Error: {error}</div>;
  }

  if (!data) {
    return <div className="error-message">No data available</div>;
  }

  const successRate =
    data.total_deployments > 0
      ? ((data.successful_deployments / data.total_deployments) * 100).toFixed(1)
      : '0';

  const apiErrorRate =
    data.api_requests > 0 ? ((data.api_errors / data.api_requests) * 100).toFixed(3) : '0';

  return (
    <div className="analytics-summary">
      <div className="summary-cards">
        {/* Flag Performance */}
        <div className="summary-card flag-performance">
          <div className="card-header">
            <h3>🚩 Flag Performance</h3>
            <span className="time-range">{timeRange}</span>
          </div>
          <div className="metrics-grid">
            <div className="metric">
              <div className="metric-value">{data.flag_evaluations.toLocaleString()}</div>
              <div className="metric-label">Evaluations</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.unique_users.toLocaleString()}</div>
              <div className="metric-label">Unique Users</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.avg_evaluation_latency_ms.toFixed(1)}ms</div>
              <div className="metric-label">Avg Latency</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.cache_hit_rate.toFixed(1)}%</div>
              <div className="metric-label">Cache Hit Rate</div>
            </div>
          </div>
          <div className="summary-footer">
            <span className="active-flags">
              {data.active_flags} of {data.total_flags} flags active
            </span>
            <span className={`error-rate ${data.error_rate < 0.1 ? 'good' : 'warning'}`}>
              {data.error_rate.toFixed(2)}% error rate
            </span>
          </div>
        </div>

        {/* Deployment Health */}
        <div className="summary-card deployment-health">
          <div className="card-header">
            <h3>🚀 Deployment Health</h3>
            <span className="time-range">{timeRange}</span>
          </div>
          <div className="metrics-grid">
            <div className="metric">
              <div className="metric-value">{data.total_deployments}</div>
              <div className="metric-label">Deployments</div>
            </div>
            <div className="metric">
              <div className="metric-value">{successRate}%</div>
              <div className="metric-label">Success Rate</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.avg_deployment_time_minutes.toFixed(1)}m</div>
              <div className="metric-label">Avg Duration</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.rollback_rate.toFixed(1)}%</div>
              <div className="metric-label">Rollback Rate</div>
            </div>
          </div>
          <div className="summary-footer">
            <span className="successful-deployments">{data.successful_deployments} successful</span>
            <span
              className={`failed-deployments ${data.failed_deployments > 0 ? 'warning' : 'good'}`}
            >
              {data.failed_deployments} failed
            </span>
          </div>
        </div>

        {/* API Performance */}
        <div className="summary-card api-performance">
          <div className="card-header">
            <h3>⚡ API Performance</h3>
            <span className="time-range">{timeRange}</span>
          </div>
          <div className="metrics-grid">
            <div className="metric">
              <div className="metric-value">{data.api_requests.toLocaleString()}</div>
              <div className="metric-label">Requests</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.avg_api_latency_ms.toFixed(0)}ms</div>
              <div className="metric-label">Avg Latency</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.p95_latency_ms.toFixed(0)}ms</div>
              <div className="metric-label">P95 Latency</div>
            </div>
            <div className="metric">
              <div className="metric-value">{data.p99_latency_ms.toFixed(0)}ms</div>
              <div className="metric-label">P99 Latency</div>
            </div>
          </div>
          <div className="summary-footer">
            <span className="requests-per-minute">
              {Math.round(
                data.api_requests /
                  (timeRange === '24h' ? 1440 : timeRange === '7d' ? 10080 : 43200),
              )}{' '}
              req/min avg
            </span>
            <span className={`error-rate ${parseFloat(apiErrorRate) < 0.1 ? 'good' : 'warning'}`}>
              {apiErrorRate}% error rate
            </span>
          </div>
        </div>
      </div>

      {/* Health Score Overview */}
      <div className="health-overview card">
        <h3>System Health Overview</h3>
        <div className="health-indicators">
          <div className={`health-indicator ${data.error_rate < 0.1 ? 'healthy' : 'warning'}`}>
            <div className="indicator-icon">🚩</div>
            <div className="indicator-text">
              <div className="indicator-title">Flag Evaluations</div>
              <div className="indicator-status">
                {data.error_rate < 0.1 ? 'Healthy' : 'Needs Attention'}
              </div>
            </div>
            <div className="indicator-value">{data.error_rate.toFixed(2)}% errors</div>
          </div>

          <div
            className={`health-indicator ${parseFloat(successRate) > 90 ? 'healthy' : 'warning'}`}
          >
            <div className="indicator-icon">🚀</div>
            <div className="indicator-text">
              <div className="indicator-title">Deployments</div>
              <div className="indicator-status">
                {parseFloat(successRate) > 90 ? 'Healthy' : 'Needs Attention'}
              </div>
            </div>
            <div className="indicator-value">{successRate}% success</div>
          </div>

          <div
            className={`health-indicator ${parseFloat(apiErrorRate) < 0.1 ? 'healthy' : 'warning'}`}
          >
            <div className="indicator-icon">⚡</div>
            <div className="indicator-text">
              <div className="indicator-title">API Performance</div>
              <div className="indicator-status">
                {parseFloat(apiErrorRate) < 0.1 ? 'Healthy' : 'Needs Attention'}
              </div>
            </div>
            <div className="indicator-value">{data.avg_api_latency_ms.toFixed(0)}ms avg</div>
          </div>

          <div className={`health-indicator ${data.cache_hit_rate > 90 ? 'healthy' : 'warning'}`}>
            <div className="indicator-icon">💾</div>
            <div className="indicator-text">
              <div className="indicator-title">Cache Performance</div>
              <div className="indicator-status">
                {data.cache_hit_rate > 90 ? 'Healthy' : 'Needs Attention'}
              </div>
            </div>
            <div className="indicator-value">{data.cache_hit_rate.toFixed(1)}% hit rate</div>
          </div>
        </div>
      </div>
    </div>
  );
}
