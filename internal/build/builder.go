// Package build provides compilation, linking, and caching functionality
// for building C and C++ projects.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/deps/fetch"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/profile"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
)

// Options holds options for a build operation
type Options struct {
	Config       *config.Config
	Variant      string    // "debug" or "release"
	BuildDir     string    // Build output root (default: ".build")
	Verbosity    Verbosity // Verbosity level (quiet/normal/verbose)
	Targets      []string  // Specific targets to build (empty = all)
	ForceRebuild bool      // Force rebuild of all files
	Jobs         int       // Number of parallel jobs
	KeepGoing    bool      // Continue building despite errors
	SkipDeps     bool      // Skip dependency building (for `clue deps build`)
	Profile      bool      // Enable detailed per-file profiling
	SaveProfile  bool      // Save profile.json to build directory
	TopN         int       // Number of slowest files to show
}

// TargetResult holds the result of building a single target
type TargetResult struct {
	Name     string
	Type     string // "executable", "static_library", or "shared_library"
	Output   string // Path to output artifact
	Sources  int    // Number of source files
	Duration time.Duration
	Success  bool
}

// Result holds the overall build result
type Result struct {
	Targets  []TargetResult
	Duration time.Duration
	Success  bool
}

// Builder orchestrates the build process
type Builder struct {
	executor            *executor
	compiler            *compiler
	linker              *linker
	cacheManager        *cache.Manager
	parallelCompiler    *parallelCompiler
	toolchain           toolchain.Toolchain
	target              toolchain.Platform
	depResults          map[string]*depBuildResult // Built dependencies
	profiler            *profile.Profiler
	targetModuleOutputs map[string]map[string]string
}

// NewBuilder creates a new Builder with the specified toolchain and target platform
func NewBuilder(toolchainName string, target toolchain.Platform, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	toolchain, err := all.NewToolchain(toolchainName, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create toolchain: %w", err)
	}
	return newBuilder(toolchain, target, verbosity, jobs, keepGoing)
}

// NewConfiguredBuilder creates a builder from project toolchain settings.
func NewConfiguredBuilder(settings config.Toolchain, target toolchain.Platform, projectDir string, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	toolchain, err := all.NewProjectToolchain(settings, target, projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create toolchain: %w", err)
	}
	return newBuilder(toolchain, target, verbosity, jobs, keepGoing)
}

func newBuilder(tc toolchain.Toolchain, target toolchain.Platform, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	// Validate toolchain exists
	if err := toolchain.ValidateToolchain(tc); err != nil {
		return nil, err
	}

	verbose := verbosity == VerbosityVerbose

	executor := newExecutor(executorConfig{
		Verbose:      verbose,
		StreamOutput: true,
		WorkDir:      "",
		Environment:  toolchain.Environment(tc),
		WrapCommand:  toolchainCommandWrapper(tc),
	})

	compiler := newCompiler(executor, tc)

	return &Builder{
		executor:         executor,
		compiler:         compiler,
		linker:           newLinker(executor, tc, target),
		parallelCompiler: newParallelCompiler(tc, jobs, keepGoing, verbosity),
		toolchain:        tc,
		target:           target,
	}, nil
}

// OutputPath returns the final artifact path
// Executable: build/variant/bin/target
// Static lib: build/variant/lib/libtarget.a
// Shared lib: build/variant/lib/libtarget.so/.dylib (platform-specific)
func (b *Builder) outputPath(buildDir, variant, target, targetType string) string {
	return plan.ArtifactPath(buildDir, variant, target, targetType, b.target)
}

// Build builds all targets in dependency order
func (b *Builder) Build(ctx context.Context, opts Options) (*Result, error) {
	start := time.Now()
	for _, target := range opts.Targets {
		if _, ok := opts.Config.Targets[target]; !ok {
			return nil, fmt.Errorf("target %q not found", target)
		}
	}

	// Initialize profiler
	b.profiler = profile.NewProfiler(opts.Profile)
	b.profiler.Start()
	b.parallelCompiler.profiler = b.profiler

	// Print platform and toolchain information (skip in quiet mode)
	if opts.Verbosity >= VerbosityNormal {
		fmt.Printf("Building for %s\n", b.target)
		if b.target.IsCrossCompile() {
			fmt.Printf("Cross-compiling using %s\n", b.toolchain)
		}
	}

	// Initialize cache manager
	var err error
	b.targetModuleOutputs = make(map[string]map[string]string)
	b.cacheManager, err = cache.NewManager(opts.BuildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Build dependencies first (unless skipped)
	if !opts.SkipDeps {
		b.depResults, err = b.buildDependencies(ctx, opts, "")
		if err != nil {
			return nil, fmt.Errorf("failed to build dependencies: %w", err)
		}
	} else {
		b.depResults = make(map[string]*depBuildResult)
	}

	// Get build order from config
	buildOrder, err := config.ComputeBuildOrder(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to determine build order: %w", err)
	}
	var selected map[string]bool
	if len(opts.Targets) > 0 {
		selected = make(map[string]bool)
		var include func(string)
		include = func(name string) {
			if selected[name] {
				return
			}
			selected[name] = true
			for _, dependency := range opts.Config.Targets[name].Depends {
				if _, internal := opts.Config.Targets[dependency]; internal {
					include(dependency)
				}
			}
		}
		for _, target := range opts.Targets {
			include(target)
		}
	}

	// Count total source files for progress
	totalSources := 0
	for _, targetName := range buildOrder {
		if selected != nil && !selected[targetName] {
			continue
		}

		target := opts.Config.Targets[targetName]
		totalSources += len(target.Sources)
	}

	// Create progress tracker
	progress := newProgress(totalSources, opts.Verbosity)

	// Build each target in order
	var results []TargetResult
	for _, targetName := range buildOrder {
		if selected != nil && !selected[targetName] {
			continue
		}

		target := opts.Config.Targets[targetName]

		// Build the target
		result, err := b.buildTarget(ctx, opts, target, progress)
		if err != nil {
			// Fail-fast: report error and stop
			progress.Error(targetName, err)
			return &Result{
				Targets:  results,
				Duration: time.Since(start),
				Success:  false,
			}, err
		}

		results = append(results, *result)
	}

	// Print summary
	progress.Summary()

	// Print profiling results if enabled
	if opts.Profile && opts.Verbosity >= VerbosityVerbose {
		b.profiler.PrintSlowestFiles(opts.TopN, os.Stdout)
	}

	// Save profile.json if requested
	if opts.SaveProfile {
		profilePath := filepath.Join(opts.BuildDir, opts.Variant, "profile.json")
		if err := b.profiler.WriteTrace(profilePath); err != nil {
			if opts.Verbosity >= VerbosityNormal {
				fmt.Printf("Warning: failed to save profile: %v\n", err)
			}
		} else if opts.Verbosity >= VerbosityNormal {
			fmt.Printf("Profile saved to: %s\n", profilePath)
		}
	}

	// Show total build time (not in quiet mode)
	if opts.Verbosity >= VerbosityNormal {
		totalDuration := time.Since(start)
		fmt.Printf("\nTotal build time: %s\n", totalDuration.String())
	}

	return &Result{
		Targets:  results,
		Duration: time.Since(start),
		Success:  true,
	}, nil
}

// BuildDependency fetches and builds one external dependency and its prerequisites.
func (b *Builder) BuildDependency(ctx context.Context, opts Options, name string) error {
	if _, ok := opts.Config.Dependencies[name]; !ok {
		return fmt.Errorf("dependency %q not found", name)
	}
	var err error
	b.cacheManager, err = cache.NewManager(opts.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager: %w", err)
	}
	b.depResults, err = b.buildDependencies(ctx, opts, name)
	return err
}

func (b *Builder) buildDependencies(ctx context.Context, opts Options, only string) (map[string]*depBuildResult, error) {
	// Check if there are any dependencies
	if len(opts.Config.Dependencies) == 0 {
		return make(map[string]*depBuildResult), nil
	}

	if opts.Verbosity >= VerbosityNormal {
		fmt.Println("Building dependencies...")
	}

	// Create dependency manager
	mgr, err := fetch.NewManager(
		".", // Current directory as project root
		opts.Config.Dependencies,
		fetch.Options{
			Verbose: opts.Verbosity == VerbosityVerbose,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependency manager: %w", err)
	}

	// Get build order using resolver (respects inter-dependency order)
	resolver := deps.NewResolver(opts.Config.Dependencies)
	buildOrder, err := resolver.BuildOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependency build order: %w", err)
	}
	if only != "" {
		for i, name := range buildOrder {
			if name == only {
				buildOrder = buildOrder[:i+1]
				break
			}
		}
		for _, name := range buildOrder {
			if err := mgr.FetchOne(ctx, name); err != nil {
				return nil, fmt.Errorf("failed to fetch dependency %q: %w", name, err)
			}
		}
	} else if err := mgr.FetchAll(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch dependencies: %w", err)
	}

	// Create dependency builder
	depBuilder := newDepBuilder(b.compiler, b.linker, b.toolchain, opts.Verbosity)
	depBuilder.cache = b.cacheManager

	// Build each dependency
	results := make(map[string]*depBuildResult)
	totalFiles := 0
	depStart := time.Now()

	for _, depName := range buildOrder {
		dep := opts.Config.Dependencies[depName]
		if dep == nil {
			return nil, fmt.Errorf("dependency %q not found", depName)
		}
		if pkg, ok := dep.(*deps.PkgConfigDependency); ok {
			usage, err := pkg.ResolveWithRunner(ctx, func(ctx context.Context, name string, args ...string) (string, error) {
				return toolchain.Output(ctx, b.toolchain, ".", name, args...)
			})
			if err != nil {
				return nil, fmt.Errorf("failed to resolve dependency %q: %w", depName, err)
			}
			results[depName] = &depBuildResult{Name: depName, Type: "pkg_config", Usage: usage}
			continue
		}

		// Get source path from cache
		sourcePath := dep.CachePath(".")

		// Build dependency
		buildOpts := depBuildOptions{
			Variant:      opts.Variant,
			Platform:     b.target,
			BuildDir:     opts.BuildDir,
			Std:          opts.Config.Toolchain.Std,
			CStd:         opts.Config.Toolchain.CStd,
			CXXStd:       opts.Config.Toolchain.CXXStd,
			Optimization: opts.Config.ActiveVariant.Optimization,
			Verbosity:    opts.Verbosity,
			ForceRebuild: opts.ForceRebuild,
		}

		result, err := depBuilder.BuildDep(ctx, dep, sourcePath, buildOpts, results)
		if err != nil {
			return nil, fmt.Errorf("failed to build dependency %q: %w", depName, err)
		}

		results[depName] = result
		totalFiles += result.SourceCount
	}

	depDuration := time.Since(depStart)
	if opts.Verbosity >= VerbosityNormal {
		fmt.Printf("Dependencies built (%d files, %s)\n\n", totalFiles, depDuration.String())
	}

	return results, nil
}
