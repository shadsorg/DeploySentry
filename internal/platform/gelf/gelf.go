package gelf

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Message represents a GELF 1.1 log message.
type Message struct {
	Version      string  `json:"version"`
	Host         string  `json:"host"`
	ShortMessage string  `json:"short_message"`
	Timestamp    float64 `json:"timestamp"`
	Level        int     `json:"level"`

	AppName  string `json:"_appName"`
	Env      string `json:"_env"`
	LogType  string `json:"_logType"`
	LogLevel string `json:"_logLevel"`

	RequestID     string `json:"_requestId,omitempty"`
	UserID        string `json:"_userId,omitempty"`
	HTTPMethod    string `json:"_httpMethod,omitempty"`
	HTTPPath      string `json:"_httpPath,omitempty"`
	HTTPStatus    int    `json:"_httpStatus,omitempty"`
	RequestTimeMs int64  `json:"_requestTimeMs,omitempty"`
	ErrorName     string `json:"_errorName,omitempty"`
	ErrorStack    string `json:"_errorStack,omitempty"`
}

// Transport defines how GELF messages are delivered.
// Exported so middleware tests can provide mock implementations.
type Transport interface {
	Send(data []byte) error
	Close() error
}

// Client sends structured log messages via GELF.
type Client struct {
	transport Transport
	appName   string
	env       string
	hostname  string
}

const maxUDPPayload = 8192

// NewClient creates a GELF client. Transport is selected automatically:
// - CF_ACCESS_CLIENT_ID set -> HTTP to ingest.crowdsoftapps.com
// - Otherwise -> UDP to localhost:12201
func NewClient(appName string) (*Client, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	env := os.Getenv("DS_ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	var t Transport
	var err error

	if os.Getenv("CF_ACCESS_CLIENT_ID") != "" {
		t, err = newHTTPTransport(
			"https://ingest.crowdsoftapps.com/gelf",
			os.Getenv("CF_ACCESS_CLIENT_ID"),
			os.Getenv("CF_ACCESS_CLIENT_SECRET"),
		)
	} else {
		t, err = newUDPTransport("localhost:12201")
	}
	if err != nil {
		return nil, fmt.Errorf("gelf: %w", err)
	}

	return &Client{
		transport: t,
		appName:   appName,
		env:       env,
		hostname:  hostname,
	}, nil
}

// NewTestClient creates a Client with the given transport for testing.
func NewTestClient(t Transport) *Client {
	return &Client{
		transport: t,
		appName:   "test",
		env:       "test",
		hostname:  "testhost",
	}
}

// Close shuts down the transport.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.transport.Close()
}

// send serializes and dispatches a message. If the payload exceeds maxUDPPayload
// and the message has an ErrorStack, the stack is truncated before re-serializing.
func (c *Client) send(msg Message) {
	msg.Version = "1.1"
	msg.Host = c.hostname
	msg.AppName = c.appName
	msg.Env = c.env
	if msg.Timestamp == 0 {
		msg.Timestamp = float64(time.Now().UnixNano()) / 1e9
	}
	msg.Level = gelfLevel(msg.LogLevel)

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("gelf: marshal error: %v", err)
		return
	}

	if len(data) > maxUDPPayload && msg.ErrorStack != "" {
		overhead := len(data) - len(msg.ErrorStack)
		maxStack := maxUDPPayload - overhead - 20
		if maxStack < 0 {
			maxStack = 0
		}
		msg.ErrorStack = msg.ErrorStack[:maxStack] + "...[truncated]"
		data, err = json.Marshal(msg)
		if err != nil {
			log.Printf("gelf: marshal error after truncation: %v", err)
			return
		}
	}

	if err := c.transport.Send(data); err != nil {
		log.Printf("gelf: send error: %v", err)
	}
}

// Info sends an informational log message.
func (c *Client) Info(shortMessage string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "info",
		LogLevel:     "info",
	})
}

// Error sends an error log message.
func (c *Client) Error(shortMessage, errorName, errorStack string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "error",
		LogLevel:     "error",
		ErrorName:    errorName,
		ErrorStack:   errorStack,
	})
}

// Request sends a request-start log message.
func (c *Client) Request(requestID, method, path, userID string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: method + " " + path,
		LogType:      "request",
		LogLevel:     "info",
		RequestID:    requestID,
		HTTPMethod:   method,
		HTTPPath:     path,
		UserID:       userID,
	})
}

// Response sends a request-complete log message.
func (c *Client) Response(requestID, method, path, userID string, status int, durationMs int64) {
	if c == nil {
		return
	}
	logLevel := "info"
	if status >= 500 {
		logLevel = "error"
	} else if status >= 400 {
		logLevel = "warn"
	}
	c.send(Message{
		ShortMessage:  fmt.Sprintf("%d %s %s %dms", status, method, path, durationMs),
		LogType:       "response",
		LogLevel:      logLevel,
		RequestID:     requestID,
		HTTPMethod:    method,
		HTTPPath:      path,
		UserID:        userID,
		HTTPStatus:    status,
		RequestTimeMs: durationMs,
	})
}

// Fatal sends a fatal-level error log message (used for panics).
func (c *Client) Fatal(shortMessage, errorName, errorStack, requestID, method, path, userID string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "error",
		LogLevel:     "fatal",
		ErrorName:    errorName,
		ErrorStack:   errorStack,
		RequestID:    requestID,
		HTTPMethod:   method,
		HTTPPath:     path,
		UserID:       userID,
	})
}

// ErrorWithContext sends an error log message with request context.
func (c *Client) ErrorWithContext(shortMessage, errorName, errorStack, requestID, method, path, userID string) {
	if c == nil {
		return
	}
	c.send(Message{
		ShortMessage: shortMessage,
		LogType:      "error",
		LogLevel:     "error",
		ErrorName:    errorName,
		ErrorStack:   errorStack,
		RequestID:    requestID,
		HTTPMethod:   method,
		HTTPPath:     path,
		UserID:       userID,
	})
}

func gelfLevel(level string) int {
	switch level {
	case "fatal":
		return 2
	case "error":
		return 3
	case "warn":
		return 4
	case "info":
		return 6
	case "debug", "trace":
		return 7
	default:
		return 6
	}
}
