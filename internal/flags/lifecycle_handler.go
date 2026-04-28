package flags

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// resolveFlagRef turns the `:id` path param into a concrete flag. It accepts
// either a flag UUID or a flag key; keys are resolved against the project
// scope attached to the authenticated API key. On failure, a JSON error is
// written and (nil, false) is returned.
func (h *Handler) resolveFlagRef(c *gin.Context) (*models.FeatureFlag, bool) {
	ref := c.Param("id")
	if ref == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "flag id or key is required"})
		return nil, false
	}

	if id, err := uuid.Parse(ref); err == nil {
		flag, err := h.service.GetFlag(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
			return nil, false
		}
		return flag, true
	}

	projectIDStr := c.GetString("project_id")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resolving a flag key requires a project-scoped API key"})
		return nil, false
	}
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project context"})
		return nil, false
	}
	flag, err := h.service.GetFlagByProjectKey(c.Request.Context(), projectID, ref)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "flag not found"})
		return nil, false
	}
	return flag, true
}

// lifecycleActor returns the authenticated user's UUID, or nil when the caller
// is an API key without a user binding.
func lifecycleActor(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uuid.UUID); ok {
			return &id
		}
	}
	return nil
}

// publishLifecycleEvent emits a webhook event with the standard lifecycle
// payload shape (documented in docs/Feature_Lifecycle.md). Extra fields are
// merged on top of the base payload.
func (h *Handler) publishLifecycleEvent(c *gin.Context, event models.WebhookEvent, flag *models.FeatureFlag, extra map[string]interface{}) {
	if h.webhookSvc == nil {
		return
	}
	payload := lifecyclePayload(flag)
	for k, v := range extra {
		payload[k] = v
	}
	var orgID uuid.UUID
	if v, exists := c.Get("org_id"); exists {
		if s, ok := v.(string); ok {
			orgID, _ = uuid.Parse(s)
		}
	}
	if err := h.webhookSvc.PublishEvent(c.Request.Context(), event, orgID, &flag.ProjectID, payload, lifecycleActor(c)); err != nil {
		log.Printf("failed to publish lifecycle webhook %s: %v", event, err)
	}
}

// lifecyclePayload is the stable payload schema shared with the CrowdSoft
// portal. Never remove fields; only add — see docs/Feature_Lifecycle.md.
func lifecyclePayload(flag *models.FeatureFlag) map[string]interface{} {
	var smoke, user interface{}
	if flag.SmokeTestStatus != nil {
		smoke = string(*flag.SmokeTestStatus)
	}
	if flag.UserTestStatus != nil {
		user = string(*flag.UserTestStatus)
	}
	var smokeNotes, userNotes interface{}
	if flag.LastSmokeTestNotes != nil {
		smokeNotes = *flag.LastSmokeTestNotes
	}
	if flag.LastUserTestNotes != nil {
		userNotes = *flag.LastUserTestNotes
	}
	var scheduled interface{}
	if flag.ScheduledRemovalAt != nil {
		scheduled = flag.ScheduledRemovalAt.UTC().Format(time.RFC3339)
	}
	return map[string]interface{}{
		"flag_id":                flag.ID,
		"flag_key":               flag.Key,
		"flag_name":              flag.Name,
		"project_id":             flag.ProjectID,
		"iteration_count":        flag.IterationCount,
		"iteration_exhausted":    flag.IterationExhausted,
		"smoke_test_status":      smoke,
		"user_test_status":       user,
		"last_smoke_test_notes":  smokeNotes,
		"last_user_test_notes":   userNotes,
		"scheduled_removal_at":   scheduled,
	}
}

// smokeTestResultRequest is the body of POST /flags/:id/smoke-test-result.
type smokeTestResultRequest struct {
	Status     string `json:"status" binding:"required"`
	Notes      string `json:"notes"`
	TestRunURL string `json:"test_run_url"`
}

func (h *Handler) recordSmokeTestResult(c *gin.Context) {
	flag, ok := h.resolveFlagRef(c)
	if !ok {
		return
	}

	var req smokeTestResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := parseLifecycleStatus(req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.service.RecordSmokeTestResult(c.Request.Context(), flag.ID, status, req.Notes, req.TestRunURL)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.smoke_test.recorded", "flag", flag.ID, "", req.Status)
	h.broadcastEvent("flag.smoke_test.recorded", flag.ID, flag.Key)

	event := models.EventFlagSmokeTestPassed
	if status == models.LifecycleTestFail {
		event = models.EventFlagSmokeTestFailed
	}
	extra := map[string]interface{}{"notes": req.Notes}
	if req.TestRunURL != "" {
		extra["test_run_url"] = req.TestRunURL
	}
	h.publishLifecycleEvent(c, event, updated, extra)

	c.JSON(http.StatusOK, updated)
}

// userTestResultRequest is the body of POST /flags/:id/user-test-result.
type userTestResultRequest struct {
	Status string `json:"status" binding:"required"`
	Notes  string `json:"notes"`
	UserID string `json:"userId"`
}

func (h *Handler) recordUserTestResult(c *gin.Context) {
	flag, ok := h.resolveFlagRef(c)
	if !ok {
		return
	}

	var req userTestResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := parseLifecycleStatus(req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if status == models.LifecycleTestFail && req.Notes == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notes are required on a failing user-test result"})
		return
	}

	updated, err := h.service.RecordUserTestResult(c.Request.Context(), flag.ID, status, req.Notes)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.user_test.recorded", "flag", flag.ID, "", req.Status)
	h.broadcastEvent("flag.user_test.recorded", flag.ID, flag.Key)

	event := models.EventFlagUserTestPassed
	if status == models.LifecycleTestFail {
		event = models.EventFlagUserTestFailed
	}
	extra := map[string]interface{}{"notes": req.Notes}
	if req.UserID != "" {
		extra["user_id"] = req.UserID
	}
	h.publishLifecycleEvent(c, event, updated, extra)

	c.JSON(http.StatusOK, updated)
}

// scheduleRemovalRequest is the body of POST /flags/:id/schedule-removal.
type scheduleRemovalRequest struct {
	Days int `json:"days" binding:"required"`
}

func (h *Handler) scheduleRemoval(c *gin.Context) {
	flag, ok := h.resolveFlagRef(c)
	if !ok {
		return
	}

	var req scheduleRemovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Days <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be a positive integer"})
		return
	}

	updated, err := h.service.ScheduleRemoval(c.Request.Context(), flag.ID, req.Days)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.scheduled_for_removal.set", "flag", flag.ID, "", "")
	h.publishLifecycleEvent(c, models.EventFlagScheduledForRemovalSet, updated, nil)

	c.JSON(http.StatusOK, updated)
}

func (h *Handler) cancelScheduledRemoval(c *gin.Context) {
	flag, ok := h.resolveFlagRef(c)
	if !ok {
		return
	}

	updated, err := h.service.CancelScheduledRemoval(c.Request.Context(), flag.ID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.scheduled_for_removal.cancelled", "flag", flag.ID, "", "")
	h.publishLifecycleEvent(c, models.EventFlagScheduledForRemovalCancelled, updated, nil)

	c.JSON(http.StatusOK, updated)
}

func (h *Handler) markIterationExhausted(c *gin.Context) {
	flag, ok := h.resolveFlagRef(c)
	if !ok {
		return
	}

	updated, err := h.service.MarkIterationExhausted(c.Request.Context(), flag.ID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.writeAudit(c, "flag.iteration_exhausted", "flag", flag.ID, "", "")
	h.publishLifecycleEvent(c, models.EventFlagIterationExhausted, updated, nil)

	c.JSON(http.StatusOK, updated)
}

// parseLifecycleStatus validates the raw status string from a request body.
func parseLifecycleStatus(raw string) (models.LifecycleTestStatus, error) {
	switch raw {
	case string(models.LifecycleTestPending), string(models.LifecycleTestPass), string(models.LifecycleTestFail):
		return models.LifecycleTestStatus(raw), nil
	default:
		return "", errors.New("status must be one of pending, pass, fail")
	}
}
