package analytics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/deploysentry/deploysentry/internal/auth"
)

// Handler provides HTTP endpoints for analytics data.
type Handler struct {
	service *Service
}

// NewHandler creates a new analytics handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers analytics endpoints with the router.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	analytics := router.Group("/analytics")

	// Summary endpoints
	analytics.GET("/summary", h.getAnalyticsSummary)
	analytics.GET("/health", h.getSystemHealth)

	// Flag analytics
	analytics.GET("/flags/stats", h.getFlagStats)
	analytics.GET("/flags/:key/usage", h.getFlagUsage)

	// Deployment analytics
	analytics.GET("/deployments/stats", h.getDeploymentStats)

	// Real-time metrics (Server-Sent Events)
	analytics.GET("/metrics/stream", h.streamMetrics)

	// Admin endpoints for analytics management
	admin := analytics.Group("/admin")
	admin.POST("/refresh", h.refreshAggregations)
	admin.GET("/export", h.exportAnalytics)
}

// getAnalyticsSummary returns high-level analytics summary.
// GET /api/v1/analytics/summary?project_id=<uuid>&environment_id=<uuid>&time_range=24h
func (h *Handler) getAnalyticsSummary(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	environmentID, err := uuid.Parse(c.Query("environment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
		return
	}

	timeRange := c.DefaultQuery("time_range", "24h")

	// Check permissions
	_, authenticated := auth.GetAuthInfo(c)
	if !authenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	summary, err := h.service.GetAnalyticsSummary(c.Request.Context(), projectID, environmentID, timeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get analytics summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// getFlagStats returns analytics for all flags in a project.
// GET /api/v1/analytics/flags/stats?project_id=<uuid>&environment_id=<uuid>&time_range=24h
func (h *Handler) getFlagStats(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	environmentID, err := uuid.Parse(c.Query("environment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
		return
	}

	timeRange := c.DefaultQuery("time_range", "24h")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	// Build query for top flags by evaluation count
	query := `
		SELECT
			flag_key,
			COUNT(*) as evaluations,
			COUNT(DISTINCT user_id) as unique_users,
			AVG(latency_ms) as avg_latency,
			AVG(CASE WHEN cache_hit THEN 1.0 ELSE 0.0 END) * 100 as cache_hit_rate,
			COUNT(CASE WHEN error_message != '' THEN 1 END) as errors
		FROM flag_evaluation_events
		WHERE project_id = $1 AND environment_id = $2 AND evaluated_at >= NOW() - $3::interval
		GROUP BY flag_key
		ORDER BY evaluations DESC
		LIMIT $4`

	rows, err := h.service.db.Query(c.Request.Context(), query, projectID, environmentID, timeRange, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query flag stats"})
		return
	}
	defer rows.Close()

	type FlagStat struct {
		FlagKey      string  `json:"flag_key"`
		Evaluations  int64   `json:"evaluations"`
		UniqueUsers  int64   `json:"unique_users"`
		AvgLatency   float64 `json:"avg_latency_ms"`
		CacheHitRate float64 `json:"cache_hit_rate"`
		ErrorCount   int64   `json:"error_count"`
	}

	var stats []FlagStat
	for rows.Next() {
		var stat FlagStat
		err := rows.Scan(&stat.FlagKey, &stat.Evaluations, &stat.UniqueUsers,
			&stat.AvgLatency, &stat.CacheHitRate, &stat.ErrorCount)
		if err != nil {
			continue
		}
		stats = append(stats, stat)
	}

	c.JSON(http.StatusOK, gin.H{
		"flag_stats": stats,
		"time_range": timeRange,
		"project_id": projectID,
		"environment_id": environmentID,
	})
}

// getFlagUsage returns detailed usage analytics for a specific flag.
// GET /api/v1/analytics/flags/:key/usage?project_id=<uuid>&environment_id=<uuid>&time_range=24h
func (h *Handler) getFlagUsage(c *gin.Context) {
	flagKey := c.Param("key")
	if flagKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "flag key is required"})
		return
	}

	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	environmentID, err := uuid.Parse(c.Query("environment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
		return
	}

	timeRange := c.DefaultQuery("time_range", "24h")

	stats, err := h.service.GetFlagUsageStats(c.Request.Context(), projectID, environmentID, flagKey, timeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get flag usage stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// getDeploymentStats returns deployment analytics.
// GET /api/v1/analytics/deployments/stats?project_id=<uuid>&environment_id=<uuid>&time_range=7d
func (h *Handler) getDeploymentStats(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	timeRange := c.DefaultQuery("time_range", "7d")

	// Get deployment analytics
	query := `
		SELECT
			strategy,
			COUNT(*) as total_deployments,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as successful,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed,
			AVG(EXTRACT(EPOCH FROM (completed_at - started_at)) / 60) as avg_duration_minutes
		FROM deployments
		WHERE project_id = $1 AND created_at >= NOW() - $2::interval
		GROUP BY strategy`

	rows, err := h.service.db.Query(c.Request.Context(), query, projectID, timeRange)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query deployment stats"})
		return
	}
	defer rows.Close()

	type DeploymentStrategyStats struct {
		Strategy     string  `json:"strategy"`
		Total        int64   `json:"total_deployments"`
		Successful   int64   `json:"successful_deployments"`
		Failed       int64   `json:"failed_deployments"`
		SuccessRate  float64 `json:"success_rate"`
		AvgDuration  float64 `json:"avg_duration_minutes"`
	}

	var stats []DeploymentStrategyStats
	for rows.Next() {
		var stat DeploymentStrategyStats
		err := rows.Scan(&stat.Strategy, &stat.Total, &stat.Successful, &stat.Failed, &stat.AvgDuration)
		if err != nil {
			continue
		}
		if stat.Total > 0 {
			stat.SuccessRate = float64(stat.Successful) / float64(stat.Total) * 100
		}
		stats = append(stats, stat)
	}

	c.JSON(http.StatusOK, gin.H{
		"deployment_stats": stats,
		"time_range": timeRange,
		"project_id": projectID,
	})
}

// getSystemHealth returns current system health metrics.
// GET /api/v1/analytics/health
func (h *Handler) getSystemHealth(c *gin.Context) {
	metrics, err := h.service.GetSystemHealthMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get system health metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// streamMetrics provides real-time metrics via Server-Sent Events.
// GET /api/v1/analytics/metrics/stream
func (h *Handler) streamMetrics(c *gin.Context) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Send metrics every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-ticker.C:
			metrics, err := h.service.GetSystemHealthMetrics(c.Request.Context())
			if err != nil {
				c.SSEvent("error", "Failed to get metrics")
				return
			}

			c.SSEvent("metrics", metrics)
			c.Writer.Flush()

		case <-clientGone:
			return
		}
	}
}

// refreshAggregations manually triggers aggregation jobs.
// POST /api/v1/analytics/admin/refresh
func (h *Handler) refreshAggregations(c *gin.Context) {
	// Check admin permissions
	_, authenticated := auth.GetAuthInfo(c)
	if !authenticated {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	// Trigger daily aggregation
	go h.service.runDailyAggregation(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"message": "Aggregation jobs triggered",
		"timestamp": time.Now(),
	})
}

// exportAnalytics exports analytics data for external analysis.
// GET /api/v1/analytics/admin/export?project_id=<uuid>&start_date=2024-01-01&end_date=2024-01-31&format=json
func (h *Handler) exportAnalytics(c *gin.Context) {
	// Check admin permissions
	_, authenticated := auth.GetAuthInfo(c)
	if !authenticated {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	startDate := c.Query("start_date") // Format: 2024-01-01
	endDate := c.Query("end_date")     // Format: 2024-01-31
	format := c.DefaultQuery("format", "json")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	// Export daily aggregated stats
	query := `
		SELECT project_id, environment_id, flag_key, stat_date,
			   evaluation_count, unique_users, cache_hit_rate, avg_latency_ms,
			   error_count, true_results, false_results
		FROM daily_flag_stats
		WHERE project_id = $1 AND stat_date >= $2::date AND stat_date <= $3::date
		ORDER BY stat_date, flag_key`

	rows, err := h.service.db.Query(c.Request.Context(), query, projectID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export data"})
		return
	}
	defer rows.Close()

	var exports []map[string]interface{}
	for rows.Next() {
		var record map[string]interface{} = make(map[string]interface{})
		var values []interface{} = make([]interface{}, 11)
		var pointers []interface{} = make([]interface{}, 11)

		for i := range values {
			pointers[i] = &values[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			continue
		}

		fields := []string{"project_id", "environment_id", "flag_key", "stat_date",
			"evaluation_count", "unique_users", "cache_hit_rate", "avg_latency_ms",
			"error_count", "true_results", "false_results"}

		for i, field := range fields {
			record[field] = values[i]
		}

		exports = append(exports, record)
	}

	switch format {
	case "csv":
		// Would implement CSV export here
		c.JSON(http.StatusNotImplemented, gin.H{"error": "CSV export not implemented yet"})
	default:
		c.JSON(http.StatusOK, gin.H{
			"data": exports,
			"count": len(exports),
			"project_id": projectID,
			"date_range": gin.H{
				"start": startDate,
				"end": endDate,
			},
		})
	}
}
