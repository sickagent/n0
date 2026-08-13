// Package auditsink persists JetStream audit events with explicit acknowledgement.
package auditsink

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sickagent/n0/pkg/shared/audit"
	"go.uber.org/zap"
)

type Repository interface {
	SaveAuditEvent(context.Context, audit.Event) error
}

type Sink struct {
	consumer jetstream.Consumer
	repo     Repository
	log      *zap.Logger
	wg       sync.WaitGroup
}

func New(consumer jetstream.Consumer, repo Repository, log *zap.Logger) *Sink {
	return &Sink{consumer: consumer, repo: repo, log: log}
}

func (s *Sink) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.run(ctx)
}

func (s *Sink) Wait() { s.wg.Wait() }

func (s *Sink) run(ctx context.Context) {
	defer s.wg.Done()
	for ctx.Err() == nil {
		batch, err := s.consumer.Fetch(32, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() == nil {
				s.log.Warn("audit fetch failed", zap.Error(err))
			}
			continue
		}
		for msg := range batch.Messages() {
			var event audit.Event
			if err := json.Unmarshal(msg.Data(), &event); err != nil {
				s.log.Error("invalid audit event", zap.Error(err))
				_ = msg.TermWithReason("invalid audit event")
				continue
			}
			if err := s.repo.SaveAuditEvent(ctx, event); err != nil {
				s.log.Error("persist audit event failed", zap.String("event_id", event.ID), zap.Error(err))
				_ = msg.NakWithDelay(5 * time.Second)
				continue
			}
			if err := msg.Ack(); err != nil {
				s.log.Warn("audit ack failed", zap.String("event_id", event.ID), zap.Error(err))
			}
		}
	}
}
