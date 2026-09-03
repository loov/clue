// Package main provides the Clue command-line entry point.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var version = "0.1.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runCLI(ctx, os.Args[1:], version)
	stop()
	os.Exit(code)
}
