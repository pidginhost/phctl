package cmdutil

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const DefaultAPITimeout = 30 * time.Second

// APIContext returns a context with the default API timeout.
func APIContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultAPITimeout)
}

// SignalContext returns a context that is cancelled on SIGINT or SIGTERM.
func SignalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()
	return ctx, cancel
}
