// Package generate provides build file generation for external tool integration.
package generate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/buildpath"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

// CompileCommand represents a single entry in compile_commands.json
type CompileCommand struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
	Output    string   `json:"output,omitempty"`
}

// CompDBOptions holds options for generating compile_commands.json
type CompDBOptions struct {
	Config     *config.Config
	Variant    string // "debug", "release", etc.
	BuildDir   string // e.g., ".build"
	OutputPath string // Output file path (default: compile_commands.json)
	Toolchain  string // "clang" or "gcc"
}

// CompileCommands creates a compile_commands.json file
func CompileCommands(opts CompDBOptions) error {
	// Get working directory with absolute path
	workDir, err := filepath.Abs(".")
	if err != nil {
		return err
	}

	// Set defaults
	if opts.BuildDir == "" {
		opts.BuildDir = opts.Config.BuildDir
	}
	if opts.OutputPath == "" {
		opts.OutputPath = "compile_commands.json"
	}
	if opts.Toolchain == "" {
		opts.Toolchain = opts.Config.Toolchain.Compiler
	}
	if opts.Variant == "" {
		opts.Variant = "debug"
	}

	// Get variant config
	variant, ok := opts.Config.Variants[opts.Variant]
	if !ok {
		// Create a default variant if not found
		variant = config.Variant{Name: opts.Variant}
	}

	var commands []CompileCommand

	// Add commands for all project targets
	for _, target := range opts.Config.Targets {
		targetCommands, err := buildTargetCommands(workDir, opts, target, variant)
		if err != nil {
			return err
		}
		commands = append(commands, targetCommands...)
	}

	// Add commands for all dependencies with build config
	for _, dep := range opts.Config.Dependencies {
		depCommands, err := buildDependencyCommands(workDir, opts, dep)
		if err != nil {
			return err
		}
		commands = append(commands, depCommands...)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(commands, "", "  ")
	if err != nil {
		return err
	}

	// Write to output file
	return os.WriteFile(opts.OutputPath, data, 0o644)
}

// buildTargetCommands creates compile commands for a target's sources
func buildTargetCommands(workDir string, opts CompDBOptions, target config.Target, variant config.Variant) ([]CompileCommand, error) {
	var commands []CompileCommand

	// Build Config from target and variant
	buildCfg := targetToBuildConfig(target, variant)
	objectNames := buildpath.ObjectNames(target.Sources)

	for _, source := range target.Sources {
		// Determine object path
		objPath := objectPath(opts.BuildDir, opts.Variant, target.Name, objectNames[source])

		// Build compiler arguments
		args := buildCompilerArgs(opts, target, source, objPath, buildCfg)

		// Make paths absolute for IDE compatibility
		srcAbs := AbsPath(source)
		objAbs := AbsPath(objPath)

		cmd := CompileCommand{
			Directory: workDir,
			File:      srcAbs,
			Arguments: args,
			Output:    objAbs,
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

// buildDependencyCommands creates compile commands for a dependency's sources
func buildDependencyCommands(workDir string, opts CompDBOptions, dep deps.Dependency) ([]CompileCommand, error) {
	// Get inline build config from the dependency
	var buildConfig *deps.InlineConfig
	switch d := dep.(type) {
	case *deps.GitDependency:
		buildConfig = d.BuildConfig
	case *deps.TarballDependency:
		buildConfig = d.BuildConfig
	case *deps.VendoredDependency:
		buildConfig = d.BuildConfig
	}

	// Skip dependencies without build config
	if buildConfig == nil {
		return nil, nil
	}

	var commands []CompileCommand
	objectNames := buildpath.ObjectNames(buildConfig.Sources)

	// Get dependency source path
	depPath := dep.CachePath(".")

	// Determine include path (per include path auto-detection pattern)
	includePath := depPath
	if len(buildConfig.Includes) > 0 {
		includePath = filepath.Join(depPath, buildConfig.Includes[0])
	} else if info, err := os.Stat(filepath.Join(depPath, "include")); err == nil && info.IsDir() {
		includePath = filepath.Join(depPath, "include")
	}

	// Build config for dependency (minimal defaults)
	buildCfg := toolchain.Config{
		Optimize:         "none",
		Warnings:         "default",
		WarningsAsErrors: false, // Don't treat warnings as errors for deps
	}

	for _, source := range buildConfig.Sources {
		srcPath := filepath.Join(depPath, source)
		objPath := depObjectPath(opts.BuildDir, opts.Variant, dep.Name(), objectNames[source])

		// Build arguments
		args := buildDepCompilerArgs(opts, buildConfig, includePath, srcPath, objPath, buildCfg)

		// Make paths absolute
		srcAbs := AbsPath(srcPath)
		objAbs := AbsPath(objPath)

		cmd := CompileCommand{
			Directory: workDir,
			File:      srcAbs,
			Arguments: args,
			Output:    objAbs,
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

// buildCompilerArgs constructs the full compiler command arguments
func buildCompilerArgs(opts CompDBOptions, target config.Target, source, objPath string, buildCfg toolchain.Config) []string {
	var args []string

	// 1. Compiler executable (based on file extension)
	compiler := compilerForSource(opts.Toolchain, source)
	args = append(args, compiler)

	// 2. Compile-only flag
	args = append(args, "-c")

	// 3. Source file (absolute path)
	args = append(args, AbsPath(source))

	// 4. Output file
	args = append(args, "-o", AbsPath(objPath))

	// 5. Include paths
	for _, include := range target.Includes {
		args = append(args, "-I"+AbsPath(include))
	}

	// 6. Defines
	for _, define := range target.Defines {
		args = append(args, "-D"+define)
	}

	// 7. Language standard
	if opts.Config.Toolchain.Std != "" {
		args = append(args, "-std="+opts.Config.Toolchain.Std)
	}

	// 8. Semantic flags (using build package for consistency)
	tc, err := build.NewToolchain(opts.Toolchain, toolchain.HostPlatform())
	if err != nil {
		// Fallback to gcc if toolchain creation fails
		tc, _ = build.NewToolchain("gcc", toolchain.HostPlatform())
	}
	semanticFlags := tc.CompilerFlags(buildCfg)
	args = append(args, semanticFlags...)

	return args
}

// buildDepCompilerArgs constructs compiler arguments for a dependency source
func buildDepCompilerArgs(opts CompDBOptions, buildConfig *deps.InlineConfig, includePath, source, objPath string, buildCfg toolchain.Config) []string {
	var args []string

	// 1. Compiler executable
	compiler := compilerForSource(opts.Toolchain, source)
	args = append(args, compiler)

	// 2. Compile-only flag
	args = append(args, "-c")

	// 3. Source file
	args = append(args, AbsPath(source))

	// 4. Output file
	args = append(args, "-o", AbsPath(objPath))

	// 5. Include paths
	args = append(args, "-I"+AbsPath(includePath))
	for _, include := range buildConfig.Includes {
		args = append(args, "-I"+AbsPath(include))
	}

	// 6. Defines
	for _, define := range buildConfig.Defines {
		args = append(args, "-D"+define)
	}

	// 7. Language standard
	if opts.Config.Toolchain.Std != "" {
		args = append(args, "-std="+opts.Config.Toolchain.Std)
	}

	// 8. Semantic flags
	tc, err := build.NewToolchain(opts.Toolchain, toolchain.HostPlatform())
	if err != nil {
		// Fallback to gcc if toolchain creation fails
		tc, _ = build.NewToolchain("gcc", toolchain.HostPlatform())
	}
	semanticFlags := tc.CompilerFlags(buildCfg)
	args = append(args, semanticFlags...)

	return args
}

// compilerForSource returns the appropriate compiler for a source file
func compilerForSource(toolchain, source string) string {
	isCPP := isCPlusPlusFile(source)
	if toolchain == "gcc" {
		if isCPP {
			return "g++"
		}
		return "gcc"
	}
	// Default to clang
	if isCPP {
		return "clang++"
	}
	return "clang"
}

// isCPlusPlusFile detects if a file is C++ based on extension
func isCPlusPlusFile(source string) bool {
	ext := strings.ToLower(filepath.Ext(source))
	switch ext {
	case ".cpp", ".cc", ".cxx", ".c++":
		return true
	}
	// Handle case-sensitive extensions
	rawExt := filepath.Ext(source)
	switch rawExt {
	case ".C", ".CPP":
		return true
	}
	return false
}

// depObjectPath returns the object file path for a dependency source
func depObjectPath(buildDir, variant, depName, objectName string) string {
	return filepath.Join(buildDir, variant, "deps", depName, "obj", objectName)
}

// Note: objectPath and targetToBuildConfig are defined in common.go
