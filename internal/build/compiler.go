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

func (c *compiler) cacheInputs(opts compileOptions) []string {
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

// CompileOptions holds options for compiling a single source file
type compileOptions struct {
	Source            string           // Source file path
	Output            string           // Output object file path
	Includes          []string         // Include directories
	SystemIncludes    []string         // Third-party include directories
	Defines           []string         // Preprocessor defines
	Flags             toolchain.Config // Semantic flags
	Std               string           // Language standard (e.g., "c++20", "c17")
	TargetType        string           // "executable", "static_library", "shared_library"
	Platform          toolchain.Platform
	ModuleOutput      string            // Path to output binary module interface
	ModuleFiles       map[string]string // Logical module/header-unit name to BMI path
	ModuleName        string
	ModuleMapper      string
	InternalPartition bool
	ModuleAware       bool
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

// isCPlusPlus detects if a source file is C++ based on extension
func (c *compiler) isCPlusPlus(source string) bool {
	return toolchain.IsCXXSource(source)
}

// compilerCmd returns the appropriate compiler command for a source file
func (c *compiler) compilerCmd(source string) string {
	isCPP := c.isCPlusPlus(source)

	if isCPP {
		return c.toolchain.CXX()
	}
	return c.toolchain.CC()
}

// CompileSource compiles a single source file to an object file
func (c *compiler) CompileSource(ctx context.Context, opts compileOptions) (*compileResult, error) {
	start := time.Now()
	if isMSVC(c.toolchain) && toolchain.IsAssemblySource(opts.Source) {
		return nil, fmt.Errorf("MSVC cannot compile GNU-style assembly source %q; use a GCC or Clang toolchain", opts.Source)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(opts.Output)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return &compileResult{
			Source:   opts.Source,
			Object:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Branch based on toolchain
	if isMSVC(c.toolchain) {
		return c.compileSourceMSVC(ctx, opts, start)
	}
	return c.compileSourceGCC(ctx, opts, start)
}

// compileSourceGCC compiles using GCC/Clang toolchain
func (c *compiler) compileSourceGCC(ctx context.Context, opts compileOptions, start time.Time) (_ *compileResult, resultErr error) {
	// Build command arguments in order
	var args []string

	// 1. Compile only flag
	args = append(args, "-c")
	args = append(args, plan.ModuleCompileFlags(c.toolchain, plan.ModuleDependency{
		Source: opts.Source, IsModule: opts.ModuleOutput != "", Provides: opts.ModuleName,
		InternalPartition: opts.InternalPartition, UsesModules: opts.ModuleAware,
	}, opts.ModuleOutput, opts.ModuleFiles, opts.ModuleMapper)...)

	// 2. Source file
	args = append(args, opts.Source)

	// 3. Output file
	args = append(args, "-o", opts.Output)

	// 4. Dependency generation flags
	depFile := filepath.Base(opts.Output[:len(opts.Output)-len(filepath.Ext(opts.Output))]) + ".d"
	depFile = filepath.Join(filepath.Dir(opts.Output), depFile)
	dependencyMode := "-MD"
	if toolchainCacheKey(c.toolchain) != "" {
		// Container system headers are represented by the immutable image ID and
		// cannot be hashed from the host filesystem.
		dependencyMode = "-MMD"
	}
	args = append(args, dependencyMode, "-MP", "-MF", depFile)

	// 5. Position-independent code for shared libraries (automatic)
	platform := opts.Platform
	if platform.OS == "" {
		platform = toolchain.HostPlatform()
	}
	if opts.TargetType == "shared_library" && platform.OS != "windows" {
		args = append(args, "-fPIC")
	}

	// 6. Include paths
	for _, include := range opts.Includes {
		args = append(args, "-I"+include)
	}
	for _, include := range opts.SystemIncludes {
		args = append(args, "-isystem", include)
	}

	// 7. Defines
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}

	// 8. Language standard
	standard := opts.Std
	if standard == "" && (opts.ModuleAware || opts.ModuleOutput != "" || len(opts.ModuleFiles) > 0) {
		standard = "c++20"
	}
	if standard != "" && !toolchain.IsAssemblySource(opts.Source) {
		args = append(args, "-std="+standard)
	}

	// 9. Semantic flags
	semanticFlags := c.toolchain.CompilerFlags(opts.Flags)
	args = append(args, semanticFlags...)

	// Get compiler command
	compiler := c.compilerCmd(opts.Source)
	if c.toolchain.Name() == "clang" && opts.InternalPartition {
		if err := c.precompileClangPartition(ctx, opts); err != nil {
			return &compileResult{
				Source: opts.Source, Object: opts.Output, DepFile: depFile,
				Duration: time.Since(start), Success: false,
			}, err
		}
	}
	finalArgs, cleanupPath, err := toolchain.MaybeUseGNUResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute compilation
	commandResult, err := c.executor.RunCommand(ctx, compiler, finalArgs...)

	duration := time.Since(start)
	result := &compileResult{
		Source:   opts.Source,
		Object:   opts.Output,
		DepFile:  depFile,
		Duration: duration,
		Success:  err == nil,
	}
	if commandResult != nil {
		result.Stdout = commandResult.Stdout
		result.Stderr = commandResult.Stderr
	}

	if err != nil {
		return result, fmt.Errorf("failed to compile %s: %w", opts.Source, err)
	}

	return result, nil
}

func (c *compiler) precompileClangPartition(ctx context.Context, opts compileOptions) (resultErr error) {
	standard := opts.Std
	if standard == "" {
		standard = "c++20"
	}
	args := []string{"-std=" + standard}
	for _, include := range opts.Includes {
		args = append(args, "-I"+include)
	}
	for _, include := range opts.SystemIncludes {
		args = append(args, "-isystem", include)
	}
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}
	args = append(args, c.toolchain.CompilerFlags(opts.Flags)...)
	args = append(args, plan.ModuleCompileFlags(c.toolchain, plan.ModuleDependency{UsesModules: true}, "", opts.ModuleFiles, "")...)
	args = append(args, "-x", "c++-module", "--precompile", opts.Source, "-o", opts.ModuleOutput)
	finalArgs, cleanupPath, err := toolchain.MaybeUseGNUResponseFileIn(filepath.Dir(opts.ModuleOutput), args)
	if err != nil {
		return fmt.Errorf("create module-partition response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}
	if _, err := c.executor.RunCommand(ctx, c.toolchain.CXX(), finalArgs...); err != nil {
		return fmt.Errorf("precompile module partition %s: %w", opts.ModuleName, err)
	}
	return nil
}

// compileSourceMSVC compiles using MSVC toolchain (cl.exe)
func (c *compiler) compileSourceMSVC(ctx context.Context, opts compileOptions, start time.Time) (_ *compileResult, resultErr error) {
	// Build MSVC-style command: cl.exe /nologo /c source.cpp /Fooutput.obj /Iinclude
	var args []string

	// 1. Semantic flags first (includes /nologo, optimization, warnings, debug, CRT, /showIncludes)
	semanticFlags := c.toolchain.CompilerFlags(opts.Flags)
	args = append(args, semanticFlags...)

	// 2. Compile only flag (MSVC style)
	args = append(args, "/c")

	// 3. Source file
	args = append(args, opts.Source)

	// 4. Output file (MSVC style: /Fo without space)
	args = append(args, "/Fo"+opts.Output)

	// 5. Include paths (MSVC style: /I)
	for _, include := range opts.Includes {
		args = append(args, "/I"+include)
	}
	for _, include := range opts.SystemIncludes {
		args = append(args, "/external:I"+include)
	}

	// 6. Defines (MSVC style: /D)
	for _, define := range opts.Defines {
		args = append(args, "/D"+define)
	}

	// 7. Language standard (MSVC style: /std:)
	standard := opts.Std
	if standard == "" && (opts.ModuleAware || opts.ModuleOutput != "" || len(opts.ModuleFiles) > 0) {
		standard = "c++20"
	}
	if standard != "" && !toolchain.IsAssemblySource(opts.Source) {
		args = append(args, "/std:"+plan.TranslateStdForMSVC(standard))
	}

	// 8. C++20 Modules
	args = append(args, plan.ModuleCompileFlags(c.toolchain, plan.ModuleDependency{
		Source: opts.Source, IsModule: opts.ModuleOutput != "", Provides: opts.ModuleName,
		InternalPartition: opts.InternalPartition, UsesModules: opts.ModuleAware,
	}, opts.ModuleOutput, opts.ModuleFiles, opts.ModuleMapper)...)

	// Use response file for many include paths
	finalArgs, cleanupPath, err := toolchain.MaybeUseResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return &compileResult{
			Source:   opts.Source,
			Object:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Get compiler command
	compiler := c.compilerCmd(opts.Source)

	// Execute compilation. Parallel builds use a capturing executor so each
	// compiler's output can be printed without interleaving.
	commandResult, err := c.executor.RunCommand(ctx, compiler, finalArgs...)

	duration := time.Since(start)
	result := &compileResult{
		Source:   opts.Source,
		Object:   opts.Output,
		Duration: duration,
		Success:  err == nil,
	}
	if commandResult != nil {
		result.Stdout = commandResult.Stdout
		result.Stderr = commandResult.Stderr
		result.Dependencies = parseShowIncludes(commandResult.Stdout)
	}

	if err != nil {
		return result, fmt.Errorf("failed to compile %s: %w", opts.Source, err)
	}
	depFile, err := writeDependencyFile(opts.Output, opts.Source, result.Dependencies)
	if err != nil {
		return result, fmt.Errorf("failed to record dependencies for %s: %w", opts.Source, err)
	}
	result.DepFile = depFile

	return result, nil
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
func (c *compiler) CompileSources(ctx context.Context, sources []compileOptions) ([]compileResult, error) {
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

// CompileHeaderUnit builds one header unit BMI.
func (c *compiler) CompileHeaderUnit(ctx context.Context, opts plan.HeaderUnitOptions) (resultErr error) {
	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return fmt.Errorf("create header-unit output directory: %w", err)
	}
	arguments := plan.HeaderUnitArguments(c.toolchain, opts)
	responseFile := toolchain.MaybeUseGNUResponseFileIn
	if isMSVC(c.toolchain) {
		responseFile = toolchain.MaybeUseResponseFileIn
	}
	args, cleanupPath, err := responseFile(filepath.Dir(opts.Output), arguments)
	if err != nil {
		return fmt.Errorf("create header-unit response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}
	if _, err = c.executor.RunCommand(ctx, c.toolchain.CXX(), args...); err != nil {
		return fmt.Errorf("compile header unit %s: %w", opts.Name, err)
	}
	if _, err = os.Stat(opts.Output); err != nil {
		return fmt.Errorf("compiler did not produce header unit %s at %s: %w", opts.Name, opts.Output, err)
	}
	return nil
}
