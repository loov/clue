package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SharedLibraryExtension returns the platform-specific shared library extension
func SharedLibraryExtension(target Platform) string {
	switch target.OS {
	case "darwin":
		return ".dylib"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

// LinkOptions holds options for linking an executable
type LinkOptions struct {
	Objects      []string // Object files to link
	Output       string   // Output executable path
	SysLibs      []string // System libraries (pthread, m, dl)
	LibPaths     []string // Library search paths (-L)
	Libs         []string // Additional libraries to link
	Flags        Config   // For raw linker flags and debug info
	UseCPlusPlus bool     // Use clang++/g++ for linking (C++ std lib)
}

// SharedLibraryOptions holds options for linking a shared library
type SharedLibraryOptions struct {
	Objects          []string // Object files to link
	Output           string   // Output .so/.dylib path
	SysLibs          []string // System libraries (pthread, m, dl)
	LibPaths         []string // Library search paths (-L)
	Libs             []string // Additional libraries to link
	Flags            Config   // For raw linker flags and debug info
	UseCPlusPlus     bool     // Use clang++/g++ for linking (C++ std lib)
	SymbolVisibility string   // "default" or "hidden"
}

// ArchiveOptions holds options for creating a static library
type ArchiveOptions struct {
	Objects []string // Object files to archive
	Output  string   // Output static library path (e.g., libfoo.a)
}

// LinkResult holds the result of a link or archive operation
type LinkResult struct {
	Output   string
	Duration time.Duration
	Success  bool
}

// Linker handles linking object files into executables and creating static libraries
type Linker struct {
	executor  *Executor
	toolchain Toolchain
	target    Platform
}

// NewLinker creates a new Linker with the given executor and toolchain
func NewLinker(executor *Executor, toolchain Toolchain, target Platform) *Linker {
	return &Linker{
		executor:  executor,
		toolchain: toolchain,
		target:    target,
	}
}

// LinkExecutable links object files into an executable binary
func (l *Linker) LinkExecutable(ctx context.Context, opts LinkOptions) (*LinkResult, error) {
	start := time.Now()

	// Determine the linker command based on C++ requirement
	linkerCmd := l.toolchain.CC()
	if opts.UseCPlusPlus {
		linkerCmd = l.toolchain.CXX()
	}

	// Build command arguments
	var args []string

	// Add all object files first
	args = append(args, opts.Objects...)

	// Add output flag
	args = append(args, "-o", opts.Output)

	// Add library search paths
	for _, path := range opts.LibPaths {
		args = append(args, "-L"+path)
	}

	// Add additional libraries
	for _, lib := range opts.Libs {
		args = append(args, "-l"+lib)
	}

	// Add system libraries
	for _, sysLib := range opts.SysLibs {
		args = append(args, "-l"+sysLib)
	}

	// Add linker flags from BuildLinkerFlags (includes debug and raw flags)
	linkerFlags := l.toolchain.LinkerFlags(opts.Flags, []string{}) // Pass empty sysLibs since we handle them above
	args = append(args, linkerFlags...)

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Execute the linker
	result, err := l.executor.RunCommand(ctx, linkerCmd, args...)
	if err != nil {
		return &LinkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("linker failed: %w", err)
	}

	return &LinkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// CreateStaticLibrary archives object files into a static library
func (l *Linker) CreateStaticLibrary(ctx context.Context, opts ArchiveOptions) (*LinkResult, error) {
	start := time.Now()

	// Build ar command arguments
	// ar crs: c=create, r=replace/insert, s=create symbol table
	args := []string{"crs", opts.Output}
	args = append(args, opts.Objects...)

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Execute the archiver using toolchain AR
	result, err := l.executor.RunCommand(ctx, l.toolchain.AR(), args...)
	if err != nil {
		return &LinkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("archiver failed: %w", err)
	}

	return &LinkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// LinkSharedLibrary links object files into a shared library (.so on Linux, .dylib on macOS)
func (l *Linker) LinkSharedLibrary(ctx context.Context, opts SharedLibraryOptions) (*LinkResult, error) {
	start := time.Now()

	// Determine the linker command based on C++ requirement
	linkerCmd := l.toolchain.CC()
	if opts.UseCPlusPlus {
		linkerCmd = l.toolchain.CXX()
	}

	// Build command arguments
	var args []string

	// Add -shared flag to create shared library
	args = append(args, "-shared")

	// Add all object files
	args = append(args, opts.Objects...)

	// Add output flag
	args = append(args, "-o", opts.Output)

	// Platform-specific shared library options
	libName := filepath.Base(opts.Output)
	switch l.target.OS {
	case "darwin":
		// macOS: Set install_name with @rpath for relocatable libraries
		args = append(args, "-install_name", "@rpath/"+libName)
	case "linux":
		// Linux: Set SONAME for library versioning
		args = append(args, "-Wl,-soname,"+libName)
	}

	// Add symbol visibility flag if hidden
	if opts.SymbolVisibility == "hidden" {
		args = append(args, "-fvisibility=hidden")
	}

	// Add library search paths
	for _, path := range opts.LibPaths {
		args = append(args, "-L"+path)
	}

	// Add additional libraries
	for _, lib := range opts.Libs {
		args = append(args, "-l"+lib)
	}

	// Add system libraries
	for _, sysLib := range opts.SysLibs {
		args = append(args, "-l"+sysLib)
	}

	// Add linker flags from BuildLinkerFlags (includes debug and raw flags)
	linkerFlags := l.toolchain.LinkerFlags(opts.Flags, []string{})
	args = append(args, linkerFlags...)

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Execute the linker
	result, err := l.executor.RunCommand(ctx, linkerCmd, args...)
	if err != nil {
		return &LinkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("linker failed: %w", err)
	}

	return &LinkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// needsCPlusPlusLinker determines if C++ linker is needed based on object files
// This is a heuristic approach - checks if object paths suggest C++ origin
func (l *Linker) needsCPlusPlusLinker(objects []string) bool {
	for _, obj := range objects {
		// Check if the object file path suggests C++ origin
		// Common C++ extensions: .cpp, .cc, .cxx, .C
		objLower := strings.ToLower(obj)
		if strings.Contains(objLower, ".cpp.") ||
			strings.Contains(objLower, ".cc.") ||
			strings.Contains(objLower, ".cxx.") ||
			strings.HasSuffix(objLower, ".cpp.o") ||
			strings.HasSuffix(objLower, ".cc.o") ||
			strings.HasSuffix(objLower, ".cxx.o") {
			return true
		}
	}
	return false
}
