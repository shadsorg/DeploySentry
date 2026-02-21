package notifications

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

// EmailConfig holds configuration for the email notification channel.
type EmailConfig struct {
	// SMTPHost is the SMTP server hostname.
	SMTPHost string `json:"smtp_host"`

	// SMTPPort is the SMTP server port.
	SMTPPort int `json:"smtp_port"`

	// Username is the SMTP authentication username.
	Username string `json:"username"`

	// Password is the SMTP authentication password.
	Password string `json:"password"`

	// FromAddress is the sender email address.
	FromAddress string `json:"from_address"`

	// FromName is the sender display name.
	FromName string `json:"from_name"`

	// Recipients is the list of email addresses to notify.
	Recipients []string `json:"recipients"`

	// EventTypes filters which event types trigger email notifications.
	// An empty slice means all events trigger emails.
	EventTypes []EventType `json:"event_types,omitempty"`

	// UseHTML enables HTML email rendering instead of plain text.
	UseHTML bool `json:"use_html"`

	// PreferenceStore is an optional store for checking per-user notification
	// preferences before sending. If nil, emails are sent to all recipients.
	PreferenceStore PreferenceStore `json:"-"`
}

// EmailChannel implements the Channel interface for delivering notifications
// via email using SMTP.
type EmailChannel struct {
	config    EmailConfig
	htmlTmpl  *template.Template
}

// NewEmailChannel creates a new email notification channel.
func NewEmailChannel(config EmailConfig) *EmailChannel {
	if config.FromName == "" {
		config.FromName = "DeploySentry"
	}
	ch := &EmailChannel{config: config}
	ch.htmlTmpl = ch.compileHTMLTemplates()
	return ch
}

// Name returns the channel identifier.
func (e *EmailChannel) Name() string {
	return "email"
}

// Supports reports whether the email channel handles the given event type.
func (e *EmailChannel) Supports(eventType EventType) bool {
	if len(e.config.EventTypes) == 0 {
		return true
	}
	for _, et := range e.config.EventTypes {
		if et == eventType {
			return true
		}
	}
	return false
}

// Send delivers a notification event via email to all configured recipients.
// When a PreferenceStore is configured, recipients who have disabled
// notifications for the event type are skipped.
func (e *EmailChannel) Send(ctx context.Context, event *Event) error {
	recipients := e.filteredRecipients(ctx, event)
	if len(recipients) == 0 {
		return nil
	}

	subject := e.formatSubject(event)

	var body string
	var contentType string
	if e.config.UseHTML {
		body = e.renderHTML(event)
		contentType = "text/html"
	} else {
		body = e.formatBody(event)
		contentType = "text/plain"
	}

	msg := e.buildMessageFull(subject, body, contentType, recipients)

	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)

	var auth smtp.Auth
	if e.config.Username != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPHost)
	}

	err := smtp.SendMail(addr, auth, e.config.FromAddress, recipients, []byte(msg))
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}

	return nil
}

// filteredRecipients returns the recipients who should receive the email for
// the given event, respecting per-user notification preferences.
func (e *EmailChannel) filteredRecipients(ctx context.Context, event *Event) []string {
	if e.config.PreferenceStore == nil {
		return e.config.Recipients
	}

	var recipients []string
	for _, r := range e.config.Recipients {
		prefs, err := e.config.PreferenceStore.GetPreferences(ctx, r, event.ProjectID)
		if err != nil {
			// On error, include the recipient (fail open for delivery).
			recipients = append(recipients, r)
			continue
		}
		if prefs.IsEnabled(event.Type, "email") {
			recipients = append(recipients, r)
		}
	}
	return recipients
}

// buildMessage constructs an RFC 2822 email message with plain text content type.
func (e *EmailChannel) buildMessage(subject, body string) string {
	return e.buildMessageFull(subject, body, "text/plain", e.config.Recipients)
}

// buildMessageFull constructs an RFC 2822 email message with the specified
// content type and recipient list.
func (e *EmailChannel) buildMessageFull(subject, body, contentType string, recipients []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s <%s>\r\n", e.config.FromName, e.config.FromAddress))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", contentType))
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// formatSubject generates an email subject line from the event.
func (e *EmailChannel) formatSubject(event *Event) string {
	project := event.Data["project_name"]
	if project == "" {
		project = event.ProjectID
	}

	switch event.Type {
	case EventDeployStarted:
		return fmt.Sprintf("[DeploySentry] Deployment started - %s", project)
	case EventDeployCompleted:
		return fmt.Sprintf("[DeploySentry] Deployment completed - %s", project)
	case EventDeployFailed:
		return fmt.Sprintf("[DeploySentry] Deployment FAILED - %s", project)
	case EventDeployRolledBack:
		return fmt.Sprintf("[DeploySentry] Deployment rolled back - %s", project)
	case EventFlagToggled:
		return fmt.Sprintf("[DeploySentry] Feature flag toggled - %s", project)
	case EventReleaseCreated:
		return fmt.Sprintf("[DeploySentry] Release created - %s", project)
	case EventReleasePromoted:
		return fmt.Sprintf("[DeploySentry] Release promoted - %s", project)
	case EventHealthDegraded:
		return fmt.Sprintf("[DeploySentry] Health degraded - %s", project)
	case EventDeployPhaseCompleted:
		return fmt.Sprintf("[DeploySentry] Deployment phase completed - %s", project)
	case EventDeployRollbackInitiated:
		return fmt.Sprintf("[DeploySentry] Rollback initiated - %s", project)
	case EventHealthAlertResolved:
		return fmt.Sprintf("[DeploySentry] Health alert resolved - %s", project)
	case EventHealthAlertTriggered:
		return fmt.Sprintf("[DeploySentry] Health alert triggered - %s", project)
	default:
		return fmt.Sprintf("[DeploySentry] %s - %s", event.Type, project)
	}
}

// formatBody generates a plain-text email body from the event.
func (e *EmailChannel) formatBody(event *Event) string {
	var sb strings.Builder
	sb.WriteString(formatEventText(event))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", event.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Organization: %s\n", event.OrgID))
	sb.WriteString(fmt.Sprintf("Project: %s\n", event.ProjectID))

	if len(event.Data) > 0 {
		sb.WriteString("\nDetails:\n")
		for k, v := range event.Data {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	sb.WriteString("\n--\nDeploySentry Notifications\n")
	return sb.String()
}

// emailTemplateData is the data passed to HTML email templates.
type emailTemplateData struct {
	Title     string
	Summary   string
	Timestamp string
	OrgID     string
	ProjectID string
	Project   string
	Data      map[string]string
}

// compileHTMLTemplates parses and returns the HTML email template.
func (e *EmailChannel) compileHTMLTemplates() *template.Template {
	const emailHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f5f5f5; }
.container { max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 8px; overflow: hidden; }
.header { background-color: #1a1a2e; color: #ffffff; padding: 24px; text-align: center; }
.header h1 { margin: 0; font-size: 20px; font-weight: 600; }
.content { padding: 24px; }
.summary { font-size: 16px; color: #333333; margin-bottom: 20px; line-height: 1.5; }
.details { background-color: #f8f9fa; border-radius: 6px; padding: 16px; margin-bottom: 20px; }
.details table { width: 100%; border-collapse: collapse; }
.details td { padding: 6px 8px; font-size: 14px; color: #555555; }
.details td:first-child { font-weight: 600; color: #333333; white-space: nowrap; width: 120px; }
.meta { font-size: 12px; color: #999999; border-top: 1px solid #eeeeee; padding-top: 16px; }
.footer { text-align: center; padding: 16px; font-size: 12px; color: #999999; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>{{.Title}}</h1>
  </div>
  <div class="content">
    <div class="summary">{{.Summary}}</div>
    {{if .Data}}
    <div class="details">
      <table>
        {{range $k, $v := .Data}}
        <tr><td>{{$k}}</td><td>{{$v}}</td></tr>
        {{end}}
      </table>
    </div>
    {{end}}
    <div class="meta">
      <p>Timestamp: {{.Timestamp}}</p>
      <p>Organization: {{.OrgID}}</p>
      <p>Project: {{.ProjectID}}</p>
    </div>
  </div>
  <div class="footer">
    DeploySentry Notifications
  </div>
</div>
</body>
</html>`

	tmpl, _ := template.New("email").Parse(emailHTML)
	return tmpl
}

// renderHTML renders the event as an HTML email body.
func (e *EmailChannel) renderHTML(event *Event) string {
	project := event.Data["project_name"]
	if project == "" {
		project = event.ProjectID
	}

	data := emailTemplateData{
		Title:     e.formatSubject(event),
		Summary:   formatEventText(event),
		Timestamp: event.Timestamp.Format("2006-01-02 15:04:05 UTC"),
		OrgID:     event.OrgID,
		ProjectID: event.ProjectID,
		Project:   project,
		Data:      event.Data,
	}

	var buf bytes.Buffer
	if err := e.htmlTmpl.Execute(&buf, data); err != nil {
		// Fall back to plain text on template error.
		return e.formatBody(event)
	}
	return buf.String()
}
