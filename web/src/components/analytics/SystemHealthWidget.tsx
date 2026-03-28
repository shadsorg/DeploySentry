import { useState, useEffect } from 'react';

interface SystemHealthWidgetProps {
  detailed?: boolean;
}

interface SystemHealthMetrics {
  timestamp: string;
  requests_per_second: number;
  avg_latency_ms: number;
  error_rate: number;
  active_connections: number;
  database_connections: number;
  query_latency_ms: number;
  cache_hit_rate: number;
  memory_usage_percent: number;
  cpu_usage_percent: number;
  memory_usage_bytes: number;
  disk_usage_percent: number;
}

export default function SystemHealthWidget({ detailed = false }: SystemHealthWidgetProps) {
  const [metrics, setMetrics] = useState<SystemHealthMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        // Mock data for development - replace with real API call
        const mockMetrics: SystemHealthMetrics = {
          timestamp: new Date().toISOString(),
          requests_per_second: 45.7,
          avg_latency_ms: 125.6,
          error_rate: 0.12,
          active_connections: 89,
          database_connections: 12,
          query_latency_ms: 8.3,
          cache_hit_rate: 94.7,
          memory_usage_percent: 67.2,
          cpu_usage_percent: 34.8,
          memory_usage_bytes: 1024 * 1024 * 512, // 512 MB
          disk_usage_percent: 45.3,
        };

        setMetrics(mockMetrics);
        setLastUpdated(new Date());
      } catch (err) {
        console.error('Failed to fetch system health:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchMetrics();

    // Update every 30 seconds
    const interval = setInterval(fetchMetrics, 30000);
    return () => clearInterval(interval);
  }, []);

  const formatBytes = (bytes: number): string => {
    if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
    if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return bytes + ' B';
  };

  const getHealthStatus = (metrics: SystemHealthMetrics) => {
    if (metrics.error_rate > 1 || metrics.avg_latency_ms > 500 || metrics.cpu_usage_percent > 80) {
      return 'critical';
    }
    if (metrics.error_rate > 0.5 || metrics.avg_latency_ms > 200 || metrics.cpu_usage_percent > 60) {
      return 'warning';
    }
    return 'healthy';
  };

  if (loading) {
    return <div className="loading-spinner">Loading health metrics...</div>;
  }

  if (!metrics) {
    return <div className="error-message">Failed to load health metrics</div>;
  }

  const healthStatus = getHealthStatus(metrics);

  return (
    <div className={`system-health-widget card ${detailed ? 'detailed' : ''}`}>
      <div className="widget-header">
        <h3>🏥 System Health</h3>
        <div className="health-status-indicator">
          <div className={`status-dot ${healthStatus}`}></div>
          <span className={`status-text ${healthStatus}`}>
            {healthStatus === 'healthy' && 'Healthy'}
            {healthStatus === 'warning' && 'Warning'}
            {healthStatus === 'critical' && 'Critical'}
          </span>
        </div>
      </div>

      <div className="health-metrics">
        {detailed ? (
          <div className="detailed-metrics">
            <div className="metric-section">
              <h4>API Performance</h4>
              <div className="metric-grid">
                <div className="metric-item">
                  <div className="metric-label">Requests/sec</div>
                  <div className="metric-value">{metrics.requests_per_second.toFixed(1)}</div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Avg Latency</div>
                  <div className={`metric-value ${metrics.avg_latency_ms > 200 ? 'warning' : ''}`}>
                    {metrics.avg_latency_ms.toFixed(0)}ms
                  </div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Error Rate</div>
                  <div className={`metric-value ${metrics.error_rate > 0.5 ? 'warning' : ''}`}>
                    {metrics.error_rate.toFixed(2)}%
                  </div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Active Connections</div>
                  <div className="metric-value">{metrics.active_connections}</div>
                </div>
              </div>
            </div>

            <div className="metric-section">
              <h4>Database</h4>
              <div className="metric-grid">
                <div className="metric-item">
                  <div className="metric-label">Connections</div>
                  <div className="metric-value">{metrics.database_connections}</div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Query Latency</div>
                  <div className="metric-value">{metrics.query_latency_ms.toFixed(1)}ms</div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Cache Hit Rate</div>
                  <div className={`metric-value ${metrics.cache_hit_rate < 90 ? 'warning' : ''}`}>
                    {metrics.cache_hit_rate.toFixed(1)}%
                  </div>
                </div>
              </div>
            </div>

            <div className="metric-section">
              <h4>Resources</h4>
              <div className="metric-grid">
                <div className="metric-item">
                  <div className="metric-label">CPU Usage</div>
                  <div className={`metric-value ${metrics.cpu_usage_percent > 60 ? 'warning' : ''}`}>
                    {metrics.cpu_usage_percent.toFixed(1)}%
                  </div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Memory</div>
                  <div className="metric-value">
                    {formatBytes(metrics.memory_usage_bytes)}
                  </div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Memory %</div>
                  <div className={`metric-value ${metrics.memory_usage_percent > 80 ? 'warning' : ''}`}>
                    {metrics.memory_usage_percent.toFixed(1)}%
                  </div>
                </div>
                <div className="metric-item">
                  <div className="metric-label">Disk Usage</div>
                  <div className={`metric-value ${metrics.disk_usage_percent > 85 ? 'warning' : ''}`}>
                    {metrics.disk_usage_percent.toFixed(1)}%
                  </div>
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="compact-metrics">
            <div className="metric-row">
              <div className="metric-compact">
                <span className="metric-icon">⚡</span>
                <span className="metric-text">{metrics.avg_latency_ms.toFixed(0)}ms</span>
              </div>
              <div className="metric-compact">
                <span className="metric-icon">📊</span>
                <span className="metric-text">{metrics.requests_per_second.toFixed(1)} req/s</span>
              </div>
            </div>
            <div className="metric-row">
              <div className="metric-compact">
                <span className="metric-icon">💾</span>
                <span className="metric-text">{metrics.cache_hit_rate.toFixed(1)}% cache</span>
              </div>
              <div className="metric-compact">
                <span className="metric-icon">🖥️</span>
                <span className="metric-text">{metrics.cpu_usage_percent.toFixed(1)}% CPU</span>
              </div>
            </div>
          </div>
        )}
      </div>

      <div className="widget-footer">
        <span className="last-updated">
          Updated {lastUpdated.toLocaleTimeString()}
        </span>
      </div>

      {healthStatus !== 'healthy' && (
        <div className={`health-alert ${healthStatus}`}>
          {healthStatus === 'critical' && (
            <>
              <strong>Critical:</strong> System performance degraded. Check error logs and resource usage.
            </>
          )}
          {healthStatus === 'warning' && (
            <>
              <strong>Warning:</strong> Performance may be impacted. Monitor closely.
            </>
          )}
        </div>
      )}
    </div>
  );
}