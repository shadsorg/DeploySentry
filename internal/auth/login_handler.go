package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// LoginHandler provides HTTP endpoints for email/password authentication,
// including user registration and login.
type LoginHandler struct {
	repo   UserRepository
	config config.AuthConfig
}

// NewLoginHandler creates a new LoginHandler with the given repository and config.
func NewLoginHandler(repo UserRepository, cfg config.AuthConfig) *LoginHandler {
	return &LoginHandler{
		repo:   repo,
		config: cfg,
	}
}

// RegisterRoutes mounts login and registration routes on the /auth group.
func (h *LoginHandler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.register)
		auth.POST("/login", h.login)
	}
}

// RegisterAuthenticatedRoutes mounts auth endpoints that require a valid
// (not-yet-expired) access token on the caller. /auth/extend issues a fresh
// access token so the caller can stay signed in without re-entering credentials.
func (h *LoginHandler) RegisterAuthenticatedRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/extend", h.extend)
	}
}

// registerRequest is the JSON body for the registration endpoint.
type registerRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name"     binding:"required"`
}

// loginRequest is the JSON body for the login endpoint.
type loginRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// register creates a new user account with an argon2id-hashed password.
func (h *LoginHandler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check for an existing account with the same email.
	existing, err := h.repo.GetUserByEmail(c.Request.Context(), req.Email)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "an account with that email already exists"})
		return
	}

	// Hash the password using argon2id.
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	now := time.Now().UTC()
	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Name:         req.Name,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := user.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.CreateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	expiry := h.config.JWTExpiration
	if expiry == 0 {
		expiry = 24 * time.Hour
	}
	token, err := GenerateJWT([]byte(h.config.JWTSecret), expiry, user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user":  user,
	})
}

// login validates email/password credentials and returns a JWT.
func (h *LoginHandler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.repo.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if !verifyPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Update the last login timestamp.
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	_ = h.repo.UpdateUser(c.Request.Context(), user)

	expiry := h.config.JWTExpiration
	if expiry == 0 {
		expiry = 24 * time.Hour
	}
	token, err := GenerateJWT([]byte(h.config.JWTSecret), expiry, user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// extend issues a fresh access token with a new expiration, without requiring
// re-authentication. The request must carry a valid (not-yet-expired) Bearer
// token — the auth middleware has already validated it and populated
// user_id/email in the gin context before this handler runs.
func (h *LoginHandler) extend(c *gin.Context) {
	userIDValue, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
		return
	}

	email := ""
	if v, exists := c.Get("email"); exists {
		if s, ok := v.(string); ok {
			email = s
		}
	}

	expiry := h.config.JWTExpiration
	if expiry == 0 {
		expiry = 24 * time.Hour
	}
	token, err := GenerateJWT([]byte(h.config.JWTSecret), expiry, userID, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// hashPassword hashes a plaintext password using argon2id with a random salt.
// The output format is: hex(salt) + "$" + hex(hash).
func hashPassword(plaintext string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plaintext), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf("%x$%x", salt, hash), nil
}

// verifyPassword checks whether a plaintext password matches a stored hash
// produced by hashPassword.
func verifyPassword(plaintext, storedHash string) bool {
	idx := strings.IndexByte(storedHash, '$')
	if idx < 0 {
		return false
	}
	saltHex := storedHash[:idx]
	hashHex := storedHash[idx+1:]

	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	computedHash := argon2.IDKey([]byte(plaintext), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}
