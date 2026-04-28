package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

const (
	// apiKeyByteLen is the number of random bytes used to generate an API key.
	apiKeyByteLen = 32
	// apiKeyPrefixLen is the number of characters stored as the key prefix for
	// identification purposes.
	apiKeyPrefixLen = 8
	// argon2Time is the number of iterations for argon2id.
	argon2Time = 1
	// argon2Memory is the memory parameter for argon2id (64 MB).
	argon2Memory = 64 * 1024
	// argon2Threads is the parallelism parameter for argon2id.
	argon2Threads = 4
	// argon2KeyLen is the output key length for argon2id.
	argon2KeyLen = 32
	// argon2SaltLen is the salt length for argon2id.
	argon2SaltLen = 16
)

// CheckIPAllowed verifies that clientIP falls within at least one of the
// allowed CIDRs. Returns true if allowedCIDRs is empty (no restriction).
func CheckIPAllowed(clientIP string, allowedCIDRs []string) bool {
	if len(allowedCIDRs) == 0 {
		return true
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// APIKeyRepository defines the persistence interface for API key operations.
type APIKeyRepository interface {
	// CreateAPIKey persists a new API key record.
	CreateAPIKey(ctx context.Context, key *models.APIKey) error

	// GetAPIKey retrieves an API key by its ID.
	GetAPIKey(ctx context.Context, id uuid.UUID) (*models.APIKey, error)

	// GetAPIKeyByPrefix retrieves an API key by its prefix.
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*models.APIKey, error)

	// ListAPIKeys returns API keys for an organization, optionally filtered by project and/or environment.
	ListAPIKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, environmentID *uuid.UUID, limit, offset int) ([]*models.APIKey, error)

	// UpdateAPIKey persists changes to an existing API key.
	UpdateAPIKey(ctx context.Context, key *models.APIKey) error

	// DeleteAPIKey soft-deletes an API key by setting its revoked_at timestamp.
	DeleteAPIKey(ctx context.Context, id uuid.UUID) error

	// UpdateLastUsed updates the last_used_at timestamp for an API key.
	UpdateLastUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
}

// APIKeyService provides operations for managing API keys, including
// generation, hashing, validation, and rotation.
type APIKeyService struct {
	repo APIKeyRepository
}

// NewAPIKeyService creates a new APIKeyService with the given repository.
func NewAPIKeyService(repo APIKeyRepository) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// GenerateKeyResult contains the generated plaintext key and the persisted
// model. The plaintext key is only available at creation time.
type GenerateKeyResult struct {
	PlaintextKey string         `json:"plaintext_key"`
	APIKey       *models.APIKey `json:"api_key"`
}

// GenerateKey creates a new API key with a cryptographically secure random
// value. The plaintext key is returned only once at creation time. The key
// hash is stored using argon2id.
func (s *APIKeyService) GenerateKey(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID, name string, scopes []models.APIKeyScope, createdBy uuid.UUID, envIDs []uuid.UUID, allowedCIDRs []string, expiresAt *time.Time) (*GenerateKeyResult, error) {
	// Generate cryptographically secure random bytes.
	rawKey := make([]byte, apiKeyByteLen)
	if _, err := rand.Read(rawKey); err != nil {
		return nil, fmt.Errorf("generating random key: %w", err)
	}

	plaintextKey := "ds_" + hex.EncodeToString(rawKey)
	prefix := plaintextKey[:apiKeyPrefixLen]

	keyHash, err := HashKey(plaintextKey)
	if err != nil {
		return nil, fmt.Errorf("hashing key: %w", err)
	}

	if envIDs == nil {
		envIDs = []uuid.UUID{}
	}
	if allowedCIDRs == nil {
		allowedCIDRs = []string{}
	}

	now := time.Now().UTC()
	apiKey := &models.APIKey{
		ID:             uuid.New(),
		OrgID:          orgID,
		ProjectID:      projectID,
		ApplicationID:  applicationID,
		EnvironmentIDs: envIDs,
		Name:           name,
		KeyPrefix:      prefix,
		KeyHash:        keyHash,
		Scopes:         scopes,
		AllowedCIDRs:   allowedCIDRs,
		ExpiresAt:      expiresAt,
		CreatedBy:      createdBy,
		CreatedAt:      now,
	}

	if err := apiKey.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("creating api key: %w", err)
	}

	return &GenerateKeyResult{
		PlaintextKey: plaintextKey,
		APIKey:       apiKey,
	}, nil
}

// HashKey hashes an API key using argon2id with a random salt.
// The output format is: hex(salt) + "$" + hex(hash).
func HashKey(plaintextKey string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plaintextKey), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash), nil
}

// VerifyKey checks whether a plaintext key matches a stored hash.
func VerifyKey(plaintextKey, storedHash string) bool {
	// Split the stored hash into salt and hash components.
	var saltHex, hashHex string
	for i, c := range storedHash {
		if c == '$' {
			saltHex = storedHash[:i]
			hashHex = storedHash[i+1:]
			break
		}
	}
	if saltHex == "" || hashHex == "" {
		return false
	}

	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	computedHash := argon2.IDKey([]byte(plaintextKey), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}

// ValidateKey looks up an API key by its prefix, verifies the hash, and checks
// expiration and revocation status. It also updates the last-used timestamp.
func (s *APIKeyService) ValidateKey(ctx context.Context, plaintextKey string) (*models.APIKey, error) {
	if len(plaintextKey) < apiKeyPrefixLen {
		return nil, fmt.Errorf("invalid api key format")
	}

	prefix := plaintextKey[:apiKeyPrefixLen]
	apiKey, err := s.repo.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("looking up api key: %w", err)
	}
	if apiKey == nil {
		return nil, fmt.Errorf("api key not found")
	}

	if apiKey.IsRevoked() {
		return nil, fmt.Errorf("api key has been revoked")
	}

	if apiKey.IsExpired() {
		return nil, fmt.Errorf("api key has expired")
	}

	if !VerifyKey(plaintextKey, apiKey.KeyHash) {
		return nil, fmt.Errorf("invalid api key")
	}

	// Update last-used timestamp in the background.
	now := time.Now().UTC()
	_ = s.repo.UpdateLastUsed(ctx, apiKey.ID, now)

	return apiKey, nil
}

// GetKey retrieves an API key by its ID.
func (s *APIKeyService) GetKey(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	key, err := s.repo.GetAPIKey(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting api key: %w", err)
	}
	return key, nil
}

// ListKeys returns API keys for an organization, optionally filtered by project.
func (s *APIKeyService) ListKeys(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, environmentID *uuid.UUID, limit, offset int) ([]*models.APIKey, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	keys, err := s.repo.ListAPIKeys(ctx, orgID, projectID, environmentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	return keys, nil
}

// RevokeKey soft-deletes an API key by setting its revoked_at timestamp.
func (s *APIKeyService) RevokeKey(ctx context.Context, id uuid.UUID) error {
	key, err := s.repo.GetAPIKey(ctx, id)
	if err != nil {
		return fmt.Errorf("getting api key for revocation: %w", err)
	}
	if key == nil {
		return fmt.Errorf("api key not found")
	}
	if key.IsRevoked() {
		return fmt.Errorf("api key already revoked")
	}

	now := time.Now().UTC()
	key.RevokedAt = &now
	if err := s.repo.UpdateAPIKey(ctx, key); err != nil {
		return fmt.Errorf("revoking api key: %w", err)
	}
	return nil
}

// RotateKey generates a new API key and revokes the old one. This allows a
// seamless rotation workflow where the caller receives a new key and the
// previous key is deprecated.
func (s *APIKeyService) RotateKey(ctx context.Context, oldKeyID uuid.UUID, createdBy uuid.UUID) (*GenerateKeyResult, error) {
	oldKey, err := s.repo.GetAPIKey(ctx, oldKeyID)
	if err != nil {
		return nil, fmt.Errorf("getting old api key: %w", err)
	}
	if oldKey == nil {
		return nil, fmt.Errorf("api key not found")
	}
	if oldKey.IsRevoked() {
		return nil, fmt.Errorf("cannot rotate a revoked api key")
	}

	// Generate a new key with the same configuration.
	result, err := s.GenerateKey(
		ctx,
		oldKey.OrgID,
		oldKey.ProjectID,
		oldKey.ApplicationID,
		oldKey.Name+" (rotated)",
		oldKey.Scopes,
		createdBy,
		oldKey.EnvironmentIDs,
		oldKey.AllowedCIDRs,
		oldKey.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("generating rotated key: %w", err)
	}

	// Revoke the old key.
	now := time.Now().UTC()
	oldKey.RevokedAt = &now
	if err := s.repo.UpdateAPIKey(ctx, oldKey); err != nil {
		return nil, fmt.Errorf("revoking old key during rotation: %w", err)
	}

	return result, nil
}
