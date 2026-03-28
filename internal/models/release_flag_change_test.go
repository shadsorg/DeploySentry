package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func validReleaseFlagChange() ReleaseFlagChange {
	return ReleaseFlagChange{
		ID:            uuid.New(),
		ReleaseID:     uuid.New(),
		FlagID:        uuid.New(),
		EnvironmentID: uuid.New(),
	}
}

func TestReleaseFlagChangeValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*ReleaseFlagChange)
		wantErr string
	}{
		{
			name:    "valid change",
			modify:  func(c *ReleaseFlagChange) {},
			wantErr: "",
		},
		{
			name:    "missing release_id",
			modify:  func(c *ReleaseFlagChange) { c.ReleaseID = uuid.Nil },
			wantErr: "release_id is required",
		},
		{
			name:    "missing flag_id",
			modify:  func(c *ReleaseFlagChange) { c.FlagID = uuid.Nil },
			wantErr: "flag_id is required",
		},
		{
			name:    "missing environment_id",
			modify:  func(c *ReleaseFlagChange) { c.EnvironmentID = uuid.Nil },
			wantErr: "environment_id is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validReleaseFlagChange()
			tc.modify(&c)
			err := c.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}
