// Package main provides the Clue command-line entry point.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"syscall"
)

var version = "0.1.0-dev"

func main() {
	// clue is short-lived and allocates mostly during config evaluation and
	// planning; collecting less often cuts GC to a small share of a no-op build.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	stopProfile := startCPUProfile(os.Getenv("CLUE_CPUPROFILE"))
	code := runCLI(ctx, os.Args[1:], buildVersion(version))
	stopProfile()
	stop()
	os.Exit(code)
}

// startCPUProfile writes a CPU profile of the run to path, for go tool pprof,
// when path is set. The returned function finishes the profile.
func startCPUProfile(path string) func() {
	if path == "" {
		return func() {}
	}
	file, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: CLUE_CPUPROFILE: %v\n", err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		fmt.Fprintf(os.Stderr, "warning: CLUE_CPUPROFILE: %v\n", err)
		_ = file.Close()
		return func() {}
	}
	return func() {
		pprof.StopCPUProfile()
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: CLUE_CPUPROFILE: %v\n", err)
		}
	}
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
