// Package main provides the CLI entry point for clue.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/loov/clue/internal/config"
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
		fmt.Println("Build command not yet implemented (Phase 2)")
		os.Exit(0)
	case "clean":
		fmt.Println("Clean command not yet implemented (Phase 2)")
		os.Exit(0)
	case "run":
		fmt.Println("Run command not yet implemented (Phase 2)")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "Available commands: validate, build, clean, run")
		os.Exit(1)
	}
}

func runValidate(dir, variant string, verbose bool) int {
	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		printError(err)
		return 1
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
			printError(err)
			return 1
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
		printError(err)
		return 1
	}
	if verbose && len(env.Variables) > 0 {
		fmt.Printf("Environment variables: %d configured\n", len(env.Variables))
		for name, value := range env.Variables {
			fmt.Printf("  %s = %s\n", name, value)
		}
	}

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
