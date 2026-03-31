package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/deploysentry/deploysentry/internal/platform/gelf"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ErrorHandlingConfig holds configuration for error handling middleware.
type ErrorHandlingConfig struct {
	// IncludeStackTrace indicates whether to include stack traces in error logs.
	IncludeStackTrace bool `json:"include_stack_trace"`

	// ExposeInternalErrors indicates whether to expose internal error details in responses.
	// Should be false in production for security.
	ExposeInternalErrors bool `json:"expose_internal_errors"`
}

// DefaultErrorHandlingConfig returns production-safe defaults for error handling.
func DefaultErrorHandlingConfig() ErrorHandlingConfig {
	return ErrorHandlingConfig{
		IncludeStackTrace:    true,  // Useful for debugging
		ExposeInternalErrors: false, // Security: don't expose internal errors
	}
}

// DevelopmentErrorHandlingConfig returns development-friendly error handling config.
func DevelopmentErrorHandlingConfig() ErrorHandlingConfig {
	return ErrorHandlingConfig{
		IncludeStackTrace:    true,
		ExposeInternalErrors: true, // OK for development
	}
}

// ErrorHandler returns middleware that provides centralized error handling.
// It catches panics, logs errors with context, and returns consistent error responses.
func ErrorHandler(config ErrorHandlingConfig, gelfClient *gelf.Client) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(c)

				// Log the panic with full context
				logEntry := fmt.Sprintf(
					"PANIC [%s] %s %s: %v",
					requestID,
					c.Request.Method,
					c.Request.URL.Path,
					err,
				)

				if config.IncludeStackTrace {
					logEntry += fmt.Sprintf("\nstack_trace:\n%s", debug.Stack())
				}

				log.Println(logEntry)

				// Send GELF fatal log for panics
				if gelfClient != nil {
					userID := ""
					if uid, exists := c.Get("user_id"); exists {
						if id, ok := uid.(uuid.UUID); ok {
							userID = id.String()
						}
					}
					gelfClient.Fatal(
						fmt.Sprintf("PANIC: %v", err),
						fmt.Sprintf("%v", err),
						string(debug.Stack()),
						requestID,
						c.Request.Method,
						c.Request.URL.Path,
						userID,
					)
				}

				// Return error response
				response := gin.H{
					"error":      "internal server error",
					"request_id": requestID,
				}

				if config.ExposeInternalErrors {
					response["details"] = fmt.Sprintf("%v", err)
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			}
		}()

		// Process the request
		c.Next()

		// Handle any errors that occurred during processing
		if len(c.Errors) > 0 {
			requestID := GetRequestID(c)

			for _, err := range c.Errors {
				logEntry := fmt.Sprintf(
					"ERROR [%s] %s %s: %v",
					requestID,
					c.Request.Method,
					c.Request.URL.Path,
					err.Err,
				)

				if err.Meta != nil {
					logEntry += fmt.Sprintf(" meta=%+v", err.Meta)
				}

				log.Println(logEntry)
			}

			// Send GELF error log for request errors
			if gelfClient != nil {
				lastErr := c.Errors.Last()
				userID := ""
				if uid, exists := c.Get("user_id"); exists {
					if id, ok := uid.(uuid.UUID); ok {
						userID = id.String()
					}
				}
				gelfClient.ErrorWithContext(
					fmt.Sprintf("ERROR: %v", lastErr.Err),
					fmt.Sprintf("%v", lastErr.Err),
					"",
					requestID,
					c.Request.Method,
					c.Request.URL.Path,
					userID,
				)
			}

			// Return error response if not already responded
			if !c.Writer.Written() {
				lastError := c.Errors.Last()

				response := gin.H{
					"error":      "request processing failed",
					"request_id": requestID,
				}

				if config.ExposeInternalErrors {
					response["details"] = lastError.Error()
				}

				// Determine appropriate status code
				status := http.StatusInternalServerError
				switch lastError.Type {
				case gin.ErrorTypeBind:
					status = http.StatusBadRequest
				case gin.ErrorTypePublic:
					status = http.StatusBadRequest
				}

				c.AbortWithStatusJSON(status, response)
			}
		}
	})
}