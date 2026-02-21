// Package messaging provides a NATS JetStream client wrapper for event-driven communication.
package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/deploysentry/deploysentry/internal/platform/config"
)

// NATS wraps a NATS connection and JetStream context.
type NATS struct {
	Conn      *nats.Conn
	JetStream jetstream.JetStream
}

// New creates a new NATS connection and initializes JetStream from the
// provided configuration. It validates the connection before returning.
func New(ctx context.Context, cfg config.NATSConfig) (*NATS, error) {
	opts := []nats.Option{
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.Timeout(cfg.ConnectTimeout),
		nats.Name("deploysentry"),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("initializing JetStream: %w", err)
	}

	return &NATS{
		Conn:      conn,
		JetStream: js,
	}, nil
}

// Close drains the NATS connection and shuts it down gracefully.
// Drain will process any buffered messages before closing.
func (n *NATS) Close() error {
	if n.Conn != nil {
		return n.Conn.Drain()
	}
	return nil
}

// Health checks whether NATS is connected and responsive.
func (n *NATS) Health() error {
	if !n.Conn.IsConnected() {
		return fmt.Errorf("NATS connection is not active (status: %s)", n.Conn.Status())
	}
	return nil
}

// EnsureStream creates or updates a JetStream stream with the given configuration.
// This is idempotent: if the stream already exists with compatible settings, it
// will be returned without error.
func (n *NATS) EnsureStream(ctx context.Context, streamCfg jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := n.JetStream.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return nil, fmt.Errorf("ensuring stream %q: %w", streamCfg.Name, err)
	}
	return stream, nil
}

// Publish publishes a message to a NATS subject. It uses JetStream publish
// with an acknowledgement to ensure delivery.
func (n *NATS) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := n.JetStream.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publishing to %q: %w", subject, err)
	}
	return nil
}

// Subscribe creates a durable JetStream consumer that processes messages on the
// given stream and subject filter. The handler function is called for each message.
func (n *NATS) Subscribe(
	ctx context.Context,
	stream string,
	consumerName string,
	filterSubject string,
	handler func(msg jetstream.Msg),
) (jetstream.ConsumeContext, error) {
	consumer, err := n.JetStream.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    5,
	})
	if err != nil {
		return nil, fmt.Errorf("creating consumer %q on stream %q: %w", consumerName, stream, err)
	}

	cc, err := consumer.Consume(handler)
	if err != nil {
		return nil, fmt.Errorf("starting consume for %q: %w", consumerName, err)
	}

	return cc, nil
}
