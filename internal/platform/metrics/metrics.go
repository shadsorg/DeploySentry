// Package metrics provides Prometheus metrics collection for DeploySentry.
package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestDuration tracks HTTP request latency by method, path, and status.
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// RequestTotal counts total HTTP requests by method, path, and status.
	RequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// ActiveConnections tracks current number of active connections.
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_connections",
			Help: "Current number of active HTTP connections",
		},
	)

	// FlagEvaluations counts flag evaluation requests by project and flag key.
	FlagEvaluations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flag_evaluations_total",
			Help: "Total number of flag evaluations",
		},
		[]string{"project_id", "flag_key", "result"},
	)

	// DeploymentEvents counts deployment state changes.
	DeploymentEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "deployment_events_total",
			Help: "Total number of deployment events",
		},
		[]string{"project_id", "event_type", "strategy"},
	)

	// DatabaseConnections tracks database connection pool metrics.
	DatabaseConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections",
			Help: "Current database connections",
		},
		[]string{"state"},
	)

	// RedisOperations counts Redis operations by command and result.
	RedisOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_operations_total",
			Help: "Total number of Redis operations",
		},
		[]string{"command", "result"},
	)
)

// init registers all metrics with Prometheus.
func init() {
	prometheus.MustRegister(
		RequestDuration,
		RequestTotal,
		ActiveConnections,
		FlagEvaluations,
		DeploymentEvents,
		DatabaseConnections,
		RedisOperations,
	)
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// InstrumentHandler returns middleware that records HTTP metrics for each request.
func InstrumentHandler() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		ActiveConnections.Inc()
		defer ActiveConnections.Dec()

		timer := prometheus.NewTimer(RequestDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			"", // status will be set after request completes
		))
		defer func() {
			timer.ObserveDuration()
			RequestTotal.WithLabelValues(
				c.Request.Method,
				c.FullPath(),
				string(rune(c.Writer.Status())),
			).Inc()
		}()

		c.Next()
	})
}