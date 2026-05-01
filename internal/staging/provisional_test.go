package staging

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewProvisional_IsProvisional(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := NewProvisional()
		if !IsProvisional(id) {
			t.Fatalf("NewProvisional minted %s but IsProvisional returned false", id)
		}
	}
}

func TestProductionUUIDs_AreNotProvisional(t *testing.T) {
	// uuid.New() (v4) always sets variant bits to 10xx_xxxx — never 11xx_xxxx.
	for i := 0; i < 1_000; i++ {
		id := uuid.New()
		if IsProvisional(id) {
			t.Fatalf("uuid.New() produced %s which IsProvisional reports true", id)
		}
	}
}

func TestNilUUID_IsNotProvisional(t *testing.T) {
	if IsProvisional(uuid.Nil) {
		t.Fatal("uuid.Nil should not be provisional")
	}
}

func TestMustNotBeProvisional_PanicsOnProvisional(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustNotBeProvisional")
		}
	}()
	MustNotBeProvisional(NewProvisional(), "test-egress")
}

func TestMustNotBeProvisional_AllowsProductionUUID(t *testing.T) {
	// Should not panic.
	MustNotBeProvisional(uuid.New(), "test-egress")
	MustNotBeProvisional(uuid.Nil, "test-egress")
}
