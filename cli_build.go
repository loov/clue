package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/diagnostic"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	"github.com/zeebo/clingy"
)

type validateCommand struct{ options *options }

func (*validateCommand) Setup(clingy.Parameters) {}

func (c *validateCommand) Execute(ctx context.Context) error {
	return result(runValidate(ctx, c.options.dir, c.options.variant, c.options.target, c.options.verbosity()))
}

type buildCommand struct {
	options *options
	targets []string
}

func (c *buildCommand) Setup(params clingy.Parameters) {
	c.targets = params.Arg("target", "target to build", clingy.Repeated).([]string)
}

func (c *buildCommand) Execute(ctx context.Context) error {
	o := c.options
	return result(runBuild(ctx, o.dir, o.variant, o.target, o.verbosity(), o.rebuildAll, o.jobs, o.keepGoing, o.profile, o.saveProfile, o.top, c.targets))
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

func runValidate(ctx context.Context, dir, variant, target string, verbosity build.Verbosity) int {
	cfg, selectedVariant, targetPlatform, err := loadConfig(dir, variant, target, verbosity)
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

	if err := validatePkgConfig(ctx, cfg, targetPlatform); err != nil {
		printError(err)
		return 1
	}

	// Print summary (skip in quiet mode)
	if verbosity == build.VerbosityQuiet {
		return 0
	}

	fmt.Printf("%s Configuration valid: %s\n",
		diagnostic.Help("[OK]"),
		cfg.Name)
	fmt.Printf("  Targets: %d\n", len(cfg.Targets))
	for _, name := range order {
		target := cfg.Targets[name]
		fmt.Printf("    - %s (%s): %d sources\n",
			name, target.Type, len(target.Sources))
	}

	return 0
}

// validatePkgConfig resolves every pkg_config dependency, so a missing
// package fails validation instead of a build that has started compiling.
// A container toolchain's packages are in its image, which clue build
// builds or pulls first, so they are checked by the build.
func validatePkgConfig(ctx context.Context, cfg *config.Config, platform toolchain.Platform) error {
	if cfg.Toolchain.Container != nil {
		return nil
	}
	var packages []*deps.PkgConfigDependency
	for _, name := range slices.Sorted(maps.Keys(cfg.Dependencies)) {
		if pkg, ok := cfg.Dependencies[name].(*deps.PkgConfigDependency); ok {
			packages = append(packages, pkg)
		}
	}
	if len(packages) == 0 {
		return nil
	}
	tc, err := all.NewProjectToolchain(cfg.Toolchain, platform, cfg.Dir)
	if err != nil {
		return err
	}
	var failures []error
	for _, pkg := range packages {
		if _, err := plan.ResolvePkgConfig(ctx, cfg, tc, pkg); err != nil {
			failures = append(failures, fmt.Errorf("dependency %q: %w", pkg.Name(), err))
		}
	}
	return errors.Join(failures...)
}

func runBuild(ctx context.Context, dir, variant, target string, verbosity build.Verbosity, rebuildAll bool, jobs int, keepGoing bool, profile, saveProfile bool, topN int, targets []string) int {
	cfg, selectedVariant, targetPlatform, err := loadConfig(dir, variant, target, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	actualJobs := resolvedJobs(jobs)

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

	result, err := build.Build(ctx, opts, targetPlatform)
	if ctx.Err() != nil {
		return 1
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
				variant = config.SelectConfigVariant(cfg, "")
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
