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
	executor     *Executor
	compiler     *Compiler
	linker       *Linker
	cacheManager *CacheManager
}

// NewBuilder creates a new Builder with the specified toolchain
func NewBuilder(toolchain string, verbose bool) *Builder {
	executor := NewExecutor(ExecutorConfig{
		Verbose:      verbose,
		StreamOutput: true,
		WorkDir:      "",
	})

	return &Builder{
		executor: executor,
		compiler: NewCompiler(executor, toolchain),
		linker:   NewLinker(executor, toolchain),
	}
}

// ObjectDir returns the path for object files: build/variant/target/obj/
func (b *Builder) ObjectDir(buildDir, variant, target string) string {
	return filepath.Join(buildDir, variant, target, "obj")
}

// OutputPath returns the final artifact path
// Executable: build/variant/bin/target
// Static lib: build/variant/lib/libtarget.a
func (b *Builder) OutputPath(buildDir, variant, target, targetType string) string {
	switch targetType {
	case "executable":
		return filepath.Join(buildDir, variant, "bin", target)
	case "static_library":
		return filepath.Join(buildDir, variant, "lib", "lib"+target+".a")
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

	// Compile all source files
	var objectFiles []string
	for _, source := range target.Sources {
		// Determine object file path
		objName := filepath.Base(source) + ".o"
		objPath := filepath.Join(objDir, objName)

		// Determine dependency file path
		depPath := filepath.Join(objDir, filepath.Base(source)+".d")

		// Check if rebuild is needed
		needsRebuild, reason, changedFile := b.cacheManager.NeedsRebuild(
			source,
			buildCfg,
			target.Includes,
			compilerPath,
			opts.ForceRebuild,
		)

		if !needsRebuild {
			// Skip cached file
			progress.Skip(target.Name, source, reason)
			objectFiles = append(objectFiles, objPath)
			continue
		}

		// Report progress
		progress.Compiling(target.Name, source)
		if opts.Verbose && changedFile != "" {
			fmt.Printf("  Reason: %s (%s)\n", reason, changedFile)
		} else if opts.Verbose {
			fmt.Printf("  Reason: %s\n", reason)
		}

		// Build compile options
		compileOpts := CompileOptions{
			Source:   source,
			Output:   objPath,
			Includes: target.Includes,
			Defines:  target.Defines,
			Flags:    buildCfg,
			Std:      opts.Config.Toolchain.Std,
		}

		// Compile source
		_, err := b.compiler.CompileSource(ctx, compileOpts)
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

		// Store result in cache
		err = b.cacheManager.StoreResult(
			source,
			objPath,
			depPath,
			buildCfg,
			target.Includes,
			compilerPath,
		)
		if err != nil && opts.Verbose {
			fmt.Printf("  Warning: failed to cache result: %v\n", err)
		}

		objectFiles = append(objectFiles, objPath)
	}

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
