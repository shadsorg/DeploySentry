package notifications

import (
	"context"
	"fmt"
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
}

// EmailChannel implements the Channel interface for delivering notifications
// via email using SMTP.
type EmailChannel struct {
	config EmailConfig
}

// NewEmailChannel creates a new email notification channel.
func NewEmailChannel(config EmailConfig) *EmailChannel {
	if config.FromName == "" {
		config.FromName = "DeploySentry"
	}
	return &EmailChannel{config: config}
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
func (e *EmailChannel) Send(ctx context.Context, event *Event) error {
	if len(e.config.Recipients) == 0 {
		return nil
	}

	subject := e.formatSubject(event)
	body := e.formatBody(event)

	msg := e.buildMessage(subject, body)

	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)

	var auth smtp.Auth
	if e.config.Username != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPHost)
	}

	err := smtp.SendMail(addr, auth, e.config.FromAddress, e.config.Recipients, []byte(msg))
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}

	return nil
}

// buildMessage constructs an RFC 2822 email message.
func (e *EmailChannel) buildMessage(subject, body string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s <%s>\r\n", e.config.FromName, e.config.FromAddress))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.config.Recipients, ", ")))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
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
	default:
		return fmt.Sprintf("[DeploySentry] %s - %s", event.Type, project)
	}
}

// formatBody generates an email body from the event.
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
