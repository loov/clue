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
)

func (c *Compiler) cacheInputs(opts CompileOptions) []string {
	encoded, _ := json.Marshal(opts)
	return append([]string{string(encoded)}, c.toolchain.CompilerFlags(opts.Flags)...)
}

// CompileOptions holds options for compiling a single source file
type CompileOptions struct {
	Source       string            // Source file path
	Output       string            // Output object file path
	Includes     []string          // Include directories
	Defines      []string          // Preprocessor defines
	Flags        Config            // Semantic flags
	Std          string            // Language standard (e.g., "c++20", "c17")
	TargetType   string            // "executable", "static_library", "shared_library"
	ModuleOutput string            // Path to output precompiled module (.pcm) when compiling module interface
	ModuleFiles  map[string]string // Map of module name to .pcm path for -fmodule-file flags
}

// CompileResult holds the result of a compilation
type CompileResult struct {
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
type Compiler struct {
	executor  *Executor
	toolchain Toolchain
}

// NewCompiler creates a new Compiler instance
func NewCompiler(executor *Executor, toolchain Toolchain) *Compiler {
	return &Compiler{
		executor:  executor,
		toolchain: toolchain,
	}
}

// isCPlusPlus detects if a source file is C++ based on extension
func (c *Compiler) isCPlusPlus(source string) bool {
	ext := filepath.Ext(source)
	if ext == ".C" {
		return true
	}
	switch strings.ToLower(ext) {
	case ".cpp", ".cc", ".cxx", ".c++", ".cppm", ".ixx", ".mpp":
		return true
	default:
		return false
	}
}

// compilerCmd returns the appropriate compiler command for a source file
func (c *Compiler) compilerCmd(source string) string {
	isCPP := c.isCPlusPlus(source)

	if isCPP {
		return c.toolchain.CXX()
	}
	return c.toolchain.CC()
}

// CompileSource compiles a single source file to an object file
func (c *Compiler) CompileSource(ctx context.Context, opts CompileOptions) (*CompileResult, error) {
	start := time.Now()

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(opts.Output)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return &CompileResult{
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
func (c *Compiler) compileSourceGCC(ctx context.Context, opts CompileOptions, start time.Time) (*CompileResult, error) {
	// Build command arguments in order
	var args []string

	// 1. Compile only flag
	args = append(args, "-c")

	// 2. Source file
	args = append(args, opts.Source)

	// 3. Output file
	args = append(args, "-o", opts.Output)

	// 4. Dependency generation flags
	depFile := filepath.Base(opts.Output[:len(opts.Output)-len(filepath.Ext(opts.Output))]) + ".d"
	depFile = filepath.Join(filepath.Dir(opts.Output), depFile)
	args = append(args, "-MMD", "-MP", "-MF", depFile)

	// 5. Position-independent code for shared libraries (automatic)
	if opts.TargetType == "shared_library" {
		args = append(args, "-fPIC")
	}

	// 6. Include paths
	for _, include := range opts.Includes {
		args = append(args, "-I"+include)
	}

	// 7. Defines
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}

	// 8. Language standard
	if opts.Std != "" {
		args = append(args, "-std="+opts.Std)
	}

	// 9. C++20 Module flags
	if opts.ModuleOutput != "" {
		// Generate precompiled module interface when compiling module source
		args = append(args, "-fmodule-output="+opts.ModuleOutput)
	}
	for _, modName := range slices.Sorted(maps.Keys(opts.ModuleFiles)) {
		pcmPath := opts.ModuleFiles[modName]
		// Reference precompiled modules when compiling consumers
		args = append(args, fmt.Sprintf("-fmodule-file=%s=%s", modName, pcmPath))
	}

	// 10. Semantic flags
	semanticFlags := c.toolchain.CompilerFlags(opts.Flags)
	args = append(args, semanticFlags...)

	// Get compiler command
	compiler := c.compilerCmd(opts.Source)

	// Execute compilation
	commandResult, err := c.executor.RunCommand(ctx, compiler, args...)

	duration := time.Since(start)
	result := &CompileResult{
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

// compileSourceMSVC compiles using MSVC toolchain (cl.exe)
func (c *Compiler) compileSourceMSVC(ctx context.Context, opts CompileOptions, start time.Time) (_ *CompileResult, resultErr error) {
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

	// 6. Defines (MSVC style: /D)
	for _, define := range opts.Defines {
		args = append(args, "/D"+define)
	}

	// 7. Language standard (MSVC style: /std:)
	if opts.Std != "" {
		args = append(args, "/std:"+TranslateStdForMSVC(opts.Std))
	}

	// 8. C++20 Modules (MSVC has different module syntax - deferred to v0.3.0)
	// Note: MSVC uses /interface, /headerUnit, /reference instead of -fmodule-output/-fmodule-file
	// For now, we skip module support in MSVC

	// Use response file for many include paths
	finalArgs, cleanupPath, err := MaybeUseResponseFile(args)
	if err != nil {
		return &CompileResult{
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
	result := &CompileResult{
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
		return strings.NewReplacer("\\", "\\\\", " ", "\\ ", "\t", "\\\t").Replace(path)
	}
	depFile := strings.TrimSuffix(output, filepath.Ext(output)) + ".d"
	paths := append([]string{source}, dependencies...)
	for i := range paths {
		paths[i] = escape(paths[i])
	}
	content := escape(output) + ": " + strings.Join(paths, " ") + "\n"
	return depFile, os.WriteFile(depFile, []byte(content), 0o644)
}

// TranslateStdForMSVC translates C/C++ standard names to MSVC format.
// MSVC uses /std:c++17, /std:c++20, /std:c++latest etc.
func TranslateStdForMSVC(std string) string {
	// Handle C++ standards with prefix
	switch std {
	case "c++11", "gnu++11":
		return "c++14" // MSVC minimum is C++14, approximate C++11 with C++14
	case "c++14", "gnu++14":
		return "c++14"
	case "c++17", "gnu++17":
		return "c++17"
	case "c++20", "gnu++20":
		return "c++20"
	case "c++23", "gnu++23":
		return "c++latest" // MSVC uses latest for experimental C++23 features
	// Handle C standards with prefix
	case "c99", "gnu99", "c9x":
		return "c11" // Approximate C99 with C11 (MSVC has limited C99)
	case "c11", "gnu11", "c1x":
		return "c11"
	case "c17", "gnu17", "c18":
		return "c17"
	case "c2x", "gnu2x", "c23":
		return "clatest" // C23 via latest
	default:
		// Pass through for known MSVC values or fallback
		return std
	}
}

// parseShowIncludes parses MSVC /showIncludes output to extract header dependencies.
// The format is: "Note: including file: <path>" (English Visual Studio)
// Note: This is localized in non-English VS - for v0.2.0 we assume English.
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
func (c *Compiler) CompileSources(ctx context.Context, sources []CompileOptions) ([]CompileResult, error) {
	var results []CompileResult

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
