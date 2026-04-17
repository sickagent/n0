package natsclient

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Client wraps a NATS connection together with JetStream and KV helpers.
type Client struct {
	Conn           *nats.Conn
	JS             jetstream.JetStream
	KV             jetstream.KeyValue
	RequestTimeout time.Duration
}

// New establishes a NATS connection, initializes JetStream and the shared KV bucket.
func New(url string, timeout time.Duration, log *zap.Logger) (*Client, error) {
	nc, err := nats.Connect(
		url,
		nats.Name("n0"),
		nats.Timeout(timeout),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn("nats disconnected", zap.Error(err))
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Info("nats reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream init: %w", err)
	}

	kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: "n0",
	})
	if err != nil {
		kv, err = js.KeyValue(context.Background(), "n0")
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("kv init: %w", err)
		}
	}

	return &Client{
		Conn:           nc,
		JS:             js,
		KV:             kv,
		RequestTimeout: timeout,
	}, nil
}

// Close terminates the NATS connection.
func (c *Client) Close() {
	c.Conn.Close()
}

// EnsureStream creates a JetStream stream if it does not exist.
func (c *Client) EnsureStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := c.JS.CreateStream(ctx, cfg)
	if err != nil {
		stream, err = c.JS.Stream(ctx, cfg.Name)
		if err != nil {
			return nil, fmt.Errorf("ensure stream %s: %w", cfg.Name, err)
		}
	}
	return stream, nil
}
