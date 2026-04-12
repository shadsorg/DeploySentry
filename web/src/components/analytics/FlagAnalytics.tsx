import { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';

interface FlagAnalyticsProps {
  projectId: string;
  environmentId: string;
  timeRange: string;
}

interface FlagStats {
  flag_key: string;
  evaluations: number;
  unique_users: number;
  avg_latency_ms: number;
  cache_hit_rate: number;
  error_count: number;
}

interface FlagUsageStats {
  flag_key: string;
  project_id: string;
  environment_id: string;
  time_range: string;
  total_evaluations: number;
  unique_users: number;
  result_distribution: Record<string, number>;
  avg_latency_ms: number;
  cache_hit_rate: number;
  error_count: number;
  hourly_evaluations?: Array<{ timestamp: string; value: number }>;
  daily_evaluations?: Array<{ timestamp: string; value: number }>;
}

export default function FlagAnalytics({ projectId, environmentId, timeRange }: FlagAnalyticsProps) {
  const [flagStats, setFlagStats] = useState<FlagStats[]>([]);
  const [selectedFlag, setSelectedFlag] = useState<string | null>(null);
  const [flagUsage, setFlagUsage] = useState<FlagUsageStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [sortBy, setSortBy] = useState<keyof FlagStats>('evaluations');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');

  useEffect(() => {
    const fetchFlagStats = async () => {
      try {
        setLoading(true);

        // Mock data for development - replace with real API call
        const mockStats: FlagStats[] = [
          {
            flag_key: 'enable-dark-mode',
            evaluations: 8534,
            unique_users: 423,
            avg_latency_ms: 12.3,
            cache_hit_rate: 96.7,
            error_count: 2,
          },
          {
            flag_key: 'checkout-v2-rollout',
            evaluations: 5421,
            unique_users: 312,
            avg_latency_ms: 18.7,
            cache_hit_rate: 91.2,
            error_count: 8,
          },
          {
            flag_key: 'search-ranking-experiment',
            evaluations: 3456,
            unique_users: 198,
            avg_latency_ms: 15.9,
            cache_hit_rate: 94.3,
            error_count: 1,
          },
          {
            flag_key: 'rate-limit-override',
            evaluations: 1872,
            unique_users: 87,
            avg_latency_ms: 9.2,
            cache_hit_rate: 98.1,
            error_count: 0,
          },
          {
            flag_key: 'admin-billing-access',
            evaluations: 934,
            unique_users: 23,
            avg_latency_ms: 11.4,
            cache_hit_rate: 97.8,
            error_count: 0,
          },
          {
            flag_key: 'notification-center-v3',
            evaluations: 567,
            unique_users: 45,
            avg_latency_ms: 22.1,
            cache_hit_rate: 87.3,
            error_count: 4,
          },
        ];

        setFlagStats(mockStats);
      } catch (err) {
        console.error('Failed to fetch flag stats:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchFlagStats();
  }, [projectId, environmentId, timeRange]);

  const fetchFlagUsage = async (flagKey: string) => {
    try {
      // Mock data for development - replace with real API call
      const mockUsage: FlagUsageStats = {
        flag_key: flagKey,
        project_id: projectId,
        environment_id: environmentId,
        time_range: timeRange,
        total_evaluations: 8534,
        unique_users: 423,
        result_distribution: {
          true: 6827,
          false: 1707,
        },
        avg_latency_ms: 12.3,
        cache_hit_rate: 96.7,
        error_count: 2,
        hourly_evaluations: [
          { timestamp: '2024-03-27T00:00:00Z', value: 234 },
          { timestamp: '2024-03-27T01:00:00Z', value: 187 },
          { timestamp: '2024-03-27T02:00:00Z', value: 156 },
          { timestamp: '2024-03-27T03:00:00Z', value: 123 },
          { timestamp: '2024-03-27T04:00:00Z', value: 98 },
          { timestamp: '2024-03-27T05:00:00Z', value: 134 },
          { timestamp: '2024-03-27T06:00:00Z', value: 189 },
          { timestamp: '2024-03-27T07:00:00Z', value: 267 },
          { timestamp: '2024-03-27T08:00:00Z', value: 345 },
          { timestamp: '2024-03-27T09:00:00Z', value: 432 },
          { timestamp: '2024-03-27T10:00:00Z', value: 498 },
          { timestamp: '2024-03-27T11:00:00Z', value: 543 },
          { timestamp: '2024-03-27T12:00:00Z', value: 578 },
          { timestamp: '2024-03-27T13:00:00Z', value: 612 },
          { timestamp: '2024-03-27T14:00:00Z', value: 634 },
          { timestamp: '2024-03-27T15:00:00Z', value: 589 },
          { timestamp: '2024-03-27T16:00:00Z', value: 534 },
          { timestamp: '2024-03-27T17:00:00Z', value: 478 },
          { timestamp: '2024-03-27T18:00:00Z', value: 423 },
          { timestamp: '2024-03-27T19:00:00Z', value: 367 },
          { timestamp: '2024-03-27T20:00:00Z', value: 298 },
          { timestamp: '2024-03-27T21:00:00Z', value: 234 },
          { timestamp: '2024-03-27T22:00:00Z', value: 189 },
          { timestamp: '2024-03-27T23:00:00Z', value: 156 },
        ],
      };

      setFlagUsage(mockUsage);
    } catch (err) {
      console.error('Failed to fetch flag usage:', err);
    }
  };

  const handleSort = (column: keyof FlagStats) => {
    if (sortBy === column) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(column);
      setSortOrder('desc');
    }
  };

  // Performance Optimization: Memoize the sorted results to prevent O(n log n) sorting
  // on every render (e.g. when state other than sort options or data changes).
  const sortedStats = useMemo(() => {
    return [...flagStats].sort((a, b) => {
      const aValue = a[sortBy];
      const bValue = b[sortBy];
      const multiplier = sortOrder === 'asc' ? 1 : -1;
      return (aValue > bValue ? 1 : -1) * multiplier;
    });
  }, [flagStats, sortBy, sortOrder]);

  const formatNumber = (num: number): string => {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toString();
  };

  if (loading) {
    return <div className="loading-spinner">Loading flag analytics...</div>;
  }

  return (
    <div className="flag-analytics">
      {!selectedFlag ? (
        <div className="flag-list-view">
          <div className="section-header">
            <h2>Flag Performance ({timeRange})</h2>
            <div className="section-actions">
              <input type="text" className="search-input" placeholder="Search flags..." />
            </div>
          </div>

          <div className="card">
            <table className="flag-stats-table">
              <thead>
                <tr>
                  <th
                    className={`sortable ${sortBy === 'flag_key' ? sortOrder : ''}`}
                    onClick={() => handleSort('flag_key')}
                  >
                    Flag Key
                  </th>
                  <th
                    className={`sortable ${sortBy === 'evaluations' ? sortOrder : ''}`}
                    onClick={() => handleSort('evaluations')}
                  >
                    Evaluations
                  </th>
                  <th
                    className={`sortable ${sortBy === 'unique_users' ? sortOrder : ''}`}
                    onClick={() => handleSort('unique_users')}
                  >
                    Users
                  </th>
                  <th
                    className={`sortable ${sortBy === 'avg_latency_ms' ? sortOrder : ''}`}
                    onClick={() => handleSort('avg_latency_ms')}
                  >
                    Avg Latency
                  </th>
                  <th
                    className={`sortable ${sortBy === 'cache_hit_rate' ? sortOrder : ''}`}
                    onClick={() => handleSort('cache_hit_rate')}
                  >
                    Cache Hit
                  </th>
                  <th
                    className={`sortable ${sortBy === 'error_count' ? sortOrder : ''}`}
                    onClick={() => handleSort('error_count')}
                  >
                    Errors
                  </th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sortedStats.map((stat) => (
                  <tr key={stat.flag_key}>
                    <td>
                      <Link to={`/flags/${stat.flag_key}`} className="flag-link">
                        {stat.flag_key}
                      </Link>
                    </td>
                    <td className="number">{formatNumber(stat.evaluations)}</td>
                    <td className="number">{formatNumber(stat.unique_users)}</td>
                    <td className="number">{stat.avg_latency_ms.toFixed(1)}ms</td>
                    <td className="number">
                      <span
                        className={`cache-rate ${stat.cache_hit_rate > 90 ? 'good' : 'warning'}`}
                      >
                        {stat.cache_hit_rate.toFixed(1)}%
                      </span>
                    </td>
                    <td className="number">
                      <span className={`error-count ${stat.error_count > 0 ? 'warning' : 'good'}`}>
                        {stat.error_count}
                      </span>
                    </td>
                    <td>
                      <button
                        className="btn btn-sm btn-secondary"
                        onClick={() => {
                          setSelectedFlag(stat.flag_key);
                          fetchFlagUsage(stat.flag_key);
                        }}
                      >
                        View Details
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <div className="flag-detail-view">
          <div className="section-header">
            <button className="btn btn-secondary" onClick={() => setSelectedFlag(null)}>
              ← Back to List
            </button>
            <h2>Flag Analytics: {selectedFlag}</h2>
          </div>

          {flagUsage && (
            <div className="flag-usage-details">
              <div className="usage-summary-cards">
                <div className="usage-card card">
                  <h3>Total Evaluations</h3>
                  <div className="usage-value">{formatNumber(flagUsage.total_evaluations)}</div>
                  <div className="usage-subtitle">{timeRange}</div>
                </div>
                <div className="usage-card card">
                  <h3>Unique Users</h3>
                  <div className="usage-value">{formatNumber(flagUsage.unique_users)}</div>
                  <div className="usage-subtitle">Active users</div>
                </div>
                <div className="usage-card card">
                  <h3>Avg Latency</h3>
                  <div className="usage-value">{flagUsage.avg_latency_ms.toFixed(1)}ms</div>
                  <div className="usage-subtitle">Evaluation time</div>
                </div>
                <div className="usage-card card">
                  <h3>Cache Hit Rate</h3>
                  <div className="usage-value">{flagUsage.cache_hit_rate.toFixed(1)}%</div>
                  <div className="usage-subtitle">Performance</div>
                </div>
              </div>

              <div className="usage-charts-row">
                <div className="chart-card card">
                  <h3>Result Distribution</h3>
                  <div className="result-distribution">
                    {Object.entries(flagUsage.result_distribution).map(([result, count]) => (
                      <div key={result} className="result-item">
                        <div className="result-label">{result}</div>
                        <div className="result-bar">
                          <div
                            className="result-fill"
                            style={{
                              width: `${(count / flagUsage.total_evaluations) * 100}%`,
                            }}
                          ></div>
                        </div>
                        <div className="result-count">{formatNumber(count)}</div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="chart-card card">
                  <h3>Usage Over Time</h3>
                  <div className="time-chart">
                    {/* Simplified bar chart */}
                    <div className="chart-bars">
                      {(flagUsage.hourly_evaluations || []).slice(-12).map((point, i) => {
                        const maxValue = Math.max(
                          ...(flagUsage.hourly_evaluations || []).map((p) => p.value),
                        );
                        const height = (point.value / maxValue) * 100;
                        return (
                          <div key={i} className="chart-bar">
                            <div
                              className="bar-fill"
                              style={{ height: `${height}%` }}
                              title={`${new Date(point.timestamp).getHours()}:00 - ${point.value} evaluations`}
                            ></div>
                          </div>
                        );
                      })}
                    </div>
                    <div className="chart-labels">
                      {(flagUsage.hourly_evaluations || []).slice(-12).map((point, i) => (
                        <div key={i} className="chart-label">
                          {new Date(point.timestamp).getHours()}
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </div>

              {flagUsage.error_count > 0 && (
                <div className="error-analysis card">
                  <h3>Error Analysis</h3>
                  <div className="error-stats">
                    <div className="error-stat">
                      <div className="error-value">{flagUsage.error_count}</div>
                      <div className="error-label">Total Errors</div>
                    </div>
                    <div className="error-stat">
                      <div className="error-value">
                        {((flagUsage.error_count / flagUsage.total_evaluations) * 100).toFixed(3)}%
                      </div>
                      <div className="error-label">Error Rate</div>
                    </div>
                  </div>
                  <div className="error-recommendation">
                    <p>⚠️ This flag has evaluation errors. Consider checking:</p>
                    <ul>
                      <li>Targeting rule configuration</li>
                      <li>Context attribute validation</li>
                      <li>SDK error logs</li>
                    </ul>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
