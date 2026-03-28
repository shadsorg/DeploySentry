package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestSizeConfig holds configuration for request size limiting middleware.
type RequestSizeConfig struct {
	// MaxBodySize is the maximum allowed request body size in bytes.
	MaxBodySize int64 `json:"max_body_size"`
}

// DefaultRequestSizeConfig returns sensible defaults for request size limits.
func DefaultRequestSizeConfig() RequestSizeConfig {
	return RequestSizeConfig{
		MaxBodySize: 32 << 20, // 32MB
	}
}

// RequestSizeLimit returns middleware that limits the size of request bodies.
// Requests exceeding the limit are rejected with HTTP 413.
func RequestSizeLimit(config RequestSizeConfig) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if c.Request.ContentLength > config.MaxBodySize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
				"limit": config.MaxBodySize,
			})
			return
		}

		// Set max bytes reader to prevent reading beyond limit
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, config.MaxBodySize)

		c.Next()
	})
}