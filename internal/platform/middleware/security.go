package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityConfig holds configuration for security headers.
type SecurityConfig struct {
	// ContentSecurityPolicy sets the CSP header to prevent XSS attacks.
	ContentSecurityPolicy string `json:"content_security_policy"`

	// ReferrerPolicy controls how much referrer information is included with requests.
	ReferrerPolicy string `json:"referrer_policy"`

	// FrameOptions prevents clickjacking attacks via iframe embedding.
	FrameOptions string `json:"frame_options"`

	// ContentTypeOptions prevents MIME type confusion attacks.
	ContentTypeOptions bool `json:"content_type_options"`

	// StrictTransportSecurity enables HSTS for HTTPS enforcement.
	StrictTransportSecurity string `json:"strict_transport_security"`

	// PermittedCrossDomainPolicies controls Flash/PDF cross-domain access.
	PermittedCrossDomainPolicies string `json:"permitted_cross_domain_policies"`
}

// DefaultSecurityConfig returns production-ready security header configuration.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; font-src 'self' https:; connect-src 'self'; frame-ancestors 'none'",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		FrameOptions:          "DENY",
		ContentTypeOptions:    true,
		StrictTransportSecurity: "max-age=31536000; includeSubDomains; preload",
		PermittedCrossDomainPolicies: "none",
	}
}

// SecurityHeaders returns middleware that adds security headers to all responses.
// These headers provide defense-in-depth against common web vulnerabilities.
func SecurityHeaders(config SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent XSS attacks
		if config.ContentSecurityPolicy != "" {
			c.Header("Content-Security-Policy", config.ContentSecurityPolicy)
		}

		// Control referrer information
		if config.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", config.ReferrerPolicy)
		}

		// Prevent clickjacking
		if config.FrameOptions != "" {
			c.Header("X-Frame-Options", config.FrameOptions)
		}

		// Prevent MIME type confusion
		if config.ContentTypeOptions {
			c.Header("X-Content-Type-Options", "nosniff")
		}

		// Enforce HTTPS (only add if request came over HTTPS)
		if c.Request.TLS != nil && config.StrictTransportSecurity != "" {
			c.Header("Strict-Transport-Security", config.StrictTransportSecurity)
		}

		// Control Flash/PDF cross-domain access
		if config.PermittedCrossDomainPolicies != "" {
			c.Header("X-Permitted-Cross-Domain-Policies", config.PermittedCrossDomainPolicies)
		}

		// Additional security headers
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("X-DNS-Prefetch-Control", "off")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		c.Next()
	}
}