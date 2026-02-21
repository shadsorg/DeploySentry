// Package auth implements authentication and authorization, including OAuth
// provider integration, JWT token management, API key authentication, and
// role-based access control.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	oauth2google "golang.org/x/oauth2/google"
)

// OAuthConfig holds the configuration for OAuth2 authentication providers.
type OAuthConfig struct {
	GitHubClientID     string        `json:"github_client_id"`
	GitHubClientSecret string        `json:"github_client_secret"`
	GoogleClientID     string        `json:"google_client_id"`
	GoogleClientSecret string        `json:"google_client_secret"`
	RedirectBaseURL    string        `json:"redirect_base_url"`
	JWTSecret          string        `json:"jwt_secret"`
	JWTExpiration      time.Duration `json:"jwt_expiration"`
	RefreshExpiration  time.Duration `json:"refresh_expiration"`
}

// OAuthUser represents the user profile returned by an OAuth provider.
type OAuthUser struct {
	ProviderID string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	AvatarURL  string `json:"avatar_url"`
}

// UserResolver is called after OAuth authentication to find or create a user.
type UserResolver interface {
	// ResolveOAuthUser finds or creates a user from the OAuth profile.
	ResolveOAuthUser(ctx context.Context, provider string, oauthUser *OAuthUser) (userID uuid.UUID, err error)
}

// TokenClaims defines the JWT claims for authenticated sessions.
type TokenClaims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	OrgID  string    `json:"org_id,omitempty"`
}

// RefreshClaims defines the JWT claims for refresh tokens.
type RefreshClaims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	// TokenID is a unique identifier for this refresh token, used for revocation.
	TokenID string `json:"token_id"`
}

// OAuthHandler manages OAuth2 authentication flows for GitHub and Google.
type OAuthHandler struct {
	githubConfig   *oauth2.Config
	googleConfig   *oauth2.Config
	jwtSecret      []byte
	jwtExpiry      time.Duration
	refreshExpiry  time.Duration
	resolver       UserResolver
	sessionManager *SessionManager
}

// NewOAuthHandler creates a new OAuthHandler with the given configuration.
func NewOAuthHandler(config OAuthConfig, resolver UserResolver) *OAuthHandler {
	h := &OAuthHandler{
		jwtSecret:     []byte(config.JWTSecret),
		jwtExpiry:     config.JWTExpiration,
		refreshExpiry: config.RefreshExpiration,
		resolver:      resolver,
	}

	if config.JWTExpiration == 0 {
		h.jwtExpiry = 24 * time.Hour
	}

	if config.RefreshExpiration == 0 {
		h.refreshExpiry = 7 * 24 * time.Hour
	}

	if config.GitHubClientID != "" {
		h.githubConfig = &oauth2.Config{
			ClientID:     config.GitHubClientID,
			ClientSecret: config.GitHubClientSecret,
			Endpoint:     github.Endpoint,
			RedirectURL:  config.RedirectBaseURL + "/auth/github/callback",
			Scopes:       []string{"user:email"},
		}
	}

	if config.GoogleClientID != "" {
		h.googleConfig = &oauth2.Config{
			ClientID:     config.GoogleClientID,
			ClientSecret: config.GoogleClientSecret,
			Endpoint:     oauth2google.Endpoint,
			RedirectURL:  config.RedirectBaseURL + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
		}
	}

	return h
}

// SetSessionManager attaches a SessionManager to the handler for session and
// token blacklisting support.
func (h *OAuthHandler) SetSessionManager(sm *SessionManager) {
	h.sessionManager = sm
}

// RegisterRoutes mounts all OAuth API routes on the given router group.
func (h *OAuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		if h.githubConfig != nil {
			auth.GET("/github", h.githubLogin)
			auth.GET("/github/callback", h.githubCallback)
		}
		if h.googleConfig != nil {
			auth.GET("/google", h.googleLogin)
			auth.GET("/google/callback", h.googleCallback)
		}
		auth.POST("/refresh", h.refreshTokenHandler)
		auth.POST("/logout", h.logoutHandler)
	}
}

func (h *OAuthHandler) githubLogin(c *gin.Context) {
	state := uuid.New().String()
	url := h.githubConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *OAuthHandler) githubCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code parameter"})
		return
	}

	token, err := h.githubConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exchange code"})
		return
	}

	oauthUser, err := h.fetchGitHubUser(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user profile"})
		return
	}

	h.completeAuth(c, "github", oauthUser)
}

func (h *OAuthHandler) googleLogin(c *gin.Context) {
	state := uuid.New().String()
	url := h.googleConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *OAuthHandler) googleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code parameter"})
		return
	}

	token, err := h.googleConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exchange code"})
		return
	}

	oauthUser, err := h.fetchGoogleUser(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user profile"})
		return
	}

	h.completeAuth(c, "google", oauthUser)
}

// completeAuth resolves the OAuth user, generates a JWT and refresh token,
// and returns both.
func (h *OAuthHandler) completeAuth(c *gin.Context, provider string, oauthUser *OAuthUser) {
	userID, err := h.resolver.ResolveOAuthUser(c.Request.Context(), provider, oauthUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve user"})
		return
	}

	tokenStr, err := h.generateJWT(userID, oauthUser.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshStr, err := h.GenerateRefreshToken(userID, oauthUser.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         tokenStr,
		"refresh_token": refreshStr,
		"user_id":       userID.String(),
	})
}

// generateJWT creates a signed JWT for the authenticated user.
func (h *OAuthHandler) generateJWT(userID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(h.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "deploysentry",
			Subject:   userID.String(),
		},
		UserID: userID,
		Email:  email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

// GenerateRefreshToken creates a signed refresh token with a longer expiration
// than the access token. Each refresh token has a unique token_id that can be
// used for revocation.
func (h *OAuthHandler) GenerateRefreshToken(userID uuid.UUID, email string) (string, error) {
	tokenID, err := generateSecureToken(16)
	if err != nil {
		return "", fmt.Errorf("generating token id: %w", err)
	}

	refreshExp := h.refreshExpiry
	if refreshExp == 0 {
		refreshExp = 7 * 24 * time.Hour
	}

	now := time.Now().UTC()
	claims := &RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshExp)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "deploysentry",
			Subject:   userID.String(),
		},
		UserID:  userID,
		Email:   email,
		TokenID: tokenID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

// refreshRequest is the JSON body for the token refresh endpoint.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// refreshTokenHandler validates a refresh token and issues a new access token
// and refresh token pair.
func (h *OAuthHandler) refreshTokenHandler(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	claims := &RefreshClaims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	// Check if the refresh token has been blacklisted.
	if h.sessionManager != nil {
		blacklisted, err := h.sessionManager.IsTokenBlacklisted(c.Request.Context(), claims.TokenID)
		if err == nil && blacklisted {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token has been revoked"})
			return
		}
	}

	// Generate new access and refresh tokens.
	newAccess, err := h.generateJWT(claims.UserID, claims.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	newRefresh, err := h.GenerateRefreshToken(claims.UserID, claims.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	// Blacklist the old refresh token so it cannot be reused.
	if h.sessionManager != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			_ = h.sessionManager.BlacklistToken(c.Request.Context(), claims.TokenID, ttl)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         newAccess,
		"refresh_token": newRefresh,
	})
}

// logoutRequest is the JSON body for the logout endpoint.
type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	SessionID    string `json:"session_id"`
}

// logoutHandler invalidates the user's session and blacklists their refresh token.
func (h *OAuthHandler) logoutHandler(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// If the body is missing or malformed, still proceed with what we have.
		req = logoutRequest{}
	}

	if h.sessionManager == nil {
		c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
		return
	}

	// Blacklist the refresh token if provided.
	if req.RefreshToken != "" {
		claims := &RefreshClaims{}
		token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return h.jwtSecret, nil
		})
		if err == nil && token.Valid {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				_ = h.sessionManager.BlacklistToken(c.Request.Context(), claims.TokenID, ttl)
			}
		}
	}

	// Delete the session if a session ID is provided.
	if req.SessionID != "" {
		_ = h.sessionManager.DeleteSession(c.Request.Context(), req.SessionID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

// generateSecureToken creates a cryptographically secure random hex string.
func generateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// githubUserResponse models the GitHub user API response.
type githubUserResponse struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// fetchGitHubUser retrieves the user profile from the GitHub API.
func (h *OAuthHandler) fetchGitHubUser(ctx context.Context, token *oauth2.Token) (*OAuthUser, error) {
	client := h.githubConfig.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("fetching github user: %w", err)
	}
	defer resp.Body.Close()

	var ghUser githubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("decoding github user: %w", err)
	}

	return &OAuthUser{
		ProviderID: fmt.Sprintf("%d", ghUser.ID),
		Email:      ghUser.Email,
		Name:       ghUser.Name,
		AvatarURL:  ghUser.AvatarURL,
	}, nil
}

// googleUserResponse models the Google userinfo API response.
type googleUserResponse struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// fetchGoogleUser retrieves the user profile from the Google userinfo API.
func (h *OAuthHandler) fetchGoogleUser(ctx context.Context, token *oauth2.Token) (*OAuthUser, error) {
	client := h.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetching google user: %w", err)
	}
	defer resp.Body.Close()

	var gUser googleUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&gUser); err != nil {
		return nil, fmt.Errorf("decoding google user: %w", err)
	}

	return &OAuthUser{
		ProviderID: gUser.Sub,
		Email:      gUser.Email,
		Name:       gUser.Name,
		AvatarURL:  gUser.Picture,
	}, nil
}
