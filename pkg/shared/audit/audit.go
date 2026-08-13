// Package audit defines the durable audit event contract shared by producers
// and sinks. Events are identified before publish so redelivery is idempotent.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const SubjectPrefix = "audit.events"

// Event is the immutable audit record persisted by Meta Service.
type Event struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id"`
	ActorID      string         `json:"actor_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Success      bool           `json:"success"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

// Publisher is implemented by NATS and lightweight test doubles.
type Publisher interface {
	Publish(subject string, data []byte) error
}

// Publish sends an event to the tenant-partitioned JetStream subject.
func Publish(p Publisher, event Event) error {
	if p == nil {
		return fmt.Errorf("audit publisher is required")
	}
	if strings.TrimSpace(event.TenantID) == "" {
		return fmt.Errorf("audit tenant_id is required")
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	subject := SubjectPrefix + "." + sanitizeSubjectToken(event.TenantID)
	if err := p.Publish(subject, payload); err != nil {
		return fmt.Errorf("publish audit event: %w", err)
	}
	return nil
}

func sanitizeSubjectToken(value string) string {
	return strings.NewReplacer(".", "_", "*", "_", ">", "_", " ", "_").Replace(value)
}

var _ Publisher = (*nats.Conn)(nil)

// JetStreamPublisher waits for the server acknowledgement, so a producer does
// not report success until the event is durably accepted by JetStream.
type JetStreamPublisher struct {
	js      jetstream.JetStream
	timeout time.Duration
}

func NewJetStreamPublisher(js jetstream.JetStream, timeout time.Duration) *JetStreamPublisher {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &JetStreamPublisher{js: js, timeout: timeout}
}

func (p *JetStreamPublisher) Publish(subject string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	_, err := p.js.Publish(ctx, subject, data)
	return err
}
