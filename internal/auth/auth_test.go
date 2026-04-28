package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

type mockKeyValidator struct {
	info *APIKeyInfo
	err  error
}

func (m *mockKeyValidator) ValidateAPIKey(ctx context.Context, key string) (*APIKeyInfo, error) {
	return m.info, m.err
}

// helper: build a signed JWT with the provided secret and claims.
func buildJWT(t *testing.T, secret string, claims jwt.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	assert.NoError(t, err)
	return s
}

// helper: parse JSON body from the recorder.
func parseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	return body
}

// =========================================================================
// RBAC Tests (rbac.go)
// =========================================================================

func TestRBACChecker_HasPermission_Owner(t *testing.T) {
	rbac := NewRBACChecker()

	allPerms := []Permission{
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermOrgManage, PermAuditRead,
	}

	for _, perm := range allPerms {
		assert.True(t, rbac.HasPermission(RoleOwner, perm), "owner should have permission %s", perm)
	}
}

func TestRBACChecker_HasPermission_Admin(t *testing.T) {
	rbac := NewRBACChecker()

	// Admin should have these permissions.
	allowed := []Permission{
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
		PermProjectManage, PermAuditRead,
	}
	for _, perm := range allowed {
		assert.True(t, rbac.HasPermission(RoleAdmin, perm), "admin should have permission %s", perm)
	}

	// Admin must NOT have org:manage.
	assert.False(t, rbac.HasPermission(RoleAdmin, PermOrgManage), "admin should NOT have org:manage")
}

// TestRBACChecker_HasPermission_OwnerAdmin_APIKeyManage regression-tests the
// gap that was reported as "owner of org gets 403 when creating an API key".
// The legacy RoleOwner / RoleAdmin (the strings actually persisted in
// org_members.role per migration 031) must be able to manage API keys; the
// dead granular RoleOrgOwner / RoleOrgAdmin had this permission but are
// never produced by ResolveOrgRole, so the legacy roles must carry it too.
func TestRBACChecker_HasPermission_OwnerAdmin_APIKeyManage(t *testing.T) {
	rbac := NewRBACChecker()
	assert.True(t, rbac.HasPermission(RoleOwner, PermAPIKeyManage), "owner should have apikey:manage")
	assert.True(t, rbac.HasPermission(RoleAdmin, PermAPIKeyManage), "admin should have apikey:manage")
}

func TestRBACChecker_HasPermission_Developer(t *testing.T) {
	rbac := NewRBACChecker()

	allowed := []Permission{
		PermDeployCreate, PermDeployRead, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagRead, PermFlagUpdate, PermFlagToggle,
		PermReleaseCreate, PermReleaseRead, PermReleasePromote,
	}
	for _, perm := range allowed {
		assert.True(t, rbac.HasPermission(RoleDeveloper, perm), "developer should have permission %s", perm)
	}

	// Developer must NOT have these.
	denied := []Permission{PermProjectManage, PermOrgManage, PermAuditRead, PermFlagArchive}
	for _, perm := range denied {
		assert.False(t, rbac.HasPermission(RoleDeveloper, perm), "developer should NOT have permission %s", perm)
	}
}

func TestRBACChecker_HasPermission_Viewer(t *testing.T) {
	rbac := NewRBACChecker()

	// Viewer should only have read permissions.
	allowed := []Permission{PermDeployRead, PermFlagRead, PermReleaseRead}
	for _, perm := range allowed {
		assert.True(t, rbac.HasPermission(RoleViewer, perm), "viewer should have permission %s", perm)
	}

	// Viewer must NOT have create/update/manage permissions.
	denied := []Permission{
		PermDeployCreate, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagUpdate, PermFlagToggle, PermFlagArchive,
		PermReleaseCreate, PermReleasePromote,
		PermProjectManage, PermOrgManage, PermAuditRead,
	}
	for _, perm := range denied {
		assert.False(t, rbac.HasPermission(RoleViewer, perm), "viewer should NOT have permission %s", perm)
	}
}

func TestRBACChecker_HasPermission_InvalidRole(t *testing.T) {
	rbac := NewRBACChecker()
	assert.False(t, rbac.HasPermission(Role("nonexistent"), PermDeployRead))
	assert.False(t, rbac.HasPermission(Role(""), PermDeployRead))
}

// ---------------------------------------------------------------------------
// GetPermissions
// ---------------------------------------------------------------------------

func TestRBACChecker_GetPermissions_Owner(t *testing.T) {
	rbac := NewRBACChecker()
	perms := rbac.GetPermissions(RoleOwner)
	assert.Len(t, perms, 21)
	assert.Contains(t, perms, PermStatusWrite)
	assert.Contains(t, perms, PermAPIKeyManage)
}

func TestRBACChecker_GetPermissions_Viewer(t *testing.T) {
	rbac := NewRBACChecker()
	perms := rbac.GetPermissions(RoleViewer)
	assert.Len(t, perms, 4)
	assert.Contains(t, perms, PermDeployRead)
	assert.Contains(t, perms, PermFlagRead)
	assert.Contains(t, perms, PermReleaseRead)
}

func TestRBACChecker_GetPermissions_InvalidRole(t *testing.T) {
	rbac := NewRBACChecker()
	perms := rbac.GetPermissions(Role("bogus"))
	assert.Nil(t, perms)
}

func TestRBACChecker_GetPermissions_ReturnsCopy(t *testing.T) {
	rbac := NewRBACChecker()
	perms1 := rbac.GetPermissions(RoleOwner)
	perms2 := rbac.GetPermissions(RoleOwner)

	// Mutate the first slice and verify it does not affect the second.
	perms1[0] = Permission("mutated")
	assert.NotEqual(t, perms1[0], perms2[0], "GetPermissions should return a copy, not the original slice")
}

// ---------------------------------------------------------------------------
// RequirePermission middleware
// ---------------------------------------------------------------------------

func TestRequirePermission_HasPermission(t *testing.T) {
	rbac := NewRBACChecker()
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("role", RoleOwner)
		RequirePermission(rbac, PermDeployCreate)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_LacksPermission(t *testing.T) {
	rbac := NewRBACChecker()
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("role", RoleViewer)
		RequirePermission(rbac, PermDeployCreate)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "insufficient permissions", body["error"])
}

func TestRequirePermission_NoRoleInContext(t *testing.T) {
	rbac := NewRBACChecker()
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", RequirePermission(rbac, PermDeployCreate), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "role not determined", body["error"])
}

func TestRequirePermission_RoleAsString(t *testing.T) {
	rbac := NewRBACChecker()
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		// Set role as a plain string instead of Role type.
		ctx.Set("role", "owner")
		RequirePermission(rbac, PermDeployCreate)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_InvalidRoleFormat(t *testing.T) {
	rbac := NewRBACChecker()
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		// Set role as an integer -- neither Role nor string.
		ctx.Set("role", 12345)
		RequirePermission(rbac, PermDeployCreate)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "invalid role format", body["error"])
}

// =========================================================================
// Auth Middleware Tests (middleware.go)
// =========================================================================

const testJWTSecret = "test-jwt-secret-key-for-unit-tests"

func TestRequireAuth_NoAuthHeader(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "authorization header required", body["error"])
}

func TestRequireAuth_InvalidScheme(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "unsupported authorization scheme", body["error"])
}

func TestRequireAuth_ValidJWT(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	userID := uuid.New()
	email := "user@example.com"

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "deploysentry",
			Subject:   userID.String(),
		},
		UserID: userID,
		Email:  email,
	}
	tokenStr := buildJWT(t, testJWTSecret, claims)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	var capturedUserID, capturedEmail, capturedMethod interface{}
	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		capturedUserID, _ = ctx.Get("user_id")
		capturedEmail, _ = ctx.Get("email")
		capturedMethod, _ = ctx.Get("auth_method")
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, capturedUserID)
	assert.Equal(t, email, capturedEmail)
	assert.Equal(t, "jwt", capturedMethod)
}

func TestRequireAuth_ExpiredJWT(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "deploysentry",
		},
		UserID: uuid.New(),
		Email:  "expired@example.com",
	}
	tokenStr := buildJWT(t, testJWTSecret, claims)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "invalid or expired token", body["error"])
}

func TestRequireAuth_InvalidJWTSignature(t *testing.T) {
	// Sign with a different secret.
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "deploysentry",
		},
		UserID: uuid.New(),
		Email:  "bad-sig@example.com",
	}
	tokenStr := buildJWT(t, "wrong-secret-entirely", claims)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_WrongSigningMethod(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)

	// Create a token with RSA "none" signing method (unsigned).
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "deploysentry",
		},
		UserID: uuid.New(),
		Email:  "wrong-alg@example.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_ValidAPIKey(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	scopes := []string{"deploy:read", "deploy:create"}

	validator := &mockKeyValidator{
		info: &APIKeyInfo{
			OrgID:          &orgID,
			ProjectID:      &projectID,
			EnvironmentIDs: []uuid.UUID{},
			Scopes:         scopes,
		},
		err: nil,
	}
	mw := NewAuthMiddleware(testJWTSecret, validator, nil)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	var capturedOrgID, capturedProjectID, capturedScopes, capturedMethod interface{}
	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		capturedOrgID, _ = ctx.Get("org_id")
		capturedProjectID, _ = ctx.Get("project_id")
		capturedScopes, _ = ctx.Get("api_key_scopes")
		capturedMethod, _ = ctx.Get("auth_method")
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "ApiKey my-secret-api-key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, orgID.String(), capturedOrgID)
	assert.Equal(t, projectID.String(), capturedProjectID)
	assert.Equal(t, scopes, capturedScopes)
	assert.Equal(t, "api_key", capturedMethod)
}

func TestRequireAuth_InvalidAPIKey(t *testing.T) {
	validator := &mockKeyValidator{
		info: nil,
		err:  errors.New("key not found"),
	}
	mw := NewAuthMiddleware(testJWTSecret, validator, nil)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "ApiKey invalid-key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "invalid API key", body["error"])
}

func TestRequireAuth_APIKeyValidatorNil(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil) // no validator

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "ApiKey some-key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "API key authentication not configured", body["error"])
}

// ---------------------------------------------------------------------------
// RequireScope
// ---------------------------------------------------------------------------

func TestRequireScope_JWTAuthMethod(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "jwt")
		mw.RequireScope("deploy:create")(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireScope_APIKeyWithMatchingScope(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", []string{"deploy:read", "deploy:create"})
		mw.RequireScope("deploy:create")(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireScope_APIKeyWithAdminScope(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", []string{"admin"})
		mw.RequireScope("deploy:create")(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireScope_APIKeyWithoutMatchingScope(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", []string{"deploy:read", "flag:read"})
		mw.RequireScope("deploy:create")(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "insufficient scope", body["error"])
}

func TestRequireScope_APIKeyNoScopesSet(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		// Deliberately not setting "api_key_scopes".
		mw.RequireScope("deploy:create")(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "insufficient permissions", body["error"])
}

// =========================================================================
// OAuth Tests (oauth.go)
// =========================================================================

func TestNewOAuthHandler_WithGitHubConfig(t *testing.T) {
	cfg := OAuthConfig{
		GitHubClientID:     "gh-client-id",
		GitHubClientSecret: "gh-client-secret",
		RedirectBaseURL:    "https://example.com",
		JWTSecret:          "secret",
		JWTExpiration:      2 * time.Hour,
	}
	h := NewOAuthHandler(cfg, nil)

	assert.NotNil(t, h.githubConfig)
	assert.Equal(t, "gh-client-id", h.githubConfig.ClientID)
	assert.Equal(t, "gh-client-secret", h.githubConfig.ClientSecret)
	assert.Equal(t, "https://example.com/auth/github/callback", h.githubConfig.RedirectURL)
	assert.Contains(t, h.githubConfig.Scopes, "user:email")
	assert.Nil(t, h.googleConfig)
	assert.Equal(t, 2*time.Hour, h.jwtExpiry)
}

func TestNewOAuthHandler_WithGoogleConfig(t *testing.T) {
	cfg := OAuthConfig{
		GoogleClientID:     "google-client-id",
		GoogleClientSecret: "google-client-secret",
		RedirectBaseURL:    "https://example.com",
		JWTSecret:          "secret",
		JWTExpiration:      4 * time.Hour,
	}
	h := NewOAuthHandler(cfg, nil)

	assert.Nil(t, h.githubConfig)
	assert.NotNil(t, h.googleConfig)
	assert.Equal(t, "google-client-id", h.googleConfig.ClientID)
	assert.Equal(t, "google-client-secret", h.googleConfig.ClientSecret)
	assert.Equal(t, "https://example.com/auth/google/callback", h.googleConfig.RedirectURL)
	assert.Contains(t, h.googleConfig.Scopes, "openid")
	assert.Contains(t, h.googleConfig.Scopes, "email")
	assert.Contains(t, h.googleConfig.Scopes, "profile")
	assert.Equal(t, 4*time.Hour, h.jwtExpiry)
}

func TestNewOAuthHandler_DefaultJWTExpiry(t *testing.T) {
	cfg := OAuthConfig{
		JWTSecret:     "secret",
		JWTExpiration: 0, // should default to 24h
	}
	h := NewOAuthHandler(cfg, nil)

	assert.Equal(t, 24*time.Hour, h.jwtExpiry)
}

// ---------------------------------------------------------------------------
// generateJWT
// ---------------------------------------------------------------------------

func TestGenerateJWT_ValidToken(t *testing.T) {
	secret := "jwt-test-secret"
	userID := uuid.New()
	email := "gen@example.com"

	h := &OAuthHandler{
		jwtSecret: []byte(secret),
		jwtExpiry: 1 * time.Hour,
	}

	tokenStr, err := h.generateJWT(userID, email)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	// Parse the token back.
	claims := &TokenClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsed.Valid)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, "deploysentry", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestGenerateJWT_Expiration(t *testing.T) {
	secret := "jwt-exp-secret"
	userID := uuid.New()
	email := "exp@example.com"

	h := &OAuthHandler{
		jwtSecret: []byte(secret),
		jwtExpiry: 2 * time.Hour,
	}

	before := time.Now().UTC()
	tokenStr, err := h.generateJWT(userID, email)
	after := time.Now().UTC()
	assert.NoError(t, err)

	claims := &TokenClaims{}
	_, err = jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.NoError(t, err)

	// Verify ExpiresAt is roughly 2 hours from now.
	expiry := claims.ExpiresAt.Time
	assert.True(t, expiry.After(before.Add(2*time.Hour-time.Second)), "expiration should be at least ~2h from now")
	assert.True(t, expiry.Before(after.Add(2*time.Hour+time.Second)), "expiration should be at most ~2h from now")

	// Verify IssuedAt is roughly now.
	issuedAt := claims.IssuedAt.Time
	assert.True(t, issuedAt.After(before.Add(-time.Second)))
	assert.True(t, issuedAt.Before(after.Add(time.Second)))
}

// =========================================================================
// OAuth Handler Route/Callback Tests (oauth.go)
// =========================================================================

type mockUserResolver struct {
	userID uuid.UUID
	err    error
}

func (m *mockUserResolver) ResolveOAuthUser(_ context.Context, _ string, _ *OAuthUser) (uuid.UUID, error) {
	return m.userID, m.err
}

func TestOAuthHandler_RegisterRoutes(t *testing.T) {
	cfg := OAuthConfig{
		GitHubClientID:     "gh-id",
		GitHubClientSecret: "gh-secret",
		GoogleClientID:     "g-id",
		GoogleClientSecret: "g-secret",
		RedirectBaseURL:    "http://localhost",
		JWTSecret:          "secret",
	}
	h := NewOAuthHandler(cfg, nil)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api"))

	// Verify routes are registered (GitHub login redirects).
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "github.com")

	// Google login redirects.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "google")
}

func TestOAuthHandler_RegisterRoutes_NoProviders(t *testing.T) {
	cfg := OAuthConfig{JWTSecret: "secret"}
	h := NewOAuthHandler(cfg, nil)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api"))

	// Routes should 404 since no providers configured.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGitHubCallback_MissingCode(t *testing.T) {
	cfg := OAuthConfig{
		GitHubClientID:     "gh-id",
		GitHubClientSecret: "gh-secret",
		RedirectBaseURL:    "http://localhost",
		JWTSecret:          "secret",
	}
	h := NewOAuthHandler(cfg, nil)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "missing code parameter", body["error"])
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	cfg := OAuthConfig{
		GoogleClientID:     "g-id",
		GoogleClientSecret: "g-secret",
		RedirectBaseURL:    "http://localhost",
		JWTSecret:          "secret",
	}
	h := NewOAuthHandler(cfg, nil)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "missing code parameter", body["error"])
}

func TestCompleteAuth_Success(t *testing.T) {
	userID := uuid.New()
	resolver := &mockUserResolver{userID: userID}

	h := &OAuthHandler{
		jwtSecret: []byte("test-secret"),
		jwtExpiry: time.Hour,
		resolver:  resolver,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	oauthUser := &OAuthUser{
		ProviderID: "123",
		Email:      "test@example.com",
		Name:       "Test User",
	}

	h.completeAuth(c, "github", oauthUser)

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	assert.NotEmpty(t, body["token"])
	assert.Equal(t, userID.String(), body["user_id"])
}

func TestCompleteAuth_ResolverError(t *testing.T) {
	resolver := &mockUserResolver{err: errors.New("resolve failed")}

	h := &OAuthHandler{
		jwtSecret: []byte("test-secret"),
		jwtExpiry: time.Hour,
		resolver:  resolver,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h.completeAuth(c, "github", &OAuthUser{Email: "test@example.com"})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, "failed to resolve user", body["error"])
}

func TestRequireAuth_JWTWithOrgID(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	userID := uuid.New()

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
		Email:  "user@example.com",
		OrgID:  "org-123",
	}
	tokenStr := buildJWT(t, testJWTSecret, claims)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	var capturedOrgID interface{}
	r.GET("/test", mw.RequireAuth(), func(ctx *gin.Context) {
		capturedOrgID, _ = ctx.Get("org_id")
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "org-123", capturedOrgID)
}

func TestRequireScope_APIKeyBadScopesType(t *testing.T) {
	mw := NewAuthMiddleware(testJWTSecret, nil, nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", "not-a-slice")
		mw.RequireScope("deploy:create")(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// -----------------------------------------------------------------------------
// RequirePermission — API key scope path (added alongside the RBAC role path).
// -----------------------------------------------------------------------------

func TestRequirePermission_APIKeyScope_DeploysWriteSatisfiesDeployCreate(t *testing.T) {
	rbac := NewRBACChecker()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", []string{"deploys:write"})
		RequirePermission(rbac, PermDeployCreate)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_APIKeyScope_AdminSatisfiesEverything(t *testing.T) {
	rbac := NewRBACChecker()
	gin.SetMode(gin.TestMode)

	perms := []Permission{
		PermDeployCreate, PermDeployPromote, PermDeployRollback,
		PermFlagCreate, PermFlagUpdate, PermFlagArchive,
		PermReleaseCreate, PermReleasePromote,
		PermStatusWrite, PermAPIKeyManage,
	}
	for _, perm := range perms {
		t.Run(string(perm), func(t *testing.T) {
			w := httptest.NewRecorder()
			_, r := gin.CreateTestContext(w)
			r.GET("/test", func(ctx *gin.Context) {
				ctx.Set("auth_method", "api_key")
				ctx.Set("api_key_scopes", []string{"admin"})
				RequirePermission(rbac, perm)(ctx)
			}, func(ctx *gin.Context) {
				ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
			assert.Equal(t, http.StatusOK, w.Code, "admin scope must satisfy %s", perm)
		})
	}
}

func TestRequirePermission_APIKeyScope_InsufficientReturnsActionableBody(t *testing.T) {
	rbac := NewRBACChecker()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", []string{"flags:read"})
		RequirePermission(rbac, PermDeployCreate)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "deploys:write")
	assert.Contains(t, w.Body.String(), "admin")
	assert.Contains(t, w.Body.String(), "flags:read") // echoes the scopes the key DID have
}

func TestRequirePermission_APIKeyScope_ReadImpliedByWrite(t *testing.T) {
	rbac := NewRBACChecker()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.GET("/test", func(ctx *gin.Context) {
		ctx.Set("auth_method", "api_key")
		ctx.Set("api_key_scopes", []string{"flags:write"})
		RequirePermission(rbac, PermFlagRead)(ctx)
	}, func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, http.StatusOK, w.Code, "flags:write should also satisfy flag:read")
}
