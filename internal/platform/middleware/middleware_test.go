package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// DefaultCORSConfig
// ---------------------------------------------------------------------------

func TestDefaultCORSConfig_AllowsAllOrigins(t *testing.T) {
	cfg := DefaultCORSConfig()
	assert.Equal(t, []string{"*"}, cfg.AllowedOrigins)
	assert.False(t, cfg.AllowCredentials)
	assert.NotEmpty(t, cfg.AllowedMethods)
	assert.NotEmpty(t, cfg.AllowedHeaders)
}

// ---------------------------------------------------------------------------
// ProductionCORSConfig
// ---------------------------------------------------------------------------

func TestProductionCORSConfig_SetsSpecificOrigins(t *testing.T) {
	origins := []string{"https://example.com", "https://app.example.com"}
	cfg := ProductionCORSConfig(origins)

	assert.Equal(t, origins, cfg.AllowedOrigins)
	assert.True(t, cfg.AllowCredentials)
	assert.NotEmpty(t, cfg.AllowedMethods)
	assert.NotEmpty(t, cfg.AllowedHeaders)
}

// ---------------------------------------------------------------------------
// CORS middleware
// ---------------------------------------------------------------------------

func TestCORS_NoOriginHeader_PassesThrough(t *testing.T) {
	router := gin.New()
	router.Use(CORS(DefaultCORSConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Deliberately no Origin header.
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightAllowedOrigin_Returns204WithCORSHeaders(t *testing.T) {
	cfg := ProductionCORSConfig([]string{"https://example.com"})
	router := gin.New()
	router.Use(CORS(cfg))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_PreflightDisallowedOrigin_Returns403(t *testing.T) {
	cfg := ProductionCORSConfig([]string{"https://example.com"})
	router := gin.New()
	router.Use(CORS(cfg))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCORS_RegularRequestAllowedOrigin_SetsAllowOriginHeader(t *testing.T) {
	cfg := ProductionCORSConfig([]string{"https://example.com"})
	router := gin.New()
	router.Use(CORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AllowAllWithoutCredentials_SetsWildcard(t *testing.T) {
	cfg := DefaultCORSConfig() // AllowedOrigins: ["*"], AllowCredentials: false
	router := gin.New()
	router.Use(CORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	// Wildcard without credentials should not set Vary.
	assert.Empty(t, w.Header().Get("Vary"))
}

func TestCORS_AllowAllWithCredentials_SetsSpecificOriginAndVary(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowCredentials = true // allowAll + credentials

	router := gin.New()
	router.Use(CORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://specific.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// When allowAll + credentials, should echo the specific origin, not "*".
	assert.Equal(t, "https://specific.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_SpecificOriginAllowed_SetsThatOrigin(t *testing.T) {
	cfg := ProductionCORSConfig([]string{"https://app.example.com", "https://admin.example.com"})
	router := gin.New()
	router.Use(CORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://admin.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
}

func TestCORS_SpecificOriginNotAllowed_NoCORSHeaders(t *testing.T) {
	cfg := ProductionCORSConfig([]string{"https://example.com"})
	router := gin.New()
	router.Use(CORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	router.ServeHTTP(w, req)

	// Non-preflight request with disallowed origin passes through without CORS headers.
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// ---------------------------------------------------------------------------
// DefaultRateLimitConfig
// ---------------------------------------------------------------------------

func TestDefaultRateLimitConfig_Returns100RequestsPerMinute(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	assert.Equal(t, 100, cfg.RequestsPerWindow)
	assert.Equal(t, "ratelimit:", cfg.KeyPrefix)
	assert.NotZero(t, cfg.Window)
}

// ---------------------------------------------------------------------------
// NewRateLimiter
// ---------------------------------------------------------------------------

func TestNewRateLimiter_SetsDefaultKeyPrefixIfEmpty(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	cfg := RateLimitConfig{
		RequestsPerWindow: 50,
		KeyPrefix:         "", // empty - should be overridden
	}

	rl := NewRateLimiter(client, cfg)
	assert.Equal(t, "ratelimit:", rl.config.KeyPrefix)
}

func TestNewRateLimiter_PreservesCustomKeyPrefix(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	cfg := RateLimitConfig{
		RequestsPerWindow: 50,
		KeyPrefix:         "custom:",
	}

	rl := NewRateLimiter(client, cfg)
	assert.Equal(t, "custom:", rl.config.KeyPrefix)
}

// ---------------------------------------------------------------------------
// keyFor
// ---------------------------------------------------------------------------

func TestKeyFor_WithAPIKeyID_ReturnsKeyPrefix(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("api_key_id", "ak-123")

	key := rl.keyFor(c)
	assert.Equal(t, "ratelimit:key:ak-123", key)
}

func TestKeyFor_WithUserID_ReturnsUserPrefix(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user_id", "usr-456")

	key := rl.keyFor(c)
	assert.Equal(t, "ratelimit:user:usr-456", key)
}

func TestKeyFor_WithoutKeyOrUser_ReturnsIPPrefix(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.168.1.100:12345"

	key := rl.keyFor(c)
	assert.Equal(t, "ratelimit:ip:192.168.1.100", key)
}

func TestKeyFor_APIKeyTakesPrecedenceOverUserID(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("api_key_id", "ak-789")
	c.Set("user_id", "usr-789")

	key := rl.keyFor(c)
	// api_key_id should take precedence over user_id.
	assert.Equal(t, "ratelimit:key:ak-789", key)
}

// ---------------------------------------------------------------------------
// RateLimitByKey
// ---------------------------------------------------------------------------

func TestRateLimitByKey_CreatesValidMiddleware(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	keyFn := func(c *gin.Context) string {
		return "custom-key"
	}

	mw := RateLimitByKey(rl, keyFn)
	assert.NotNil(t, mw)
}

func TestRateLimiter_Middleware_CreatesValidHandlerFunc(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())
	mw := rl.Middleware()
	assert.NotNil(t, mw)
}

func TestRateLimiter_Middleware_AllowsOnRedisError(t *testing.T) {
	// Use a Redis client with a bad address so pipeline fails.
	client := redis.NewClient(&redis.Options{Addr: "localhost:1"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	// On Redis error, the middleware should allow the request through.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitByKey_AllowsOnRedisError(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:1"})
	defer client.Close()

	rl := NewRateLimiter(client, DefaultRateLimitConfig())

	router := gin.New()
	router.Use(RateLimitByKey(rl, func(c *gin.Context) string {
		return "test-key"
	}))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCORS_ExposedHeaders_Set(t *testing.T) {
	cfg := DefaultCORSConfig()
	router := gin.New()
	router.Use(CORS(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_Preflight_AllowAll(t *testing.T) {
	cfg := DefaultCORSConfig()
	router := gin.New()
	router.Use(CORS(cfg))
	router.OPTIONS("/test", func(c *gin.Context) {})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://any.com")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}
