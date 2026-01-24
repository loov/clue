package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CompileOptions holds options for compiling a single source file
type CompileOptions struct {
	Source     string   // Source file path
	Output     string   // Output object file path
	Includes   []string // Include directories
	Defines    []string // Preprocessor defines
	Flags      Config   // Semantic flags
	Std        string   // Language standard (e.g., "c++20", "c17")
	TargetType string   // "executable", "static_library", "shared_library"
}

// CompileResult holds the result of a compilation
type CompileResult struct {
	Source   string
	Object   string
	DepFile  string // Path to generated .d file
	Duration time.Duration
	Success  bool
}

// Compiler handles source file compilation
type Compiler struct {
	executor  *Executor
	toolchain *Toolchain
}

// NewCompiler creates a new Compiler instance
func NewCompiler(executor *Executor, toolchain *Toolchain) *Compiler {
	return &Compiler{
		executor:  executor,
		toolchain: toolchain,
	}
}

// isCPlusPlus detects if a source file is C++ based on extension
func (c *Compiler) isCPlusPlus(source string) bool {
	ext := filepath.Ext(source)
	switch ext {
	case ".cpp", ".cc", ".cxx", ".C", ".CPP":
		return true
	default:
		return false
	}
}

// compilerCmd returns the appropriate compiler command for a source file
func (c *Compiler) compilerCmd(source string) string {
	isCPP := c.isCPlusPlus(source)

	if isCPP {
		return c.toolchain.CXX
	}
	return c.toolchain.CC
}

// CompileSource compiles a single source file to an object file
func (c *Compiler) CompileSource(ctx context.Context, opts CompileOptions) (*CompileResult, error) {
	start := time.Now()

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

	// 9. Semantic flags
	semanticFlags := CompilerFlagsWithToolchain(opts.Flags, c.toolchain.Name)
	args = append(args, semanticFlags...)

	// 10. Raw compiler flags (already included in semantic flags via BuildCompilerFlags)
	// No need to add again

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(opts.Output)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return &CompileResult{
			Source:   opts.Source,
			Object:   opts.Output,
			DepFile:  depFile,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Get compiler command
	compiler := c.compilerCmd(opts.Source)

	// Execute compilation
	err := c.executor.RunCompiler(ctx, compiler, args)

	duration := time.Since(start)
	result := &CompileResult{
		Source:   opts.Source,
		Object:   opts.Output,
		DepFile:  depFile,
		Duration: duration,
		Success:  err == nil,
	}

	if err != nil {
		return result, fmt.Errorf("failed to compile %s: %w", opts.Source, err)
	}

	return result, nil
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
