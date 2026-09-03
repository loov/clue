package build

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Context provides a cancellable context for build operations
// with support for graceful shutdown and double Ctrl+C force exit.
type Context struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	stop   func()
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

// SetupSignalHandling creates a Context that responds to SIGINT and SIGTERM.
// The first signal cancels the context for graceful shutdown.
// A second signal during graceful shutdown forces immediate exit.
func SetupSignalHandling() *Context {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)

	bc := &Context{
		Ctx:    ctx,
		Cancel: stop,
		stop:   stop,
		done:   make(chan struct{}),
	}

	// Handle double Ctrl+C for immediate exit
	bc.wg.Go(func() {
		select {
		case <-bc.done:
			return
		case <-ctx.Done(): // First signal received
		}
		stop() // Stop signal notifications on this context

		// Print cancellation message
		fmt.Fprintf(os.Stderr, "\nBuild cancelled. Waiting for in-flight compilations...\n")
		fmt.Fprintf(os.Stderr, "(Press Ctrl+C again to force exit)\n")

		// Setup for second signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigChan)
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()

		select {
		case <-bc.done:
			return
		case <-sigChan:
			fmt.Fprintf(os.Stderr, "\nForce exit\n")
			os.Exit(130) // 128 + SIGINT(2)
		case <-timer.C:
			// Timeout waiting for graceful shutdown
			// Let the normal build completion handle this
			return
		}
	})

	return bc
}

// Close releases signal resources and waits for the handler to stop.
func (bc *Context) Close() {
	bc.once.Do(func() {
		close(bc.done)
		bc.stop()
	})
	bc.wg.Wait()
}

// IsCancelled checks if the build context has been cancelled
func (bc *Context) IsCancelled() bool {
	select {
	case <-bc.Ctx.Done():
		return true
	default:
		return false
	}
}
