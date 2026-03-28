package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func validFlagEnvironmentState() FlagEnvironmentState {
	return FlagEnvironmentState{
		ID:            uuid.New(),
		FlagID:        uuid.New(),
		EnvironmentID: uuid.New(),
	}
}

func TestFlagEnvironmentStateValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*FlagEnvironmentState)
		wantErr string
	}{
		{
			name:    "valid state",
			modify:  func(s *FlagEnvironmentState) {},
			wantErr: "",
		},
		{
			name:    "missing flag_id",
			modify:  func(s *FlagEnvironmentState) { s.FlagID = uuid.Nil },
			wantErr: "flag_id is required",
		},
		{
			name:    "missing environment_id",
			modify:  func(s *FlagEnvironmentState) { s.EnvironmentID = uuid.Nil },
			wantErr: "environment_id is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validFlagEnvironmentState()
			tc.modify(&s)
			err := s.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}
