package rollout

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RolloutHandler serves rollout list, detail, and 6 runtime control endpoints.
type RolloutHandler struct {
	svc *RolloutService
}

// NewRolloutHandler builds the handler.
func NewRolloutHandler(svc *RolloutService) *RolloutHandler {
	return &RolloutHandler{svc: svc}
}

// RegisterRoutes mounts routes under /orgs/:orgSlug/.
func (h *RolloutHandler) RegisterRoutes(api *gin.RouterGroup) {
	org := api.Group("/orgs/:orgSlug")
	org.GET("/rollouts", h.list)
	org.GET("/rollouts/:id", h.get)
	org.POST("/rollouts/:id/pause", h.pause)
	org.POST("/rollouts/:id/resume", h.resume)
	org.POST("/rollouts/:id/rollback", h.rollback)
	org.POST("/rollouts/:id/promote", h.promote)
	org.POST("/rollouts/:id/approve", h.approve)
	org.POST("/rollouts/:id/force-promote", h.forcePromote)
	org.GET("/rollouts/:id/events", h.events)
	org.GET("/rollouts/:id/events/stream", h.eventsStream)
}

func (h *RolloutHandler) list(c *gin.Context) {
	opts := RolloutListOptions{}
	if s := c.Query("status"); s != "" {
		st := models.RolloutStatus(s)
		opts.Status = &st
	}
	if t := c.Query("target_type"); t != "" {
		tt := models.TargetType(t)
		opts.TargetType = &tt
	}
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			opts.Limit = n
		}
	}
	rows, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *RolloutHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ro, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ro)
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func actorFromContext(c *gin.Context) uuid.UUID {
	// Auth middleware sets "user_id" in the context. Fallback to uuid.Nil.
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

// ctx is a type alias used inside runControl closures to keep signatures terse.
type ctx = *gin.Context

func (h *RolloutHandler) runControl(c *gin.Context, fn func(c ctx, id uuid.UUID, actor uuid.UUID, reason string) error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	actor := actorFromContext(c)
	if err := fn(c, id, actor, body.Reason); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrReasonRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidStateForOp):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *RolloutHandler) pause(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Pause(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) resume(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Resume(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) rollback(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Rollback(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) promote(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Promote(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) approve(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.Approve(c.Request.Context(), id, actor, reason)
	})
}
func (h *RolloutHandler) forcePromote(c *gin.Context) {
	h.runControl(c, func(c ctx, id, actor uuid.UUID, reason string) error {
		return h.svc.ForcePromote(c.Request.Context(), id, actor, reason)
	})
}

// eventsStream is a Server-Sent Events endpoint that pushes rollout state
// updates to the client. It emits an initial "snapshot" event, then polls
// every 2 seconds for status changes and new events. The stream closes when
// the client disconnects or the rollout reaches a terminal state.
func (h *RolloutHandler) eventsStream(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ctx := c.Request.Context()

	emit := func(event string, payload any) bool {
		data, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	// Initial snapshot.
	ro, err := h.svc.Get(ctx, id)
	if err != nil {
		emit("error", gin.H{"error": err.Error()})
		return
	}
	if !emit("snapshot", ro) {
		return
	}

	lastStatus := ro.Status
	lastPhaseIdx := ro.CurrentPhaseIndex

	// Emit recent events oldest-first so the client builds a chronological log.
	var lastEventID uuid.UUID
	initialEvents, _ := h.svc.Events(ctx, id, 10)
	for i := len(initialEvents) - 1; i >= 0; i-- {
		emit("event", initialEvents[i])
		lastEventID = initialEvents[i].ID
	}

	// Close immediately if already terminal.
	if ro.IsTerminal() {
		emit("done", ro)
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ro, err := h.svc.Get(ctx, id)
			if err != nil {
				emit("error", gin.H{"error": err.Error()})
				return
			}
			if ro.Status != lastStatus || ro.CurrentPhaseIndex != lastPhaseIdx {
				emit("update", ro)
				lastStatus = ro.Status
				lastPhaseIdx = ro.CurrentPhaseIndex
			}
			// Emit new events since lastEventID in chronological order.
			evs, _ := h.svc.Events(ctx, id, 50)
			var fresh []*models.RolloutEvent
			for _, ev := range evs {
				if lastEventID != uuid.Nil && ev.ID == lastEventID {
					break
				}
				fresh = append(fresh, ev)
			}
			for i := len(fresh) - 1; i >= 0; i-- {
				emit("event", fresh[i])
				lastEventID = fresh[i].ID
			}
			if ro.IsTerminal() {
				emit("done", ro)
				return
			}
		}
	}
}

// events streams rollout events (simple long-poll for now; SSE upgrade can be
// added without API change).
func (h *RolloutHandler) events(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	events, err := h.svc.Events(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events})
}
