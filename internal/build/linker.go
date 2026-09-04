package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loov/clue/internal/toolchain"
)

// LinkOptions holds options for linking an executable
type linkOptions struct {
	Objects  []string        // Object files to link
	Output   string          // Output executable path
	SysLibs  []string        // System libraries (pthread, m, dl)
	LibPaths []string        // Library search paths (-L)
	Libs     []string        // Additional libraries to link
	Flags    toolchain.Flags // For raw linker flags and debug info
	UseCXX   bool            // Use the C++ driver when the link graph contains C++
}

// SharedLibraryOptions holds options for linking a shared library
type sharedLibraryOptions struct {
	Objects          []string        // Object files to link
	Output           string          // Output .so/.dylib path
	SysLibs          []string        // System libraries (pthread, m, dl)
	LibPaths         []string        // Library search paths (-L)
	Libs             []string        // Additional libraries to link
	Flags            toolchain.Flags // For raw linker flags and debug info
	SymbolVisibility string          // "default" or "hidden"
	UseCXX           bool            // Use the C++ driver when the link graph contains C++
}

// ArchiveOptions holds options for creating a static library
type archiveOptions struct {
	Objects []string // Object files to archive
	Output  string   // Output static library path (e.g., libfoo.a)
}

// LinkResult holds the result of a link or archive operation
type linkResult struct {
	Output      string
	ImportLib   string // Import library path for DLLs
	Duration    time.Duration
	Success     bool
	CleanupPath string // Response file to cleanup (internal use)
}

// Linker handles linking object files into executables and creating static libraries
type linker struct {
	executor  *executor
	toolchain toolchain.Toolchain
	target    toolchain.Platform
}

// NewLinker creates a new Linker with the given executor and toolchain
func newLinker(executor *executor, toolchain toolchain.Toolchain, target toolchain.Platform) *linker {
	return &linker{
		executor:  executor,
		toolchain: toolchain,
		target:    target,
	}
}

// isMSVC returns true if the toolchain is MSVC
func isMSVC(tc toolchain.Toolchain) bool {
	return tc.Name() == "msvc"
}

// LinkExecutable links object files into an executable binary
func (l *linker) LinkExecutable(ctx context.Context, opts linkOptions) (*linkResult, error) {
	start := time.Now()

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	// Branch based on toolchain
	if isMSVC(l.toolchain) {
		return l.linkExecutableMSVC(ctx, opts, start)
	}
	return l.linkExecutableGCC(ctx, opts, start)
}

// linkExecutableGCC links using GCC/Clang toolchain
func (l *linker) linkExecutableGCC(ctx context.Context, opts linkOptions, start time.Time) (_ *linkResult, resultErr error) {
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
		if flag := toolchain.SystemLibraryFlag(l.toolchain.Name(), l.target, sysLib); flag != "" {
			args = append(args, flag)
		}
	}

	// Add linker flags from BuildLinkerFlags (includes debug and raw flags)
	linkerFlags := l.toolchain.LinkerFlags(opts.Flags, []string{}) // Pass empty sysLibs since we handle them above
	args = append(args, linkerFlags...)
	finalArgs, cleanupPath, err := toolchain.MaybeUseGNUResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute the linker
	result, err := l.executor.RunCommand(ctx, l.linkDriver(opts.UseCXX), finalArgs...)
	if err != nil {
		return &linkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("linker failed: %w", err)
	}

	return &linkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// linkExecutableMSVC links using MSVC toolchain (link.exe)
func (l *linker) linkExecutableMSVC(ctx context.Context, opts linkOptions, start time.Time) (_ *linkResult, resultErr error) {
	// Build MSVC-style command: link.exe /nologo objects... /OUT:output.exe libs...
	var args []string

	// Get linker flags first (includes /nologo, /DEBUG, etc.)
	linkerFlags := l.toolchain.LinkerFlags(opts.Flags, []string{})
	args = append(args, linkerFlags...)

	// Add all object files
	args = append(args, opts.Objects...)

	// Add output flag (MSVC style)
	args = append(args, "/OUT:"+opts.Output)

	// Add library search paths (MSVC style)
	for _, path := range opts.LibPaths {
		args = append(args, "/LIBPATH:"+path)
	}

	// Add additional libraries
	for _, lib := range opts.Libs {
		if strings.HasSuffix(lib, ".lib") {
			args = append(args, lib)
		} else {
			args = append(args, lib+".lib")
		}
	}

	// Add system libraries (translated to MSVC format)
	for _, sysLib := range opts.SysLibs {
		if flag := toolchain.SystemLibraryFlag(l.toolchain.Name(), l.target, sysLib); flag != "" {
			args = append(args, flag)
		}
	}

	// Use response file for long command lines
	finalArgs, cleanupPath, err := toolchain.MaybeUseResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute link.exe
	linker := l.msvcTool("link.exe")
	result, err := l.executor.RunCommand(ctx, linker, finalArgs...)
	if err != nil {
		// Show full command line on linker errors (per CONTEXT.md)
		cmdLine := linker + " " + strings.Join(args, " ")
		return &linkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("linker failed: %w\nCommand: %s", err, cmdLine)
	}

	return &linkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// CreateStaticLibrary archives object files into a static library
func (l *linker) CreateStaticLibrary(ctx context.Context, opts archiveOptions) (*linkResult, error) {
	start := time.Now()

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	// Archivers update existing files in place, which would retain members for
	// sources removed from the target.
	if err := os.Remove(opts.Output); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to replace static library: %w", err)
	}

	// Branch based on toolchain
	if isMSVC(l.toolchain) {
		return l.createStaticLibraryMSVC(ctx, opts, start)
	}
	return l.createStaticLibraryGCC(ctx, opts, start)
}

// createStaticLibraryGCC creates a static library using ar
func (l *linker) createStaticLibraryGCC(ctx context.Context, opts archiveOptions, start time.Time) (_ *linkResult, resultErr error) {
	// Build ar command arguments
	// ar crs: c=create, r=replace/insert, s=create symbol table
	args := []string{"crs", opts.Output}
	args = append(args, opts.Objects...)
	finalArgs, cleanupPath, err := toolchain.MaybeUseResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute the archiver using toolchain AR
	result, err := l.executor.RunCommand(ctx, l.toolchain.AR(), finalArgs...)
	if err != nil {
		return &linkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("archiver failed: %w", err)
	}

	return &linkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// createStaticLibraryMSVC creates a static library using lib.exe
func (l *linker) createStaticLibraryMSVC(ctx context.Context, opts archiveOptions, start time.Time) (_ *linkResult, resultErr error) {
	// Build lib.exe command: lib.exe /nologo /OUT:output.lib objects...
	var args []string

	// Add /nologo to suppress banner
	args = append(args, "/nologo")

	// Add output flag
	args = append(args, "/OUT:"+opts.Output)

	// Add all object files
	args = append(args, opts.Objects...)

	// Use response file for many objects
	finalArgs, cleanupPath, err := toolchain.MaybeUseResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute lib.exe
	result, err := l.executor.RunCommand(ctx, l.toolchain.AR(), finalArgs...)
	if err != nil {
		return &linkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("archiver failed: %w", err)
	}

	return &linkResult{
		Output:   opts.Output,
		Duration: result.Duration,
		Success:  true,
	}, nil
}

// LinkSharedLibrary links object files into a shared library (.so on Linux, .dylib on macOS, .dll on Windows)
func (l *linker) LinkSharedLibrary(ctx context.Context, opts sharedLibraryOptions) (*linkResult, error) {
	start := time.Now()

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Branch based on toolchain
	if isMSVC(l.toolchain) {
		return l.linkSharedLibraryMSVC(ctx, opts, start)
	}
	return l.linkSharedLibraryGCC(ctx, opts, start)
}

// linkSharedLibraryGCC links a shared library using GCC/Clang
func (l *linker) linkSharedLibraryGCC(ctx context.Context, opts sharedLibraryOptions, start time.Time) (_ *linkResult, resultErr error) {
	// Build command arguments
	var args []string

	// Add -shared flag to create shared library
	args = append(args, "-shared")

	// Add all object files
	args = append(args, opts.Objects...)

	// Add output flag
	args = append(args, "-o", opts.Output)
	importLibPath := ""
	if l.target.OS == "windows" {
		importLibPath = strings.TrimSuffix(opts.Output, filepath.Ext(opts.Output)) + ".lib"
		compiler := strings.ToLower(l.toolchain.CC())
		if l.toolchain.Name() == "gcc" || strings.Contains(compiler, "mingw") || strings.Contains(compiler, "w64") {
			args = append(args, "-Wl,--out-implib,"+importLibPath)
		} else {
			args = append(args, "-Wl,-implib:"+importLibPath)
		}
	}

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
		if flag := toolchain.SystemLibraryFlag(l.toolchain.Name(), l.target, sysLib); flag != "" {
			args = append(args, flag)
		}
	}

	// Add linker flags from BuildLinkerFlags (includes debug and raw flags)
	linkerFlags := l.toolchain.LinkerFlags(opts.Flags, []string{})
	args = append(args, linkerFlags...)
	finalArgs, cleanupPath, err := toolchain.MaybeUseGNUResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute the linker
	result, err := l.executor.RunCommand(ctx, l.linkDriver(opts.UseCXX), finalArgs...)
	if err != nil {
		return &linkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("linker failed: %w", err)
	}

	return &linkResult{
		Output:    opts.Output,
		ImportLib: importLibPath,
		Duration:  result.Duration,
		Success:   true,
	}, nil
}

func (l *linker) linkDriver(useCXX bool) string {
	if useCXX {
		return l.toolchain.CXX()
	}
	return l.toolchain.CC()
}

func (l *linker) msvcTool(name string) string {
	compiler := l.toolchain.CC()
	if filepath.IsAbs(compiler) {
		return filepath.Join(filepath.Dir(compiler), name)
	}
	return name
}

// linkSharedLibraryMSVC links a DLL using MSVC link.exe
func (l *linker) linkSharedLibraryMSVC(ctx context.Context, opts sharedLibraryOptions, start time.Time) (_ *linkResult, resultErr error) {
	// Build MSVC-style command: link.exe /nologo /DLL objects... /OUT:output.dll /IMPLIB:output.lib
	var args []string

	// Get linker flags first (includes /nologo, /DEBUG, etc.)
	linkerFlags := l.toolchain.LinkerFlags(opts.Flags, []string{})
	args = append(args, linkerFlags...)

	// Add /DLL flag to create DLL
	args = append(args, "/DLL")

	// Add all object files
	args = append(args, opts.Objects...)

	// Add output flag
	args = append(args, "/OUT:"+opts.Output)

	// Generate import library alongside DLL
	// Replace .dll extension with .lib for import library
	importLibPath := strings.TrimSuffix(opts.Output, filepath.Ext(opts.Output)) + ".lib"
	args = append(args, "/IMPLIB:"+importLibPath)

	// Add library search paths (MSVC style)
	for _, path := range opts.LibPaths {
		args = append(args, "/LIBPATH:"+path)
	}

	// Add additional libraries
	for _, lib := range opts.Libs {
		if strings.HasSuffix(lib, ".lib") {
			args = append(args, lib)
		} else {
			args = append(args, lib+".lib")
		}
	}

	// Add system libraries (translated to MSVC format)
	for _, sysLib := range opts.SysLibs {
		if flag := toolchain.SystemLibraryFlag(l.toolchain.Name(), l.target, sysLib); flag != "" {
			args = append(args, flag)
		}
	}

	// Use response file for long command lines
	finalArgs, cleanupPath, err := toolchain.MaybeUseResponseFileIn(filepath.Dir(opts.Output), args)
	if err != nil {
		return nil, fmt.Errorf("failed to create response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	// Execute link.exe
	linker := l.msvcTool("link.exe")
	result, err := l.executor.RunCommand(ctx, linker, finalArgs...)
	if err != nil {
		// Show full command line on linker errors (per CONTEXT.md)
		cmdLine := linker + " " + strings.Join(args, " ")
		return &linkResult{
			Output:   opts.Output,
			Duration: time.Since(start),
			Success:  false,
		}, fmt.Errorf("linker failed: %w\nCommand: %s", err, cmdLine)
	}

	return &linkResult{
		Output:    opts.Output,
		ImportLib: importLibPath,
		Duration:  result.Duration,
		Success:   true,
	}, nil
}
