package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds configuration for Cross-Origin Resource Sharing middleware.
type CORSConfig struct {
	// AllowedOrigins is the list of origins permitted to make cross-origin requests.
	// Use ["*"] to allow all origins (not recommended for production).
	AllowedOrigins []string `json:"allowed_origins"`

	// AllowedMethods is the list of HTTP methods permitted in cross-origin requests.
	AllowedMethods []string `json:"allowed_methods"`

	// AllowedHeaders is the list of headers permitted in cross-origin requests.
	AllowedHeaders []string `json:"allowed_headers"`

	// ExposedHeaders is the list of headers that browsers are allowed to access.
	ExposedHeaders []string `json:"exposed_headers"`

	// AllowCredentials indicates whether credentials (cookies, auth headers) are
	// allowed in cross-origin requests.
	AllowCredentials bool `json:"allow_credentials"`

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached.
	MaxAge time.Duration `json:"max_age"`
}

// DefaultCORSConfig returns a CORS configuration suitable for development.
// For production, configure specific allowed origins.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Requested-With",
			"X-Request-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Window",
		},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

// ProductionCORSConfig returns a CORS configuration suitable for production
// with the given allowed origins.
func ProductionCORSConfig(origins []string) CORSConfig {
	config := DefaultCORSConfig()
	config.AllowedOrigins = origins
	config.AllowCredentials = true
	return config
}

// CORS returns a Gin middleware that handles Cross-Origin Resource Sharing.
func CORS(config CORSConfig) gin.HandlerFunc {
	allowedOrigins := make(map[string]bool, len(config.AllowedOrigins))
	allowAll := false
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			allowAll = true
			break
		}
		allowedOrigins[origin] = true
	}

	methodsStr := strings.Join(config.AllowedMethods, ", ")
	headersStr := strings.Join(config.AllowedHeaders, ", ")
	exposedStr := strings.Join(config.ExposedHeaders, ", ")
	maxAgeStr := strconv.Itoa(int(config.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// Check if the origin is allowed.
		if !allowAll && !allowedOrigins[origin] {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		// Set the allowed origin.
		if allowAll && !config.AllowCredentials {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if exposedStr != "" {
			c.Header("Access-Control-Expose-Headers", exposedStr)
		}

		// Handle preflight requests.
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", methodsStr)
			c.Header("Access-Control-Allow-Headers", headersStr)
			c.Header("Access-Control-Max-Age", maxAgeStr)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
