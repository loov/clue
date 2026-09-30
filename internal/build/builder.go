// Package build executes C and C++ builds and related project operations.
package build

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	modulesMu           sync.Mutex // guards targetModuleOutputs; targets build concurrently
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
func (b *Builder) Build(ctx context.Context, opts Options) (result *Result, err error) {
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
	b.targetModuleOutputs = make(map[string]map[string]string)
	b.cacheManager, err = cache.NewManager(opts.BuildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}
	// Record what compiled, even when a later step fails.
	defer func() { err = errors.Join(err, b.cacheManager.Flush()) }()

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

	results, err := b.buildTargets(ctx, opts, buildOrder, selected, progress)
	if err != nil {
		return &Result{Targets: results, Duration: time.Since(start), Success: false}, err
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

// errDependencyFailed marks targets skipped because a target they depend on failed.
var errDependencyFailed = errors.New("a dependency failed")

// buildTargets builds each selected target as soon as the targets it depends on
// are built; compiles of concurrent targets share the -j limit. Without
// KeepGoing the first failure stops targets that have not started yet.
func (b *Builder) buildTargets(ctx context.Context, opts Options, order []string, selected map[string]bool, progress *progress) ([]TargetResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	position := make(map[string]int, len(order))
	done := make([]chan struct{}, len(order))
	for index, name := range order {
		if selected == nil || selected[name] {
			position[name] = index
			done[index] = make(chan struct{})
		}
	}
	results := make([]*TargetResult, len(order))
	errs := make([]error, len(order))
	var workers sync.WaitGroup
	for index, name := range order {
		if done[index] == nil {
			continue
		}
		workers.Go(func() {
			defer close(done[index])
			target := opts.Config.Targets[name]
			for _, dependency := range target.Depends {
				other, ok := position[dependency]
				if !ok {
					continue
				}
				select {
				case <-done[other]:
				case <-ctx.Done():
				}
				// errs[other] is only safe to read once done[other] is closed.
				if ctx.Err() != nil || errs[other] != nil {
					errs[index] = fmt.Errorf("target %q not built: %q: %w", name, dependency, errDependencyFailed)
					return
				}
			}
			result, err := b.buildTarget(ctx, opts, target, progress)
			if err != nil {
				errs[index] = err
				progress.Error(name, err)
				if !opts.KeepGoing {
					cancel()
				}
				return
			}
			results[index] = result
		})
	}
	workers.Wait()

	var built []TargetResult
	var failures []error
	for index := range order {
		if results[index] != nil {
			built = append(built, *results[index])
		}
		// Report the targets that failed, not the ones skipped because of them.
		if errs[index] != nil && !errors.Is(errs[index], errDependencyFailed) && !errors.Is(errs[index], context.Canceled) {
			failures = append(failures, errs[index])
		}
	}
	if len(failures) == 0 {
		for _, err := range errs {
			if err != nil {
				failures = append(failures, err)
				break
			}
		}
	}
	return built, errors.Join(failures...)
}

// buildDependency resolves a pkg-config dependency or builds a fetched one.
func (b *Builder) buildDependency(ctx context.Context, opts Options, depBuilder *depBuilder, dep deps.Dependency, built map[string]*depBuildResult) (*depBuildResult, error) {
	if pkg, ok := dep.(*deps.PkgConfigDependency); ok {
		usage, err := pkg.ResolveWithRunner(ctx, func(ctx context.Context, name string, args ...string) (string, error) {
			return toolchain.Output(ctx, b.toolchain, ".", name, args...)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependency %q: %w", dep.Name(), err)
		}
		return &depBuildResult{Name: dep.Name(), Type: "pkg_config", Usage: usage}, nil
	}
	buildOpts := depBuildOptions{
		Variant:      opts.Variant,
		Platform:     b.target,
		BuildDir:     opts.BuildDir,
		Std:          opts.Config.Toolchain.Std,
		CStd:         opts.Config.Toolchain.CStd,
		CXXStd:       opts.Config.Toolchain.CXXStd,
		Optimization: opts.Config.ActiveVariant.Optimization,
		Defines:      opts.Config.ActiveVariant.Defines,
		Verbosity:    opts.Verbosity,
		ForceRebuild: opts.ForceRebuild,
	}
	result, err := depBuilder.BuildDep(ctx, dep, dep.CachePath("."), buildOpts, built)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency %q: %w", dep.Name(), err)
	}
	return result, nil
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
	return errors.Join(err, b.cacheManager.Flush())
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
	depBuilder.parallel = b.parallelCompiler

	// Build each dependency once the dependencies it builds against are built.
	results := make(map[string]*depBuildResult)
	var resultsMu sync.Mutex
	var totalFiles atomic.Int64
	depStart := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(map[string]chan struct{}, len(buildOrder))
	for _, depName := range buildOrder {
		done[depName] = make(chan struct{})
	}
	errs := make([]error, len(buildOrder))
	var workers sync.WaitGroup
	for index, depName := range buildOrder {
		workers.Go(func() {
			defer close(done[depName])
			dep := opts.Config.Dependencies[depName]
			if dep == nil {
				errs[index] = fmt.Errorf("dependency %q not found", depName)
				cancel()
				return
			}
			for _, other := range deps.DeclaredDepends(dep) {
				if wait, ok := done[other]; ok {
					select {
					case <-wait:
					case <-ctx.Done():
					}
				}
			}
			if ctx.Err() != nil {
				return
			}
			result, err := b.buildDependency(ctx, opts, depBuilder, dep, func() map[string]*depBuildResult {
				resultsMu.Lock()
				defer resultsMu.Unlock()
				return maps.Clone(results)
			}())
			if err != nil {
				errs[index] = err
				cancel()
				return
			}
			resultsMu.Lock()
			results[depName] = result
			resultsMu.Unlock()
			totalFiles.Add(int64(result.SourceCount))
		})
	}
	workers.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	depDuration := time.Since(depStart)
	if opts.Verbosity >= VerbosityNormal {
		fmt.Printf("Dependencies built (%d files, %s)\n\n", totalFiles.Load(), depDuration.String())
	}

	return results, nil
}
