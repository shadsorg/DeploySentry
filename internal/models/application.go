package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Application represents a deployable application within a project.
type Application struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description,omitempty"`
	RepoURL     string     `json:"repo_url,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Validate checks that the Application has all required fields populated.
func (a *Application) Validate() error {
	if a.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if a.Name == "" {
		return errors.New("name is required")
	}
	if a.Slug == "" {
		return errors.New("slug is required")
	}
	return nil
}
