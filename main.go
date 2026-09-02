// Package main provides the CLI entry point for clue.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	clerrors "github.com/loov/clue/internal/errors"
	"github.com/loov/clue/internal/generate"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/watch"
)

var version = "0.1.0-dev"

type cliOptions struct {
	variant, dir, target                  string
	noColor, quiet, verbose, version, all bool
	rebuildAll, keepGoing                 bool
	profile, saveProfile                  bool
	jobs, top                             int
}

func registerFlags(fs *flag.FlagSet, opts *cliOptions) {
	fs.StringVar(&opts.variant, "variant", "", "Build variant (debug, release, or custom)")
	fs.StringVar(&opts.dir, "dir", ".", "Directory containing clue.cue")
	fs.BoolVar(&opts.noColor, "no-color", false, "Disable colored output")
	fs.BoolVar(&opts.quiet, "quiet", false, "Suppress all non-error output")
	fs.BoolVar(&opts.verbose, "v", false, "Verbose output")
	fs.BoolVar(&opts.version, "version", false, "Print version and exit")
	fs.BoolVar(&opts.all, "all", false, "Clean all build variants (for clean command)")
	fs.BoolVar(&opts.rebuildAll, "rebuild-all", false, "Force rebuild of all files")
	fs.IntVar(&opts.jobs, "j", 0, "Number of parallel jobs (0 = half of CPU cores, -1 = unlimited)")
	fs.BoolVar(&opts.keepGoing, "keep-going", false, "Continue building despite errors")
	fs.StringVar(&opts.target, "target", "", "Cross-compilation target (e.g., linux-arm64, darwin-amd64)")
	fs.BoolVar(&opts.profile, "profile", false, "Enable build profiling")
	fs.BoolVar(&opts.saveProfile, "save-profile", false, "Save profile to profile.json in build directory")
	fs.IntVar(&opts.top, "top", 10, "Number of slowest files to show (used with -v)")
}

func parseCLI(args []string) (cliOptions, string, []string, error) {
	// Let flag parse the prefix to locate the command without guessing which
	// arguments are flag values, then parse both sides of the command together.
	var probeOptions cliOptions
	probe := flag.NewFlagSet("clue", flag.ContinueOnError)
	registerFlags(probe, &probeOptions)
	if err := probe.Parse(args); err != nil {
		return cliOptions{}, "", nil, err
	}

	command := "validate"
	commandIndex := len(args)
	if rest := probe.Args(); len(rest) > 0 {
		command = rest[0]
		commandIndex = len(args) - len(rest)
	}

	flagArgs := append([]string{}, args[:commandIndex]...)
	if commandIndex < len(args) {
		flagArgs = append(flagArgs, args[commandIndex+1:]...)
	}

	var opts cliOptions
	fs := flag.NewFlagSet("clue", flag.ContinueOnError)
	registerFlags(fs, &opts)
	if err := fs.Parse(flagArgs); err != nil {
		return cliOptions{}, "", nil, err
	}
	return opts, command, fs.Args(), nil
}

func main() {
	opts, command, args, err := parseCLI(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}

	if opts.version {
		fmt.Printf("clue version %s\n", version)
		os.Exit(0)
	}
	if err := os.Chdir(opts.dir); err != nil {
		fmt.Fprintf(os.Stderr, "failed to enter project directory %q: %v\n", opts.dir, err)
		os.Exit(1)
	}
	opts.dir = "."

	// Validate verbosity flags
	if err := build.ValidateVerbosityFlags(opts.quiet, opts.verbose); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Configure colors
	if opts.noColor {
		clerrors.SetNoColor(true)
	}

	// Determine verbosity level
	verbosity := build.VerbosityNormal
	if opts.quiet {
		verbosity = build.VerbosityQuiet
	} else if opts.verbose {
		verbosity = build.VerbosityVerbose
	}

	switch command {
	case "validate":
		os.Exit(runValidate(opts.dir, opts.variant, verbosity))
	case "build":
		os.Exit(runBuild(opts.dir, opts.variant, opts.target, verbosity, opts.rebuildAll, opts.jobs, opts.keepGoing, opts.profile, opts.saveProfile, opts.top, args))
	case "clean":
		os.Exit(runClean(opts.dir, opts.variant, opts.all, verbosity))
	case "deps":
		os.Exit(runDeps(opts.dir, opts.verbose, args))
	case "generate":
		os.Exit(runGenerate(opts.dir, opts.variant, opts.target, args))
	case "run":
		os.Exit(runRun(opts.dir, opts.variant, verbosity, opts.jobs, args))
	case "watch":
		os.Exit(runWatch(opts.dir, opts.variant, opts.target, verbosity, opts.jobs, opts.keepGoing))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "Available commands: validate, build, clean, deps, generate, run, watch")
		os.Exit(1)
	}
}

// isProfilingEnabled checks if profiling should be enabled (precedence: flag > env)
func isProfilingEnabled(flagValue bool) bool {
	if flagValue {
		return true
	}
	if env := os.Getenv("CLUE_PROFILE"); env != "" {
		return env == "1" || strings.EqualFold(env, "true")
	}
	return false
}

// loadConfig loads and prepares configuration with variant and environment variables
func loadConfig(dir, variant string, verbosity build.Verbosity) (*config.Config, string, error) {
	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		return nil, "", err
	}

	if verbosity >= build.VerbosityNormal {
		fmt.Printf("Loaded configuration: %s\n", cfg.Name)
		fmt.Printf("  Version: %s\n", cfg.Version)
		fmt.Printf("  Toolchain: %s (std: %s)\n", cfg.Toolchain.Compiler, cfg.Toolchain.Std)
		fmt.Printf("  Targets: %d\n", len(cfg.Targets))
		fmt.Printf("  Variants: %d\n", len(cfg.Variants))
	}

	// Select and apply variant
	selector := config.NewVariantSelector()
	if variant != "" {
		selector.SetCLIFlag(variant)
	}
	selectedVariant := selector.Select()

	// Only apply variant if variants are defined
	if len(cfg.Variants) > 0 {
		cfg, err = config.ApplyVariant(cfg, selectedVariant)
		if err != nil {
			return nil, "", err
		}
		if verbosity >= build.VerbosityNormal {
			fmt.Printf("Applied variant: %s\n", selectedVariant)
			fmt.Printf("  Optimization: %s\n", cfg.ActiveVariant.Optimization)
			fmt.Printf("  Debug info: %t\n", cfg.ActiveVariant.DebugInfo)
		}
	}

	// Resolve environment variables
	env, err := config.ResolveEnvVars(cfg)
	if err != nil {
		return nil, "", err
	}

	// Apply environment-based conditionals to config
	if len(env.Variables) > 0 {
		cfg, err = config.ApplyEnvVars(cfg, env)
		if err != nil {
			return nil, "", err
		}
	}

	// Print env var status in verbose mode
	if verbosity >= build.VerbosityNormal && len(env.Used) > 0 {
		fmt.Printf("Environment variables from system: %s\n", strings.Join(env.Used, ", "))
	}
	if verbosity >= build.VerbosityNormal && len(env.Variables) > 0 {
		names := make([]string, 0, len(env.Variables))
		for name := range env.Variables {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Printf("Environment variables configured: %s\n", strings.Join(names, ", "))
	}

	return cfg, selectedVariant, nil
}

func runValidate(dir, variant string, verbosity build.Verbosity) int {
	cfg, selectedVariant, err := loadConfig(dir, variant, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	_ = selectedVariant // Not used in validate

	// Build dependency graph
	order, err := config.GetBuildOrder(cfg)
	if err != nil {
		printError(err)
		return 1
	}

	if verbosity >= build.VerbosityNormal {
		fmt.Printf("Build order: %s\n", strings.Join(order, " -> "))
	}

	// Print summary (skip in quiet mode)
	if verbosity == build.VerbosityQuiet {
		return 0
	}

	fmt.Printf("%s Configuration valid: %s\n",
		clerrors.Help("[OK]"),
		cfg.Name)
	fmt.Printf("  Targets: %d\n", len(cfg.Targets))
	for _, name := range order {
		target := cfg.Targets[name]
		fmt.Printf("    - %s (%s): %d sources\n",
			name, target.Type, len(target.Sources))
	}

	return 0
}

func runBuild(dir, variant, target string, verbosity build.Verbosity, rebuildAll bool, jobs int, keepGoing bool, profile, saveProfile bool, topN int, targets []string) int {
	cfg, selectedVariant, err := loadConfig(dir, variant, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	// Compute actual job count
	actualJobs := jobs
	if actualJobs == 0 {
		// Default: half of CPU cores (minimum 1)
		actualJobs = max(runtime.NumCPU()/2, 1)
	} else if actualJobs < 0 {
		// Unlimited: use all cores
		actualJobs = runtime.NumCPU()
	}

	// Determine target platform
	var targetPlatform toolchain.Platform
	if target == "" {
		targetPlatform = toolchain.HostPlatform()
	} else {
		var err error
		targetPlatform, err = toolchain.ParseTarget(target)
		if err != nil {
			printError(err)
			return 1
		}
	}

	// Show platform info before build (skip in quiet mode)
	if verbosity >= build.VerbosityNormal {
		if target == "" || targetPlatform == toolchain.HostPlatform() {
			fmt.Printf("Building for %s\n", targetPlatform)
		} else {
			fmt.Printf("Cross-compiling for %s\n", targetPlatform)
		}
	}

	// Create builder with toolchain and target platform
	builder, err := build.NewBuilder(cfg.Toolchain.Compiler, targetPlatform, verbosity, actualJobs, keepGoing)
	if err != nil {
		printError(err)
		return 1
	}

	// Build options
	opts := build.Options{
		Config:       cfg,
		Variant:      selectedVariant,
		BuildDir:     cfg.BuildDir,
		Verbosity:    verbosity,
		Targets:      targets,
		ForceRebuild: rebuildAll,
		Jobs:         actualJobs,
		KeepGoing:    keepGoing,
		Profile:      isProfilingEnabled(profile),
		SaveProfile:  saveProfile,
		TopN:         topN,
	}

	// Setup signal handling
	buildCtx := build.SetupSignalHandling()

	// Execute build with cancellable context
	result, err := builder.Build(buildCtx.Ctx, opts)

	// Handle cancellation
	if buildCtx.IsCancelled() && err == nil {
		fmt.Println("\nBuild cancelled.")
		return 130
	}

	if err != nil {
		printError(err)
		return 1
	}

	// Report success
	if !result.Success {
		return 1
	}

	return 0
}

func runClean(dir, variant string, all bool, verbosity build.Verbosity) int {
	buildDir := ".build"
	cfg, configErr := config.NewLoader().Load(dir)
	if configErr == nil {
		buildDir = cfg.BuildDir
	}

	// If not cleaning all, need to determine variant
	if !all {
		// If variant not specified, use default from selector
		if variant == "" {
			// Load config to get default variant behavior
			if configErr != nil {
				// If can't load config, default to "debug"
				variant = "debug"
			} else {
				selector := config.NewVariantSelector()
				variant = selector.Select()
			}
		}
	}

	// Execute clean
	result, err := build.Clean(build.CleanOptions{
		BuildDir: buildDir,
		Variant:  variant,
		All:      all,
	})
	if err != nil {
		printError(err)
		return 1
	}

	// Print result (skip in quiet mode)
	if verbosity >= build.VerbosityNormal {
		fmt.Println(result.String())
	}

	return 0
}

func runDeps(dir string, verbose bool, args []string) int {
	// Parse deps subcommand
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clue deps <list|fetch|clean|update> [options]")
		fmt.Fprintln(os.Stderr, "\nSubcommands:")
		fmt.Fprintln(os.Stderr, "  list       Show dependency status")
		fmt.Fprintln(os.Stderr, "  fetch      Download dependencies")
		fmt.Fprintln(os.Stderr, "  clean      Remove dependency cache")
		fmt.Fprintln(os.Stderr, "  update     Check for dependency updates (not yet implemented)")
		return 1
	}

	subCmd := args[0]

	// Load config
	verbosity := build.VerbosityNormal
	if verbose {
		verbosity = build.VerbosityVerbose
	}
	cfg, _, err := loadConfig(dir, "", verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	// Setup context for operations
	ctx := context.Background()

	switch subCmd {
	case "list":
		if err := deps.RunList(cfg.Dependencies, verbose); err != nil {
			printError(err)
			return 1
		}

	case "fetch":
		// Parse fetch options
		name := ""
		if len(args) > 1 {
			name = args[1]
		}

		if err := deps.RunFetch(ctx, cfg.Dependencies, deps.FetchOptions{
			Verbose: verbose,
			Name:    name,
		}); err != nil {
			printError(err)
			return 1
		}

	case "clean":
		// Parse clean options
		name := ""
		if len(args) > 1 {
			name = args[1]
		}

		if err := deps.RunClean(cfg.Dependencies, name); err != nil {
			printError(err)
			return 1
		}

	case "update":
		if err := deps.RunUpdate(ctx, cfg.Dependencies); err != nil {
			printError(err)
			return 1
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown deps subcommand: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "Available subcommands: list, fetch, clean, update")
		return 1
	}

	return 0
}

func runGenerate(dir, variant, target string, args []string) int {
	// Parse subcommand
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clue generate <ninja|compile-commands|all> [options]")
		fmt.Fprintln(os.Stderr, "\nSubcommands:")
		fmt.Fprintln(os.Stderr, "  ninja              Generate build.ninja")
		fmt.Fprintln(os.Stderr, "  compile-commands   Generate compile_commands.json")
		fmt.Fprintln(os.Stderr, "  all                Generate both files")
		return 1
	}

	subCmd := args[0]

	// Load config without applying variant - generators handle variants internally
	loader := config.NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		printError(err)
		return 1
	}

	// Resolve environment variables (but don't apply variant)
	env, err := config.ResolveEnvVars(cfg)
	if err != nil {
		printError(err)
		return 1
	}
	if len(env.Variables) > 0 {
		cfg, err = config.ApplyEnvVars(cfg, env)
		if err != nil {
			printError(err)
			return 1
		}
	}

	// Determine selected variant for compile-commands
	selector := config.NewVariantSelector()
	if variant != "" {
		selector.SetCLIFlag(variant)
	}
	selectedVariant := selector.Select()

	// Determine target platform
	var targetPlatform toolchain.Platform
	if target == "" {
		targetPlatform = toolchain.HostPlatform()
	} else {
		targetPlatform, err = toolchain.ParseTarget(target)
		if err != nil {
			printError(err)
			return 1
		}
	}

	switch subCmd {
	case "ninja":
		return generateNinja(dir, cfg, targetPlatform)
	case "compile-commands":
		return generateCompileCommands(dir, cfg, selectedVariant, targetPlatform)
	case "all":
		if ret := generateNinja(dir, cfg, targetPlatform); ret != 0 {
			return ret
		}
		return generateCompileCommands(dir, cfg, selectedVariant, targetPlatform)
	default:
		fmt.Fprintf(os.Stderr, "Unknown generate subcommand: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "Available subcommands: ninja, compile-commands, all")
		return 1
	}
}

func printError(err error) {
	switch e := err.(type) {
	case *clerrors.RichError:
		fmt.Fprint(os.Stderr, e.Format())
	case *clerrors.ErrorList:
		fmt.Fprint(os.Stderr, e.Format())
	default:
		fmt.Fprintf(os.Stderr, "%s %v\n", clerrors.Error("error:"), err)
	}
}

func generateNinja(dir string, cfg *config.Config, platform toolchain.Platform) int {
	// Collect all variant names
	variants := make([]string, 0, len(cfg.Variants))
	for name := range cfg.Variants {
		variants = append(variants, name)
	}
	// If no variants defined, use "debug" as default
	if len(variants) == 0 {
		variants = []string{"debug"}
	}
	// Sort for consistent output
	sort.Strings(variants)

	outputPath := filepath.Join(dir, "build.ninja")
	err := generate.Ninja(generate.NinjaOptions{
		Config:     cfg,
		Variants:   variants,
		BuildDir:   cfg.BuildDir,
		OutputPath: outputPath,
		Toolchain:  cfg.Toolchain.Compiler,
		Platform:   platform,
	})
	if err != nil {
		printError(err)
		return 1
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return 0
}

func generateCompileCommands(dir string, cfg *config.Config, variant string, platform toolchain.Platform) int {
	outputPath := filepath.Join(dir, "compile_commands.json")
	err := generate.CompileCommands(generate.CompDBOptions{
		Config:     cfg,
		Variant:    variant,
		BuildDir:   cfg.BuildDir,
		OutputPath: outputPath,
		Toolchain:  cfg.Toolchain.Compiler,
		Platform:   platform,
	})
	if err != nil {
		printError(err)
		return 1
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return 0
}

func runRun(dir, variant string, verbosity build.Verbosity, jobs int, args []string) int {
	// Parse target name (first arg) and remaining args
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clue run <target> [args...]")
		fmt.Fprintln(os.Stderr, "\nRuns an executable target, building it first if necessary.")
		return 1
	}

	targetName := args[0]
	execArgs := args[1:]

	// Load configuration
	cfg, selectedVariant, err := loadConfig(dir, variant, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	// Compute actual job count (same as runBuild)
	actualJobs := jobs
	if actualJobs == 0 {
		actualJobs = max(runtime.NumCPU()/2, 1)
	} else if actualJobs < 0 {
		actualJobs = runtime.NumCPU()
	}

	// Setup signal handling for build phase
	buildCtx := build.SetupSignalHandling()

	// Run target
	result, err := build.RunTarget(buildCtx.Ctx, build.RunOptions{
		Config:    cfg,
		Variant:   selectedVariant,
		BuildDir:  cfg.BuildDir,
		Target:    targetName,
		Args:      execArgs,
		Verbosity: verbosity,
		Jobs:      actualJobs,
	})

	if buildCtx.IsCancelled() {
		fmt.Println("\nCancelled.")
		return 130
	}

	if err != nil {
		printError(err)
		return 1
	}

	return result.ExitCode
}

func runWatch(dir, variant, target string, verbosity build.Verbosity, jobs int, keepGoing bool) int {
	// Load initial config
	cfg, selectedVariant, err := loadConfig(dir, variant, verbosity)
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

	// Determine target platform (same as runBuild)
	var targetPlatform toolchain.Platform
	if target == "" {
		targetPlatform = toolchain.HostPlatform()
	} else {
		var err error
		targetPlatform, err = toolchain.ParseTarget(target)
		if err != nil {
			printError(err)
			return 1
		}
	}

	buildCuePath := filepath.Join(dir, "clue.cue")

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
		if isConfigChange {
			fmt.Printf("[%s] Config changed: %s - reloading...\n", now, trigger)
			// Reload config
			var err error
			cfg, selectedVariant, err = loadConfig(dir, variant, verbosity)
			if err != nil {
				printError(err)
				buildMu.Unlock()
				return
			}
		} else {
			fmt.Printf("[%s] Change detected: %s\n", now, trigger)
		}
		fmt.Printf("[%s] Rebuilding...\n", now)

		// Create new context for this build
		ctx, cancel := context.WithCancel(context.Background())
		currentCancel = cancel
		buildMu.Unlock()

		// Create builder
		builder, err := build.NewBuilder(cfg.Toolchain.Compiler, targetPlatform, verbosity, actualJobs, keepGoing)
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
		if ctx.Err() == context.Canceled {
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
