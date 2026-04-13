package postgres

import (
	"testing"

	"github.com/google/uuid"
)

func TestAdvisoryLockKey_Deterministic(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	k1 := advisoryLockKey(id)
	k2 := advisoryLockKey(id)
	if k1 != k2 {
		t.Errorf("advisoryLockKey not deterministic: %d != %d", k1, k2)
	}
}

func TestAdvisoryLockKey_DifferentUUIDs(t *testing.T) {
	id1 := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	id2 := uuid.MustParse("660e8400-e29b-41d4-a716-446655440000")
	k1 := advisoryLockKey(id1)
	k2 := advisoryLockKey(id2)
	if k1 == k2 {
		t.Errorf("different UUIDs should produce different keys: %d == %d", k1, k2)
	}
}
