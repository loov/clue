package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// CompileCommand represents a single entry in compile_commands.json
type CompileCommand struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
	Output    string   `json:"output,omitzero"`
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
func CompileCommands(ctx context.Context, opts CompDBOptions) error {
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
	tc, err := configuredToolchain(opts.Config.Toolchain, opts.Toolchain, opts.Platform)
	if err != nil {
		return err
	}
	external, err := resolveExternalDependencies(ctx, opts.Config, tc, opts.BuildDir, opts.Variant, opts.Platform)
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
	targetOrder, err := config.ComputeBuildOrder(opts.Config)
	if err != nil {
		return err
	}
	targetModuleOutputs := make(map[string]map[string]string)
	for _, name := range targetOrder {
		target := opts.Config.Targets[name]
		targetCommands, err := buildTargetCommands(workDir, opts, target, variant, tc, targetModuleOutputs, external)
		if err != nil {
			return err
		}
		commands = append(commands, targetCommands...)
	}

	// Add commands for all dependencies with build config
	dependencyNames := make([]string, 0, len(opts.Config.Dependencies))
	for name := range opts.Config.Dependencies {
		dependencyNames = append(dependencyNames, name)
	}
	slices.Sort(dependencyNames)
	for _, name := range dependencyNames {
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
func buildTargetCommands(workDir string, opts CompDBOptions, target config.Target, variant config.Variant, tc toolchain.Toolchain, targetModuleOutputs map[string]map[string]string, external map[string]plan.ExternalDependency) ([]CompileCommand, error) {
	if target.Type == "custom" || target.Type == "interface_library" && len(target.HeaderUnits) == 0 {
		return nil, nil
	}
	var commands []CompileCommand
	var err error
	target, err = plan.PrepareUnityTarget(target, opts.BuildDir, opts.Variant)
	if err != nil {
		return nil, err
	}

	targetPlan := plan.ForTarget(opts.Config, target, variant, opts.BuildDir, opts.Variant, opts.Platform)
	target = targetPlan.Target
	buildCfg, usage := targetPlan.Flags, targetPlan.Usage
	dependencyPlan, err := plan.ResolveDependencies(opts.Config, target, opts.BuildDir, opts.Variant, opts.Platform, external)
	if err != nil {
		return nil, err
	}
	target.Defines = append(target.Defines, dependencyPlan.Usage.Defines...)
	target.Includes = append(target.Includes, dependencyPlan.Usage.Includes...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyPlan.Usage.CompilerFlags...)
	bmiDir := filepath.Join(opts.BuildDir, opts.Variant, target.Name, "modules")
	availableModules, err := plan.DependencyModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	headerOutputs := plan.HeaderUnitOutputs(tc, target.HeaderUnits, bmiDir)
	for name, output := range headerOutputs {
		if _, exists := availableModules[name]; exists {
			return nil, fmt.Errorf("header unit %q is also provided by a dependency target", name)
		}
		availableModules[name] = output
	}
	modules, err := plan.ResolveModules(tc, target.Sources, bmiDir, availableModules)
	if err != nil {
		return nil, err
	}
	providedModules := modules.ProvidedModules()
	maps.Copy(providedModules, headerOutputs)
	targetModuleOutputs[target.Name] = providedModules
	builtHeaderUnits, err := plan.DependencyModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	for _, unit := range target.HeaderUnits {
		name := plan.HeaderUnitName(unit.Name, unit.System)
		headerOpts := plan.HeaderUnitOptions{
			Source: unit.Path, Name: name, System: unit.System, Output: headerOutputs[name],
			Includes: target.Includes, SystemIncludes: target.SystemIncludes, Defines: target.Defines,
			Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
			ModuleFiles: builtHeaderUnits, ModuleMapper: modules.MapperPath(),
		}
		arguments := plan.HeaderUnitArguments(tc, absoluteHeaderUnitOptions(headerOpts))
		command, wrapped := toolchain.Command(tc, tc.CXX(), arguments)
		file := unit.Path
		if !unit.System {
			file = AbsPath(file)
		}
		commands = append(commands, CompileCommand{
			Directory: workDir, File: file, Arguments: append([]string{command}, wrapped...), Output: AbsPath(headerOutputs[name]),
		})
		builtHeaderUnits[name] = headerOutputs[name]
	}
	sourcePlans := make(map[string]plan.Source, len(targetPlan.Sources))
	for _, source := range targetPlan.Sources {
		sourcePlans[source.Source] = source
	}

	for _, source := range modules.CompilationOrder() {
		sourcePlan := sourcePlans[source]
		objPath := sourcePlan.Object
		includes := slices.Clone(target.Includes)
		if tc.Name() == "msvc" {
			includes = append(includes, toolchainEnvironmentPaths(tc, "INCLUDE")...)
		}
		compileOpts := modules.ForSource(source, plan.CompileOptions{
			Source: source, Output: objPath, Includes: includes, SystemIncludes: target.SystemIncludes,
			Defines: target.Defines, Flags: buildCfg, Std: sourcePlan.Standard, TargetType: target.Type,
			Platform: opts.Platform,
		})
		compileOpts = absoluteCompileOptions(compileOpts)
		invocation, err := plan.Compile(tc, compileOpts)
		if err != nil {
			return nil, err
		}
		command, arguments := toolchain.Command(tc, invocation.Tool, invocation.Arguments)

		commands = append(commands, CompileCommand{
			Directory: workDir, File: compileOpts.Source,
			Arguments: append([]string{command}, arguments...), Output: compileOpts.Output,
		})
	}

	return commands, nil
}

func absoluteHeaderUnitOptions(opts plan.HeaderUnitOptions) plan.HeaderUnitOptions {
	if !opts.System {
		opts.Source = AbsPath(opts.Source)
	}
	opts.Output = AbsPath(opts.Output)
	includes := make([]string, len(opts.Includes))
	for index, include := range opts.Includes {
		includes[index] = AbsPath(include)
	}
	opts.Includes = includes
	systemIncludes := make([]string, len(opts.SystemIncludes))
	for index, include := range opts.SystemIncludes {
		systemIncludes[index] = AbsPath(include)
	}
	opts.SystemIncludes = systemIncludes
	moduleFiles := make(map[string]string, len(opts.ModuleFiles))
	for name, output := range opts.ModuleFiles {
		moduleFiles[name] = AbsPath(output)
	}
	opts.ModuleFiles = moduleFiles
	if opts.ModuleMapper != "" {
		opts.ModuleMapper = AbsPath(opts.ModuleMapper)
	}
	return opts
}

// buildDependencyCommands creates compile commands for a dependency's sources
func buildDependencyCommands(workDir string, opts CompDBOptions, dep deps.Dependency, variant config.Variant, tc toolchain.Toolchain) ([]CompileCommand, error) {
	var commands []CompileCommand

	// Get dependency source path
	depPath := dep.CachePath(".")
	resolved, err := deps.ResolveBuildConfig(dep, depPath)
	if err != nil {
		return nil, err
	}
	objectNames := plan.ObjectNames(resolved.Sources)
	includes := append(append([]string(nil), resolved.Includes...), deps.IncludePath(dep, depPath))

	optimization := variant.Optimization
	if optimization == "" {
		optimization = "none"
	}
	buildCfg := toolchain.Flags{
		Optimize:         optimization,
		Warnings:         "default",
		WarningsAsErrors: false, // Don't treat warnings as errors for deps
	}

	for _, source := range resolved.Sources {
		srcPath := filepath.Join(depPath, source)
		objPath := depObjectPath(opts.BuildDir, opts.Variant, dep.Name(), objectNames[source])
		compileIncludes := slices.Clone(includes)
		if tc.Name() == "msvc" {
			compileIncludes = append(compileIncludes, toolchainEnvironmentPaths(tc, "INCLUDE")...)
		}
		compileOpts := absoluteCompileOptions(plan.CompileOptions{
			Source: srcPath, Output: objPath, Includes: compileIncludes, Defines: resolved.Defines,
			Flags: buildCfg, Std: opts.Config.Toolchain.Standard(source), Platform: opts.Platform,
		})
		invocation, err := plan.Compile(tc, compileOpts)
		if err != nil {
			return nil, err
		}
		command, arguments := toolchain.Command(tc, invocation.Tool, invocation.Arguments)
		commands = append(commands, CompileCommand{
			Directory: workDir, File: compileOpts.Source,
			Arguments: append([]string{command}, arguments...), Output: compileOpts.Output,
		})
	}

	return commands, nil
}

func absoluteCompileOptions(opts plan.CompileOptions) plan.CompileOptions {
	opts.Source = AbsPath(opts.Source)
	opts.Output = AbsPath(opts.Output)
	for index, include := range opts.Includes {
		opts.Includes[index] = AbsPath(include)
	}
	for index, include := range opts.SystemIncludes {
		opts.SystemIncludes[index] = AbsPath(include)
	}
	if opts.ModuleOutput != "" {
		opts.ModuleOutput = AbsPath(opts.ModuleOutput)
	}
	if opts.ModuleMapper != "" {
		opts.ModuleMapper = AbsPath(opts.ModuleMapper)
	}
	for name, output := range opts.ModuleFiles {
		opts.ModuleFiles[name] = AbsPath(output)
	}
	return opts
}

// depObjectPath returns the object file path for a dependency source
func depObjectPath(buildDir, variant, depName, objectName string) string {
	return filepath.Join(buildDir, variant, "deps", depName, "obj", objectName)
}
