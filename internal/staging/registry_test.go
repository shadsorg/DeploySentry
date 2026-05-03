package staging

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
)

func TestCommitRegistry_DispatchUnknown(t *testing.T) {
	r := NewCommitRegistry()
	row := &models.StagedChange{ResourceType: "flag", Action: "toggle"}
	if _, err := r.Dispatch(context.Background(), nil, row); !errors.Is(err, ErrNoCommitHandler) {
		t.Fatalf("expected ErrNoCommitHandler, got %v", err)
	}
}

func TestCommitRegistry_IsCommittable(t *testing.T) {
	r := NewCommitRegistry()
	if r.IsCommittable("flag", "toggle") {
		t.Fatal("empty registry must not report committable")
	}
	r.Register("flag", "toggle", func(context.Context, pgx.Tx, *models.StagedChange) (string, error) {
		return "flag.toggled", nil
	})
	if !r.IsCommittable("flag", "toggle") {
		t.Fatal("registered handler should be committable")
	}
}

func TestCommitRegistry_Dispatch(t *testing.T) {
	r := NewCommitRegistry()
	called := false
	r.Register("flag", "toggle", func(_ context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		called = true
		if row.ResourceType != "flag" || row.Action != "toggle" {
			t.Fatalf("dispatch passed wrong row: %+v", row)
		}
		return "flag.toggled", nil
	})
	rid := uuid.New()
	row := &models.StagedChange{ResourceType: "flag", Action: "toggle", ResourceID: &rid}
	action, err := r.Dispatch(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if action != "flag.toggled" {
		t.Fatalf("expected flag.toggled, got %s", action)
	}
	if !called {
		t.Fatal("handler was not invoked")
	}
}

func TestCommitRegistry_RegisterOverwrites(t *testing.T) {
	r := NewCommitRegistry()
	r.Register("flag", "toggle", func(context.Context, pgx.Tx, *models.StagedChange) (string, error) {
		return "first", nil
	})
	r.Register("flag", "toggle", func(context.Context, pgx.Tx, *models.StagedChange) (string, error) {
		return "second", nil
	})
	action, _ := r.Dispatch(context.Background(), nil, &models.StagedChange{ResourceType: "flag", Action: "toggle"})
	if action != "second" {
		t.Fatalf("expected second handler to win, got %s", action)
	}
}

func TestCreateRegistryDispatchReturnsRealID(t *testing.T) {
	r := NewCreateRegistry()
	wantReal := uuid.New()
	called := false
	r.Register("flag", "create", func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		called = true
		return wantReal, "flag.created", nil, nil
	})
	row := &models.StagedChange{ResourceType: "flag", Action: "create"}
	gotReal, audit, hook, err := r.Dispatch(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !called {
		t.Fatal("handler not invoked")
	}
	if gotReal != wantReal {
		t.Errorf("realID mismatch: got %v want %v", gotReal, wantReal)
	}
	if audit != "flag.created" {
		t.Errorf("audit mismatch: %v", audit)
	}
	if hook != nil {
		t.Errorf("expected nil postCommit hook")
	}
}

func TestCreateRegistryIsCreatable(t *testing.T) {
	r := NewCreateRegistry()
	if r.IsCreatable("flag", "create") {
		t.Fatal("empty registry should not report flag.create creatable")
	}
	r.Register("flag", "create", func(context.Context, pgx.Tx, *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		return uuid.Nil, "", nil, nil
	})
	if !r.IsCreatable("flag", "create") {
		t.Fatal("registered key should be creatable")
	}
}

func TestCreateRegistryDispatchUnknownErrors(t *testing.T) {
	r := NewCreateRegistry()
	row := &models.StagedChange{ResourceType: "flag", Action: "create"}
	_, _, _, err := r.Dispatch(context.Background(), nil, row)
	if !errors.Is(err, ErrNoCreateHandler) {
		t.Errorf("expected ErrNoCreateHandler, got %v", err)
	}
}
