package notifications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock Channel
// ---------------------------------------------------------------------------

type mockChannel struct {
	name       string
	supports   map[EventType]bool
	sendErr    error
	sentEvents []*Event
	mu         sync.Mutex
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Supports(et EventType) bool {
	if m.supports == nil {
		return true
	}
	return m.supports[et]
}

func (m *mockChannel) Send(ctx context.Context, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentEvents = append(m.sentEvents, event)
	return m.sendErr
}

// helper to safely read sentEvents from the mock
func (m *mockChannel) getSentEvents() []*Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Event, len(m.sentEvents))
	copy(out, m.sentEvents)
	return out
}

// ---------------------------------------------------------------------------
// Helper: sample event used across many tests
// ---------------------------------------------------------------------------

func sampleEvent(eventType EventType) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		OrgID:     "org-1",
		ProjectID: "proj-1",
		Data: map[string]string{
			"project_name": "my-service",
			"version":      "v1.2.3",
			"error":        "timeout",
			"reason":       "high error rate",
			"flag_key":     "dark-mode",
			"enabled":      "true",
			"environment":  "production",
			"score":        "42",
		},
	}
}

// ===========================================================================
// NotificationService Tests
// ===========================================================================

func TestRegisterChannel(t *testing.T) {
	svc := NewNotificationService()

	ch1 := &mockChannel{name: "ch1"}
	ch2 := &mockChannel{name: "ch2"}

	svc.RegisterChannel(ch1)
	svc.RegisterChannel(ch2)

	// Dispatch an event and verify both channels received it.
	event := sampleEvent(EventDeployStarted)
	err := svc.Dispatch(context.Background(), event)
	assert.NoError(t, err)
	assert.Len(t, ch1.getSentEvents(), 1)
	assert.Len(t, ch2.getSentEvents(), 1)
}

func TestDispatch_SendsToAllSupportingChannels(t *testing.T) {
	svc := NewNotificationService()

	ch1 := &mockChannel{name: "ch1"}
	ch2 := &mockChannel{name: "ch2"}
	ch3 := &mockChannel{name: "ch3"}
	svc.RegisterChannel(ch1)
	svc.RegisterChannel(ch2)
	svc.RegisterChannel(ch3)

	event := sampleEvent(EventDeployCompleted)
	err := svc.Dispatch(context.Background(), event)
	assert.NoError(t, err)

	assert.Len(t, ch1.getSentEvents(), 1)
	assert.Len(t, ch2.getSentEvents(), 1)
	assert.Len(t, ch3.getSentEvents(), 1)
}

func TestDispatch_SkipsUnsupportedChannels(t *testing.T) {
	svc := NewNotificationService()

	supporting := &mockChannel{
		name:     "supporting",
		supports: map[EventType]bool{EventDeployFailed: true},
	}
	notSupporting := &mockChannel{
		name:     "not-supporting",
		supports: map[EventType]bool{EventDeployFailed: false},
	}

	svc.RegisterChannel(supporting)
	svc.RegisterChannel(notSupporting)

	event := sampleEvent(EventDeployFailed)
	err := svc.Dispatch(context.Background(), event)
	assert.NoError(t, err)

	assert.Len(t, supporting.getSentEvents(), 1)
	assert.Len(t, notSupporting.getSentEvents(), 0)
}

func TestDispatch_SetsTimestampIfZero(t *testing.T) {
	svc := NewNotificationService()

	ch := &mockChannel{name: "ch"}
	svc.RegisterChannel(ch)

	event := &Event{
		Type:      EventDeployStarted,
		OrgID:     "org-1",
		ProjectID: "proj-1",
		Data:      map[string]string{"version": "v1"},
	}
	assert.True(t, event.Timestamp.IsZero())

	before := time.Now().UTC()
	err := svc.Dispatch(context.Background(), event)
	after := time.Now().UTC()

	assert.NoError(t, err)
	assert.False(t, event.Timestamp.IsZero())
	assert.True(t, event.Timestamp.Equal(before) || event.Timestamp.After(before))
	assert.True(t, event.Timestamp.Equal(after) || event.Timestamp.Before(after))
}

func TestDispatch_CollectsErrorsFromFailedChannels(t *testing.T) {
	svc := NewNotificationService()

	failing := &mockChannel{
		name:    "failing",
		sendErr: fmt.Errorf("smtp down"),
	}
	svc.RegisterChannel(failing)

	event := sampleEvent(EventDeployFailed)
	err := svc.Dispatch(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failing")
	assert.Contains(t, err.Error(), "smtp down")
}

func TestDispatch_ReturnsNilWhenAllSucceed(t *testing.T) {
	svc := NewNotificationService()

	svc.RegisterChannel(&mockChannel{name: "a"})
	svc.RegisterChannel(&mockChannel{name: "b"})

	event := sampleEvent(EventReleaseCreated)
	err := svc.Dispatch(context.Background(), event)
	assert.NoError(t, err)
}

func TestDispatch_MultipleChannelsSomeFailSomeSucceed(t *testing.T) {
	svc := NewNotificationService()

	ok1 := &mockChannel{name: "ok1"}
	fail1 := &mockChannel{name: "fail1", sendErr: fmt.Errorf("network error")}
	ok2 := &mockChannel{name: "ok2"}
	fail2 := &mockChannel{name: "fail2", sendErr: fmt.Errorf("auth error")}

	svc.RegisterChannel(ok1)
	svc.RegisterChannel(fail1)
	svc.RegisterChannel(ok2)
	svc.RegisterChannel(fail2)

	event := sampleEvent(EventDeployStarted)
	err := svc.Dispatch(context.Background(), event)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fail1")
	assert.Contains(t, err.Error(), "network error")
	assert.Contains(t, err.Error(), "fail2")
	assert.Contains(t, err.Error(), "auth error")

	// Successful channels still received the event.
	assert.Len(t, ok1.getSentEvents(), 1)
	assert.Len(t, ok2.getSentEvents(), 1)
	// Failed channels also received the event (Send was called).
	assert.Len(t, fail1.getSentEvents(), 1)
	assert.Len(t, fail2.getSentEvents(), 1)
}

func TestDispatchAsync_DoesNotPanic(t *testing.T) {
	svc := NewNotificationService()

	ch := &mockChannel{name: "async-ch"}
	svc.RegisterChannel(ch)

	event := sampleEvent(EventDeployCompleted)

	// DispatchAsync should not panic.
	assert.NotPanics(t, func() {
		svc.DispatchAsync(context.Background(), event)
	})

	// Give the goroutine time to complete.
	time.Sleep(100 * time.Millisecond)

	assert.Len(t, ch.getSentEvents(), 1)
}

// ===========================================================================
// Email Channel Tests
// ===========================================================================

func TestNewEmailChannel_DefaultFromName(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{
		FromAddress: "deploy@example.com",
	})
	assert.Equal(t, "DeploySentry", ch.config.FromName)
}

func TestNewEmailChannel_CustomFromName(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{
		FromName:    "Custom Sender",
		FromAddress: "deploy@example.com",
	})
	assert.Equal(t, "Custom Sender", ch.config.FromName)
}

func TestEmailChannel_Name(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{})
	assert.Equal(t, "email", ch.Name())
}

func TestEmailChannel_Supports_EmptyEventTypes(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{})
	// Empty EventTypes means all events are supported.
	assert.True(t, ch.Supports(EventDeployStarted))
	assert.True(t, ch.Supports(EventDeployCompleted))
	assert.True(t, ch.Supports(EventDeployFailed))
	assert.True(t, ch.Supports(EventDeployRolledBack))
	assert.True(t, ch.Supports(EventFlagToggled))
	assert.True(t, ch.Supports(EventReleaseCreated))
	assert.True(t, ch.Supports(EventReleasePromoted))
	assert.True(t, ch.Supports(EventHealthDegraded))
}

func TestEmailChannel_Supports_SpecificEventTypes(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{
		EventTypes: []EventType{EventDeployFailed, EventHealthDegraded},
	})
	assert.True(t, ch.Supports(EventDeployFailed))
	assert.True(t, ch.Supports(EventHealthDegraded))
	assert.False(t, ch.Supports(EventDeployStarted))
	assert.False(t, ch.Supports(EventDeployCompleted))
	assert.False(t, ch.Supports(EventReleaseCreated))
}

func TestEmailChannel_FormatSubject(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{})

	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventDeployStarted, "[DeploySentry] Deployment started - my-service"},
		{EventDeployCompleted, "[DeploySentry] Deployment completed - my-service"},
		{EventDeployFailed, "[DeploySentry] Deployment FAILED - my-service"},
		{EventDeployRolledBack, "[DeploySentry] Deployment rolled back - my-service"},
		{EventFlagToggled, "[DeploySentry] Feature flag toggled - my-service"},
		{EventReleaseCreated, "[DeploySentry] Release created - my-service"},
		{EventReleasePromoted, "[DeploySentry] Release promoted - my-service"},
		{EventHealthDegraded, "[DeploySentry] Health degraded - my-service"},
	}

	for _, tc := range tests {
		t.Run(string(tc.eventType), func(t *testing.T) {
			event := sampleEvent(tc.eventType)
			subject := ch.formatSubject(event)
			assert.Equal(t, tc.expected, subject)
		})
	}
}

func TestEmailChannel_FormatSubject_DefaultCase(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{})
	event := sampleEvent(EventType("custom.event"))
	subject := ch.formatSubject(event)
	assert.Equal(t, "[DeploySentry] custom.event - my-service", subject)
}

func TestEmailChannel_FormatSubject_FallsBackToProjectID(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{})
	event := &Event{
		Type:      EventDeployStarted,
		ProjectID: "proj-fallback",
		Data:      map[string]string{},
	}
	subject := ch.formatSubject(event)
	assert.Contains(t, subject, "proj-fallback")
}

func TestEmailChannel_FormatBody(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{})
	event := sampleEvent(EventDeployStarted)
	body := ch.formatBody(event)

	assert.Contains(t, body, "Deployment started")
	assert.Contains(t, body, "my-service")
	assert.Contains(t, body, "2025-06-15 12:00:00 UTC")
	assert.Contains(t, body, "org-1")
	assert.Contains(t, body, "proj-1")
	assert.Contains(t, body, "Details:")
	assert.Contains(t, body, "DeploySentry Notifications")
}

func TestEmailChannel_BuildMessage(t *testing.T) {
	ch := NewEmailChannel(EmailConfig{
		FromAddress: "deploy@example.com",
		FromName:    "DeploySentry",
		Recipients:  []string{"alice@example.com", "bob@example.com"},
	})

	msg := ch.buildMessage("Test Subject", "Test Body")

	// Verify RFC 2822 headers.
	assert.Contains(t, msg, "From: DeploySentry <deploy@example.com>\r\n")
	assert.Contains(t, msg, "To: alice@example.com, bob@example.com\r\n")
	assert.Contains(t, msg, "Subject: Test Subject\r\n")
	assert.Contains(t, msg, "MIME-Version: 1.0\r\n")
	assert.Contains(t, msg, "Content-Type: text/plain; charset=UTF-8\r\n")
	// Header/body separator.
	assert.Contains(t, msg, "\r\n\r\n")
	// Body content appears after the separator.
	parts := strings.SplitN(msg, "\r\n\r\n", 2)
	require.Len(t, parts, 2)
	assert.Equal(t, "Test Body", parts[1])
}

// ===========================================================================
// Slack Channel Tests
// ===========================================================================

func TestNewSlackChannel_DefaultTimeout(t *testing.T) {
	ch := NewSlackChannel(SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
	})
	assert.Equal(t, 10*time.Second, ch.config.Timeout)
}

func TestNewSlackChannel_DefaultUsername(t *testing.T) {
	ch := NewSlackChannel(SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
	})
	assert.Equal(t, "DeploySentry", ch.config.Username)
}

func TestNewSlackChannel_CustomValues(t *testing.T) {
	ch := NewSlackChannel(SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
		Username:   "MyBot",
		Timeout:    5 * time.Second,
	})
	assert.Equal(t, "MyBot", ch.config.Username)
	assert.Equal(t, 5*time.Second, ch.config.Timeout)
}

func TestSlackChannel_Name(t *testing.T) {
	ch := NewSlackChannel(SlackConfig{})
	assert.Equal(t, "slack", ch.Name())
}

func TestSlackChannel_SupportsAll(t *testing.T) {
	ch := NewSlackChannel(SlackConfig{})
	allTypes := []EventType{
		EventDeployStarted,
		EventDeployCompleted,
		EventDeployFailed,
		EventDeployRolledBack,
		EventFlagToggled,
		EventReleaseCreated,
		EventReleasePromoted,
		EventHealthDegraded,
		EventType("unknown.event"),
	}
	for _, et := range allTypes {
		assert.True(t, ch.Supports(et), "Slack should support %s", et)
	}
}

func TestSlackChannel_FormatMessage(t *testing.T) {
	ch := NewSlackChannel(SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
		Channel:    "#deployments",
		Username:   "DeploySentry",
		IconEmoji:  ":rocket:",
	})

	event := sampleEvent(EventDeployCompleted)
	msg := ch.formatMessage(event)

	assert.Equal(t, "#deployments", msg.Channel)
	assert.Equal(t, "DeploySentry", msg.Username)
	assert.Equal(t, ":rocket:", msg.IconEmoji)
	assert.Contains(t, msg.Text, "Deployment completed")
	assert.Contains(t, msg.Text, "my-service")

	// Verify block structure.
	require.Len(t, msg.Blocks, 1)
	assert.Equal(t, "section", msg.Blocks[0].Type)
	require.NotNil(t, msg.Blocks[0].Text)
	assert.Equal(t, "mrkdwn", msg.Blocks[0].Text.Type)
	assert.Equal(t, msg.Text, msg.Blocks[0].Text.Text)
}

func TestSlackChannel_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload slackMessage
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)
		assert.Contains(t, payload.Text, "Deployment started")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := NewSlackChannel(SlackConfig{
		WebhookURL: server.URL,
		Channel:    "#alerts",
	})

	event := sampleEvent(EventDeployStarted)
	err := ch.Send(context.Background(), event)
	assert.NoError(t, err)
}

func TestSlackChannel_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ch := NewSlackChannel(SlackConfig{
		WebhookURL: server.URL,
	})

	event := sampleEvent(EventDeployFailed)
	err := ch.Send(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ===========================================================================
// Webhook Channel Tests
// ===========================================================================

func TestNewWebhookChannel_Defaults(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{
		URL: "https://example.com/webhook",
	})
	assert.Equal(t, 3, ch.config.MaxRetries)
	assert.Equal(t, 1*time.Second, ch.config.RetryDelay)
	assert.Equal(t, 10*time.Second, ch.config.Timeout)
}

func TestNewWebhookChannel_CustomValues(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{
		URL:        "https://example.com/webhook",
		MaxRetries: 5,
		RetryDelay: 2 * time.Second,
		Timeout:    30 * time.Second,
	})
	assert.Equal(t, 5, ch.config.MaxRetries)
	assert.Equal(t, 2*time.Second, ch.config.RetryDelay)
	assert.Equal(t, 30*time.Second, ch.config.Timeout)
}

func TestWebhookChannel_Name(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{URL: "https://example.com"})
	assert.Equal(t, "webhook", ch.Name())
}

func TestWebhookChannel_Supports_EmptyEventTypes(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{URL: "https://example.com"})
	assert.True(t, ch.Supports(EventDeployStarted))
	assert.True(t, ch.Supports(EventDeployCompleted))
	assert.True(t, ch.Supports(EventDeployFailed))
	assert.True(t, ch.Supports(EventFlagToggled))
	assert.True(t, ch.Supports(EventType("custom.event")))
}

func TestWebhookChannel_Supports_SpecificEventTypes(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{
		URL:        "https://example.com",
		EventTypes: []EventType{EventDeployFailed, EventDeployRolledBack},
	})
	assert.True(t, ch.Supports(EventDeployFailed))
	assert.True(t, ch.Supports(EventDeployRolledBack))
	assert.False(t, ch.Supports(EventDeployStarted))
	assert.False(t, ch.Supports(EventDeployCompleted))
	assert.False(t, ch.Supports(EventReleaseCreated))
}

func TestWebhookChannel_Sign_EmptySecret(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{
		URL:    "https://example.com",
		Secret: "",
	})
	sig := ch.sign([]byte("test payload"))
	assert.Equal(t, "", sig)
}

func TestWebhookChannel_Sign_NonEmptySecret(t *testing.T) {
	ch := NewWebhookChannel(WebhookConfig{
		URL:    "https://example.com",
		Secret: "my-secret",
	})

	payload := []byte("test payload")
	sig := ch.sign(payload)

	// Verify it starts with "sha256=".
	assert.True(t, strings.HasPrefix(sig, "sha256="))

	// Independently compute the expected HMAC.
	mac := hmac.New(sha256.New, []byte("my-secret"))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestVerifySignature_Valid(t *testing.T) {
	secret := "webhook-secret"
	payload := []byte(`{"event":"deploy"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	assert.True(t, VerifySignature(payload, sig, secret))
}

func TestVerifySignature_Invalid(t *testing.T) {
	secret := "webhook-secret"
	payload := []byte(`{"event":"deploy"}`)

	assert.False(t, VerifySignature(payload, "sha256=invalid", secret))
	assert.False(t, VerifySignature(payload, "", secret))
	assert.False(t, VerifySignature(payload, "sha256=0000000000000000000000000000000000000000000000000000000000000000", secret))
}

func TestWebhookChannel_Send_SuccessfulDelivery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and validate the payload.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload webhookPayload
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)
		assert.Equal(t, EventDeployCompleted, payload.EventType)
		assert.Equal(t, "org-1", payload.OrgID)
		assert.Equal(t, "proj-1", payload.ProjectID)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	ch := NewWebhookChannel(WebhookConfig{
		URL:    server.URL,
		Secret: "test-secret",
	})

	event := sampleEvent(EventDeployCompleted)
	err := ch.Send(context.Background(), event)
	assert.NoError(t, err)
}

func TestWebhookChannel_Send_SetsCorrectHeaders(t *testing.T) {
	secret := "header-test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Content-Type header.
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify signature header is present and valid.
		sig := r.Header.Get("X-DeploySentry-Signature")
		assert.True(t, strings.HasPrefix(sig, "sha256="), "Signature should start with sha256=")

		// Verify the signature against the payload.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.True(t, VerifySignature(body, sig, secret))

		// Verify delivery ID header is a valid UUID.
		delivery := r.Header.Get("X-DeploySentry-Delivery")
		assert.NotEmpty(t, delivery)
		_, parseErr := uuid.Parse(delivery)
		assert.NoError(t, parseErr, "X-DeploySentry-Delivery should be a valid UUID")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := NewWebhookChannel(WebhookConfig{
		URL:    server.URL,
		Secret: secret,
	})

	event := sampleEvent(EventReleaseCreated)
	err := ch.Send(context.Background(), event)
	assert.NoError(t, err)
}

func TestWebhookChannel_Send_RetryOnFailureThenSucceed(t *testing.T) {
	var attempt int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempt, 1)
		if current < 3 {
			// Fail the first two attempts.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
			return
		}
		// Succeed on the third attempt.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	ch := NewWebhookChannel(WebhookConfig{
		URL:        server.URL,
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond, // short delay for testing
		Timeout:    5 * time.Second,
	})

	event := sampleEvent(EventDeployFailed)
	err := ch.Send(context.Background(), event)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempt))
}

func TestWebhookChannel_Send_AllRetriesFail(t *testing.T) {
	var attempt int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempt, 1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	ch := NewWebhookChannel(WebhookConfig{
		URL:        server.URL,
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
		Timeout:    5 * time.Second,
	})

	event := sampleEvent(EventDeployFailed)
	err := ch.Send(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempt))
}

func TestWebhookChannel_Send_NoSignatureWhenNoSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-DeploySentry-Signature")
		assert.Equal(t, "", sig, "Signature should be empty when no secret is configured")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := NewWebhookChannel(WebhookConfig{
		URL: server.URL,
	})

	event := sampleEvent(EventDeployStarted)
	err := ch.Send(context.Background(), event)
	assert.NoError(t, err)
}

// ===========================================================================
// formatEventText Tests
// ===========================================================================

func TestFormatEventText_AllEventTypes(t *testing.T) {
	tests := []struct {
		eventType EventType
		contains  []string
	}{
		{
			EventDeployStarted,
			[]string{"Deployment started", "my-service", "v1.2.3"},
		},
		{
			EventDeployCompleted,
			[]string{"Deployment completed", "my-service", "v1.2.3"},
		},
		{
			EventDeployFailed,
			[]string{"Deployment FAILED", "my-service", "v1.2.3", "timeout"},
		},
		{
			EventDeployRolledBack,
			[]string{"Deployment rolled back", "my-service", "v1.2.3", "high error rate"},
		},
		{
			EventFlagToggled,
			[]string{"Feature flag", "dark-mode", "true", "my-service"},
		},
		{
			EventReleaseCreated,
			[]string{"Release", "v1.2.3", "created", "my-service"},
		},
		{
			EventReleasePromoted,
			[]string{"Release", "v1.2.3", "promoted", "production", "my-service"},
		},
		{
			EventHealthDegraded,
			[]string{"Health degraded", "my-service", "42"},
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.eventType), func(t *testing.T) {
			event := sampleEvent(tc.eventType)
			text := formatEventText(event)
			for _, s := range tc.contains {
				assert.Contains(t, text, s, "formatEventText for %s should contain %q", tc.eventType, s)
			}
		})
	}
}

func TestFormatEventText_DefaultCase(t *testing.T) {
	event := sampleEvent(EventType("custom.unknown"))
	text := formatEventText(event)
	assert.Contains(t, text, "custom.unknown")
	assert.Contains(t, text, "my-service")
}

func TestFormatEventText_FallsBackToProjectID(t *testing.T) {
	event := &Event{
		Type:      EventDeployStarted,
		ProjectID: "fallback-proj",
		Data:      map[string]string{"version": "v2.0.0"},
	}
	text := formatEventText(event)
	assert.Contains(t, text, "fallback-proj")
	assert.Contains(t, text, "v2.0.0")
}
