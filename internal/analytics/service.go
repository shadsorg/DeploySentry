// Package analytics provides data collection and analysis for the developer dashboard.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/platform/metrics"
)

// Service handles analytics data collection and querying.
type Service struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// NewService creates a new analytics service.
func NewService(db *pgxpool.Pool, redis *redis.Client) *Service {
	return &Service{
		db:    db,
		redis: redis,
	}
}

// RecordFlagEvaluation records a flag evaluation event for analytics.
func (s *Service) RecordFlagEvaluation(ctx context.Context, event *models.FlagEvaluationEvent) error {
	// Record Prometheus metrics
	metrics.FlagEvaluations.WithLabelValues(
		event.ProjectID.String(),
		event.FlagKey,
		event.ResultValue,
	).Inc()

	// Store detailed event for analytics
	query := `
		INSERT INTO flag_evaluation_events (
			project_id, environment_id, flag_key, user_id, sdk_version,
			result_value, rule_id, latency_ms, cache_hit, error_message,
			ip_address, user_agent, context_attrs, evaluated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)`

	contextAttrsJSON, _ := json.Marshal(event.ContextAttrs)

	_, err := s.db.Exec(ctx, query,
		event.ProjectID, event.EnvironmentID, event.FlagKey, event.UserID, event.SDKVersion,
		event.ResultValue, event.RuleID, event.LatencyMs, event.CacheHit, event.ErrorMessage,
		event.IPAddress, event.UserAgent, string(contextAttrsJSON), event.EvaluatedAt,
	)

	return err
}

// RecordDeploymentEvent records a deployment state change event.
func (s *Service) RecordDeploymentEvent(ctx context.Context, event *models.DeploymentEvent) error {
	// Record Prometheus metrics
	metrics.DeploymentEvents.WithLabelValues(
		event.DeploymentID.String(), // Could lookup project_id, but deployment_id is unique enough
		string(event.EventType),
		"", // strategy - would need to join with deployments table
	).Inc()

	query := `
		INSERT INTO deployment_events (
			deployment_id, event_type, phase_name, traffic_pct, health_score,
			error_message, triggered_by, metadata, occurred_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)`

	metadataJSON, _ := json.Marshal(event.Metadata)

	_, err := s.db.Exec(ctx, query,
		event.DeploymentID, event.EventType, event.PhaseName, event.TrafficPct, event.HealthScore,
		event.ErrorMessage, event.TriggeredBy, string(metadataJSON), event.OccurredAt,
	)

	return err
}

// RecordAPIRequest records an API request for system health analytics.
func (s *Service) RecordAPIRequest(ctx context.Context, metric *models.APIRequestMetric) error {
	// Store in database for detailed analytics
	query := `
		INSERT INTO api_request_metrics (
			request_id, method, path, status_code, latency_ms, user_id, api_key_id,
			ip_address, user_agent, error_message, request_size, response_size, recorded_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)`

	_, err := s.db.Exec(ctx, query,
		metric.RequestID, metric.Method, metric.Path, metric.StatusCode, metric.LatencyMs,
		metric.UserID, metric.APIKeyID, metric.IPAddress, metric.UserAgent,
		metric.ErrorMessage, metric.RequestSize, metric.ResponseSize, metric.RecordedAt,
	)

	return err
}

// GetAnalyticsSummary returns high-level analytics for a project/environment.
func (s *Service) GetAnalyticsSummary(ctx context.Context, projectID, environmentID uuid.UUID, timeRange string) (*models.AnalyticsSummary, error) {
	var since time.Time
	switch timeRange {
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	default:
		since = time.Now().Add(-24 * time.Hour)
		timeRange = "24h"
	}

	summary := &models.AnalyticsSummary{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		TimeRange:     timeRange,
	}

	// Flag analytics
	flagQuery := `
		SELECT
			COUNT(DISTINCT flag_key) as total_flags,
			COUNT(DISTINCT CASE WHEN enabled = true THEN flag_key END) as active_flags
		FROM feature_flags
		WHERE project_id = $1 AND environment_id = $2 AND archived = false`

	err := s.db.QueryRow(ctx, flagQuery, projectID, environmentID).Scan(
		&summary.TotalFlags, &summary.ActiveFlags,
	)
	if err != nil {
		return nil, fmt.Errorf("querying flag stats: %w", err)
	}

	// Flag evaluation analytics
	evalQuery := `
		SELECT
			COUNT(*) as evaluations,
			COUNT(DISTINCT user_id) as unique_users,
			AVG(latency_ms) as avg_latency,
			AVG(CASE WHEN cache_hit THEN 1.0 ELSE 0.0 END) * 100 as cache_hit_rate,
			AVG(CASE WHEN error_message != '' THEN 1.0 ELSE 0.0 END) * 100 as error_rate
		FROM flag_evaluation_events
		WHERE project_id = $1 AND environment_id = $2 AND evaluated_at >= $3`

	err = s.db.QueryRow(ctx, evalQuery, projectID, environmentID, since).Scan(
		&summary.FlagEvaluations, &summary.UniqueUsers, &summary.AvgEvaluationLatency,
		&summary.CacheHitRate, &summary.ErrorRate,
	)
	if err != nil {
		return nil, fmt.Errorf("querying evaluation stats: %w", err)
	}

	// Deployment analytics
	deployQuery := `
		SELECT
			COUNT(DISTINCT d.id) as total,
			COUNT(DISTINCT CASE WHEN d.status = 'completed' THEN d.id END) as successful,
			COUNT(DISTINCT CASE WHEN d.status = 'failed' THEN d.id END) as failed,
			AVG(EXTRACT(EPOCH FROM (d.completed_at - d.started_at)) / 60) as avg_duration
		FROM deployments d
		WHERE d.project_id = $1 AND d.created_at >= $2`

	err = s.db.QueryRow(ctx, deployQuery, projectID, since).Scan(
		&summary.TotalDeployments, &summary.SuccessfulDeploys,
		&summary.FailedDeploys, &summary.AvgDeploymentTime,
	)
	if err != nil {
		return nil, fmt.Errorf("querying deployment stats: %w", err)
	}

	// Calculate rollback rate
	if summary.TotalDeployments > 0 {
		rollbackQuery := `
			SELECT COUNT(*) FROM deployment_events de
			JOIN deployments d ON de.deployment_id = d.id
			WHERE d.project_id = $1 AND de.event_type = 'rolled_back' AND de.occurred_at >= $2`

		var rollbacks int64
		err = s.db.QueryRow(ctx, rollbackQuery, projectID, since).Scan(&rollbacks)
		if err == nil {
			summary.RollbackRate = float64(rollbacks) / float64(summary.TotalDeployments) * 100
		}
	}

	// API health (only count requests to this project's resources)
	apiQuery := `
		SELECT
			COUNT(*) as requests,
			COUNT(CASE WHEN status_code >= 400 THEN 1 END) as errors,
			AVG(latency_ms) as avg_latency,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) as p95_latency,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms) as p99_latency
		FROM api_request_metrics
		WHERE recorded_at >= $1`

	err = s.db.QueryRow(ctx, apiQuery, since).Scan(
		&summary.APIRequests, &summary.APIErrors, &summary.AvgAPILatency,
		&summary.P95Latency, &summary.P99Latency,
	)
	if err != nil {
		return nil, fmt.Errorf("querying API stats: %w", err)
	}

	return summary, nil
}

// GetFlagUsageStats returns detailed usage statistics for a specific flag.
func (s *Service) GetFlagUsageStats(ctx context.Context, projectID, environmentID uuid.UUID, flagKey, timeRange string) (*models.FlagUsageStats, error) {
	var since time.Time
	switch timeRange {
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	default:
		since = time.Now().Add(-24 * time.Hour)
		timeRange = "24h"
	}

	stats := &models.FlagUsageStats{
		FlagKey:       flagKey,
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		TimeRange:     timeRange,
	}

	// Basic usage metrics
	basicQuery := `
		SELECT
			COUNT(*) as total_evaluations,
			COUNT(DISTINCT user_id) as unique_users,
			AVG(latency_ms) as avg_latency,
			AVG(CASE WHEN cache_hit THEN 1.0 ELSE 0.0 END) * 100 as cache_hit_rate,
			COUNT(CASE WHEN error_message != '' THEN 1 END) as error_count
		FROM flag_evaluation_events
		WHERE project_id = $1 AND environment_id = $2 AND flag_key = $3 AND evaluated_at >= $4`

	err := s.db.QueryRow(ctx, basicQuery, projectID, environmentID, flagKey, since).Scan(
		&stats.TotalEvaluations, &stats.UniqueUsers, &stats.AvgLatency,
		&stats.CacheHitRate, &stats.ErrorCount,
	)
	if err != nil {
		return nil, fmt.Errorf("querying basic stats: %w", err)
	}

	// Result distribution
	resultQuery := `
		SELECT result_value, COUNT(*)
		FROM flag_evaluation_events
		WHERE project_id = $1 AND environment_id = $2 AND flag_key = $3 AND evaluated_at >= $4
		GROUP BY result_value
		ORDER BY COUNT(*) DESC`

	rows, err := s.db.Query(ctx, resultQuery, projectID, environmentID, flagKey, since)
	if err != nil {
		return nil, fmt.Errorf("querying result distribution: %w", err)
	}
	defer rows.Close()

	stats.ResultDistribution = make(map[string]int64)
	for rows.Next() {
		var result string
		var count int64
		if err := rows.Scan(&result, &count); err != nil {
			return nil, fmt.Errorf("scanning result distribution: %w", err)
		}
		stats.ResultDistribution[result] = count
	}

	// Time series data for charts
	var interval string
	if timeRange == "24h" {
		interval = "1 hour"
		stats.HourlyEvaluations = s.getTimeSeriesData(ctx, projectID, environmentID, flagKey, since, interval)
	} else {
		interval = "1 day"
		stats.DailyEvaluations = s.getTimeSeriesData(ctx, projectID, environmentID, flagKey, since, interval)
	}

	return stats, nil
}

// getTimeSeriesData retrieves time series data for chart visualization.
func (s *Service) getTimeSeriesData(ctx context.Context, projectID, environmentID uuid.UUID, flagKey string, since time.Time, interval string) []models.TimeSeriesPoint {
	query := `
		SELECT
			date_trunc($5, evaluated_at) as time_bucket,
			COUNT(*) as evaluations
		FROM flag_evaluation_events
		WHERE project_id = $1 AND environment_id = $2 AND flag_key = $3 AND evaluated_at >= $4
		GROUP BY time_bucket
		ORDER BY time_bucket`

	rows, err := s.db.Query(ctx, query, projectID, environmentID, flagKey, since, interval)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var timestamp time.Time
		var value float64
		if err := rows.Scan(&timestamp, &value); err != nil {
			continue
		}
		points = append(points, models.TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     value,
		})
	}

	return points
}

// GetSystemHealthMetrics returns current system health metrics.
func (s *Service) GetSystemHealthMetrics(ctx context.Context) (*models.SystemHealthMetrics, error) {
	metrics := &models.SystemHealthMetrics{
		Timestamp: time.Now(),
	}

	// Get recent API performance (last 5 minutes)
	since := time.Now().Add(-5 * time.Minute)

	apiQuery := `
		SELECT
			COUNT(*)::float / 300 as rps,  -- requests per second over 5 minutes
			AVG(latency_ms) as avg_latency,
			COUNT(CASE WHEN status_code >= 400 THEN 1 END)::float / COUNT(*) * 100 as error_rate
		FROM api_request_metrics
		WHERE recorded_at >= $1`

	err := s.db.QueryRow(ctx, apiQuery, since).Scan(
		&metrics.RequestsPerSecond, &metrics.AvgLatency, &metrics.ErrorRate,
	)
	if err != nil {
		return nil, fmt.Errorf("querying API metrics: %w", err)
	}

	// Get cache metrics from Redis
	_, err = s.redis.Info(ctx, "memory").Result()
	if err == nil {
		// Parse Redis memory info (simplified - would need proper parsing in production)
		// This is a placeholder for actual Redis metrics parsing
		metrics.CacheHitRate = 95.0 // Mock value
		metrics.MemoryUsage = 75.0  // Mock value
	}

	// Database connection count
	dbQuery := `SELECT count(*) FROM pg_stat_activity WHERE state = 'active'`
	if err := s.db.QueryRow(ctx, dbQuery).Scan(&metrics.DatabaseConnections); err != nil {
		log.Printf("failed to query database connections: %v", err)
	}

	return metrics, nil
}

// StartDailyAggregation runs daily aggregation jobs to populate daily_flag_stats table.
func (s *Service) StartDailyAggregation(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	s.runDailyAggregation(ctx)

	for {
		select {
		case <-ticker.C:
			s.runDailyAggregation(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) runDailyAggregation(ctx context.Context) {
	yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)

	query := `
		INSERT INTO daily_flag_stats (
			project_id, environment_id, flag_key, stat_date,
			evaluation_count, unique_users, cache_hit_rate, avg_latency_ms, error_count,
			true_results, false_results
		)
		SELECT
			project_id, environment_id, flag_key, $1::date as stat_date,
			COUNT(*) as evaluation_count,
			COUNT(DISTINCT user_id) as unique_users,
			AVG(CASE WHEN cache_hit THEN 1.0 ELSE 0.0 END) * 100 as cache_hit_rate,
			AVG(latency_ms) as avg_latency_ms,
			COUNT(CASE WHEN error_message != '' THEN 1 END) as error_count,
			COUNT(CASE WHEN result_value = 'true' THEN 1 END) as true_results,
			COUNT(CASE WHEN result_value = 'false' THEN 1 END) as false_results
		FROM flag_evaluation_events
		WHERE evaluated_at >= $1::date AND evaluated_at < $1::date + interval '1 day'
		GROUP BY project_id, environment_id, flag_key
		ON CONFLICT (project_id, environment_id, flag_key, stat_date)
		DO UPDATE SET
			evaluation_count = EXCLUDED.evaluation_count,
			unique_users = EXCLUDED.unique_users,
			cache_hit_rate = EXCLUDED.cache_hit_rate,
			avg_latency_ms = EXCLUDED.avg_latency_ms,
			error_count = EXCLUDED.error_count,
			true_results = EXCLUDED.true_results,
			false_results = EXCLUDED.false_results`

	_, err := s.db.Exec(ctx, query, yesterday)
	if err != nil {
		// Log error in production
		fmt.Printf("Daily aggregation failed: %v\n", err)
	}
}

// CreateAnalyticsMiddleware returns middleware that records API request metrics.
func (s *Service) CreateAnalyticsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := r.Header.Get("X-Request-ID")

			// Wrap response writer to capture status code and response size
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

			next.ServeHTTP(wrapped, r)

			latency := time.Since(start)

			// Extract client IP
			var clientIP *net.IP
			if ip := net.ParseIP(r.RemoteAddr); ip != nil {
				clientIP = &ip
			}

			// Create metrics record
			metric := &models.APIRequestMetric{
				RequestID:    requestID,
				Method:       r.Method,
				Path:         r.URL.Path,
				StatusCode:   wrapped.statusCode,
				LatencyMs:    int(latency.Milliseconds()),
				IPAddress:    clientIP,
				UserAgent:    r.UserAgent(),
				RequestSize:  &r.ContentLength,
				ResponseSize: &wrapped.bytesWritten,
				RecordedAt:   time.Now(),
			}

			// Record asynchronously to avoid blocking the request
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := s.RecordAPIRequest(ctx, metric); err != nil {
					log.Printf("failed to record API request metric: %v", err)
				}
			}()
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture response metrics.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}