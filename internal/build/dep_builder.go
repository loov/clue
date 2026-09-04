package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// DepBuildOptions holds options for building a dependency
type DepBuildOptions struct {
	Variant      string             // Build variant (e.g., "debug", "release")
	Platform     toolchain.Platform // Target platform
	BuildDir     string             // Build output root (default: ".build")
	Std          string             // Project language standard
	CStd         string             // C language standard
	CXXStd       string             // C++ language standard
	Optimization string             // Active variant optimization
	Verbosity    Verbosity          // Verbosity level (quiet/normal/verbose)
	ForceRebuild bool               // Force dependency sources to rebuild
}

// DepBuildResult holds the result of building a dependency
type DepBuildResult struct {
	Name        string        // Dependency name
	Type        string        // Built, header-only, or prebuilt library type
	LibPath     string        // Path to built library
	IncludePath string        // Path to include headers
	Depends     []string      // Other external dependencies
	Usage       deps.Usage    // Compile and link metadata for consumers
	SourceCount int           // Number of source files compiled
	Duration    time.Duration // Time taken to build
	RequiresCXX bool          // Link consumers with the C++ driver
}

// DepBuilder builds individual dependencies
type DepBuilder struct {
	compiler  *Compiler
	linker    *Linker
	toolchain toolchain.Toolchain
	verbosity Verbosity
	cache     *cache.Manager
}

// NewDepBuilder creates a new dependency builder
func NewDepBuilder(compiler *Compiler, linker *Linker, toolchain toolchain.Toolchain, verbosity Verbosity) *DepBuilder {
	return &DepBuilder{
		compiler:  compiler,
		linker:    linker,
		toolchain: toolchain,
		verbosity: verbosity,
	}
}

// BuildDep builds a single dependency library.
func (db *DepBuilder) BuildDep(ctx context.Context, dep deps.Dependency, sourcePath string, opts DepBuildOptions, builtDeps map[string]*DepBuildResult) (*DepBuildResult, error) {
	start := time.Now()

	// Determine sources, includes, and defines
	cfg, err := db.determineConfig(dep, sourcePath, builtDeps)
	if err != nil {
		return nil, err
	}
	includePath := deps.IncludePath(dep, sourcePath)
	if cfg.Type == "header_only" {
		return &DepBuildResult{
			Name: dep.Name(), Type: cfg.Type, IncludePath: includePath,
			Depends: cfg.Depends, Duration: time.Since(start),
		}, nil
	}
	if cfg.Type == "prebuilt_static" || cfg.Type == "prebuilt_shared" {
		library := filepath.Join(sourcePath, cfg.Library)
		info, err := os.Stat(library)
		if err != nil {
			return nil, fmt.Errorf("prebuilt library %q: %w", library, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("prebuilt library %q is not a regular file", library)
		}
		return &DepBuildResult{
			Name: dep.Name(), Type: cfg.Type, LibPath: library, IncludePath: includePath,
			Depends: cfg.Depends, Duration: time.Since(start),
		}, nil
	}
	if cfg.Type == "external_static" || cfg.Type == "external_shared" {
		executorConfig := ExecutorConfig{StreamOutput: true, WorkDir: sourcePath}
		if db.compiler != nil {
			executorConfig = db.compiler.executor.config
			executorConfig.WorkDir = sourcePath
		}
		executor := NewExecutor(executorConfig)
		for _, command := range cfg.Commands {
			result, err := executor.RunCommand(ctx, command[0], command[1:]...)
			if err != nil {
				if result != nil && result.Stderr != "" {
					return nil, fmt.Errorf("external build command %q failed: %w: %s", command[0], err, strings.TrimSpace(result.Stderr))
				}
				return nil, fmt.Errorf("external build command %q failed: %w", command[0], err)
			}
		}
		library := filepath.Join(sourcePath, cfg.Library)
		info, err := os.Stat(library)
		if err != nil {
			return nil, fmt.Errorf("external build did not produce library %q: %w", library, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("external build output %q is not a regular file", library)
		}
		return &DepBuildResult{
			Name: dep.Name(), Type: cfg.Type, LibPath: library, IncludePath: includePath,
			Depends: cfg.Depends, Duration: time.Since(start),
		}, nil
	}

	// Print progress (collapsed output)
	if opts.Verbosity != VerbosityVerbose {
		fmt.Printf("  Building %s [%d files]\n", dep.Name(), len(cfg.Sources))
	}

	// Create output directories
	objDir := filepath.Join(opts.BuildDir, opts.Variant, "deps", dep.Name(), "obj")
	libDir := filepath.Join(opts.BuildDir, opts.Variant, "deps", dep.Name(), "lib")

	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create library directory: %w", err)
	}

	// Determine include path for compilation
	compilationIncludes := append(cfg.Includes, includePath)

	// Compile each source file to object file
	var objectFiles []string
	requiresCXX := false
	objectNames := plan.ObjectNames(cfg.Sources)
	optimization := opts.Optimization
	if optimization == "" {
		optimization = "none"
	}
	for _, src := range cfg.Sources {
		requiresCXX = requiresCXX || toolchain.IsCXXSource(src)
		absPath := filepath.Join(sourcePath, src)
		objPath := filepath.Join(objDir, objectNames[src])

		// Compile source
		compileOpts := CompileOptions{
			Source:   absPath,
			Output:   objPath,
			Includes: compilationIncludes,
			Defines:  cfg.Defines,
			Flags: toolchain.Config{
				Optimize:         optimization,
				Warnings:         "default",
				WarningsAsErrors: false, // Don't fail dependency builds on warnings
				Debug:            "none",
				RawCompiler:      cfg.CompilerFlags,
			},
			Std: config.Toolchain{
				Std: opts.Std, CStd: opts.CStd, CXXStd: opts.CXXStd,
			}.Standard(absPath),
			TargetType: cfg.Type,
			Platform:   opts.Platform,
		}
		compilerPath := toolIdentityPath(db.toolchain, db.compiler.compilerCmd(absPath))
		cacheInputs := db.compiler.cacheInputs(compileOpts)
		if db.cache != nil {
			needsRebuild, _, _ := db.cache.NeedsRebuild(
				absPath, objPath, cacheInputs, compilationIncludes, compilerPath, opts.ForceRebuild,
			)
			if !needsRebuild {
				objectFiles = append(objectFiles, objPath)
				continue
			}
		}

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("    Compiling %s\n", src)
		}

		result, err := db.compiler.CompileSource(ctx, compileOpts)
		if err != nil {
			// Expand on error
			return nil, fmt.Errorf("failed to compile %s: %w", src, err)
		}

		objectFiles = append(objectFiles, result.Object)
		if db.cache != nil {
			if err := db.cache.StoreResult(absPath, result.Object, result.DepFile, cacheInputs, compilationIncludes, compilerPath); err != nil && opts.Verbosity == VerbosityVerbose {
				fmt.Printf("    Warning: failed to cache %s: %v\n", src, err)
			}
		}
	}

	libName := plan.StaticLibraryName(dep.Name(), opts.Platform)
	if cfg.Type == "shared_library" {
		libName = plan.SharedLibraryName(dep.Name(), opts.Platform)
	}
	libPath := filepath.Join(libDir, libName)
	var dependencyArtifacts []string

	if cfg.Type == "shared_library" {
		var libPaths, libs, linkFiles []string
		seen := make(map[string]bool)
		var addLibrary func(string)
		addLibrary = func(name string) {
			if seen[name] {
				return
			}
			seen[name] = true
			if result, ok := builtDeps[name]; ok {
				if result.LibPath == "" {
					for _, child := range result.Depends {
						addLibrary(child)
					}
					return
				}
				dependencyArtifacts = append(dependencyArtifacts, result.LibPath)
				if result.Type == "prebuilt_static" || result.Type == "prebuilt_shared" ||
					result.Type == "external_static" || result.Type == "external_shared" {
					linkFiles = append(linkFiles, result.LibPath)
				} else {
					libPaths = append(libPaths, filepath.Dir(result.LibPath))
					libs = append(libs, result.Name)
				}
				for _, child := range result.Depends {
					addLibrary(child)
				}
			}
		}
		for _, name := range cfg.Depends {
			addLibrary(name)
			if dependency := builtDeps[name]; dependency != nil {
				requiresCXX = requiresCXX || dependency.RequiresCXX
			}
		}
		linkOpts := SharedLibraryOptions{
			Objects: append(objectFiles, linkFiles...), Output: libPath, LibPaths: libPaths, Libs: libs,
			Flags: toolchain.Config{Optimize: optimization, Warnings: "default", RawLinker: cfg.LinkerFlags}, UseCXX: requiresCXX,
		}
		fingerprint, fingerprintErr := linkFingerprint(db.toolchain, db.linker.linkDriver(requiresCXX), linkOpts, append(objectFiles, dependencyArtifacts...))
		if fingerprintErr != nil {
			return nil, fingerprintErr
		}
		if opts.ForceRebuild || !linkIsCurrent(libPath, fingerprint) {
			_, err = db.linker.LinkSharedLibrary(ctx, linkOpts)
			if err == nil {
				if cacheErr := storeLinkFingerprint(libPath, fingerprint); cacheErr != nil && opts.Verbosity == VerbosityVerbose {
					fmt.Printf("    Warning: failed to cache link result: %v\n", cacheErr)
				}
			}
		}
	} else {
		archiveOpts := ArchiveOptions{Objects: objectFiles, Output: libPath}
		fingerprint, fingerprintErr := linkFingerprint(db.toolchain, db.toolchain.AR(), archiveOpts, objectFiles)
		if fingerprintErr != nil {
			return nil, fingerprintErr
		}
		if opts.ForceRebuild || !linkIsCurrent(libPath, fingerprint) {
			_, err = db.linker.CreateStaticLibrary(ctx, archiveOpts)
			if err == nil {
				if cacheErr := storeLinkFingerprint(libPath, fingerprint); cacheErr != nil && opts.Verbosity == VerbosityVerbose {
					fmt.Printf("    Warning: failed to cache archive result: %v\n", cacheErr)
				}
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", cfg.Type, err)
	}

	return &DepBuildResult{
		Name:        dep.Name(),
		Type:        cfg.Type,
		LibPath:     libPath,
		IncludePath: includePath,
		Depends:     cfg.Depends,
		SourceCount: len(cfg.Sources),
		Duration:    time.Since(start),
		RequiresCXX: requiresCXX,
	}, nil
}

// determineConfig determines sources, includes, and defines for a dependency
func (db *DepBuilder) determineConfig(dep deps.Dependency, sourcePath string, builtDeps map[string]*DepBuildResult) (*deps.BuildConfig, error) {
	cfg, err := deps.ResolveBuildConfig(dep, sourcePath)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var addUsage func(string)
	addUsage = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		builtDep := builtDeps[name]
		if builtDep == nil {
			return
		}
		if builtDep.IncludePath != "" {
			cfg.Includes = append(cfg.Includes, builtDep.IncludePath)
		}
		cfg.Includes = append(cfg.Includes, builtDep.Usage.Includes...)
		cfg.Defines = append(cfg.Defines, builtDep.Usage.Defines...)
		cfg.CompilerFlags = append(cfg.CompilerFlags, builtDep.Usage.CompilerFlags...)
		cfg.LinkerFlags = append(cfg.LinkerFlags, builtDep.Usage.LinkerFlags...)
		for _, child := range builtDep.Depends {
			addUsage(child)
		}
	}
	for _, name := range cfg.Depends {
		addUsage(name)
	}
	return &cfg, nil
}
