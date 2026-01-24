// Package main provides the CLI entry point for clue.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	clerrors "github.com/loov/clue/internal/errors"
	"github.com/loov/clue/internal/generate"
)

var version = "0.1.0-dev"

func main() {
	// Parse command line flags
	variantFlag := flag.String("variant", "", "Build variant (debug, release, or custom)")
	dirFlag := flag.String("dir", ".", "Directory containing clue.cue")
	noColorFlag := flag.Bool("no-color", false, "Disable colored output")
	quietFlag := flag.Bool("quiet", false, "Suppress all non-error output")
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

	// Validate verbosity flags
	if err := build.ValidateVerbosityFlags(*quietFlag, *verboseFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

	// Determine verbosity level
	verbosity := build.VerbosityNormal
	if *quietFlag {
		verbosity = build.VerbosityQuiet
	} else if *verboseFlag {
		verbosity = build.VerbosityVerbose
	}

	switch command {
	case "validate":
		os.Exit(runValidate(*dirFlag, *variantFlag, verbosity))
	case "build":
		os.Exit(runBuild(*dirFlag, *variantFlag, *targetFlag, verbosity, *rebuildAllFlag, *jobsFlag, *keepGoingFlag, flag.Args()[1:]))
	case "clean":
		os.Exit(runClean(*dirFlag, *variantFlag, *allFlag, verbosity))
	case "deps":
		os.Exit(runDeps(*dirFlag, *verboseFlag, flag.Args()[1:]))
	case "generate":
		os.Exit(runGenerate(*dirFlag, *variantFlag, *targetFlag, flag.Args()[1:]))
	case "run":
		os.Exit(runRun(*dirFlag, *variantFlag, verbosity, *jobsFlag, flag.Args()[1:]))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "Available commands: validate, build, clean, deps, generate, run")
		os.Exit(1)
	}
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
		fmt.Printf("Environment variables: %d configured\n", len(env.Variables))
		for name, value := range env.Variables {
			fmt.Printf("  %s = %s\n", name, value)
		}
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

func runBuild(dir, variant, target string, verbosity build.Verbosity, rebuildAll bool, jobs int, keepGoing bool, targets []string) int {
	cfg, selectedVariant, err := loadConfig(dir, variant, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	// Determine build directory (default .build)
	buildDir := ".build"

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

	// Show platform info before build (skip in quiet mode)
	if verbosity >= build.VerbosityNormal {
		if target == "" || targetPlatform == build.HostPlatform() {
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
		BuildDir:     buildDir,
		Verbosity:    verbosity,
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

func runClean(dir, variant string, all bool, verbosity build.Verbosity) int {
	// Determine build directory relative to project directory
	buildDir := filepath.Join(dir, ".build")

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
	var targetPlatform build.Platform
	if target == "" {
		targetPlatform = build.HostPlatform()
	} else {
		targetPlatform, err = build.ParseTarget(target)
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

func generateNinja(dir string, cfg *config.Config, platform build.Platform) int {
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

func generateCompileCommands(dir string, cfg *config.Config, variant string, _ build.Platform) int {
	outputPath := filepath.Join(dir, "compile_commands.json")
	err := generate.CompileCommands(generate.CompDBOptions{
		Config:     cfg,
		Variant:    variant,
		BuildDir:   cfg.BuildDir,
		OutputPath: outputPath,
		Toolchain:  cfg.Toolchain.Compiler,
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
		BuildDir:  ".build",
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
