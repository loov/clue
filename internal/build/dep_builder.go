package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/loov/clue/internal/deps"
)

// DepBuildOptions holds options for building a dependency
type DepBuildOptions struct {
	Variant  string   // Build variant (e.g., "debug", "release")
	Platform Platform // Target platform
	BuildDir string   // Build output root (default: ".build")
	Verbose  bool     // Show full compiler output
}

// DepBuildResult holds the result of building a dependency
type DepBuildResult struct {
	Name        string        // Dependency name
	LibPath     string        // Path to built .a file
	IncludePath string        // Path to include headers
	SourceCount int           // Number of source files compiled
	Duration    time.Duration // Time taken to build
}

// DepBuilder builds individual dependencies
type DepBuilder struct {
	compiler  *Compiler
	linker    *Linker
	toolchain *Toolchain
	verbose   bool
}

// NewDepBuilder creates a new dependency builder
func NewDepBuilder(compiler *Compiler, linker *Linker, toolchain *Toolchain, verbose bool) *DepBuilder {
	return &DepBuilder{
		compiler:  compiler,
		linker:    linker,
		toolchain: toolchain,
		verbose:   verbose,
	}
}

// BuildDep builds a single dependency to a static library
func (db *DepBuilder) BuildDep(ctx context.Context, dep deps.Dependency, sourcePath string, opts DepBuildOptions) (*DepBuildResult, error) {
	start := time.Now()

	// Determine sources and includes
	sources, includes, err := db.determineBuildConfig(dep, sourcePath)
	if err != nil {
		return nil, err
	}

	// Print progress (collapsed output)
	if !opts.Verbose {
		fmt.Printf("  Building %s [%d files]\n", dep.Name(), len(sources))
	}

	// Create output directories
	objDir := filepath.Join(opts.BuildDir, opts.Variant, "deps", dep.Name(), "obj")
	libDir := filepath.Join(opts.BuildDir, opts.Variant, "deps", dep.Name(), "lib")

	if err := os.MkdirAll(objDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create library directory: %w", err)
	}

	// Determine include path for compilation
	includePath := db.determineIncludePath(dep, sourcePath, includes)
	compilationIncludes := append(includes, includePath)

	// Compile each source file to object file
	var objectFiles []string
	for _, src := range sources {
		absPath := filepath.Join(sourcePath, src)
		objName := filepath.Base(src) + ".o"
		objPath := filepath.Join(objDir, objName)

		// Compile source
		compileOpts := CompileOptions{
			Source:   absPath,
			Output:   objPath,
			Includes: compilationIncludes,
			Defines:  []string{},
			Flags: BuildConfig{
				Optimize:         opts.Variant, // Use variant as optimization level
				Warnings:         "default",
				WarningsAsErrors: false, // Don't fail dependency builds on warnings
				Debug:            "none",
				RawCompiler:      []string{},
			},
			Std: "c++20", // Default to C++20 for dependencies
		}

		if opts.Verbose {
			fmt.Printf("    Compiling %s\n", src)
		}

		result, err := db.compiler.CompileSource(ctx, compileOpts)
		if err != nil {
			// Expand on error
			return nil, fmt.Errorf("failed to compile %s: %w", src, err)
		}

		objectFiles = append(objectFiles, result.Object)
	}

	// Create static library
	libName := "lib" + dep.Name() + ".a"
	libPath := filepath.Join(libDir, libName)

	archiveOpts := ArchiveOptions{
		Objects: objectFiles,
		Output:  libPath,
	}

	_, err = db.linker.CreateStaticLibrary(ctx, archiveOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create static library: %w", err)
	}

	return &DepBuildResult{
		Name:        dep.Name(),
		LibPath:     libPath,
		IncludePath: includePath,
		SourceCount: len(sources),
		Duration:    time.Since(start),
	}, nil
}

// determineBuildConfig determines sources and includes for a dependency
func (db *DepBuilder) determineBuildConfig(dep deps.Dependency, sourcePath string) ([]string, []string, error) {
	// Check for inline config first
	var inlineConfig *deps.InlineConfig

	switch d := dep.(type) {
	case *deps.GitDependency:
		inlineConfig = d.BuildConfig
	case *deps.TarballDependency:
		inlineConfig = d.BuildConfig
	case *deps.VendoredDependency:
		inlineConfig = d.BuildConfig
	}

	// If inline config exists, use it
	if inlineConfig != nil {
		sources, err := db.expandSourceGlobs(inlineConfig.Sources, sourcePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to expand source globs: %w", err)
		}

		// Resolve include paths relative to source path
		var includes []string
		for _, inc := range inlineConfig.Includes {
			includes = append(includes, filepath.Join(sourcePath, inc))
		}

		return sources, includes, nil
	}

	// Try loading clue.cue from source path
	clueFile := filepath.Join(sourcePath, "clue.cue")
	if _, err := os.Stat(clueFile); err != nil {
		return nil, nil, fmt.Errorf("no build configuration for dependency %q: no inline config and no clue.cue found", dep.Name())
	}

	// Load clue.cue
	sources, includes, err := db.loadClueConfig(clueFile, sourcePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load clue.cue: %w", err)
	}

	return sources, includes, nil
}

// loadClueConfig loads build configuration from a clue.cue file
func (db *DepBuilder) loadClueConfig(clueFile, sourcePath string) ([]string, []string, error) {
	data, err := os.ReadFile(clueFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read clue.cue: %w", err)
	}

	ctx := cuecontext.New()
	val := ctx.CompileBytes(data, cue.Filename(clueFile))
	if err := val.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to parse clue.cue: %w", err)
	}

	// Extract first target's sources and includes
	targetsVal := val.LookupPath(cue.ParsePath("targets"))
	if !targetsVal.Exists() {
		return nil, nil, fmt.Errorf("clue.cue has no targets")
	}

	iter, err := targetsVal.Fields()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to iterate targets: %w", err)
	}

	if !iter.Next() {
		return nil, nil, fmt.Errorf("clue.cue has no targets")
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
		return nil, nil, fmt.Errorf("clue.cue target has no sources")
	}

	// Expand globs
	sources, err = db.expandSourceGlobs(sources, sourcePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to expand source globs: %w", err)
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

	return sources, includes, nil
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
func (db *DepBuilder) determineIncludePath(dep deps.Dependency, sourcePath string, configIncludes []string) string {
	// If inline config specifies includes, use the first one
	var inlineConfig *deps.InlineConfig

	switch d := dep.(type) {
	case *deps.GitDependency:
		inlineConfig = d.BuildConfig
	case *deps.TarballDependency:
		inlineConfig = d.BuildConfig
	case *deps.VendoredDependency:
		inlineConfig = d.BuildConfig
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
