package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Context keys for authentication information
const (
	ContextKeyOrgID   = "org_id"
	ContextKeyUserID  = "user_id"
	ContextKeyProjectID = "project_id"
)

// AuthInfo contains the authentication context information
type AuthInfo struct {
	OrgID     string
	UserID    string
	ProjectID string
	Email     string
	Method    string
}

// GetAuthInfo extracts authentication information from the Gin context
func GetAuthInfo(c *gin.Context) (*AuthInfo, bool) {
	info := &AuthInfo{}

	if orgID, exists := c.Get(ContextKeyOrgID); exists {
		if orgIDStr, ok := orgID.(string); ok {
			info.OrgID = orgIDStr
		}
	}

	if userID, exists := c.Get(ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok {
			info.UserID = userIDStr
		}
	}

	if projectID, exists := c.Get(ContextKeyProjectID); exists {
		if projectIDStr, ok := projectID.(string); ok {
			info.ProjectID = projectIDStr
		}
	}

	if email, exists := c.Get("email"); exists {
		if emailStr, ok := email.(string); ok {
			info.Email = emailStr
		}
	}

	if method, exists := c.Get("auth_method"); exists {
		if methodStr, ok := method.(string); ok {
			info.Method = methodStr
		}
	}

	// At minimum we need either org_id or user_id to consider authenticated
	return info, info.OrgID != "" || info.UserID != ""
}

// APIKeyValidator defines the interface for validating API keys.
type APIKeyValidator interface {
	// ValidateAPIKey checks the key and returns the associated org/project info.
	ValidateAPIKey(ctx context.Context, key string) (*APIKeyInfo, error)
}

// APIKeyInfo holds the identity information extracted from a validated API key.
type APIKeyInfo struct {
	OrgID          *uuid.UUID  `json:"org_id,omitempty"`
	ProjectID      *uuid.UUID  `json:"project_id,omitempty"`
	ApplicationID  *uuid.UUID  `json:"application_id,omitempty"`
	EnvironmentIDs []uuid.UUID `json:"environment_ids,omitempty"`
	Scopes         []string    `json:"scopes"`
	AllowedCIDRs   []string    `json:"allowed_cidrs,omitempty"`
}

// EnvironmentSlugResolver resolves an environment slug to its UUID within an org.
type EnvironmentSlugResolver interface {
	ResolveEnvironmentSlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error)
}

// AuthMiddleware provides Gin middleware for authenticating requests via
// JWT bearer tokens or API keys.
type AuthMiddleware struct {
	jwtSecret    []byte
	keyValidator APIKeyValidator
	envResolver  EnvironmentSlugResolver
}

// NewAuthMiddleware creates a new AuthMiddleware with the given JWT secret
// and optional API key validator.
func NewAuthMiddleware(jwtSecret string, keyValidator APIKeyValidator, envResolver EnvironmentSlugResolver) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:    []byte(jwtSecret),
		keyValidator: keyValidator,
		envResolver:  envResolver,
	}
}

// RequireAuth returns a Gin middleware that requires a valid JWT or API key
// in the Authorization header. On success, it sets "user_id", "email",
// and "auth_method" on the Gin context.
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Fallback: browser EventSource cannot set custom headers, so
		// browser-based SDKs pass the API key as a ?token= query param
		// on SSE connections. Accept it as an ApiKey credential.
		if authHeader == "" {
			if token := c.Query("token"); token != "" {
				authHeader = "ApiKey " + token
			}
		}

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		// Try Bearer token (JWT) first.
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if m.authenticateJWT(c, tokenStr) {
				c.Next()
				return
			}
			return
		}

		// Try API key.
		if strings.HasPrefix(authHeader, "ApiKey ") {
			apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
			if m.authenticateAPIKey(c, apiKey) {
				c.Next()
				return
			}
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unsupported authorization scheme"})
	}
}

// RequireScope returns a Gin middleware that checks the authenticated
// request has the required scope. Must be used after RequireAuth.
func (m *AuthMiddleware) RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		method, _ := c.Get("auth_method")
		if method == "api_key" {
			scopes, exists := c.Get("api_key_scopes")
			if !exists {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
				return
			}
			scopeSlice, ok := scopes.([]string)
			if !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
				return
			}
			for _, s := range scopeSlice {
				if s == "admin" || s == scope {
					c.Next()
					return
				}
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient scope"})
			return
		}

		// JWT-authenticated users pass scope checks (RBAC handles fine-grained permissions).
		c.Next()
	}
}

// authenticateJWT validates a JWT token and sets context values.
func (m *AuthMiddleware) authenticateJWT(c *gin.Context, tokenStr string) bool {
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return false
	}

	c.Set(ContextKeyUserID, claims.UserID)
	c.Set("email", claims.Email)
	c.Set("auth_method", "jwt")
	if claims.OrgID != "" {
		c.Set(ContextKeyOrgID, claims.OrgID)
	}

	return true
}

// authenticateAPIKey validates an API key and sets context values.
func (m *AuthMiddleware) authenticateAPIKey(c *gin.Context, key string) bool {
	if m.keyValidator == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key authentication not configured"})
		return false
	}

	info, err := m.keyValidator.ValidateAPIKey(c.Request.Context(), key)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
		return false
	}

	// Enforce IP allowlist if the key has allowed CIDRs configured.
	if len(info.AllowedCIDRs) > 0 {
		clientIP := c.ClientIP()
		if !CheckIPAllowed(clientIP, info.AllowedCIDRs) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "request IP not in API key allowlist"})
			return false
		}
	}

	if info.OrgID != nil {
		c.Set("org_id", info.OrgID.String())
	}
	if info.ProjectID != nil {
		c.Set("project_id", info.ProjectID.String())
	}
	if info.ApplicationID != nil {
		c.Set("api_key_app_id", info.ApplicationID.String())
	}
	if len(info.EnvironmentIDs) > 0 {
		envStrs := make([]string, len(info.EnvironmentIDs))
		for i, eid := range info.EnvironmentIDs {
			envStrs[i] = eid.String()
		}
		c.Set("api_key_environment_ids", envStrs)
	}
	c.Set("api_key_scopes", info.Scopes)
	c.Set("auth_method", "api_key")

	// Enforce environment restriction: if the key is scoped to specific
	// environments, reject requests targeting an environment not in the list.
	if len(info.EnvironmentIDs) > 0 {
		targetEnv := c.Query("environment_id")
		if targetEnv == "" {
			if eid, exists := c.Get("environment_id"); exists {
				if s, ok := eid.(string); ok {
					targetEnv = s
				}
			}
		}
		if targetEnv != "" {
			// Resolve slug to UUID if needed.
			targetID, parseErr := uuid.Parse(targetEnv)
			if parseErr != nil && m.envResolver != nil && info.OrgID != nil {
				resolved, resolveErr := m.envResolver.ResolveEnvironmentSlug(c.Request.Context(), *info.OrgID, targetEnv)
				if resolveErr == nil {
					targetID = resolved
				}
			}
			allowed := false
			for _, eid := range info.EnvironmentIDs {
				if eid == targetID {
					allowed = true
					break
				}
			}
			if !allowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "api key is not authorized for this environment",
				})
				return false
			}
		}
	}

	return true
}
