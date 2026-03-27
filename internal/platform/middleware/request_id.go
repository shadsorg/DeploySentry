package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the HTTP header used to pass request IDs.
	RequestIDHeader = "X-Request-ID"

	// RequestIDContextKey is the context key used to store request IDs.
	RequestIDContextKey = "request_id"
)

// RequestID returns middleware that adds a unique request ID to each request.
// The request ID is either extracted from the X-Request-ID header (if present)
// or generated as a new UUID. It's stored in the request context and included
// in the response headers for request tracing.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID was provided in the header
		requestID := c.GetHeader(RequestIDHeader)

		// Generate a new request ID if none was provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for use by handlers and loggers
		c.Set(RequestIDContextKey, requestID)

		// Include in response headers for client-side tracing
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID extracts the request ID from the Gin context.
// Returns an empty string if no request ID is found.
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDContextKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}