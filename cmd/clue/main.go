// Package main provides the CLI entry point for clue.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	clerrors "github.com/loov/clue/internal/errors"
)

var (
	version = "0.1.0-dev"
)

func main() {
	// Parse command line flags
	variantFlag := flag.String("variant", "", "Build variant (debug, release, or custom)")
	dirFlag := flag.String("dir", ".", "Directory containing clue.cue")
	noColorFlag := flag.Bool("no-color", false, "Disable colored output")
	verboseFlag := flag.Bool("v", false, "Verbose output")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	allFlag := flag.Bool("all", false, "Clean all build variants (for clean command)")
	rebuildAllFlag := flag.Bool("rebuild-all", false, "Force rebuild of all files")
	jobsFlag := flag.Int("j", 0, "Number of parallel jobs (0 = half of CPU cores, -1 = unlimited)")
	keepGoingFlag := flag.Bool("keep-going", false, "Continue building despite errors")
	targetFlag := flag.String("target", "", "Cross-compilation target (e.g., linux-arm64, darwin-amd64)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("clue version %s\n", version)
		os.Exit(0)
	}

	// Configure colors
	if *noColorFlag {
		clerrors.SetNoColor(true)
	}

	// Get command (default to validate)
	args := flag.Args()
	command := "validate"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "validate":
		os.Exit(runValidate(*dirFlag, *variantFlag, *verboseFlag))
	case "build":
		os.Exit(runBuild(*dirFlag, *variantFlag, *targetFlag, *verboseFlag, *rebuildAllFlag, *jobsFlag, *keepGoingFlag, flag.Args()[1:]))
	case "clean":
		os.Exit(runClean(*dirFlag, *variantFlag, *allFlag))
	case "deps":
		os.Exit(runDeps(*dirFlag, *verboseFlag, flag.Args()[1:]))
	case "run":
		fmt.Println("Run command not yet implemented (Phase 2)")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "Available commands: validate, build, clean, deps, run")
		os.Exit(1)
	}
}

// loadConfig loads and prepares configuration with variant and environment variables
func loadConfig(dir, variant string, verbose bool) (*config.Config, string, error) {
	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		return nil, "", err
	}

	if verbose {
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
		if verbose {
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
	if verbose && len(env.Used) > 0 {
		fmt.Printf("Environment variables from system: %s\n", strings.Join(env.Used, ", "))
	}
	if verbose && len(env.Variables) > 0 {
		fmt.Printf("Environment variables: %d configured\n", len(env.Variables))
		for name, value := range env.Variables {
			fmt.Printf("  %s = %s\n", name, value)
		}
	}

	return cfg, selectedVariant, nil
}

func runValidate(dir, variant string, verbose bool) int {
	cfg, selectedVariant, err := loadConfig(dir, variant, verbose)
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

	if verbose {
		fmt.Printf("Build order: %s\n", strings.Join(order, " -> "))
	}

	// Print summary
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

func runBuild(dir, variant, target string, verbose bool, rebuildAll bool, jobs int, keepGoing bool, targets []string) int {
	cfg, selectedVariant, err := loadConfig(dir, variant, verbose)
	if err != nil {
		printError(err)
		return 1
	}

	// Determine build directory (default "build")
	buildDir := "build"

	// Compute actual job count
	actualJobs := jobs
	if actualJobs == 0 {
		// Default: half of CPU cores (minimum 1)
		actualJobs = runtime.NumCPU() / 2
		if actualJobs < 1 {
			actualJobs = 1
		}
	} else if actualJobs < 0 {
		// Unlimited: use all cores
		actualJobs = runtime.NumCPU()
	}

	// Determine target platform
	var targetPlatform build.Platform
	if target == "" {
		targetPlatform = build.HostPlatform()
	} else {
		var err error
		targetPlatform, err = build.ParseTarget(target)
		if err != nil {
			printError(err)
			return 1
		}
	}

	// Show platform info before build
	if target == "" || targetPlatform == build.HostPlatform() {
		fmt.Printf("Building for %s\n", targetPlatform)
	} else {
		fmt.Printf("Cross-compiling for %s\n", targetPlatform)
	}

	// Create builder with toolchain and target platform
	builder, err := build.NewBuilder(cfg.Toolchain.Compiler, targetPlatform, verbose, actualJobs, keepGoing)
	if err != nil {
		printError(err)
		return 1
	}

	// Build options
	opts := build.BuildOptions{
		Config:       cfg,
		Variant:      selectedVariant,
		BuildDir:     buildDir,
		Verbose:      verbose,
		Targets:      targets,
		ForceRebuild: rebuildAll,
		Jobs:         actualJobs,
		KeepGoing:    keepGoing,
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

func runClean(dir, variant string, all bool) int {
	// Determine build directory relative to project directory
	buildDir := filepath.Join(dir, "build")

	// If not cleaning all, need to determine variant
	if !all {
		// If variant not specified, use default from selector
		if variant == "" {
			// Load config to get default variant behavior
			_, err := config.NewLoader().Load(dir)
			if err != nil {
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

	// Print result
	fmt.Println(result.String())

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
	cfg, _, err := loadConfig(dir, "", verbose)
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
			CIMode:  false,
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
