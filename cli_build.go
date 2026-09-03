package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	clerrors "github.com/loov/clue/internal/errors"
	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/clingy"
)

type validateCommand struct{ options *options }

func (*validateCommand) Setup(clingy.Parameters) {}

func (c *validateCommand) Execute(context.Context) error {
	return result(runValidate(c.options.dir, c.options.variant, c.options.target, c.options.verbosity()))
}

type buildCommand struct {
	options *options
	targets []string
}

func (c *buildCommand) Setup(params clingy.Parameters) {
	c.targets = params.Arg("target", "target to build", clingy.Repeated).([]string)
}

func (c *buildCommand) Execute(context.Context) error {
	o := c.options
	return result(runBuild(o.dir, o.variant, o.target, o.verbosity(), o.rebuildAll, o.jobs, o.keepGoing, o.profile, o.saveProfile, o.top, c.targets))
}

type cleanCommand struct{ options *options }

func (*cleanCommand) Setup(clingy.Parameters) {}

func (c *cleanCommand) Execute(context.Context) error {
	o := c.options
	return result(runClean(o.dir, o.variant, o.target, o.all, o.verbosity()))
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

func runValidate(dir, variant, target string, verbosity build.Verbosity) int {
	cfg, selectedVariant, _, err := loadConfig(dir, variant, target, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	_ = selectedVariant // Not used in validate

	// Build dependency graph
	order, err := config.ComputeBuildOrder(cfg)
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
	cfg, selectedVariant, targetPlatform, err := loadConfig(dir, variant, target, verbosity)
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

	// Show platform info before build (skip in quiet mode)
	if verbosity >= build.VerbosityNormal {
		if target == "" || targetPlatform == toolchain.HostPlatform() {
			fmt.Printf("Building for %s\n", targetPlatform)
		} else {
			fmt.Printf("Cross-compiling for %s\n", targetPlatform)
		}
	}

	// Create builder with toolchain and target platform
	builder, err := build.NewConfiguredBuilder(cfg.Toolchain, targetPlatform, dir, verbosity, actualJobs, keepGoing)
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
	defer buildCtx.Close()

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

func runClean(dir, variant, target string, all bool, verbosity build.Verbosity) int {
	buildDir := ".build"
	platform, err := parseTargetPlatform(target)
	if err != nil {
		printError(err)
		return 1
	}
	cfg, configErr := config.NewLoader().LoadForTarget(dir, platform)
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
