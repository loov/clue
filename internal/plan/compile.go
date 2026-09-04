package plan

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// DependencyMode controls compiler dependency-file generation.
type DependencyMode uint8

const (
	DependencyModeNone DependencyMode = iota
	DependencyModeProject
	DependencyModeAll
)

// Invocation is a tool command planned for a build adapter.
type Invocation struct {
	Tool, DependencyFile string
	Arguments            []string
}

// CompileOptions holds options for compiling a single source file.
type CompileOptions struct {
	Source            string          // Source file path
	Output            string          // Output object file path
	Includes          []string        // Include directories
	SystemIncludes    []string        // Third-party include directories
	Defines           []string        // Preprocessor defines
	Flags             toolchain.Flags // Semantic flags
	Std               string          // Language standard (e.g., "c++20", "c17")
	TargetType        string          // "executable", "static_library", "shared_library"
	Platform          toolchain.Platform
	ModuleOutput      string            // Path to output binary module interface
	ModuleFiles       map[string]string // Logical module/header-unit name to BMI path
	ModuleName        string
	ModuleMapper      string
	InternalPartition bool
	ModuleAware       bool
	DependencyMode    DependencyMode
}

// Compile returns the compiler invocation for one source file.
func Compile(tc toolchain.Toolchain, opts CompileOptions) (Invocation, error) {
	if tc.Name() == "msvc" && toolchain.IsAssemblySource(opts.Source) {
		return Invocation{}, fmt.Errorf("MSVC cannot compile GNU-style assembly source %q; use a GCC or Clang toolchain", opts.Source)
	}
	if tc.Name() == "msvc" {
		return compileMSVC(tc, opts), nil
	}
	return compileGNU(tc, opts), nil
}

// CompileModulePartition returns the Clang precompile invocation needed for an
// internal module partition.
func CompileModulePartition(tc toolchain.Toolchain, opts CompileOptions) Invocation {
	standard := compileStandard(opts)
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
	args = append(args, tc.CompilerFlags(opts.Flags)...)
	args = append(args, ModuleCompileFlags(tc, ModuleDependency{UsesModules: true}, "", opts.ModuleFiles, "")...)
	args = append(args, "-x", "c++-module", "--precompile", opts.Source, "-o", opts.ModuleOutput)
	return Invocation{Tool: tc.CXX(), Arguments: args}
}

func compileGNU(tc toolchain.Toolchain, opts CompileOptions) Invocation {
	args := []string{"-c"}
	args = append(args, compileModuleFlags(tc, opts)...)
	args = append(args, opts.Source, "-o", opts.Output)
	dependencyFile := dependencyFile(opts)
	if dependencyFile != "" {
		mode := "-MMD"
		if opts.DependencyMode == DependencyModeAll {
			mode = "-MD"
		}
		args = append(args, mode, "-MP", "-MF", dependencyFile)
	}
	platform := opts.Platform
	if platform.OS == "" {
		platform = toolchain.HostPlatform()
	}
	if opts.TargetType == "shared_library" && platform.OS != "windows" {
		args = append(args, "-fPIC")
	}
	for _, include := range opts.Includes {
		args = append(args, "-I"+include)
	}
	for _, include := range opts.SystemIncludes {
		args = append(args, "-isystem", include)
	}
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}
	if standard := compileStandard(opts); standard != "" && !toolchain.IsAssemblySource(opts.Source) {
		args = append(args, "-std="+standard)
	}
	args = append(args, tc.CompilerFlags(opts.Flags)...)
	return Invocation{Tool: compileTool(tc, opts.Source), Arguments: args, DependencyFile: dependencyFile}
}

func compileMSVC(tc toolchain.Toolchain, opts CompileOptions) Invocation {
	args := append([]string(nil), tc.CompilerFlags(opts.Flags)...)
	args = append(args, "/c", opts.Source, "/Fo"+opts.Output)
	for _, include := range opts.Includes {
		args = append(args, "/I"+include)
	}
	for _, include := range opts.SystemIncludes {
		args = append(args, "/external:I"+include)
	}
	for _, define := range opts.Defines {
		args = append(args, "/D"+define)
	}
	if standard := compileStandard(opts); standard != "" && !toolchain.IsAssemblySource(opts.Source) {
		args = append(args, "/std:"+TranslateStdForMSVC(standard))
	}
	args = append(args, compileModuleFlags(tc, opts)...)
	return Invocation{Tool: compileTool(tc, opts.Source), Arguments: args, DependencyFile: dependencyFile(opts)}
}

func compileModuleFlags(tc toolchain.Toolchain, opts CompileOptions) []string {
	return ModuleCompileFlags(tc, ModuleDependency{
		Source: opts.Source, IsModule: opts.ModuleOutput != "", Provides: opts.ModuleName,
		InternalPartition: opts.InternalPartition, UsesModules: opts.ModuleAware,
	}, opts.ModuleOutput, opts.ModuleFiles, opts.ModuleMapper)
}

func compileStandard(opts CompileOptions) string {
	if opts.Std == "" && (opts.ModuleAware || opts.ModuleOutput != "" || len(opts.ModuleFiles) > 0) {
		return "c++20"
	}
	return opts.Std
}

func compileTool(tc toolchain.Toolchain, source string) string {
	if toolchain.IsCXXSource(source) {
		return tc.CXX()
	}
	return tc.CC()
}

func dependencyFile(opts CompileOptions) string {
	if opts.DependencyMode == DependencyModeNone {
		return ""
	}
	return filepath.Join(filepath.Dir(opts.Output), strings.TrimSuffix(filepath.Base(opts.Output), filepath.Ext(opts.Output))+".d")
}
