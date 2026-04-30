package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
)

func TestRevertRegistry_IsRevertible(t *testing.T) {
	r := NewRevertRegistry()
	r.Register("flag", "flag.archived", func(_ context.Context, _ *models.AuditLogEntry, _ bool) (string, error) {
		return "flag.archived.reverted", nil
	})

	t.Run("registered key returns true", func(t *testing.T) {
		if !r.IsRevertible("flag", "flag.archived") {
			t.Error("expected IsRevertible to return true for registered (flag, flag.archived)")
		}
	})

	t.Run("unregistered key returns false", func(t *testing.T) {
		if r.IsRevertible("flag", "flag.created") {
			t.Error("expected IsRevertible to return false for unregistered (flag, flag.created)")
		}
	})
}

func TestRevertRegistry_Revert_NotRevertible(t *testing.T) {
	r := NewRevertRegistry()

	entry := &models.AuditLogEntry{
		EntityType: "flag",
		Action:     "X",
	}

	_, err := r.Revert(context.Background(), entry, false)
	if !errors.Is(err, ErrNotRevertible) {
		t.Errorf("expected ErrNotRevertible, got %v", err)
	}
}

func TestRevertRegistry_Revert_DispatchesHandler(t *testing.T) {
	r := NewRevertRegistry()

	var capturedForce bool
	r.Register("flag", "flag.archived", func(_ context.Context, _ *models.AuditLogEntry, force bool) (string, error) {
		capturedForce = force
		return "test.action.reverted", nil
	})

	entry := &models.AuditLogEntry{
		EntityType: "flag",
		Action:     "flag.archived",
	}

	newAction, err := r.Revert(context.Background(), entry, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if newAction != "test.action.reverted" {
		t.Errorf("expected newAction %q, got %q", "test.action.reverted", newAction)
	}
	if !capturedForce {
		t.Error("expected force=true to be passed through to the handler")
	}
}
