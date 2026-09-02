// Package generate provides build file generation for external tool integration.
package generate

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	Platform   toolchain.Platform
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
	if opts.Platform.OS == "" {
		opts.Platform = toolchain.HostPlatform()
	}
	tc, err := build.NewToolchain(opts.Toolchain, opts.Platform)
	if err != nil {
		return err
	}

	// Get variant config
	variant, ok := opts.Config.Variants[opts.Variant]
	if !ok {
		// Create a default variant if not found
		variant = config.Variant{Name: opts.Variant}
	}

	var commands []CompileCommand

	// Add commands for all project targets
	for _, name := range slices.Sorted(maps.Keys(opts.Config.Targets)) {
		target := opts.Config.Targets[name]
		targetCommands, err := buildTargetCommands(workDir, opts, target, variant, tc)
		if err != nil {
			return err
		}
		commands = append(commands, targetCommands...)
	}

	// Add commands for all dependencies with build config
	for _, name := range slices.Sorted(maps.Keys(opts.Config.Dependencies)) {
		dep := opts.Config.Dependencies[name]
		if _, ok := dep.(*deps.PkgConfigDependency); ok {
			continue
		}
		depCommands, err := buildDependencyCommands(workDir, opts, dep, variant, tc)
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
func buildTargetCommands(workDir string, opts CompDBOptions, target config.Target, variant config.Variant, tc toolchain.Toolchain) ([]CompileCommand, error) {
	if target.Type == "custom" {
		return nil, nil
	}
	var commands []CompileCommand

	// Build Config from target and variant
	buildCfg := targetToBuildConfig(target, variant)
	usage := config.CompileUsage(opts.Config, target)
	dependencyUsage, err := targetDependencyUsage(opts.Config, target)
	if err != nil {
		return nil, err
	}
	target.Defines = append(append(usage.Defines, dependencyUsage.Defines...), variant.Defines...)
	target.Includes = append(usage.Includes, dependencyUsage.Includes...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyUsage.CompilerFlags...)
	objectNames := buildpath.ObjectNames(target.Sources)
	modules, err := resolveTargetModules(tc, target.Sources, build.CompileOptions{
		Includes: target.Includes,
		Defines:  target.Defines,
		Flags:    buildCfg,
		Std:      opts.Config.Toolchain.Standard("module.cppm"),
	}, filepath.Join(opts.BuildDir, opts.Variant, target.Name, "modules"))
	if err != nil {
		return nil, err
	}

	for _, source := range modules.ordered {
		// Determine object path
		objPath := objectPath(opts.BuildDir, opts.Variant, target.Name, objectNames[source])

		// Build compiler arguments
		args := buildCompilerArgs(tc, opts.Config.Toolchain.Standard(source), target.Includes, target.Defines, source, objPath, buildCfg)
		for _, flag := range modules.flags(source) {
			if value, ok := strings.CutPrefix(flag, "-fmodule-output="); ok {
				flag = "-fmodule-output=" + AbsPath(value)
			} else if value, ok := strings.CutPrefix(flag, "-fmodule-file="); ok {
				name, path, found := strings.Cut(value, "=")
				if found {
					flag = "-fmodule-file=" + name + "=" + AbsPath(path)
				}
			}
			args = append(args, flag)
		}

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
func buildDependencyCommands(workDir string, opts CompDBOptions, dep deps.Dependency, variant config.Variant, tc toolchain.Toolchain) ([]CompileCommand, error) {
	var commands []CompileCommand

	// Get dependency source path
	depPath := dep.CachePath(".")
	resolved, err := build.ResolveDepConfig(dep, depPath)
	if err != nil {
		return nil, err
	}
	objectNames := buildpath.ObjectNames(resolved.Sources)
	includes := append(append([]string(nil), resolved.Includes...), dependencyIncludePath(dep))

	optimization := variant.Optimization
	if optimization == "" {
		optimization = "none"
	}
	buildCfg := toolchain.Config{
		Optimize:         optimization,
		Warnings:         "default",
		WarningsAsErrors: false, // Don't treat warnings as errors for deps
	}

	for _, source := range resolved.Sources {
		srcPath := filepath.Join(depPath, source)
		objPath := depObjectPath(opts.BuildDir, opts.Variant, dep.Name(), objectNames[source])

		// Build arguments
		args := buildCompilerArgs(tc, opts.Config.Toolchain.Standard(source), includes, resolved.Defines, srcPath, objPath, buildCfg)

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
func buildCompilerArgs(tc toolchain.Toolchain, std string, includes, defines []string, source, objPath string, buildCfg toolchain.Config) []string {
	var args []string
	msvc := tc.Name() == "msvc"

	// 1. Compiler executable (based on file extension)
	compiler := compilerForSource(tc, source)
	args = append(args, compiler)

	// 2. Compile-only flag
	if msvc {
		args = append(args, "/c")
	} else {
		args = append(args, "-c")
	}

	// 3. Source file (absolute path)
	args = append(args, AbsPath(source))

	// 4. Output file
	if msvc {
		args = append(args, "/Fo"+AbsPath(objPath))
	} else {
		args = append(args, "-o", AbsPath(objPath))
	}

	// 5. Include paths
	for _, include := range includes {
		prefix := "-I"
		if msvc {
			prefix = "/I"
		}
		args = append(args, prefix+AbsPath(include))
	}
	if msvc {
		for _, include := range toolchainEnvironmentPaths(tc, "INCLUDE") {
			args = append(args, "/I"+include)
		}
	}

	// 6. Defines
	for _, define := range defines {
		prefix := "-D"
		if msvc {
			prefix = "/D"
		}
		args = append(args, prefix+define)
	}

	// 7. Language standard
	if std != "" {
		if msvc {
			args = append(args, "/std:"+build.TranslateStdForMSVC(std))
		} else {
			args = append(args, "-std="+std)
		}
	}

	// 8. Semantic flags (using build package for consistency)
	semanticFlags := tc.CompilerFlags(buildCfg)
	args = append(args, semanticFlags...)

	return args
}

// compilerForSource returns the appropriate compiler for a source file
func compilerForSource(tc toolchain.Toolchain, source string) string {
	if isCPlusPlusFile(source) {
		return tc.CXX()
	}
	return tc.CC()
}

// isCPlusPlusFile detects if a file is C++ based on extension
func isCPlusPlusFile(source string) bool {
	return toolchain.IsCXXSource(source)
}

// depObjectPath returns the object file path for a dependency source
func depObjectPath(buildDir, variant, depName, objectName string) string {
	return filepath.Join(buildDir, variant, "deps", depName, "obj", objectName)
}

// Note: objectPath and targetToBuildConfig are defined in common.go
