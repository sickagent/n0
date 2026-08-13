package graceful

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ContextWithShutdown returns a context that is cancelled immediately when an
// interrupt is received. The retained timeout parameter keeps the public API
// compatible; shutdown deadlines belong to individual resource drains.
func ContextWithShutdown(_ time.Duration) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// WithTimeout creates a bounded cleanup context that is independent from the
// already-cancelled application context.
func WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}
