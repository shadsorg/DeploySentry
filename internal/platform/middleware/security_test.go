package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Define a simple test handler
	handler := func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	}

	// Create a new router with the security middleware
	r := gin.New()
	config := DefaultSecurityConfig()
	r.Use(SecurityHeaders(config))
	r.GET("/test", handler)

	// Create a test request
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Perform the request
	r.ServeHTTP(w, req)

	// Verify the headers
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header is missing")
	}

	if contains(csp, "'unsafe-inline'") {
		t.Errorf("Content-Security-Policy should NOT contain 'unsafe-inline', got %q", csp)
	}

	expectedCSP := "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; font-src 'self' https:; connect-src 'self'; frame-ancestors 'none'"
	if csp != expectedCSP {
		t.Errorf("expected CSP to be %q, got %q", expectedCSP, csp)
	}

	// Verify other security headers
	expectedHeaders := map[string]string{
		"Referrer-Policy":           config.ReferrerPolicy,
		"X-Frame-Options":          config.FrameOptions,
		"X-Content-Type-Options":    "nosniff",
		"X-XSS-Protection":         "1; mode=block",
		"X-DNS-Prefetch-Control":    "off",
	}

	for header, expectedValue := range expectedHeaders {
		if got := w.Header().Get(header); got != expectedValue {
			t.Errorf("expected header %s to be %q, got %q", header, expectedValue, got)
		}
	}
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	config := DefaultSecurityConfig()
	r.Use(SecurityHeaders(config))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Test without TLS (HSTS should NOT be present)
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if hsts := w.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("Strict-Transport-Security should not be set for non-TLS requests, got %q", hsts)
	}

	// Test with TLS (HSTS SHOULD be present)
	reqTLS, _ := http.NewRequest("GET", "/test", nil)
	reqTLS.TLS = &tls.ConnectionState{} // Simulate TLS
	wTLS := httptest.NewRecorder()
	r.ServeHTTP(wTLS, reqTLS)
	if hsts := wTLS.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Error("Strict-Transport-Security should be set for TLS requests")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
