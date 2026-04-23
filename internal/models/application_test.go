package models

import (
	"strings"
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

func TestValidateMonitoringLinks_HappyPath(t *testing.T) {
	out, err := ValidateMonitoringLinks([]MonitoringLink{
		{Label: "Datadog", URL: "https://app.datadoghq.com/x", Icon: "datadog"},
		{Label: "Runbook", URL: "https://notion.so/x", Icon: "custom"},
	})
	assert.NoError(t, err)
	assert.Len(t, out, 2)
	assert.Equal(t, "Datadog", out[0].Label)
}

func TestValidateMonitoringLinks_TrimsAndClearsBlankIcon(t *testing.T) {
	out, err := ValidateMonitoringLinks([]MonitoringLink{
		{Label: "  Logs  ", URL: "https://x.com", Icon: " "},
	})
	assert.NoError(t, err)
	assert.Equal(t, "Logs", out[0].Label)
	assert.Equal(t, "", out[0].Icon)
}

func TestValidateMonitoringLinks_MaxCount(t *testing.T) {
	links := make([]MonitoringLink, MonitoringLinkMaxCount+1)
	for i := range links {
		links[i] = MonitoringLink{Label: "L", URL: "https://x.com"}
	}
	_, err := ValidateMonitoringLinks(links)
	assert.Error(t, err)
}

func TestValidateMonitoringLinks_RequiresFields(t *testing.T) {
	_, err := ValidateMonitoringLinks([]MonitoringLink{{URL: "https://x.com"}})
	assert.Error(t, err)

	_, err = ValidateMonitoringLinks([]MonitoringLink{{Label: "L"}})
	assert.Error(t, err)
}

func TestValidateMonitoringLinks_RejectsNonHTTP(t *testing.T) {
	_, err := ValidateMonitoringLinks([]MonitoringLink{{Label: "L", URL: "ftp://x.com"}})
	assert.Error(t, err)

	_, err = ValidateMonitoringLinks([]MonitoringLink{{Label: "L", URL: "javascript:alert(1)"}})
	assert.Error(t, err)
}

func TestValidateMonitoringLinks_RejectsUnknownIcon(t *testing.T) {
	_, err := ValidateMonitoringLinks([]MonitoringLink{{Label: "L", URL: "https://x.com", Icon: "not-real"}})
	assert.Error(t, err)
}

func TestValidateMonitoringLinks_EnforcesLabelLen(t *testing.T) {
	_, err := ValidateMonitoringLinks([]MonitoringLink{{Label: strings.Repeat("x", MonitoringLinkMaxLabelLen+1), URL: "https://x.com"}})
	assert.Error(t, err)
}
