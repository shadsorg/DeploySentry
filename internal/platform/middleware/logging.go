package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/shadsorg/deploysentry/internal/platform/gelf"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LoggingConfig holds configuration for the logging middleware.
type LoggingConfig struct {
	// SkipPaths is a list of URL paths to skip logging for.
	SkipPaths []string `json:"skip_paths"`

	// LogRequestBody indicates whether to log request bodies (be careful with sensitive data).
	LogRequestBody bool `json:"log_request_body"`

	// LogResponseBody indicates whether to log response bodies.
	LogResponseBody bool `json:"log_response_body"`

	// MaxBodyLogSize is the maximum number of bytes of request/response body to log.
	MaxBodyLogSize int64 `json:"max_body_log_size"`

	// LogLevel controls verbosity; when "trace", skip-paths filtering is disabled.
	LogLevel string `json:"log_level"`
}

// DefaultLoggingConfig returns sensible defaults for request logging.
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		SkipPaths: []string{"/health", "/ready", "/metrics"},
		LogRequestBody: false,  // Disabled by default for security
		LogResponseBody: false, // Disabled by default to reduce noise
		MaxBodyLogSize: 1024,   // 1KB max
	}
}

// StructuredLogger returns middleware that provides structured request/response logging.
func StructuredLogger(config LoggingConfig, gelfClient *gelf.Client) gin.HandlerFunc {
	skipPaths := make(map[string]bool, len(config.SkipPaths))
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	return gin.HandlerFunc(func(c *gin.Context) {
		// Skip logging for certain paths (unless trace-level logging is enabled)
		if config.LogLevel != "trace" && skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		start := time.Now()
		requestID := GetRequestID(c)

		// Read and replace request body if logging is enabled
		var requestBody string
		if config.LogRequestBody && c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, config.MaxBodyLogSize))
			if err == nil {
				requestBody = string(bodyBytes)
				// Replace the body for downstream handlers
				c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		// Capture response body if logging is enabled
		var responseBody bytes.Buffer
		if config.LogResponseBody {
			writer := &responseWriter{
				ResponseWriter: c.Writer,
				body: &responseBody,
				maxSize: config.MaxBodyLogSize,
			}
			c.Writer = writer
		}

		// Send GELF request log
		method := c.Request.Method
		path := c.Request.URL.Path
		if gelfClient != nil {
			gelfClient.Request(requestID, method, path, "")
		}

		// Process the request
		c.Next()

		// Log the completed request
		duration := time.Since(start)
		status := c.Writer.Status()

		// Send GELF response log
		if gelfClient != nil {
			userID := ""
			if uid, exists := c.Get("user_id"); exists {
				if id, ok := uid.(uuid.UUID); ok {
					userID = id.String()
				}
			}
			gelfClient.Response(requestID, method, path, userID, status, int64(duration.Milliseconds()))
		}

		logEntry := fmt.Sprintf(
			"[%s] %s %s %d %s",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			status,
			duration,
		)

		if c.Request.Header.Get("User-Agent") != "" {
			logEntry += fmt.Sprintf(" user_agent=%q", c.Request.Header.Get("User-Agent"))
		}

		if clientIP := c.ClientIP(); clientIP != "" {
			logEntry += fmt.Sprintf(" ip=%s", clientIP)
		}

		if requestBody != "" {
			logEntry += fmt.Sprintf(" request_body=%q", requestBody)
		}

		if config.LogResponseBody && responseBody.Len() > 0 {
			logEntry += fmt.Sprintf(" response_body=%q", responseBody.String())
		}

		// Add error information if request failed
		if len(c.Errors) > 0 {
			logEntry += fmt.Sprintf(" errors=%q", c.Errors.String())
		}

		log.Println(logEntry)
	})
}

// responseWriter wraps gin.ResponseWriter to capture response body.
type responseWriter struct {
	gin.ResponseWriter
	body    *bytes.Buffer
	maxSize int64
}

func (w *responseWriter) Write(data []byte) (int, error) {
	// Capture response body up to max size
	if w.body.Len() < int(w.maxSize) {
		remaining := w.maxSize - int64(w.body.Len())
		if int64(len(data)) > remaining {
			w.body.Write(data[:remaining])
		} else {
			w.body.Write(data)
		}
	}

	// Write to actual response
	return w.ResponseWriter.Write(data)
}