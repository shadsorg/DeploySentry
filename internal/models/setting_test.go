package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func validSetting() Setting {
	orgID := uuid.New()
	return Setting{
		ID:    uuid.New(),
		OrgID: &orgID,
		Key:   "max_flags",
		Value: json.RawMessage(`100`),
	}
}

func TestSettingValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Setting)
		wantErr string
	}{
		{
			name:    "valid setting",
			modify:  func(s *Setting) {},
			wantErr: "",
		},
		{
			name:    "missing key",
			modify:  func(s *Setting) { s.Key = "" },
			wantErr: "key is required",
		},
		{
			name:    "missing value",
			modify:  func(s *Setting) { s.Value = nil },
			wantErr: "value is required",
		},
		{
			name:    "no scope set",
			modify:  func(s *Setting) { s.OrgID = nil },
			wantErr: "exactly one scope (org_id, project_id, application_id, environment_id) must be set",
		},
		{
			name: "multiple scopes set",
			modify: func(s *Setting) {
				pid := uuid.New()
				s.ProjectID = &pid
			},
			wantErr: "only one scope (org_id, project_id, application_id, environment_id) may be set",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validSetting()
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

func TestSettingScopeLevel(t *testing.T) {
	orgID := uuid.New()
	projID := uuid.New()
	appID := uuid.New()
	envID := uuid.New()

	tests := []struct {
		name  string
		setup func(*Setting)
		want  string
	}{
		{name: "org", setup: func(s *Setting) { s.OrgID = &orgID }, want: "org"},
		{name: "project", setup: func(s *Setting) { s.ProjectID = &projID }, want: "project"},
		{name: "application", setup: func(s *Setting) { s.ApplicationID = &appID }, want: "application"},
		{name: "environment", setup: func(s *Setting) { s.EnvironmentID = &envID }, want: "environment"},
		{name: "none", setup: func(s *Setting) {}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Setting{}
			tc.setup(&s)
			assert.Equal(t, tc.want, s.ScopeLevel())
		})
	}
}
