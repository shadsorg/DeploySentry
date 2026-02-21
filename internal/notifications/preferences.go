package notifications

import (
	"context"
	"sync"
)

// NotificationPreferences maps event types and channels to enabled/disabled
// state for a specific user or project scope.
type NotificationPreferences struct {
	// UserID is the user these preferences belong to. Empty for project-level
	// defaults.
	UserID string `json:"user_id,omitempty"`

	// ProjectID is the project these preferences apply to. Empty for
	// global user defaults.
	ProjectID string `json:"project_id,omitempty"`

	// Rules maps an event type to per-channel enabled state.
	// A missing entry means the default (enabled) applies.
	Rules map[EventType]ChannelPreferences `json:"rules"`
}

// ChannelPreferences maps channel names to their enabled/disabled state.
type ChannelPreferences struct {
	// Channels maps channel name (e.g. "slack", "email", "webhook") to
	// whether that channel is enabled for the event type.
	Channels map[string]bool `json:"channels"`
}

// IsEnabled reports whether notifications for the given event type and channel
// are enabled. If no preference is recorded for the combination, the default
// is enabled (true).
func (p *NotificationPreferences) IsEnabled(eventType EventType, channel string) bool {
	if p == nil || p.Rules == nil {
		return true
	}
	cp, ok := p.Rules[eventType]
	if !ok {
		return true
	}
	enabled, ok := cp.Channels[channel]
	if !ok {
		return true
	}
	return enabled
}

// PreferenceStore defines the interface for persisting and retrieving
// notification preferences.
type PreferenceStore interface {
	// GetPreferences retrieves notification preferences for the given user
	// and project. Either userID or projectID may be empty for global lookups.
	GetPreferences(ctx context.Context, userID, projectID string) (*NotificationPreferences, error)

	// SavePreferences persists notification preferences.
	SavePreferences(ctx context.Context, prefs *NotificationPreferences) error

	// DeletePreferences removes notification preferences for a user/project.
	DeletePreferences(ctx context.Context, userID, projectID string) error
}

// InMemoryPreferenceStore is a thread-safe in-memory implementation of
// PreferenceStore, useful for testing and development.
type InMemoryPreferenceStore struct {
	mu    sync.RWMutex
	store map[string]*NotificationPreferences
}

// NewInMemoryPreferenceStore creates a new in-memory preference store.
func NewInMemoryPreferenceStore() *InMemoryPreferenceStore {
	return &InMemoryPreferenceStore{
		store: make(map[string]*NotificationPreferences),
	}
}

// preferenceKey generates a unique key from the user and project identifiers.
func preferenceKey(userID, projectID string) string {
	return userID + ":" + projectID
}

// GetPreferences retrieves notification preferences from memory.
func (s *InMemoryPreferenceStore) GetPreferences(_ context.Context, userID, projectID string) (*NotificationPreferences, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := preferenceKey(userID, projectID)
	prefs, ok := s.store[key]
	if !ok {
		// Return default (all enabled) preferences when none are stored.
		return &NotificationPreferences{
			UserID:    userID,
			ProjectID: projectID,
		}, nil
	}
	return prefs, nil
}

// SavePreferences stores notification preferences in memory.
func (s *InMemoryPreferenceStore) SavePreferences(_ context.Context, prefs *NotificationPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := preferenceKey(prefs.UserID, prefs.ProjectID)
	s.store[key] = prefs
	return nil
}

// DeletePreferences removes notification preferences from memory.
func (s *InMemoryPreferenceStore) DeletePreferences(_ context.Context, userID, projectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := preferenceKey(userID, projectID)
	delete(s.store, key)
	return nil
}
