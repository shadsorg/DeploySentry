// Package messaging provides a message queue abstraction for internal communication.
package messaging

import (
	"context"
)

// Publisher defines the interface for publishing messages to a topic.
type Publisher interface {
	// Publish publishes a message to the given topic.
	Publish(ctx context.Context, topic string, data []byte) error
}

// Subscriber defines the interface for subscribing to messages from a topic.
type Subscriber interface {
	// Subscribe subscribes to messages from the given topic.
	// The callback is invoked for each message received.
	Subscribe(ctx context.Context, topic string, callback func([]byte) error) error
}

// MessageQueue defines the interface for a message queue service that supports
// both publishing and subscribing.
type MessageQueue interface {
	Publisher
	Subscriber
}