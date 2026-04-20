package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type fakeConsumer struct {
	msgs  []jetstream.Msg
	count int
}

func (c *fakeConsumer) Fetch(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	c.count++
	if c.count > 1 {
		return &fakeBatch{msgs: nil}, nil
	}
	return &fakeBatch{msgs: c.msgs}, nil
}

func (c *fakeConsumer) FetchBytes(maxBytes int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	return &fakeBatch{msgs: c.msgs}, nil
}

func (c *fakeConsumer) FetchNoWait(batch int) (jetstream.MessageBatch, error) {
	return &fakeBatch{msgs: c.msgs}, nil
}

func (c *fakeConsumer) Consume(handler jetstream.MessageHandler, opts ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
	return nil, nil
}

func (c *fakeConsumer) Messages(opts ...jetstream.PullMessagesOpt) (jetstream.MessagesContext, error) {
	return nil, nil
}

func (c *fakeConsumer) Next(opts ...jetstream.FetchOpt) (jetstream.Msg, error) {
	return nil, nil
}

func (c *fakeConsumer) Info(ctx context.Context) (*jetstream.ConsumerInfo, error) {
	return nil, nil
}

func (c *fakeConsumer) CachedInfo() *jetstream.ConsumerInfo {
	return nil
}

type fakeBatch struct {
	msgs []jetstream.Msg
}

func (b *fakeBatch) Messages() <-chan jetstream.Msg {
	ch := make(chan jetstream.Msg, len(b.msgs))
	for _, m := range b.msgs {
		ch <- m
	}
	close(ch)
	return ch
}

func (b *fakeBatch) Error() error {
	return nil
}

type fakeMsg struct {
	data []byte
	ack  bool
	nak  bool
}

func (m *fakeMsg) Data() []byte                              { return m.data }
func (m *fakeMsg) Headers() nats.Header                      { return nil }
func (m *fakeMsg) Subject() string                           { return "" }
func (m *fakeMsg) Reply() string                             { return "" }
func (m *fakeMsg) Ack() error                                { m.ack = true; return nil }
func (m *fakeMsg) DoubleAck(ctx context.Context) error       { m.ack = true; return nil }
func (m *fakeMsg) Nak() error                                { m.nak = true; return nil }
func (m *fakeMsg) NakWithDelay(d time.Duration) error        { m.nak = true; return nil }
func (m *fakeMsg) InProgress() error                         { return nil }
func (m *fakeMsg) Term() error                               { return nil }
func (m *fakeMsg) TermWithReason(reason string) error        { return nil }
func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }

type fakeProcessor struct {
	jobs []Job
	err  error
}

func (p *fakeProcessor) Process(ctx context.Context, job Job) error {
	p.jobs = append(p.jobs, job)
	return p.err
}

func TestPool_ProcessJob(t *testing.T) {
	log := zap.NewNop()
	payload, err := json.Marshal(Job{ID: "job-42", ConnectionID: "conn-1", SQL: "SELECT 1"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	msg := &fakeMsg{data: payload}
	cons := &fakeConsumer{msgs: []jetstream.Msg{msg}}
	proc := &fakeProcessor{}

	pool := NewPool(cons, proc, log, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool.Start(ctx)
	time.Sleep(500 * time.Millisecond)
	pool.Stop()

	if len(proc.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(proc.jobs))
	}
	if proc.jobs[0].ID != "job-42" {
		t.Errorf("expected job-42, got %s", proc.jobs[0].ID)
	}
	if !msg.ack {
		t.Error("expected message to be acked")
	}
}

func TestPool_ProcessJobFailure(t *testing.T) {
	log := zap.NewNop()
	payload, err := json.Marshal(Job{ID: "job-fail", ConnectionID: "conn-1", SQL: "SELECT 1"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	msg := &fakeMsg{data: payload}
	cons := &fakeConsumer{msgs: []jetstream.Msg{msg}}
	proc := &fakeProcessor{err: errors.New("boom")}

	pool := NewPool(cons, proc, log, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool.Start(ctx)
	time.Sleep(500 * time.Millisecond)
	pool.Stop()

	if len(proc.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(proc.jobs))
	}
	if !msg.nak {
		t.Error("expected message to be nacked")
	}
}

func TestNoopProcessor(t *testing.T) {
	log := zap.NewNop()
	p := NewNoopProcessor(log)
	if err := p.Process(context.Background(), Job{ID: "x"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
