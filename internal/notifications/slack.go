package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackConfig holds configuration for the Slack notification channel.
type SlackConfig struct {
	// WebhookURL is the Slack incoming webhook URL.
	WebhookURL string `json:"webhook_url"`

	// Channel is the Slack channel to post to (overrides webhook default).
	Channel string `json:"channel,omitempty"`

	// Username is the bot username displayed in Slack.
	Username string `json:"username,omitempty"`

	// IconEmoji is the emoji used as the bot's avatar.
	IconEmoji string `json:"icon_emoji,omitempty"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout"`
}

// slackMessage represents a Slack webhook payload.
type slackMessage struct {
	Channel   string            `json:"channel,omitempty"`
	Username  string            `json:"username,omitempty"`
	IconEmoji string            `json:"icon_emoji,omitempty"`
	Text      string            `json:"text"`
	Blocks    []slackBlock      `json:"blocks,omitempty"`
}

// slackBlock represents a Slack block kit element.
type slackBlock struct {
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

// slackText represents text content in a Slack block.
type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackChannel implements the Channel interface for delivering notifications
// via Slack incoming webhooks.
type SlackChannel struct {
	config SlackConfig
	client *http.Client
}

// NewSlackChannel creates a new Slack notification channel.
func NewSlackChannel(config SlackConfig) *SlackChannel {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Username == "" {
		config.Username = "DeploySentry"
	}
	return &SlackChannel{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the channel identifier.
func (s *SlackChannel) Name() string {
	return "slack"
}

// Supports reports whether the Slack channel handles the given event type.
// Slack receives all notification event types.
func (s *SlackChannel) Supports(_ EventType) bool {
	return true
}

// Send delivers a notification event to Slack via the configured webhook URL.
func (s *SlackChannel) Send(ctx context.Context, event *Event) error {
	msg := s.formatMessage(event)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

// formatMessage converts a notification event into a Slack message payload.
func (s *SlackChannel) formatMessage(event *Event) *slackMessage {
	text := formatEventText(event)

	msg := &slackMessage{
		Channel:   s.config.Channel,
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		Text:      text,
		Blocks: []slackBlock{
			{
				Type: "section",
				Text: &slackText{
					Type: "mrkdwn",
					Text: text,
				},
			},
		},
	}

	return msg
}

// formatEventText generates a human-readable text representation of an event.
func formatEventText(event *Event) string {
	project := event.Data["project_name"]
	if project == "" {
		project = event.ProjectID
	}

	switch event.Type {
	case EventDeployStarted:
		return fmt.Sprintf("Deployment started for *%s* (version: %s)", project, event.Data["version"])
	case EventDeployCompleted:
		return fmt.Sprintf("Deployment completed for *%s* (version: %s)", project, event.Data["version"])
	case EventDeployFailed:
		return fmt.Sprintf("Deployment FAILED for *%s* (version: %s): %s", project, event.Data["version"], event.Data["error"])
	case EventDeployRolledBack:
		return fmt.Sprintf("Deployment rolled back for *%s* (version: %s): %s", project, event.Data["version"], event.Data["reason"])
	case EventFlagToggled:
		return fmt.Sprintf("Feature flag *%s* toggled to %s in *%s*", event.Data["flag_key"], event.Data["enabled"], project)
	case EventReleaseCreated:
		return fmt.Sprintf("Release *%s* created for *%s*", event.Data["version"], project)
	case EventReleasePromoted:
		return fmt.Sprintf("Release *%s* promoted to %s in *%s*", event.Data["version"], event.Data["environment"], project)
	case EventHealthDegraded:
		return fmt.Sprintf("Health degraded for *%s*: score %s", project, event.Data["score"])
	default:
		return fmt.Sprintf("Event %s for %s", event.Type, project)
	}
}
