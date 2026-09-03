package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/watch"
	"github.com/zeebo/clingy"
)

type watchCommand struct{ options *options }

func (*watchCommand) Setup(clingy.Parameters) {}

func (c *watchCommand) Execute(context.Context) error {
	o := c.options
	return result(runWatch(o.dir, o.variant, o.target, o.verbosity(), o.jobs, o.keepGoing))
}

func runWatch(dir, variant, target string, verbosity build.Verbosity, jobs int, keepGoing bool) int {
	// Load initial config
	cfg, selectedVariant, targetPlatform, err := loadConfig(dir, variant, target, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	// Compute jobs (same as runBuild)
	actualJobs := jobs
	if actualJobs == 0 {
		actualJobs = max(runtime.NumCPU()/2, 1)
	} else if actualJobs < 0 {
		actualJobs = runtime.NumCPU()
	}

	buildCuePath := filepath.Join(dir, "clue.cue")
	_, configErr := os.Stat(buildCuePath)
	autoDiscover := os.IsNotExist(configErr)

	// Track current build cancel function
	var currentCancel context.CancelFunc
	var buildMu sync.Mutex

	// Reload config and run build
	doBuild := func(trigger string, isConfigChange bool) {
		buildMu.Lock()
		// Cancel any in-progress build
		if currentCancel != nil {
			currentCancel()
		}

		// Clear screen
		fmt.Print("\033[H\033[2J")

		// Print timestamp and trigger
		now := time.Now().Format("15:04:05")
		if isConfigChange || autoDiscover {
			fmt.Printf("[%s] Project changed: %s - reloading...\n", now, trigger)
			// Reload config
			var err error
			cfg, selectedVariant, targetPlatform, err = loadConfig(dir, variant, target, verbosity)
			if err != nil {
				printError(err)
				buildMu.Unlock()
				return
			}
			_, configErr = os.Stat(buildCuePath)
			autoDiscover = os.IsNotExist(configErr)
		} else {
			fmt.Printf("[%s] Change detected: %s\n", now, trigger)
		}
		fmt.Printf("[%s] Rebuilding...\n", now)

		// Create new context for this build
		ctx, cancel := context.WithCancel(context.Background())
		currentCancel = cancel
		buildMu.Unlock()

		// Create builder
		builder, err := build.NewConfiguredBuilder(cfg.Toolchain, targetPlatform, dir, verbosity, actualJobs, keepGoing)
		if err != nil {
			printError(err)
			return
		}

		// Build options
		opts := build.Options{
			Config:    cfg,
			Variant:   selectedVariant,
			BuildDir:  cfg.BuildDir,
			Verbosity: verbosity,
			Jobs:      actualJobs,
			KeepGoing: keepGoing,
		}

		// Run build
		_, err = builder.Build(ctx, opts)
		if errors.Is(ctx.Err(), context.Canceled) {
			fmt.Println("Build interrupted - new changes detected")
			return
		}
		if err != nil {
			printError(err)
		}
	}

	// Initial build
	fmt.Println("Starting watch mode...")
	doBuild("initial build", false)

	// Setup watcher
	watcher, err := watch.NewWatcher(watch.Config{
		SourceDirs:   []string{"."},
		BuildCuePath: buildCuePath,
		DebounceDur:  300 * time.Millisecond,
		OnRebuild:    doBuild,
		OnError:      printError,
	})
	if err != nil {
		printError(err)
		return 1
	}
	defer watcher.Stop()

	// Start watching
	if err := watcher.Start(); err != nil {
		printError(err)
		return 1
	}

	// Show watching status with directory count
	fmt.Printf("\nWatching %d directories for changes (Ctrl+C to stop)...\n", watcher.WatchCount())

	// Wait for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nStopping watch mode...")
	return 0
}
