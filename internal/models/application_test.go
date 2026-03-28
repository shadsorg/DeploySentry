package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func validApplication() Application {
	return Application{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "My App",
		Slug:      "my-app",
	}
}

func TestApplicationValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Application)
		wantErr string
	}{
		{
			name:    "valid application",
			modify:  func(a *Application) {},
			wantErr: "",
		},
		{
			name:    "missing project_id",
			modify:  func(a *Application) { a.ProjectID = uuid.Nil },
			wantErr: "project_id is required",
		},
		{
			name:    "missing name",
			modify:  func(a *Application) { a.Name = "" },
			wantErr: "name is required",
		},
		{
			name:    "missing slug",
			modify:  func(a *Application) { a.Slug = "" },
			wantErr: "slug is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := validApplication()
			tc.modify(&a)
			err := a.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}
