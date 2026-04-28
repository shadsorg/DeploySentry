package notifications

import (
	"net/http"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/gin-gonic/gin"
)

// ChannelConfig describes a registered notification channel and its
// per-event-type support status.
type ChannelConfig struct {
	// Name is the channel identifier (e.g. "slack", "email", "pagerduty").
	Name string `json:"name"`

	// SupportedEvents lists the event types this channel handles.
	SupportedEvents []string `json:"supported_events"`
}

// PreferencesResponse is the JSON body returned by GET /notifications/preferences.
type PreferencesResponse struct {
	// OrgID is the organization these preferences apply to.
	OrgID string `json:"org_id"`

	// Channels lists every registered notification channel and the events
	// it supports.
	Channels []ChannelConfig `json:"channels"`

	// EventRouting maps each event type to the per-channel enabled state for
	// this org. A missing entry means the default (enabled) applies.
	EventRouting map[EventType]map[string]bool `json:"event_routing"`
}

// PreferencesRequest is the JSON body accepted by PUT /notifications/preferences.
type PreferencesRequest struct {
	// EventRouting maps event types to per-channel enabled/disabled overrides.
	EventRouting map[EventType]map[string]bool `json:"event_routing" binding:"required"`
}

// PreferencesHandler exposes HTTP endpoints for managing notification
// preferences scoped to an organization.
type PreferencesHandler struct {
	store   PreferenceStore
	service *NotificationService
	rbac    *auth.RBACChecker
}

// NewPreferencesHandler creates a new PreferencesHandler.
func NewPreferencesHandler(store PreferenceStore, service *NotificationService, rbac *auth.RBACChecker) *PreferencesHandler {
	return &PreferencesHandler{
		store:   store,
		service: service,
		rbac:    rbac,
	}
}

// RegisterRoutes mounts notification preference endpoints on the provided
// router group under /notifications.
func (h *PreferencesHandler) RegisterRoutes(rg *gin.RouterGroup) {
	n := rg.Group("/notifications")
	{
		n.GET("/preferences", auth.RequirePermission(h.rbac, auth.PermSettingsRead), h.getPreferences)
		n.PUT("/preferences", auth.RequirePermission(h.rbac, auth.PermSettingsWrite), h.savePreferences)
		n.DELETE("/preferences", auth.RequirePermission(h.rbac, auth.PermSettingsWrite), h.resetPreferences)
	}
}

// getPreferences handles GET /notifications/preferences.
// It loads the org's stored preferences and merges them with the list of
// registered channels so callers see all available options together.
func (h *PreferencesHandler) getPreferences(c *gin.Context) {
	orgID := h.orgIDFromContext(c)

	prefs, err := h.store.GetPreferences(c.Request.Context(), "", orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load preferences"})
		return
	}

	resp := PreferencesResponse{
		OrgID:        orgID,
		Channels:     h.buildChannelConfigs(),
		EventRouting: h.buildEventRouting(prefs),
	}

	c.JSON(http.StatusOK, resp)
}

// savePreferences handles PUT /notifications/preferences.
// It merges the supplied event routing overrides into the existing preferences
// for this org and persists them.
func (h *PreferencesHandler) savePreferences(c *gin.Context) {
	var req PreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orgID := h.orgIDFromContext(c)

	// Load existing preferences so we do a merge rather than a full replace.
	existing, err := h.store.GetPreferences(c.Request.Context(), "", orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load existing preferences"})
		return
	}

	if existing.Rules == nil {
		existing.Rules = make(map[EventType]ChannelPreferences)
	}

	// Merge incoming overrides on top of existing rules.
	for eventType, channelMap := range req.EventRouting {
		cp, ok := existing.Rules[eventType]
		if !ok {
			cp = ChannelPreferences{Channels: make(map[string]bool)}
		}
		if cp.Channels == nil {
			cp.Channels = make(map[string]bool)
		}
		for ch, enabled := range channelMap {
			cp.Channels[ch] = enabled
		}
		existing.Rules[eventType] = cp
	}

	if err := h.store.SavePreferences(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save preferences"})
		return
	}

	resp := PreferencesResponse{
		OrgID:        orgID,
		Channels:     h.buildChannelConfigs(),
		EventRouting: h.buildEventRouting(existing),
	}

	c.JSON(http.StatusOK, resp)
}

// resetPreferences handles DELETE /notifications/preferences.
// It removes all stored preference overrides for this org, restoring defaults.
func (h *PreferencesHandler) resetPreferences(c *gin.Context) {
	orgID := h.orgIDFromContext(c)

	if err := h.store.DeletePreferences(c.Request.Context(), "", orgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "preferences reset to defaults"})
}

// buildChannelConfigs reads the registered channels from the service (under
// RLock) and returns their names alongside every event type they support.
func (h *PreferencesHandler) buildChannelConfigs() []ChannelConfig {
	// All known event types for support probing.
	allEvents := []EventType{
		EventDeployStarted,
		EventDeployCreated,
		EventDeployPhaseCompleted,
		EventDeployCompleted,
		EventDeployFailed,
		EventDeployRolledBack,
		EventDeployRollbackInitiated,
		EventDeployRollbackCompleted,
		EventFlagCreated,
		EventFlagUpdated,
		EventFlagToggled,
		EventFlagArchived,
		EventReleaseCreated,
		EventReleasePromoted,
		EventReleaseHealthDegraded,
		EventHealthDegraded,
		EventHealthAlertTriggered,
		EventHealthAlertResolved,
	}

	h.service.mu.RLock()
	channels := make([]Channel, len(h.service.channels))
	copy(channels, h.service.channels)
	h.service.mu.RUnlock()

	configs := make([]ChannelConfig, 0, len(channels))
	for _, ch := range channels {
		var supported []string
		for _, et := range allEvents {
			if ch.Supports(et) {
				supported = append(supported, string(et))
			}
		}
		configs = append(configs, ChannelConfig{
			Name:            ch.Name(),
			SupportedEvents: supported,
		})
	}
	return configs
}

// buildEventRouting converts the stored Rules map into the flat
// map[EventType]map[string]bool form used in API responses.
func (h *PreferencesHandler) buildEventRouting(prefs *NotificationPreferences) map[EventType]map[string]bool {
	routing := make(map[EventType]map[string]bool)
	if prefs == nil || prefs.Rules == nil {
		return routing
	}
	for et, cp := range prefs.Rules {
		m := make(map[string]bool, len(cp.Channels))
		for ch, enabled := range cp.Channels {
			m[ch] = enabled
		}
		routing[et] = m
	}
	return routing
}

// orgIDFromContext extracts the org_id string set by ResolveOrgRole middleware.
// Falls back to an empty string when not present (e.g. when no orgSlug is in
// the route).
func (h *PreferencesHandler) orgIDFromContext(c *gin.Context) string {
	if v, exists := c.Get("org_id"); exists {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
