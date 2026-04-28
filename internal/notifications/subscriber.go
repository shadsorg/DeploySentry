package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/shadsorg/deploysentry/internal/platform/messaging"
	"github.com/nats-io/nats.go/jetstream"
)

// EventSubscriber listens to NATS JetStream for domain events and dispatches
// them through the NotificationService.
type EventSubscriber struct {
	nats    *messaging.NATS
	service *NotificationService

	// consumers holds the active JetStream consume contexts so they can be
	// stopped during shutdown.
	consumers []jetstream.ConsumeContext
}

// SubscriberConfig controls how the EventSubscriber connects to JetStream.
type SubscriberConfig struct {
	// StreamName is the NATS JetStream stream to subscribe to.
	StreamName string `json:"stream_name"`

	// Subjects is the list of subject patterns to subscribe to.
	// Each subject gets its own durable consumer.
	Subjects []string `json:"subjects"`
}

// DefaultSubscriberConfig returns a configuration that subscribes to all
// standard DeploySentry domain event subjects.
func DefaultSubscriberConfig() SubscriberConfig {
	return SubscriberConfig{
		StreamName: "DEPLOYSENTRY",
		Subjects: []string{
			"deployments.>",
			"flags.>",
			"releases.>",
			"health.>",
		},
	}
}

// NewEventSubscriber creates a new subscriber that bridges NATS JetStream
// events to the notification service.
func NewEventSubscriber(nats *messaging.NATS, service *NotificationService) *EventSubscriber {
	return &EventSubscriber{
		nats:    nats,
		service: service,
	}
}

// Start begins consuming messages from JetStream for each configured subject.
// It creates durable consumers so that messages are not lost across restarts.
func (s *EventSubscriber) Start(ctx context.Context, cfg SubscriberConfig) error {
	// Ensure the stream exists with the required subjects.
	_, err := s.nats.EnsureStream(ctx, jetstream.StreamConfig{
		Name:     cfg.StreamName,
		Subjects: cfg.Subjects,
	})
	if err != nil {
		return fmt.Errorf("ensuring notification stream: %w", err)
	}

	for _, subject := range cfg.Subjects {
		consumerName := fmt.Sprintf("notifications-%s", sanitizeConsumerName(subject))

		cc, err := s.nats.Subscribe(ctx, cfg.StreamName, consumerName, subject, s.handleMessage)
		if err != nil {
			// Stop any consumers that were already started.
			s.Stop()
			return fmt.Errorf("subscribing to %q: %w", subject, err)
		}
		s.consumers = append(s.consumers, cc)
	}

	return nil
}

// Stop gracefully stops all active JetStream consumers.
func (s *EventSubscriber) Stop() {
	for _, cc := range s.consumers {
		cc.Stop()
	}
	s.consumers = nil
}

// handleMessage processes a single NATS JetStream message by unmarshaling
// it into an Event and dispatching it through the notification service.
func (s *EventSubscriber) handleMessage(msg jetstream.Msg) {
	var event Event
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		log.Printf("notifications: failed to unmarshal event from %s: %v", msg.Subject(), err)
		// Terminate the message so it is not redelivered for a bad payload.
		if termErr := msg.Term(); termErr != nil {
			log.Printf("notifications: failed to term message: %v", termErr)
		}
		return
	}

	ctx := context.Background()
	if err := s.service.Dispatch(ctx, &event); err != nil {
		log.Printf("notifications: dispatch error for %s: %v", event.Type, err)
		// NAK so the message can be redelivered according to the consumer policy.
		if nakErr := msg.Nak(); nakErr != nil {
			log.Printf("notifications: failed to nak message: %v", nakErr)
		}
		return
	}

	if err := msg.Ack(); err != nil {
		log.Printf("notifications: failed to ack message: %v", err)
	}
}

// sanitizeConsumerName converts a NATS subject pattern into a valid durable
// consumer name by replacing wildcards and dots.
func sanitizeConsumerName(subject string) string {
	result := make([]byte, 0, len(subject))
	for i := 0; i < len(subject); i++ {
		switch subject[i] {
		case '.', '>', '*':
			result = append(result, '-')
		default:
			result = append(result, subject[i])
		}
	}
	return string(result)
}
