package graceful

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ContextWithShutdown returns a context that is cancelled when an OS interrupt
// signal is received, after an optional graceful shutdown timeout.
func ContextWithShutdown(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		time.Sleep(timeout)
		cancel()
	}()
	return ctx, cancel
}
