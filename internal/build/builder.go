package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/config"
)

// BuildOptions holds options for a build operation
type BuildOptions struct {
	Config       *config.Config
	Variant      string   // "debug" or "release"
	BuildDir     string   // Build output root (default: ".build")
	Verbose      bool     // Show full compiler commands
	Targets      []string // Specific targets to build (empty = all)
	ForceRebuild bool     // Force rebuild of all files
	Jobs         int      // Number of parallel jobs
	KeepGoing    bool     // Continue building despite errors
}

// TargetResult holds the result of building a single target
type TargetResult struct {
	Name     string
	Type     string        // "executable" or "static_library"
	Output   string        // Path to output artifact
	Sources  int           // Number of source files
	Duration time.Duration
	Success  bool
}

// BuildResult holds the overall build result
type BuildResult struct {
	Targets  []TargetResult
	Duration time.Duration
	Success  bool
}

// Builder orchestrates the build process
type Builder struct {
	executor         *Executor
	compiler         *Compiler
	linker           *Linker
	cacheManager     *CacheManager
	parallelCompiler *ParallelCompiler
	toolchain        *Toolchain
	target           Platform
}

// NewBuilder creates a new Builder with the specified toolchain and target platform
func NewBuilder(toolchainName string, target Platform, verbose bool, jobs int, keepGoing bool) (*Builder, error) {
	// Discover toolchain for the target platform
	toolchain, err := DiscoverToolchain(toolchainName, target)
	if err != nil {
		return nil, fmt.Errorf("failed to discover toolchain: %w", err)
	}

	// Validate toolchain exists
	if err := ValidateToolchain(toolchain); err != nil {
		return nil, err
	}

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
		parallelCompiler: NewParallelCompiler(compiler, toolchain, jobs, keepGoing, verbose),
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

// targetToBuildConfig converts config.Target and config.Variant to BuildConfig
func (b *Builder) targetToBuildConfig(target config.Target, variant config.Variant) BuildConfig {
	cfg := BuildConfig{
		Optimize:         variant.Optimization,
		Warnings:         "default", // Default if not specified
		WarningsAsErrors: true,      // Default to true
		Debug:            "none",    // Default if not specified
		RawCompiler:      target.Flags.Compiler,
		RawLinker:        target.Flags.Linker,
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
	if variant.DebugInfo {
		cfg.Debug = "full"
	}

	// Merge variant raw flags
	cfg.RawCompiler = append(cfg.RawCompiler, variant.Flags.Compiler...)
	cfg.RawLinker = append(cfg.RawLinker, variant.Flags.Linker...)

	return cfg
}

// BuildTarget builds a single target
func (b *Builder) BuildTarget(ctx context.Context, opts BuildOptions, target config.Target, progress *Progress) (*TargetResult, error) {
	start := time.Now()

	// Calculate paths
	objDir := b.ObjectDir(opts.BuildDir, opts.Variant, target.Name)
	outputPath := b.OutputPath(opts.BuildDir, opts.Variant, target.Name, target.Type)

	// Create directories
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Build configuration from target and variant
	buildCfg := b.targetToBuildConfig(target, opts.Config.ActiveVariant)

	// Get compiler path for cache key
	compilerPath, err := exec.LookPath(opts.Config.Toolchain.Compiler)
	if err != nil {
		compilerPath = opts.Config.Toolchain.Compiler
	}

	// Collect compile options for all sources that need rebuilding
	var toCompile []CompileOptions
	var preExistingObjects []string

	for _, source := range target.Sources {
		objName := filepath.Base(source) + ".o"
		objPath := filepath.Join(objDir, objName)

		needsRebuild, reason, changedFile := b.cacheManager.NeedsRebuild(
			source, buildCfg, target.Includes, compilerPath, opts.ForceRebuild,
		)

		if !needsRebuild {
			progress.Skip(target.Name, source, reason)
			preExistingObjects = append(preExistingObjects, objPath)
			continue
		}

		if opts.Verbose && changedFile != "" {
			fmt.Printf("  Will compile: %s (reason: %s, changed: %s)\n", source, reason, changedFile)
		}

		toCompile = append(toCompile, CompileOptions{
			Source:   source,
			Output:   objPath,
			Includes: target.Includes,
			Defines:  target.Defines,
			Flags:    buildCfg,
			Std:      opts.Config.Toolchain.Std,
		})
	}

	// Parallel compilation
	var compiledObjects []string
	if len(toCompile) > 0 {
		results, err := b.parallelCompiler.CompileParallel(ctx, toCompile)
		if err != nil && !opts.KeepGoing {
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
				depPath := filepath.Join(objDir, filepath.Base(r.Source)+".d")
				err := b.cacheManager.StoreResult(r.Source, r.Object, depPath, buildCfg, target.Includes, compilerPath)
				if err != nil && opts.Verbose {
					fmt.Printf("  Warning: failed to cache result: %v\n", err)
				}
			}
		}
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
		for _, dep := range target.Depends {
			depTarget := opts.Config.Targets[dep]
			if depTarget.Type == "static_library" {
				// Add library search path
				depLibPath := filepath.Join(opts.BuildDir, opts.Variant, "lib")
				libPaths = append(libPaths, depLibPath)
				// Add library name (without lib prefix and .a suffix)
				libs = append(libs, dep)
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

	default:
		return nil, fmt.Errorf("unsupported target type: %s", target.Type)
	}

	duration := time.Since(start)
	progress.Complete(outputPath, len(target.Sources), duration)

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
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	start := time.Now()

	// Print platform and toolchain information
	fmt.Printf("Building for %s\n", b.target)
	if b.target.IsCrossCompile() {
		fmt.Printf("Cross-compiling using %s\n", b.toolchain)
	}

	// Initialize cache manager
	var err error
	b.cacheManager, err = NewCacheManager(opts.BuildDir, opts.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Get build order from config
	buildOrder, err := config.GetBuildOrder(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to determine build order: %w", err)
	}

	// Count total source files for progress
	totalSources := 0
	for _, targetName := range buildOrder {
		// Skip if specific targets requested and this isn't one
		if len(opts.Targets) > 0 {
			found := false
			for _, t := range opts.Targets {
				if t == targetName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		target := opts.Config.Targets[targetName]
		totalSources += len(target.Sources)
	}

	// Create progress tracker
	progress := NewProgress(totalSources, opts.Verbose)

	// Build each target in order
	var results []TargetResult
	for _, targetName := range buildOrder {
		// Skip if specific targets requested and this isn't one
		if len(opts.Targets) > 0 {
			found := false
			for _, t := range opts.Targets {
				if t == targetName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		target := opts.Config.Targets[targetName]

		// Build the target
		result, err := b.BuildTarget(ctx, opts, target, progress)
		if err != nil {
			// Fail-fast: report error and stop
			progress.Error(targetName, err)
			return &BuildResult{
				Targets:  results,
				Duration: time.Since(start),
				Success:  false,
			}, err
		}

		results = append(results, *result)
	}

	// Print summary
	progress.Summary()

	return &BuildResult{
		Targets:  results,
		Duration: time.Since(start),
		Success:  true,
	}, nil
}
