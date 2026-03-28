package models

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// FlagEvaluationEvent records details about individual flag evaluations
// for usage analytics and debugging.
type FlagEvaluationEvent struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	ProjectID     uuid.UUID              `json:"project_id" db:"project_id"`
	EnvironmentID uuid.UUID              `json:"environment_id" db:"environment_id"`
	FlagKey       string                 `json:"flag_key" db:"flag_key"`
	UserID        string                 `json:"user_id,omitempty" db:"user_id"`
	SDKVersion    string                 `json:"sdk_version,omitempty" db:"sdk_version"`
	ResultValue   string                 `json:"result_value" db:"result_value"`
	RuleID        *uuid.UUID             `json:"rule_id,omitempty" db:"rule_id"`
	LatencyMs     int                    `json:"latency_ms" db:"latency_ms"`
	CacheHit      bool                   `json:"cache_hit" db:"cache_hit"`
	ErrorMessage  string                 `json:"error_message,omitempty" db:"error_message"`
	IPAddress     *net.IP                `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent     string                 `json:"user_agent,omitempty" db:"user_agent"`
	ContextAttrs  map[string]interface{} `json:"context_attrs" db:"context_attrs"`
	EvaluatedAt   time.Time              `json:"evaluated_at" db:"evaluated_at"`
}

// DeploymentEventType represents the type of deployment event.
type DeploymentEventType string

const (
	DeploymentEventCreated        DeploymentEventType = "created"
	DeploymentEventStarted        DeploymentEventType = "started"
	DeploymentEventPhaseCompleted DeploymentEventType = "phase_completed"
	DeploymentEventPromoted       DeploymentEventType = "promoted"
	DeploymentEventPaused         DeploymentEventType = "paused"
	DeploymentEventResumed        DeploymentEventType = "resumed"
	DeploymentEventCompleted      DeploymentEventType = "completed"
	DeploymentEventFailed         DeploymentEventType = "failed"
	DeploymentEventRolledBack     DeploymentEventType = "rolled_back"
	DeploymentEventCancelled      DeploymentEventType = "cancelled"
)

// DeploymentEvent records state changes and metrics for deployment analytics.
type DeploymentEvent struct {
	ID           uuid.UUID               `json:"id" db:"id"`
	DeploymentID uuid.UUID               `json:"deployment_id" db:"deployment_id"`
	EventType    DeploymentEventType     `json:"event_type" db:"event_type"`
	PhaseName    string                  `json:"phase_name,omitempty" db:"phase_name"`
	TrafficPct   int                     `json:"traffic_pct,omitempty" db:"traffic_pct"`
	HealthScore  *float64                `json:"health_score,omitempty" db:"health_score"`
	ErrorMessage string                  `json:"error_message,omitempty" db:"error_message"`
	TriggeredBy  *uuid.UUID              `json:"triggered_by,omitempty" db:"triggered_by"`
	Metadata     map[string]interface{}  `json:"metadata" db:"metadata"`
	OccurredAt   time.Time               `json:"occurred_at" db:"occurred_at"`
}

// APIRequestMetric records HTTP request details for API health analytics.
type APIRequestMetric struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	RequestID    string     `json:"request_id" db:"request_id"`
	Method       string     `json:"method" db:"method"`
	Path         string     `json:"path" db:"path"`
	StatusCode   int        `json:"status_code" db:"status_code"`
	LatencyMs    int        `json:"latency_ms" db:"latency_ms"`
	UserID       *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	APIKeyID     *uuid.UUID `json:"api_key_id,omitempty" db:"api_key_id"`
	IPAddress    *net.IP    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    string     `json:"user_agent,omitempty" db:"user_agent"`
	ErrorMessage string     `json:"error_message,omitempty" db:"error_message"`
	RequestSize  *int64     `json:"request_size,omitempty" db:"request_size"`
	ResponseSize *int64     `json:"response_size,omitempty" db:"response_size"`
	RecordedAt   time.Time  `json:"recorded_at" db:"recorded_at"`
}

// DailyFlagStats contains pre-aggregated daily statistics for efficient dashboard queries.
type DailyFlagStats struct {
	ID              uuid.UUID `json:"id" db:"id"`
	ProjectID       uuid.UUID `json:"project_id" db:"project_id"`
	EnvironmentID   uuid.UUID `json:"environment_id" db:"environment_id"`
	FlagKey         string    `json:"flag_key" db:"flag_key"`
	StatDate        time.Time `json:"stat_date" db:"stat_date"`
	EvaluationCount int64     `json:"evaluation_count" db:"evaluation_count"`
	UniqueUsers     int64     `json:"unique_users" db:"unique_users"`
	CacheHitRate    float64   `json:"cache_hit_rate" db:"cache_hit_rate"`
	AvgLatencyMs    float64   `json:"avg_latency_ms" db:"avg_latency_ms"`
	ErrorCount      int64     `json:"error_count" db:"error_count"`
	TrueResults     int64     `json:"true_results" db:"true_results"`
	FalseResults    int64     `json:"false_results" db:"false_results"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// AnalyticsSummary provides high-level metrics for dashboard overview.
type AnalyticsSummary struct {
	ProjectID       uuid.UUID `json:"project_id"`
	EnvironmentID   uuid.UUID `json:"environment_id"`
	TimeRange       string    `json:"time_range"` // "24h", "7d", "30d"

	// Flag Analytics
	TotalFlags           int64   `json:"total_flags"`
	ActiveFlags          int64   `json:"active_flags"`
	FlagEvaluations      int64   `json:"flag_evaluations"`
	UniqueUsers          int64   `json:"unique_users"`
	AvgEvaluationLatency float64 `json:"avg_evaluation_latency_ms"`
	CacheHitRate         float64 `json:"cache_hit_rate"`
	ErrorRate            float64 `json:"error_rate"`

	// Deployment Analytics
	TotalDeployments     int64   `json:"total_deployments"`
	SuccessfulDeploys    int64   `json:"successful_deployments"`
	FailedDeploys        int64   `json:"failed_deployments"`
	AvgDeploymentTime    float64 `json:"avg_deployment_time_minutes"`
	RollbackRate         float64 `json:"rollback_rate"`

	// API Health
	APIRequests          int64   `json:"api_requests"`
	APIErrors            int64   `json:"api_errors"`
	AvgAPILatency        float64 `json:"avg_api_latency_ms"`
	P95Latency           float64 `json:"p95_latency_ms"`
	P99Latency           float64 `json:"p99_latency_ms"`
}

// FlagUsageStats provides detailed usage statistics for a specific flag.
type FlagUsageStats struct {
	FlagKey         string               `json:"flag_key"`
	ProjectID       uuid.UUID            `json:"project_id"`
	EnvironmentID   uuid.UUID            `json:"environment_id"`
	TimeRange       string               `json:"time_range"`

	// Usage Metrics
	TotalEvaluations int64              `json:"total_evaluations"`
	UniqueUsers      int64              `json:"unique_users"`
	ResultDistribution map[string]int64 `json:"result_distribution"` // value -> count

	// Performance Metrics
	AvgLatency       float64            `json:"avg_latency_ms"`
	CacheHitRate     float64            `json:"cache_hit_rate"`
	ErrorCount       int64              `json:"error_count"`

	// Geographic Distribution
	CountryStats     map[string]int64   `json:"country_stats,omitempty"`

	// Time Series Data (for charts)
	HourlyEvaluations []TimeSeriesPoint `json:"hourly_evaluations"`
	DailyEvaluations  []TimeSeriesPoint `json:"daily_evaluations"`
}

// DeploymentAnalytics provides detailed analytics for deployment performance.
type DeploymentAnalytics struct {
	ProjectID     uuid.UUID `json:"project_id"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	TimeRange     string    `json:"time_range"`

	// Deployment Success Metrics
	TotalDeployments    int64   `json:"total_deployments"`
	SuccessRate         float64 `json:"success_rate"`
	RollbackRate        float64 `json:"rollback_rate"`
	AvgDeploymentTime   float64 `json:"avg_deployment_time_minutes"`

	// Deployment Strategy Performance
	StrategyStats       map[string]DeploymentStrategyStats `json:"strategy_stats"`

	// Health Score Trends
	AvgHealthScore      float64              `json:"avg_health_score"`
	HealthScoreTrend    []TimeSeriesPoint    `json:"health_score_trend"`

	// Time Series Data
	DailyDeployments    []TimeSeriesPoint    `json:"daily_deployments"`
	DeploymentDuration  []TimeSeriesPoint    `json:"deployment_duration"`
}

// DeploymentStrategyStats contains performance metrics by deployment strategy.
type DeploymentStrategyStats struct {
	Strategy        string  `json:"strategy"`
	Count           int64   `json:"count"`
	SuccessRate     float64 `json:"success_rate"`
	AvgDuration     float64 `json:"avg_duration_minutes"`
	RollbackRate    float64 `json:"rollback_rate"`
}

// TimeSeriesPoint represents a data point in time series analytics.
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"` // Optional label for the point
}

// SystemHealthMetrics provides real-time system health information.
type SystemHealthMetrics struct {
	Timestamp        time.Time `json:"timestamp"`

	// API Performance
	RequestsPerSecond float64   `json:"requests_per_second"`
	AvgLatency       float64   `json:"avg_latency_ms"`
	ErrorRate        float64   `json:"error_rate"`
	ActiveConnections int64    `json:"active_connections"`

	// Database Performance
	DatabaseConnections int64   `json:"database_connections"`
	QueryLatency       float64  `json:"query_latency_ms"`
	SlowQueries        int64    `json:"slow_queries"`

	// Cache Performance
	CacheHitRate       float64  `json:"cache_hit_rate"`
	CacheEvictions     int64    `json:"cache_evictions"`
	MemoryUsage        float64  `json:"memory_usage_percent"`

	// Resource Usage
	CPUUsage           float64  `json:"cpu_usage_percent"`
	MemoryUsageBytes   int64    `json:"memory_usage_bytes"`
	DiskUsage          float64  `json:"disk_usage_percent"`
}