package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/loov/clue/internal/buildpath"
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

// ResolvedDepConfig is the source-level build configuration used for a dependency.
type ResolvedDepConfig struct {
	Sources  []string
	Includes []string
	Defines  []string
	Depends  []string
	Library  string
	Commands [][]string
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
		Depends: cfg.Depends, Library: cfg.Library, Commands: cfg.Commands, Type: cfg.Type,
	}, nil
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
	includePath := db.determineIncludePath(dep, sourcePath, cfg.Includes)
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
	objectNames := buildpath.ObjectNames(cfg.Sources)
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

// depConfig holds the resolved build configuration for a dependency
type depConfig struct {
	Sources       []string
	Includes      []string
	Defines       []string
	Depends       []string
	Library       string
	Commands      [][]string
	CompilerFlags []string
	LinkerFlags   []string
	Type          string
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
		var includes, compilerFlags, linkerFlags []string
		defines := append([]string(nil), inlineConfig.Defines...)
		for _, inc := range inlineConfig.Includes {
			includes = append(includes, filepath.Join(sourcePath, inc))
		}

		// Add include paths from depended-on dependencies
		if len(inlineConfig.Depends) > 0 && builtDeps != nil {
			seen := make(map[string]bool)
			var addUsage func(string)
			addUsage = func(name string) {
				if seen[name] {
					return
				}
				seen[name] = true
				builtDep, exists := builtDeps[name]
				if !exists {
					return
				}
				if builtDep.IncludePath != "" {
					includes = append(includes, builtDep.IncludePath)
				}
				includes = append(includes, builtDep.Usage.Includes...)
				defines = append(defines, builtDep.Usage.Defines...)
				compilerFlags = append(compilerFlags, builtDep.Usage.CompilerFlags...)
				linkerFlags = append(linkerFlags, builtDep.Usage.LinkerFlags...)
				for _, child := range builtDep.Depends {
					addUsage(child)
				}
			}
			for _, depName := range inlineConfig.Depends {
				addUsage(depName)
			}
		}

		return &depConfig{
			Sources:       sources,
			Includes:      includes,
			Defines:       defines,
			Depends:       inlineConfig.Depends,
			Library:       inlineConfig.Library,
			Commands:      inlineConfig.Commands,
			CompilerFlags: compilerFlags,
			LinkerFlags:   linkerFlags,
			Type:          targetType,
		}, nil
	}

	// Try loading clue.cue from source path
	clueFile := filepath.Join(sourcePath, "clue.cue")
	if _, err := os.Stat(clueFile); err != nil {
		return nil, fmt.Errorf("no build configuration for dependency %q: no inline config and no clue.cue found", dep.Name())
	}

	// Load clue.cue
	cfg, err := db.loadClueConfig(clueFile, sourcePath, dep.Name(), dep.BuildTarget())
	if err != nil {
		return nil, fmt.Errorf("failed to load clue.cue: %w", err)
	}

	return cfg, nil
}

// loadClueConfig loads build configuration from a clue.cue file
func (db *DepBuilder) loadClueConfig(clueFile, sourcePath, dependencyName, configuredTarget string) (*depConfig, error) {
	data, err := os.ReadFile(clueFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read clue.cue: %w", err)
	}

	ctx := cuecontext.New()
	val := ctx.CompileBytes(data, cue.Filename(clueFile))
	if err := val.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse clue.cue: %w", err)
	}

	// Index targets so a dependency never silently builds whichever one happens
	// to be declared first.
	targetsVal := val.LookupPath(cue.ParsePath("targets"))
	if !targetsVal.Exists() {
		return nil, fmt.Errorf("clue.cue has no targets")
	}

	iter, err := targetsVal.Fields()
	if err != nil {
		return nil, fmt.Errorf("failed to iterate targets: %w", err)
	}

	targets := make(map[string]cue.Value)
	var names []string
	for iter.Next() {
		name := iter.Selector().Unquoted()
		names = append(names, name)
		targets[name] = iter.Value()
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("clue.cue has no targets")
	}
	slices.Sort(names)
	targetName := configuredTarget
	if targetName == "" {
		if _, ok := targets[dependencyName]; ok {
			targetName = dependencyName
		} else if len(names) == 1 {
			targetName = names[0]
		} else {
			return nil, fmt.Errorf("clue.cue has multiple targets %v; set dependency target", names)
		}
	}
	targetVal, ok := targets[targetName]
	if !ok {
		return nil, fmt.Errorf("clue.cue target %q not found; available targets: %v", targetName, names)
	}

	var sources, includes, defines, externalDepends []string
	seen := make(map[string]bool)
	var collect func(string) error
	collect = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		target, ok := targets[name]
		if !ok {
			externalDepends = append(externalDepends, name)
			return nil
		}
		sources = append(sources, extractCUEStrings(target, "sources")...)
		for _, include := range extractCUEStrings(target, "includes") {
			includes = append(includes, filepath.Join(sourcePath, include))
		}
		defines = append(defines, extractCUEStrings(target, "defines")...)
		if public := target.LookupPath(cue.ParsePath("public")); public.Exists() {
			for _, include := range extractCUEStrings(public, "includes") {
				includes = append(includes, filepath.Join(sourcePath, include))
			}
			defines = append(defines, extractCUEStrings(public, "defines")...)
		}
		for _, dependency := range extractCUEStrings(target, "depends") {
			if err := collect(dependency); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(targetName); err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("clue.cue target %q has no sources", targetName)
	}

	// Expand globs
	sources, err = db.expandSourceGlobs(sources, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand source globs: %w", err)
	}

	targetType := "static_library"
	if typeVal := targetVal.LookupPath(cue.ParsePath("type")); typeVal.Exists() {
		targetType, _ = typeVal.String()
	}
	if targetType != "static_library" && targetType != "shared_library" {
		return nil, fmt.Errorf("dependency target %q must be a static_library or shared_library", targetName)
	}

	return &depConfig{
		Sources: sources, Includes: includes,
		Defines: defines,
		Depends: externalDepends,
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
