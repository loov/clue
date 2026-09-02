package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/loov/clue/internal/buildpath"
	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/deps"
)

// DepBuildOptions holds options for building a dependency
type DepBuildOptions struct {
	Variant      string    // Build variant (e.g., "debug", "release")
	Platform     Platform  // Target platform
	BuildDir     string    // Build output root (default: ".build")
	Std          string    // Project language standard
	Optimization string    // Active variant optimization
	Verbosity    Verbosity // Verbosity level (quiet/normal/verbose)
	ForceRebuild bool      // Force dependency sources to rebuild
}

// DepBuildResult holds the result of building a dependency
type DepBuildResult struct {
	Name        string        // Dependency name
	Type        string        // "static_library" or "shared_library"
	LibPath     string        // Path to built library
	IncludePath string        // Path to include headers
	SourceCount int           // Number of source files compiled
	Duration    time.Duration // Time taken to build
}

// ResolvedDepConfig is the source-level build configuration used for a dependency.
type ResolvedDepConfig struct {
	Sources  []string
	Includes []string
	Defines  []string
	Depends  []string
	Type     string
}

// ResolveDepConfig resolves either an inline dependency build or its clue.cue file.
func ResolveDepConfig(dep deps.Dependency, sourcePath string) (ResolvedDepConfig, error) {
	cfg, err := (&DepBuilder{}).determineConfig(dep, sourcePath, nil)
	if err != nil {
		return ResolvedDepConfig{}, err
	}
	return ResolvedDepConfig{
		Sources: cfg.Sources, Includes: cfg.Includes, Defines: cfg.Defines,
		Depends: cfg.Depends, Type: cfg.Type,
	}, nil
}

// DepBuilder builds individual dependencies
type DepBuilder struct {
	compiler  *Compiler
	linker    *Linker
	toolchain Toolchain
	verbosity Verbosity
	cache     *cache.Manager
}

// NewDepBuilder creates a new dependency builder
func NewDepBuilder(compiler *Compiler, linker *Linker, toolchain Toolchain, verbosity Verbosity) *DepBuilder {
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
	includePath := db.determineIncludePath(dep, sourcePath, cfg.Includes)
	compilationIncludes := append(cfg.Includes, includePath)

	// Compile each source file to object file
	var objectFiles []string
	objectNames := buildpath.ObjectNames(cfg.Sources)
	optimization := opts.Optimization
	if optimization == "" {
		optimization = "none"
	}
	for _, src := range cfg.Sources {
		absPath := filepath.Join(sourcePath, src)
		objPath := filepath.Join(objDir, objectNames[src])

		// Compile source
		compileOpts := CompileOptions{
			Source:   absPath,
			Output:   objPath,
			Includes: compilationIncludes,
			Defines:  cfg.Defines,
			Flags: Config{
				Optimize:         optimization,
				Warnings:         "default",
				WarningsAsErrors: false, // Don't fail dependency builds on warnings
				Debug:            "none",
				RawCompiler:      []string{},
			},
			Std:        opts.Std,
			TargetType: cfg.Type,
		}
		compilerPath, err := exec.LookPath(db.compiler.compilerCmd(absPath))
		if err != nil {
			compilerPath = db.compiler.compilerCmd(absPath)
		}
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

	libName := StaticLibraryName(dep.Name(), opts.Platform)
	if cfg.Type == "shared_library" {
		libName = SharedLibraryName(dep.Name(), opts.Platform)
	}
	libPath := filepath.Join(libDir, libName)
	dependencyArtifacts := make([]string, 0, len(cfg.Depends))
	for _, name := range cfg.Depends {
		if result, ok := builtDeps[name]; ok {
			dependencyArtifacts = append(dependencyArtifacts, result.LibPath)
		}
	}

	if cfg.Type == "shared_library" {
		var libPaths, libs []string
		for _, name := range cfg.Depends {
			if result, ok := builtDeps[name]; ok {
				libPaths = append(libPaths, filepath.Dir(result.LibPath))
				libs = append(libs, result.Name)
			}
		}
		linkOpts := SharedLibraryOptions{
			Objects: objectFiles, Output: libPath, LibPaths: libPaths, Libs: libs,
			Flags: Config{Optimize: optimization, Warnings: "default"},
		}
		fingerprint, fingerprintErr := linkFingerprint(db.toolchain.CXX(), linkOpts, append(objectFiles, dependencyArtifacts...))
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
		fingerprint, fingerprintErr := linkFingerprint(db.toolchain.AR(), archiveOpts, objectFiles)
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
		SourceCount: len(cfg.Sources),
		Duration:    time.Since(start),
	}, nil
}

// depConfig holds the resolved build configuration for a dependency
type depConfig struct {
	Sources  []string
	Includes []string
	Defines  []string
	Depends  []string
	Type     string
}

// determineConfig determines sources, includes, and defines for a dependency
func (db *DepBuilder) determineConfig(dep deps.Dependency, sourcePath string, builtDeps map[string]*DepBuildResult) (*depConfig, error) {
	// Check for inline config first
	inlineConfig := dep.InlineBuild()

	// If inline config exists, use it
	if inlineConfig != nil {
		targetType := inlineConfig.Type
		if targetType == "" {
			targetType = "static_library"
		}
		sources, err := db.expandSourceGlobs(inlineConfig.Sources, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand source globs: %w", err)
		}

		// Resolve include paths relative to source path
		var includes []string
		for _, inc := range inlineConfig.Includes {
			includes = append(includes, filepath.Join(sourcePath, inc))
		}

		// Add include paths from depended-on dependencies
		if len(inlineConfig.Depends) > 0 && builtDeps != nil {
			for _, depName := range inlineConfig.Depends {
				if builtDep, exists := builtDeps[depName]; exists {
					includes = append(includes, builtDep.IncludePath)
				}
			}
		}

		return &depConfig{
			Sources:  sources,
			Includes: includes,
			Defines:  inlineConfig.Defines,
			Depends:  inlineConfig.Depends,
			Type:     targetType,
		}, nil
	}

	// Try loading clue.cue from source path
	clueFile := filepath.Join(sourcePath, "clue.cue")
	if _, err := os.Stat(clueFile); err != nil {
		return nil, fmt.Errorf("no build configuration for dependency %q: no inline config and no clue.cue found", dep.Name())
	}

	// Load clue.cue
	cfg, err := db.loadClueConfig(clueFile, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load clue.cue: %w", err)
	}

	return cfg, nil
}

// loadClueConfig loads build configuration from a clue.cue file
func (db *DepBuilder) loadClueConfig(clueFile, sourcePath string) (*depConfig, error) {
	data, err := os.ReadFile(clueFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read clue.cue: %w", err)
	}

	ctx := cuecontext.New()
	val := ctx.CompileBytes(data, cue.Filename(clueFile))
	if err := val.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse clue.cue: %w", err)
	}

	// Extract first target's sources and includes
	targetsVal := val.LookupPath(cue.ParsePath("targets"))
	if !targetsVal.Exists() {
		return nil, fmt.Errorf("clue.cue has no targets")
	}

	iter, err := targetsVal.Fields()
	if err != nil {
		return nil, fmt.Errorf("failed to iterate targets: %w", err)
	}

	if !iter.Next() {
		return nil, fmt.Errorf("clue.cue has no targets")
	}

	targetVal := iter.Value()

	// Extract sources
	var sources []string
	sourcesVal := targetVal.LookupPath(cue.ParsePath("sources"))
	if sourcesVal.Exists() {
		sourceIter, _ := sourcesVal.List()
		for sourceIter.Next() {
			if s, err := sourceIter.Value().String(); err == nil {
				sources = append(sources, s)
			}
		}
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("clue.cue target has no sources")
	}

	// Expand globs
	sources, err = db.expandSourceGlobs(sources, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand source globs: %w", err)
	}

	// Extract includes
	var includes []string
	includesVal := targetVal.LookupPath(cue.ParsePath("includes"))
	if includesVal.Exists() {
		includeIter, _ := includesVal.List()
		for includeIter.Next() {
			if s, err := includeIter.Value().String(); err == nil {
				// Resolve relative to source path
				includes = append(includes, filepath.Join(sourcePath, s))
			}
		}
	}

	targetType := "static_library"
	if typeVal := targetVal.LookupPath(cue.ParsePath("type")); typeVal.Exists() {
		targetType, _ = typeVal.String()
	}

	return &depConfig{
		Sources: sources, Includes: includes,
		Defines: extractCUEStrings(targetVal, "defines"),
		Depends: extractCUEStrings(targetVal, "depends"),
		Type:    targetType,
	}, nil
}

func extractCUEStrings(value cue.Value, field string) []string {
	var result []string
	list, err := value.LookupPath(cue.ParsePath(field)).List()
	if err != nil {
		return nil
	}
	for list.Next() {
		if item, err := list.Value().String(); err == nil {
			result = append(result, item)
		}
	}
	return result
}

// expandSourceGlobs expands glob patterns in source list
func (db *DepBuilder) expandSourceGlobs(patterns []string, sourcePath string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		// If pattern contains glob characters, expand it
		if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
			matches, err := filepath.Glob(filepath.Join(sourcePath, pattern))
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}

			for _, match := range matches {
				// Convert back to relative path
				rel, err := filepath.Rel(sourcePath, match)
				if err != nil {
					rel = match
				}
				if !seen[rel] {
					result = append(result, rel)
					seen[rel] = true
				}
			}
		} else {
			// No glob, use as-is
			if !seen[pattern] {
				result = append(result, pattern)
				seen[pattern] = true
			}
		}
	}

	return result, nil
}

// determineIncludePath determines the include path for a dependency
func (db *DepBuilder) determineIncludePath(dep deps.Dependency, sourcePath string, _ []string) string {
	// If inline config specifies headers, use parent directory of sourcePath
	inlineConfig := dep.InlineBuild()

	if inlineConfig != nil && len(inlineConfig.Headers) > 0 {
		return filepath.Dir(sourcePath)
	}

	if inlineConfig != nil && len(inlineConfig.Includes) > 0 {
		return filepath.Join(sourcePath, inlineConfig.Includes[0])
	}

	// Check if sourcePath/include exists
	includeDir := filepath.Join(sourcePath, "include")
	if _, err := os.Stat(includeDir); err == nil {
		return includeDir
	}

	// Default to source path root
	return sourcePath
}
