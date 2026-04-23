package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDeploymentsCursor_RoundTrip(t *testing.T) {
	ts := time.Date(2026, 4, 23, 12, 34, 56, 789, time.UTC)
	id := uuid.New()

	encoded := encodeDeploymentsCursor(ts, id)
	assert.NotEmpty(t, encoded)

	gotTS, gotID, err := decodeDeploymentsCursor(encoded)
	assert.NoError(t, err)
	assert.True(t, gotTS.Equal(ts))
	assert.Equal(t, id, gotID)
}

func TestDeploymentsCursor_RejectsGarbage(t *testing.T) {
	_, _, err := decodeDeploymentsCursor("!!!not base64!!!")
	assert.Error(t, err)

	_, _, err = decodeDeploymentsCursor("Zm9vYmFy") // "foobar"
	assert.Error(t, err)
}
