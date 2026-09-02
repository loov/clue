// Package build provides compilation, linking, and caching functionality
// for building C and C++ projects.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/buildpath"
	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/profile"
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
	executor         *Executor
	compiler         *Compiler
	linker           *Linker
	cacheManager     *cache.Manager
	parallelCompiler *ParallelCompiler
	toolchain        Toolchain
	target           Platform
	depResults       map[string]*DepBuildResult // Built dependencies
	profiler         *profile.Profiler
}

// NewBuilder creates a new Builder with the specified toolchain and target platform
func NewBuilder(toolchainName string, target Platform, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	// Create toolchain for the target platform
	toolchain, err := NewToolchain(toolchainName, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create toolchain: %w", err)
	}

	// Validate toolchain exists
	if err := ValidateToolchain(toolchain); err != nil {
		return nil, err
	}

	verbose := verbosity == VerbosityVerbose

	executor := NewExecutor(ExecutorConfig{
		Verbose:      verbose,
		StreamOutput: true,
		WorkDir:      "",
	})

	compiler := NewCompiler(executor, toolchain)

	return &Builder{
		executor:         executor,
		compiler:         compiler,
		linker:           NewLinker(executor, toolchain, target),
		parallelCompiler: NewParallelCompiler(toolchain, jobs, keepGoing, verbosity),
		toolchain:        toolchain,
		target:           target,
	}, nil
}

// ObjectDir returns the path for object files: build/variant/target/obj/
func (b *Builder) ObjectDir(buildDir, variant, target string) string {
	return filepath.Join(buildDir, variant, target, "obj")
}

// OutputPath returns the final artifact path
// Executable: build/variant/bin/target
// Static lib: build/variant/lib/libtarget.a
// Shared lib: build/variant/lib/libtarget.so/.dylib (platform-specific)
func (b *Builder) OutputPath(buildDir, variant, target, targetType string) string {
	switch targetType {
	case "executable":
		return filepath.Join(buildDir, variant, "bin", target)
	case "static_library":
		return filepath.Join(buildDir, variant, "lib", "lib"+target+".a")
	case "shared_library":
		ext := SharedLibraryExtension(b.target)
		return filepath.Join(buildDir, variant, "lib", "lib"+target+ext)
	default:
		return filepath.Join(buildDir, variant, "bin", target)
	}
}

// targetToConfig converts config.Target and config.Variant to Config
func (b *Builder) targetToConfig(target config.Target, variant config.Variant) Config {
	cfg := Config{
		Optimize:         variant.Optimization,
		Warnings:         "default", // Default if not specified
		WarningsAsErrors: true,      // Default to true
		Debug:            "none",    // Default if not specified
		RawCompiler:      target.Flags.Compiler,
		RawLinker:        target.Flags.Linker,
		Sanitizers:       append([]string(nil), target.Sanitizers...),
	}
	if target.LTO != nil {
		cfg.LTO = *target.LTO
	}
	if target.PIC != nil {
		cfg.PIC = *target.PIC
	}
	if target.Coverage != nil {
		cfg.Coverage = *target.Coverage
	}

	// Apply target-specific semantic flags (override defaults)
	if target.Optimize != "" {
		cfg.Optimize = target.Optimize
	}
	if target.Warnings != "" {
		cfg.Warnings = target.Warnings
	}
	if target.Debug != "" {
		cfg.Debug = target.Debug
	}
	if target.WarningsAsErrors != nil {
		cfg.WarningsAsErrors = *target.WarningsAsErrors
	}

	// Apply variant debug info (overrides target)
	if variant.DebugInfoSet || variant.DebugInfo {
		if variant.DebugInfo {
			cfg.Debug = "full"
		} else {
			cfg.Debug = "none"
		}
	}
	if variant.Sanitizers != nil {
		cfg.Sanitizers = append([]string(nil), variant.Sanitizers...)
	}
	if variant.LTO != nil {
		cfg.LTO = *variant.LTO
	}
	if variant.PIC != nil {
		cfg.PIC = *variant.PIC
	}
	if variant.Coverage != nil {
		cfg.Coverage = *variant.Coverage
	}

	// Merge variant raw flags
	cfg.RawCompiler = append(cfg.RawCompiler, variant.Flags.Compiler...)
	cfg.RawLinker = append(cfg.RawLinker, variant.Flags.Linker...)

	return cfg
}

// BuildTarget builds a single target
func (b *Builder) BuildTarget(ctx context.Context, opts Options, target config.Target, progress *Progress) (*TargetResult, error) {
	start := time.Now()

	// Calculate paths
	objDir := b.ObjectDir(opts.BuildDir, opts.Variant, target.Name)
	outputPath := b.OutputPath(opts.BuildDir, opts.Variant, target.Name, target.Type)

	// Create directories
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Build configuration from target and variant
	buildCfg := b.targetToConfig(target, opts.Config.ActiveVariant)

	// Collect include paths from dependencies
	includes := append([]string{}, target.Includes...)
	for _, dep := range target.Depends {
		if depResult, isExternalDep := b.depResults[dep]; isExternalDep {
			includes = append(includes, depResult.IncludePath)
		}
	}

	// Check for C++20 modules
	moduleSources, err := DetectModuleSources(target.Sources)
	if err != nil {
		return nil, fmt.Errorf("module detection failed: %w", err)
	}

	// Module compilation setup
	var moduleDeps []ModuleDependency
	var orderedModules []string
	bmiDir := filepath.Join(opts.BuildDir, opts.Variant, "modules")

	if len(moduleSources) > 0 {
		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Detected %d module source(s), scanning dependencies...\n", len(moduleSources))
		}

		// Check clang-scan-deps availability
		if err := CheckScanDepsAvailable(); err != nil {
			return nil, err
		}

		// Scan module dependencies
		moduleDeps, err = ScanModuleDeps(moduleSources, opts.Config.Toolchain.Std, includes)
		if err != nil {
			return nil, fmt.Errorf("module dependency scan failed: %w", err)
		}

		// Order module sources
		orderedModules, err = OrderModuleCompilation(moduleDeps)
		if err != nil {
			return nil, err
		}

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Module compilation order: %v\n", orderedModules)
		}

		// Create BMI directory
		if err := os.MkdirAll(bmiDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create BMI directory: %w", err)
		}
	}

	// Determine compilation order: use ordered modules if available, otherwise original sources
	sourcesToCompile := target.Sources
	if len(orderedModules) > 0 {
		// Build set of module sources for quick lookup
		moduleSet := make(map[string]bool)
		for _, src := range orderedModules {
			moduleSet[src] = true
		}

		// Reorder: module interfaces first (in dependency order), then non-module sources
		// Module interfaces must be compiled before any file that imports them
		var reordered []string
		reordered = append(reordered, orderedModules...)
		for _, src := range target.Sources {
			if !moduleSet[src] {
				reordered = append(reordered, src)
			}
		}
		sourcesToCompile = reordered

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Compilation order: %v\n", sourcesToCompile)
		}
	}
	moduleInfo := make(map[string]ModuleDependency, len(moduleDeps))
	moduleOutputs := make(map[string]string, len(moduleDeps))
	for _, dependency := range moduleDeps {
		moduleInfo[dependency.Source] = dependency
		if dependency.Provides != "" {
			moduleOutputs[dependency.Provides] = filepath.Join(bmiDir, dependency.Provides+".pcm")
		}
	}

	// Collect compile options for all sources that need rebuilding
	var toCompile []CompileOptions
	var preExistingObjects []string
	type sourceCacheInputs struct {
		flags        []string
		compilerPath string
	}
	cacheInputs := make(map[string]sourceCacheInputs, len(target.Sources))

	objectNames := buildpath.ObjectNames(target.Sources)
	defines := append(append([]string(nil), target.Defines...), opts.Config.ActiveVariant.Defines...)

	for _, source := range sourcesToCompile {
		objPath := filepath.Join(objDir, objectNames[source])
		compileOpts := CompileOptions{
			Source:     source,
			Output:     objPath,
			Includes:   includes,
			Defines:    defines,
			Flags:      buildCfg,
			Std:        opts.Config.Toolchain.Std,
			TargetType: target.Type,
		}
		if module, ok := moduleInfo[source]; ok {
			compileOpts.ModuleOutput = moduleOutputs[module.Provides]
			for _, required := range module.Requires {
				if pcm, ok := moduleOutputs[required]; ok {
					if compileOpts.ModuleFiles == nil {
						compileOpts.ModuleFiles = make(map[string]string)
					}
					compileOpts.ModuleFiles[required] = pcm
				}
			}
		}
		compilerPath, err := exec.LookPath(b.compiler.compilerCmd(source))
		if err != nil {
			compilerPath = b.compiler.compilerCmd(source)
		}
		inputs := b.compiler.cacheInputs(compileOpts)
		cacheInputs[source] = sourceCacheInputs{flags: inputs, compilerPath: compilerPath}

		needsRebuild, reason, changedFile := b.cacheManager.NeedsRebuild(
			source, objPath, inputs, includes, compilerPath, opts.ForceRebuild,
		)

		if !needsRebuild {
			progress.Skip(target.Name, source, reason)
			preExistingObjects = append(preExistingObjects, objPath)
			continue
		}

		if opts.Verbosity == VerbosityVerbose && changedFile != "" {
			fmt.Printf("  Will compile: %s (reason: %s, changed: %s)\n", source, reason, changedFile)
		}

		toCompile = append(toCompile, compileOpts)
	}
	storeResult := func(result ParallelResult) {
		inputs := cacheInputs[result.Source]
		err := b.cacheManager.StoreResult(result.Source, result.Object, result.DepFile, inputs.flags, includes, inputs.compilerPath)
		if err != nil && opts.Verbosity == VerbosityVerbose {
			fmt.Printf("  Warning: failed to cache result: %v\n", err)
		}
	}

	// Compilation - handle modules specially to ensure correct order
	var compiledObjects []string
	var compileErr error
	if len(toCompile) > 0 {
		// Check if we have modules that need sequential compilation
		if len(orderedModules) > 0 {
			// Build maps for module compilation
			moduleSet := make(map[string]bool)
			for _, dep := range moduleDeps {
				moduleSet[dep.Source] = true
			}

			var moduleCompile []CompileOptions
			var otherCompile []CompileOptions
			for _, opt := range toCompile {
				if moduleSet[opt.Source] {
					moduleCompile = append(moduleCompile, opt)
				} else {
					otherCompile = append(otherCompile, opt)
				}
			}

			// Compile modules SEQUENTIALLY in dependency order
			// This ensures each module interface is built before files that import it
			for _, opt := range moduleCompile {
				results, err := b.parallelCompiler.CompileParallel(ctx, []CompileOptions{opt})
				if err != nil {
					compileErr = errors.Join(compileErr, err)
					if !opts.KeepGoing {
						return &TargetResult{
							Name: target.Name, Type: target.Type, Output: outputPath,
							Sources: len(target.Sources), Duration: time.Since(start), Success: false,
						}, err
					}
				}
				for _, r := range results {
					if r.Error == nil {
						compiledObjects = append(compiledObjects, r.Object)
						storeResult(r)
					}
				}
			}

			// Compile non-module sources with all module dependencies
			if len(otherCompile) > 0 {
				results, err := b.parallelCompiler.CompileParallel(ctx, otherCompile)
				if err != nil {
					compileErr = errors.Join(compileErr, err)
					if !opts.KeepGoing {
						return &TargetResult{
							Name: target.Name, Type: target.Type, Output: outputPath,
							Sources: len(target.Sources), Duration: time.Since(start), Success: false,
						}, err
					}
				}
				for _, r := range results {
					if r.Error == nil {
						compiledObjects = append(compiledObjects, r.Object)
						storeResult(r)
					}
				}
			}
		} else {
			// No modules - standard parallel compilation
			results, err := b.parallelCompiler.CompileParallel(ctx, toCompile)
			if err != nil {
				return &TargetResult{
					Name: target.Name, Type: target.Type, Output: outputPath,
					Sources: len(target.Sources), Duration: time.Since(start), Success: false,
				}, err
			}

			// Collect successful compilations and cache results
			for _, r := range results {
				if r.Error == nil {
					compiledObjects = append(compiledObjects, r.Object)
					// Store in cache
					storeResult(r)
				}
			}
		}
	}
	if compileErr != nil {
		return &TargetResult{
			Name: target.Name, Type: target.Type, Output: outputPath,
			Sources: len(target.Sources), Duration: time.Since(start), Success: false,
		}, compileErr
	}

	// Combine pre-existing and newly compiled objects
	objectFiles := append(preExistingObjects, compiledObjects...)

	// Link or archive based on target type
	switch target.Type {
	case "executable":
		progress.Linking(target.Name)

		// Determine if we need C++ linker
		useCPlusPlus := b.linker.needsCPlusPlusLinker(objectFiles)

		// Build library paths and libraries from dependencies
		var libPaths []string
		var libs []string
		hasSharedLibDeps := false

		for _, dep := range target.Depends {
			// Check if it's an external dependency
			if depResult, isExternalDep := b.depResults[dep]; isExternalDep {
				// Add external dependency library path and library
				libDir := filepath.Dir(depResult.LibPath)
				libPaths = append(libPaths, libDir)
				libs = append(libs, depResult.Name)
			} else if depTarget, isTargetDep := opts.Config.Targets[dep]; isTargetDep {
				// Check if it's a target dependency
				if depTarget.Type == "static_library" || depTarget.Type == "shared_library" {
					// Add library search path
					depLibPath := filepath.Join(opts.BuildDir, opts.Variant, "lib")
					libPaths = append(libPaths, depLibPath)
					// Add library name (without lib prefix and extension)
					libs = append(libs, dep)
					if depTarget.Type == "shared_library" {
						hasSharedLibDeps = true
					}
				}
			} else {
				// Unknown dependency
				return nil, fmt.Errorf("unknown dependency or target: %q", dep)
			}
		}

		// Add rpath for shared library dependencies
		if hasSharedLibDeps {
			switch b.target.OS {
			case "darwin":
				// Look for libs relative to executable
				buildCfg.RawLinker = append(buildCfg.RawLinker, "-Wl,-rpath,@executable_path/../lib")
			case "linux":
				// $ORIGIN is Linux equivalent of @executable_path
				buildCfg.RawLinker = append(buildCfg.RawLinker, "-Wl,-rpath,$ORIGIN/../lib")
			}
		}

		linkOpts := LinkOptions{
			Objects:      objectFiles,
			Output:       outputPath,
			SysLibs:      target.SysLibs,
			LibPaths:     libPaths,
			Libs:         libs,
			Flags:        buildCfg,
			UseCPlusPlus: useCPlusPlus,
		}

		_, err := b.linker.LinkExecutable(ctx, linkOpts)
		if err != nil {
			return &TargetResult{
				Name:     target.Name,
				Type:     target.Type,
				Output:   outputPath,
				Sources:  len(target.Sources),
				Duration: time.Since(start),
				Success:  false,
			}, err
		}

	case "static_library":
		progress.Archiving(target.Name)

		archiveOpts := ArchiveOptions{
			Objects: objectFiles,
			Output:  outputPath,
		}

		_, err := b.linker.CreateStaticLibrary(ctx, archiveOpts)
		if err != nil {
			return &TargetResult{
				Name:     target.Name,
				Type:     target.Type,
				Output:   outputPath,
				Sources:  len(target.Sources),
				Duration: time.Since(start),
				Success:  false,
			}, err
		}

	case "shared_library":
		progress.Linking(target.Name)

		// Determine if we need C++ linker
		useCPlusPlus := b.linker.needsCPlusPlusLinker(objectFiles)

		// Build library paths and libraries from dependencies
		var libPaths []string
		var libs []string

		for _, dep := range target.Depends {
			// Check if it's an external dependency
			if depResult, isExternalDep := b.depResults[dep]; isExternalDep {
				libDir := filepath.Dir(depResult.LibPath)
				libPaths = append(libPaths, libDir)
				libs = append(libs, depResult.Name)
			} else if depTarget, isTargetDep := opts.Config.Targets[dep]; isTargetDep {
				if depTarget.Type == "static_library" || depTarget.Type == "shared_library" {
					depLibPath := filepath.Join(opts.BuildDir, opts.Variant, "lib")
					libPaths = append(libPaths, depLibPath)
					libs = append(libs, dep)
				}
			} else {
				return nil, fmt.Errorf("unknown dependency or target: %q", dep)
			}
		}

		sharedOpts := SharedLibraryOptions{
			Objects:          objectFiles,
			Output:           outputPath,
			SysLibs:          target.SysLibs,
			LibPaths:         libPaths,
			Libs:             libs,
			Flags:            buildCfg,
			UseCPlusPlus:     useCPlusPlus,
			SymbolVisibility: "default", // Could be configurable via target config later
		}

		_, err := b.linker.LinkSharedLibrary(ctx, sharedOpts)
		if err != nil {
			return &TargetResult{
				Name:     target.Name,
				Type:     target.Type,
				Output:   outputPath,
				Sources:  len(target.Sources),
				Duration: time.Since(start),
				Success:  false,
			}, err
		}

	default:
		return nil, fmt.Errorf("unsupported target type: %s", target.Type)
	}

	duration := time.Since(start)

	// Get cache stats from progress
	_, cachedCount := progress.Stats()
	progress.Complete(outputPath, len(target.Sources), cachedCount, duration)

	return &TargetResult{
		Name:     target.Name,
		Type:     target.Type,
		Output:   outputPath,
		Sources:  len(target.Sources),
		Duration: duration,
		Success:  true,
	}, nil
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
	b.cacheManager, err = cache.NewManager(opts.BuildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Build dependencies first (unless skipped)
	if !opts.SkipDeps {
		b.depResults, err = b.buildDependencies(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to build dependencies: %w", err)
		}
	} else {
		b.depResults = make(map[string]*DepBuildResult)
	}

	// Get build order from config
	buildOrder, err := config.GetBuildOrder(opts.Config)
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
	progress := NewProgress(totalSources, opts.Verbosity)

	// Build each target in order
	var results []TargetResult
	for _, targetName := range buildOrder {
		if selected != nil && !selected[targetName] {
			continue
		}

		target := opts.Config.Targets[targetName]

		// Build the target
		result, err := b.BuildTarget(ctx, opts, target, progress)
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

// buildDependencies builds all external dependencies before the main targets
func (b *Builder) buildDependencies(ctx context.Context, opts Options) (map[string]*DepBuildResult, error) {
	// Check if there are any dependencies
	if len(opts.Config.Dependencies) == 0 {
		return make(map[string]*DepBuildResult), nil
	}

	if opts.Verbosity >= VerbosityNormal {
		fmt.Println("Building dependencies...")
	}

	// Create dependency manager
	mgr, err := deps.NewManager(
		".", // Current directory as project root
		opts.Config.Dependencies,
		deps.ManagerOptions{
			Verbose: opts.Verbosity == VerbosityVerbose,
			CIMode:  false,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependency manager: %w", err)
	}

	// Ensure dependencies are fetched
	if err := mgr.FetchAll(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch dependencies: %w", err)
	}

	// Get build order using resolver (respects inter-dependency order)
	resolver := deps.NewResolver(opts.Config.Dependencies)
	buildOrder, err := resolver.BuildOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependency build order: %w", err)
	}

	// Create dependency builder
	depBuilder := NewDepBuilder(b.compiler, b.linker, b.toolchain, opts.Verbosity)

	// Build each dependency
	results := make(map[string]*DepBuildResult)
	totalFiles := 0
	depStart := time.Now()

	for _, depName := range buildOrder {
		dep := opts.Config.Dependencies[depName]
		if dep == nil {
			return nil, fmt.Errorf("dependency %q not found", depName)
		}

		// Get source path from cache
		sourcePath := dep.CachePath(".")

		// Build dependency
		buildOpts := DepBuildOptions{
			Variant:   opts.Variant,
			Platform:  b.target,
			BuildDir:  opts.BuildDir,
			Verbosity: opts.Verbosity,
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
