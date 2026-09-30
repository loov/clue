// Package main provides the Clue command-line entry point.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

var version = "0.1.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runCLI(ctx, os.Args[1:], buildVersion(version))
	stop()
	os.Exit(code)
}

// buildVersion adds what the Go toolchain recorded about the build: the module
// version for "go install ...@version", otherwise the VCS commit of the checkout.
func buildVersion(base string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return base
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	revision := settings["vcs.revision"]
	if revision == "" {
		return base
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if settings["vcs.modified"] == "true" {
		revision += "-dirty"
	}
	return base + " (" + revision + ")"
}
