package worker

import (
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Job represents an analytical query job.
type Job struct {
	ID           string
	TenantID     string
	ConnectionID string
	SQL          string
}

// Processor handles the actual execution of a job.
type Processor interface {
	Process(ctx context.Context, job Job) error
}

// Pool manages a fixed number of goroutines consuming jobs from NATS JetStream.
type Pool struct {
	consumer   jetstream.Consumer
	processor  Processor
	log        *zap.Logger
	wg         sync.WaitGroup
	stop       chan struct{}
	numWorkers int
}

// NewPool creates a new worker pool.
func NewPool(cons jetstream.Consumer, proc Processor, log *zap.Logger, numWorkers int) *Pool {
	if numWorkers <= 0 {
		numWorkers = 4
	}
	return &Pool{
		consumer:   cons,
		processor:  proc,
		log:        log,
		stop:       make(chan struct{}),
		numWorkers: numWorkers,
	}
}

// Start launches the worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.run(ctx, i)
	}
}

// Stop gracefully shuts down the pool.
func (p *Pool) Stop() {
	close(p.stop)
	p.wg.Wait()
}

func (p *Pool) run(ctx context.Context, id int) {
	defer p.wg.Done()
	p.log.Info("worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			p.log.Info("worker context done", zap.Int("worker_id", id))
			return
		case <-p.stop:
			p.log.Info("worker stopped", zap.Int("worker_id", id))
			return
		default:
		}

		msgs, err := p.consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			p.log.Error("fetch error", zap.Int("worker_id", id), zap.Error(err))
			continue
		}

		for msg := range msgs.Messages() {
			job := Job{ID: string(msg.Data())}
			if err := p.processor.Process(ctx, job); err != nil {
				p.log.Error("job processing failed", zap.Int("worker_id", id), zap.Error(err))
				_ = msg.NakWithDelay(5 * time.Second)
				continue
			}
			if err := msg.Ack(); err != nil {
				p.log.Error("ack failed", zap.Int("worker_id", id), zap.Error(err))
			}
		}
	}
}

// NoopProcessor is a placeholder processor for bootstrapping.
type NoopProcessor struct {
	log *zap.Logger
}

// NewNoopProcessor creates a no-op processor.
func NewNoopProcessor(log *zap.Logger) *NoopProcessor {
	return &NoopProcessor{log: log}
}

// Process implements the Processor interface.
func (n *NoopProcessor) Process(ctx context.Context, job Job) error {
	n.log.Info("processing job", zap.String("job_id", job.ID))
	return nil
}
