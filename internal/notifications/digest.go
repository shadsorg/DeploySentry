package notifications

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DigestConfig controls the behavior of the DigestService.
type DigestConfig struct {
	// Interval is how often digest summaries are flushed and sent.
	Interval time.Duration `json:"interval"`

	// MaxBatchSize is the maximum number of events per digest before
	// an early flush is triggered.
	MaxBatchSize int `json:"max_batch_size"`
}

// DefaultDigestConfig returns sensible defaults for digest batching.
func DefaultDigestConfig() DigestConfig {
	return DigestConfig{
		Interval:     15 * time.Minute,
		MaxBatchSize: 50,
	}
}

// DigestService batches notification events into periodic summaries. Instead
// of delivering each event individually, it accumulates events and flushes
// them as a single digest notification at configured intervals.
type DigestService struct {
	config  DigestConfig
	service *NotificationService

	mu      sync.Mutex
	batches map[string][]*Event // keyed by projectID
	stopCh  chan struct{}
	done    chan struct{}
}

// NewDigestService creates a new DigestService that wraps the given
// NotificationService for batched delivery.
func NewDigestService(config DigestConfig, service *NotificationService) *DigestService {
	if config.Interval <= 0 {
		config.Interval = DefaultDigestConfig().Interval
	}
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = DefaultDigestConfig().MaxBatchSize
	}
	return &DigestService{
		config:  config,
		service: service,
		batches: make(map[string][]*Event),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the digest ticker that periodically flushes batched events.
func (d *DigestService) Start() {
	go d.run()
}

// Stop signals the digest ticker to stop and waits for it to finish.
func (d *DigestService) Stop() {
	close(d.stopCh)
	<-d.done
}

// Add queues an event for the next digest flush. If the batch for the
// event's project reaches MaxBatchSize, an immediate flush is triggered
// for that project.
func (d *DigestService) Add(event *Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	d.mu.Lock()
	d.batches[event.ProjectID] = append(d.batches[event.ProjectID], event)
	shouldFlush := len(d.batches[event.ProjectID]) >= d.config.MaxBatchSize
	projectID := event.ProjectID
	d.mu.Unlock()

	if shouldFlush {
		d.flushProject(projectID)
	}
}

// run is the main loop that periodically flushes all batched events.
func (d *DigestService) run() {
	defer close(d.done)

	ticker := time.NewTicker(d.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.flushAll()
		case <-d.stopCh:
			// Final flush before stopping.
			d.flushAll()
			return
		}
	}
}

// flushAll dispatches digest summaries for all projects with pending events.
func (d *DigestService) flushAll() {
	d.mu.Lock()
	batches := d.batches
	d.batches = make(map[string][]*Event)
	d.mu.Unlock()

	for projectID, events := range batches {
		if len(events) == 0 {
			continue
		}
		d.dispatchDigest(projectID, events)
	}
}

// flushProject dispatches a digest summary for a single project.
func (d *DigestService) flushProject(projectID string) {
	d.mu.Lock()
	events := d.batches[projectID]
	delete(d.batches, projectID)
	d.mu.Unlock()

	if len(events) == 0 {
		return
	}
	d.dispatchDigest(projectID, events)
}

// dispatchDigest creates a digest summary event and dispatches it through
// the notification service.
func (d *DigestService) dispatchDigest(projectID string, events []*Event) {
	summary := buildDigestSummary(events)

	// Determine the org from the first event.
	orgID := ""
	if len(events) > 0 {
		orgID = events[0].OrgID
	}

	digestEvent := &Event{
		Type:      EventType("digest.summary"),
		Timestamp: time.Now().UTC(),
		OrgID:     orgID,
		ProjectID: projectID,
		Data: map[string]string{
			"event_count": fmt.Sprintf("%d", len(events)),
			"summary":     summary,
		},
	}

	ctx := context.Background()
	_ = d.service.Dispatch(ctx, digestEvent)
}

// buildDigestSummary creates a human-readable summary of batched events.
func buildDigestSummary(events []*Event) string {
	if len(events) == 0 {
		return "No events in this digest period."
	}

	// Count events by type.
	counts := make(map[EventType]int)
	for _, e := range events {
		counts[e.Type]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Notification digest: %d events\n\n", len(events)))

	for eventType, count := range counts {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", eventType, count))
	}

	return sb.String()
}

// Pending returns the total number of events waiting to be flushed.
func (d *DigestService) Pending() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	total := 0
	for _, events := range d.batches {
		total += len(events)
	}
	return total
}
