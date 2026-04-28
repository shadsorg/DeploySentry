package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/shadsorg/deploysentry/internal/platform/cache"
)

const (
	// sessionPrefix is the Redis key prefix for user sessions.
	sessionPrefix = "session:"
	// blacklistPrefix is the Redis key prefix for blacklisted tokens.
	blacklistPrefix = "token:blacklist:"
	// defaultSessionTTL is the default session duration.
	defaultSessionTTL = 24 * time.Hour
)

// Session represents an authenticated user session stored in Redis.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	OrgID     string    `json:"org_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionManager manages user sessions via Redis.
type SessionManager struct {
	redis *cache.Redis
	ttl   time.Duration
}

// NewSessionManager creates a new SessionManager backed by the given Redis client.
// If ttl is zero, defaultSessionTTL is used.
func NewSessionManager(r *cache.Redis, ttl time.Duration) *SessionManager {
	if ttl == 0 {
		ttl = defaultSessionTTL
	}
	return &SessionManager{
		redis: r,
		ttl:   ttl,
	}
}

// CreateSession creates a new session for the given user and stores it in Redis.
// It returns the session with a generated ID.
func (sm *SessionManager) CreateSession(ctx context.Context, userID, orgID, email, ipAddress, userAgent string) (*Session, error) {
	now := time.Now().UTC()
	session := &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		OrgID:     orgID,
		Email:     email,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: now.Add(sm.ttl),
		CreatedAt: now,
	}

	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("marshaling session: %w", err)
	}

	key := sessionPrefix + session.ID
	if err := sm.redis.Set(ctx, key, string(data), sm.ttl); err != nil {
		return nil, fmt.Errorf("storing session in redis: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by its ID from Redis.
// Returns nil and no error if the session does not exist or has expired.
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := sessionPrefix + sessionID
	data, err := sm.redis.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("getting session from redis: %w", err)
	}

	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("unmarshaling session: %w", err)
	}

	// Check if the session has expired (belt-and-suspenders with Redis TTL).
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = sm.redis.Delete(ctx, key)
		return nil, nil
	}

	return &session, nil
}

// DeleteSession removes a session from Redis, effectively logging the user out.
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	key := sessionPrefix + sessionID
	if err := sm.redis.Delete(ctx, key); err != nil {
		return fmt.Errorf("deleting session from redis: %w", err)
	}
	return nil
}

// RefreshSession extends the expiration of an existing session. Returns the
// updated session or nil if the session does not exist.
func (sm *SessionManager) RefreshSession(ctx context.Context, sessionID string) (*Session, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	session.ExpiresAt = time.Now().UTC().Add(sm.ttl)

	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("marshaling refreshed session: %w", err)
	}

	key := sessionPrefix + session.ID
	if err := sm.redis.Set(ctx, key, string(data), sm.ttl); err != nil {
		return nil, fmt.Errorf("storing refreshed session in redis: %w", err)
	}

	return session, nil
}

// BlacklistToken adds a token ID to the blacklist in Redis. The entry will
// automatically expire after the given TTL, which should match or exceed the
// remaining lifetime of the token.
func (sm *SessionManager) BlacklistToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	key := blacklistPrefix + tokenID
	if err := sm.redis.Set(ctx, key, "1", ttl); err != nil {
		return fmt.Errorf("blacklisting token: %w", err)
	}
	return nil
}

// IsTokenBlacklisted checks whether a token ID has been blacklisted.
func (sm *SessionManager) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := blacklistPrefix + tokenID
	exists, err := sm.redis.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("checking token blacklist: %w", err)
	}
	return exists, nil
}
