// Package build executes C and C++ builds and related project operations.
package build

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	depBuilds           *dependencyBuilds // dependencies, built alongside the targets
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

// Build creates the builders required by the selected targets and builds them.
// A selection containing only host targets and tasks needs no cross toolchain.
func Build(ctx context.Context, opts Options, platform toolchain.Platform) (*Result, error) {
	selected, hostTargets := selectTargets(opts.Config, opts.Targets)
	onlyHost := len(hostTargets) > 0
	for name := range selected {
		target := opts.Config.Targets[name]
		if target.Type != "task" || len(externalDependencies(opts.Config, target)) > 0 {
			onlyHost = false
			break
		}
	}
	if onlyHost {
		if _, err := config.ComputeBuildOrder(opts.Config); err != nil {
			return nil, err
		}
		start := time.Now()
		targets, err := buildHostTargets(ctx, opts, hostTargets)
		return &Result{Targets: targets, Duration: time.Since(start), Success: err == nil}, err
	}
	builder, err := NewConfiguredBuilder(opts.Config.Toolchain, platform, opts.Config.Dir, opts.Verbosity, opts.Jobs, opts.KeepGoing)
	if err != nil {
		return nil, err
	}
	return builder.Build(ctx, opts)
}

// Build builds all targets in dependency order
func (b *Builder) Build(ctx context.Context, opts Options) (result *Result, err error) {
	start := time.Now()
	for _, target := range opts.Targets {
		if _, ok := opts.Config.Targets[target]; !ok {
			return nil, fmt.Errorf("target %q not found", target)
		}
	}
	selected, hostTargets := selectTargets(opts.Config, opts.Targets)
	var hostResults []TargetResult
	if len(hostTargets) > 0 {
		hostResults, err = buildHostTargets(ctx, opts, hostTargets)
		if err != nil {
			return &Result{Targets: hostResults, Duration: time.Since(start), Success: false}, err
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

	// Fetch dependencies, then build them alongside the project targets.
	// Without KeepGoing, a failed dependency also stops the targets.
	ctx, stopBuild := context.WithCancel(ctx)
	defer stopBuild()
	b.depBuilds = &dependencyBuilds{results: make(map[string]*depBuildResult), cancel: func() {}}
	if !opts.SkipDeps && len(opts.Config.Dependencies) > 0 {
		order, err := b.fetchDependencies(ctx, opts, "")
		if err != nil {
			return nil, fmt.Errorf("failed to build dependencies: %w", err)
		}
		b.depBuilds = b.startDependencyBuilds(ctx, opts, order, stopBuild)
		defer b.depBuilds.stop() // on early returns
	}
	// Globs inside dependency checkouts can be expanded now that they are fetched.
	if err := config.ExpandTargetGlobs(opts.Config); err != nil {
		return nil, err
	}

	// Get build order from config
	buildOrder, err := config.ComputeBuildOrder(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to determine build order: %w", err)
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
	results = append(hostResults, results...)
	if depErr := b.depBuilds.waitAll(); depErr != nil {
		// The dependency's error explains the targets it stopped.
		err = errors.Join(fmt.Errorf("failed to build dependencies: %w", depErr), err)
	}
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

// selectTargets returns the targets to build, those named and the targets
// they depend on, or nil for all of them; without names, every target but
// tasks, which clue run builds for, and optional targets. The host targets of
// a cross build are returned apart, as the host configuration builds them.
func selectTargets(cfg *config.Config, names []string) (selected map[string]bool, host []string) {
	if len(names) == 0 {
		for name, target := range cfg.Targets {
			if !target.Optional && target.Type != "task" {
				names = append(names, name)
			}
		}
		if len(names) == len(cfg.Targets) && cfg.Host == nil {
			return nil, nil
		}
		slices.Sort(names)
	}
	selected = make(map[string]bool)
	var include func(string)
	include = func(name string) {
		target := cfg.Targets[name]
		if target.Host && cfg.Host != nil {
			if !slices.Contains(host, name) {
				host = append(host, name)
			}
			return
		}
		if selected[name] {
			return
		}
		selected[name] = true
		for _, dependency := range target.Depends {
			if _, internal := cfg.Targets[dependency]; internal {
				include(dependency)
			}
		}
	}
	for _, name := range names {
		include(name)
	}
	return selected, host
}

// buildHostTargets builds host targets of a cross build, with the targets
// they depend on, for the machine running clue.
func buildHostTargets(ctx context.Context, opts Options, names []string) ([]TargetResult, error) {
	host := opts.Config.Host
	builder, err := NewConfiguredBuilder(host.Toolchain, toolchain.HostPlatform(), ".", opts.Verbosity, opts.Jobs, opts.KeepGoing)
	if err != nil {
		return nil, fmt.Errorf("host targets: %w", err)
	}
	opts.Config, opts.BuildDir, opts.Targets = host, host.BuildDir, names
	result, err := builder.Build(ctx, opts)
	if result == nil {
		return nil, err
	}
	return result.Targets, err
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
			if err := b.depBuilds.wait(ctx, externalDependencies(opts.Config, target)); err != nil {
				errs[index] = fmt.Errorf("target %q not built: %w", name, err)
				return
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
	_, err = b.buildDependencies(ctx, opts, name)
	return errors.Join(err, b.cacheManager.Flush())
}

func (b *Builder) buildDependencies(ctx context.Context, opts Options, only string) (map[string]*depBuildResult, error) {
	// Check if there are any dependencies
	if len(opts.Config.Dependencies) == 0 {
		return make(map[string]*depBuildResult), nil
	}
	buildOrder, err := b.fetchDependencies(ctx, opts, only)
	if err != nil {
		return nil, err
	}
	b.depBuilds = b.startDependencyBuilds(ctx, opts, buildOrder, nil)
	if err := b.depBuilds.waitAll(); err != nil {
		return nil, err
	}
	return b.depBuilds.snapshot(), nil
}

// fetchDependencies fetches the dependencies (or those up to only) and
// returns the order they build in.
func (b *Builder) fetchDependencies(ctx context.Context, opts Options, only string) ([]string, error) {
	if opts.Verbosity >= VerbosityNormal {
		fmt.Println("Building dependencies...")
	}

	options := fetch.Options{
		Verbose: opts.Verbosity == VerbosityVerbose,
		Quiet:   opts.Verbosity == VerbosityQuiet,
	}
	if only == "" {
		if err := FetchDependencies(ctx, opts.Config, options); err != nil {
			return nil, err
		}
	}

	// Build order (respects inter-dependency order)
	buildOrder, err := deps.NewResolver(opts.Config.Dependencies).BuildOrder()
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
		manager, err := fetch.NewManager(".", opts.Config.Dependencies, options)
		if err != nil {
			return nil, fmt.Errorf("failed to create dependency manager: %w", err)
		}
		for _, name := range buildOrder {
			if err := manager.FetchOne(ctx, name); err != nil {
				return nil, fmt.Errorf("failed to fetch dependency %q: %w", name, err)
			}
		}
	}
	return buildOrder, nil
}

// FetchDependencies fetches every dependency, including those that fetched
// dependencies declare in their clue.cue, which it adds to cfg.
func FetchDependencies(ctx context.Context, cfg *config.Config, options fetch.Options) error {
	_, err := fetchAll(ctx, cfg, options)
	return err
}

// TidyLock fetches every dependency and drops the lock entries of
// dependencies that cfg does not declare. Those can be dependencies that only
// the configuration of another target declares.
func TidyLock(ctx context.Context, cfg *config.Config, options fetch.Options) error {
	manager, err := fetchAll(ctx, cfg, options)
	if err != nil {
		return err
	}
	return manager.PruneLock()
}

func fetchAll(ctx context.Context, cfg *config.Config, options fetch.Options) (*fetch.Manager, error) {
	for {
		manager, err := fetch.NewManager(".", cfg.Dependencies, options)
		if err != nil {
			return nil, fmt.Errorf("failed to create dependency manager: %w", err)
		}
		if err := manager.FetchAll(ctx); err != nil {
			return nil, fmt.Errorf("failed to fetch dependencies: %w", err)
		}
		added, err := config.ExpandDependencies(cfg)
		if err != nil {
			return nil, err
		}
		if added == 0 {
			return manager, nil
		}
	}
}

// dependencyBuilds builds dependencies in the background, each once the
// dependencies it builds against are built, so that project targets can
// start as soon as the dependencies they use are ready.
type dependencyBuilds struct {
	done    map[string]chan struct{}
	mu      sync.Mutex
	results map[string]*depBuildResult
	errs    []error
	workers sync.WaitGroup
	cancel  context.CancelFunc

	verbosity Verbosity
	files     *atomic.Int64 // sources compiled, for the summary
	start     time.Time
}

// Without KeepGoing, the first failure calls stopBuild, or stops the other
// dependency builds when it is nil.
func (b *Builder) startDependencyBuilds(ctx context.Context, opts Options, buildOrder []string, stopBuild context.CancelFunc) *dependencyBuilds {
	depBuilder := newDepBuilder(b.compiler, b.linker, b.toolchain, opts.Verbosity)
	depBuilder.cache = b.cacheManager
	depBuilder.parallel = b.parallelCompiler

	ctx, cancel := context.WithCancel(ctx)
	if stopBuild == nil {
		stopBuild = cancel
	}
	fail := func() {
		if !opts.KeepGoing {
			stopBuild()
		}
	}
	builds := &dependencyBuilds{
		done: make(map[string]chan struct{}, len(buildOrder)), results: make(map[string]*depBuildResult),
		errs: make([]error, len(buildOrder)), cancel: cancel,
	}
	for _, depName := range buildOrder {
		builds.done[depName] = make(chan struct{})
	}
	var totalFiles atomic.Int64
	depStart := time.Now()
	for index, depName := range buildOrder {
		builds.workers.Go(func() {
			defer close(builds.done[depName])
			dep := opts.Config.Dependencies[depName]
			if dep == nil {
				builds.errs[index] = fmt.Errorf("dependency %q not found", depName)
				fail()
				return
			}
			for _, other := range deps.DeclaredDepends(dep) {
				wait, ok := builds.done[other]
				if !ok {
					continue
				}
				select {
				case <-wait:
				case <-ctx.Done():
					return
				}
				if !builds.built(other) {
					builds.errs[index] = fmt.Errorf("dependency %q not built: %q: %w", depName, other, errDependencyFailed)
					return
				}
			}
			if ctx.Err() != nil {
				return
			}
			result, err := b.buildDependency(ctx, opts, depBuilder, dep, builds.snapshot())
			if err != nil {
				builds.errs[index] = err
				fail()
				return
			}
			builds.mu.Lock()
			builds.results[depName] = result
			builds.mu.Unlock()
			totalFiles.Add(int64(result.SourceCount))
		})
	}
	builds.verbosity, builds.files, builds.start = opts.Verbosity, &totalFiles, depStart
	return builds
}

// snapshot returns the dependencies built so far.
func (d *dependencyBuilds) snapshot() map[string]*depBuildResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	return maps.Clone(d.results)
}

// err returns the failures, not the dependencies skipped because of them.
func (d *dependencyBuilds) err() error {
	var failures []error
	for _, err := range d.errs {
		if !errors.Is(err, errDependencyFailed) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// built reports whether the named dependency was built.
func (d *dependencyBuilds) built(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.results[name]
	return ok
}

// wait blocks until the named dependencies are built.
func (d *dependencyBuilds) wait(ctx context.Context, names []string) error {
	for _, name := range names {
		if done, ok := d.done[name]; ok {
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
			if !d.built(name) {
				return fmt.Errorf("dependency %q: %w", name, errDependencyFailed)
			}
		}
	}
	return nil
}

// stop cancels the dependency builds that have not finished and waits for them.
func (d *dependencyBuilds) stop() {
	d.cancel()
	d.workers.Wait()
}

// waitAll blocks until every dependency build has finished.
func (d *dependencyBuilds) waitAll() error {
	d.workers.Wait()
	d.cancel()
	if err := d.err(); err != nil {
		return err
	}
	if d.verbosity >= VerbosityNormal && len(d.done) > 0 {
		fmt.Printf("Dependencies built (%d files, %s)\n", d.files.Load(), time.Since(d.start).String())
	}
	return nil
}

// externalDependencies returns the dependencies a target uses, directly or
// through the project targets it depends on.
func externalDependencies(cfg *config.Config, target config.Target) []string {
	var names []string
	seen := make(map[string]bool)
	var visit func(config.Target)
	visit = func(current config.Target) {
		for _, name := range current.Depends {
			if seen[name] {
				continue
			}
			seen[name] = true
			if dependency, ok := cfg.Targets[name]; ok {
				if !dependency.Host {
					visit(dependency)
				}
			} else if _, ok := cfg.Dependencies[name]; ok {
				names = append(names, name)
			}
		}
	}
	visit(target)
	return names
}
