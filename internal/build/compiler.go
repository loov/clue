package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

func (c *compiler) cacheInputs(opts plan.CompileOptions) []string {
	encoded, _ := json.Marshal(opts)
	inputs := append([]string{string(encoded), toolchainCacheKey(c.toolchain)}, c.toolchain.CompilerFlags(opts.Flags)...)
	inputs = append(inputs, toolchainCacheEnvironment(c.toolchain)...)
	for _, name := range slices.Sorted(maps.Keys(opts.ModuleFiles)) {
		hash, err := cache.ComputeFileHash(opts.ModuleFiles[name])
		if err != nil {
			hash = "missing"
		}
		inputs = append(inputs, "module:"+name+"="+hash)
	}
	return inputs
}

// CompileResult holds the result of a compilation
type compileResult struct {
	Source       string
	Object       string
	DepFile      string   // Path to generated .d file (GCC/Clang)
	Dependencies []string // Parsed dependencies (MSVC /showIncludes)
	Stdout       string
	Stderr       string
	Duration     time.Duration
	Success      bool
}

// Compiler handles source file compilation
type compiler struct {
	executor  *executor
	toolchain toolchain.Toolchain
}

// NewCompiler creates a new Compiler instance
func newCompiler(executor *executor, toolchain toolchain.Toolchain) *compiler {
	return &compiler{
		executor:  executor,
		toolchain: toolchain,
	}
}

// CompileSource compiles a single source file to an object file
func (c *compiler) CompileSource(ctx context.Context, opts plan.CompileOptions) (_ *compileResult, resultErr error) {
	start := time.Now()
	outputDir := filepath.Dir(opts.Output)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return &compileResult{
			Source:   opts.Source,
			Object:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}
	opts.DependencyMode = plan.DependencyModeAll
	if toolchainCacheKey(c.toolchain) != "" {
		opts.DependencyMode = plan.DependencyModeProject
	}
	invocation, err := plan.Compile(c.toolchain, opts)
	if err != nil {
		return nil, err
	}
	if c.toolchain.Name() == "clang" && opts.InternalPartition {
		if err := c.precompileClangPartition(ctx, opts); err != nil {
			return &compileResult{
				Source: opts.Source, Object: opts.Output, DepFile: invocation.DependencyFile,
				Duration: time.Since(start), Success: false,
			}, err
		}
	}
	finalArgs, cleanupPath, err := c.responseFile(opts.Output, invocation.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	commandResult, err := c.executor.RunCommand(ctx, invocation.Tool, finalArgs...)

	result := &compileResult{
		Source:   opts.Source,
		Object:   opts.Output,
		DepFile:  invocation.DependencyFile,
		Duration: time.Since(start),
		Success:  err == nil,
	}
	if commandResult != nil {
		result.Stdout = commandResult.Stdout
		result.Stderr = commandResult.Stderr
		if isMSVC(c.toolchain) {
			result.Dependencies = parseShowIncludes(commandResult.Stdout)
		}
	}
	if err != nil {
		return result, fmt.Errorf("failed to compile %s: %w", opts.Source, err)
	}
	if isMSVC(c.toolchain) {
		depFile, err := writeDependencyFile(opts.Output, opts.Source, result.Dependencies)
		if err != nil {
			return result, fmt.Errorf("failed to record dependencies for %s: %w", opts.Source, err)
		}
		result.DepFile = depFile
	}
	return result, nil
}

func (c *compiler) responseFile(output string, args []string) ([]string, string, error) {
	if isMSVC(c.toolchain) {
		return toolchain.MaybeUseResponseFileIn(filepath.Dir(output), args)
	}
	return toolchain.MaybeUseGNUResponseFileIn(filepath.Dir(output), args)
}

func (c *compiler) precompileClangPartition(ctx context.Context, opts plan.CompileOptions) (resultErr error) {
	invocation := plan.CompileModulePartition(c.toolchain, opts)
	finalArgs, cleanupPath, err := toolchain.MaybeUseGNUResponseFileIn(filepath.Dir(opts.ModuleOutput), invocation.Arguments)
	if err != nil {
		return fmt.Errorf("create module-partition response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}
	if _, err := c.executor.RunCommand(ctx, invocation.Tool, finalArgs...); err != nil {
		return fmt.Errorf("precompile module partition %s: %w", opts.ModuleName, err)
	}
	return nil
}

func writeDependencyFile(output, source string, dependencies []string) (string, error) {
	escape := func(path string) string {
		return strings.NewReplacer(" ", "\\ ", "\t", "\\\t").Replace(path)
	}
	depFile := strings.TrimSuffix(output, filepath.Ext(output)) + ".d"
	paths := append([]string{source}, dependencies...)
	for i := range paths {
		paths[i] = escape(paths[i])
	}
	content := escape(output) + ": " + strings.Join(paths, " ") + "\n"
	return depFile, os.WriteFile(depFile, []byte(content), 0o644)
}

// parseShowIncludes parses MSVC /showIncludes output to extract header dependencies.
// The toolchain forces VSLANG=1033, so the format is stable across installations.
func parseShowIncludes(stdout string) []string {
	var deps []string
	prefix := "Note: including file:"

	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			// Extract path after prefix
			path := strings.TrimSpace(line[len(prefix):])
			if path != "" {
				deps = append(deps, filepath.Clean(path))
			}
		}
	}
	return deps
}

// CompileSources compiles multiple source files with fail-fast behavior
// On first error, stops and returns error with all successful results up to the failure
func (c *compiler) CompileSources(ctx context.Context, sources []plan.CompileOptions) ([]compileResult, error) {
	var results []compileResult

	for _, opts := range sources {
		result, err := c.CompileSource(ctx, opts)
		if err != nil {
			// Fail fast: return successful results and the error
			return results, err
		}
		results = append(results, *result)
	}

	return results, nil
}
