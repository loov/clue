package generate

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Duncaen/go-ninja"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

func linkInputPath(output, targetType string, platform toolchain.Platform) string {
	if platform.OS == "windows" && targetType == "shared_library" {
		return strings.TrimSuffix(output, filepath.Ext(output)) + ".lib"
	}
	return output
}

func addImportLibraryOutput(statement *ninja.Build, output string, platform toolchain.Platform) {
	if platform.OS != "windows" {
		return
	}
	importLibrary := linkInputPath(output, "shared_library", platform)
	statement.OutImplicit = []string{importLibrary}
	statement.Vars = append(statement.Vars, ninja.Var{Key: "implib", Val: importLibrary})
}

func runtimeLibraryFlags(output string, paths []string, platform toolchain.Platform) ([]string, error) {
	flags, err := plan.RuntimeLibraryFlags(output, paths, platform)
	for index := range flags {
		flags[index] = strings.ReplaceAll(flags[index], "$", "$$")
	}
	return flags, err
}

func ninjaArtifactPaths(artifacts []plan.Artifact, platform toolchain.Platform) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		output := ninjaPathLocal(artifact.Path)
		paths = append(paths, linkInputPath(output, artifact.Type, platform))
	}
	return paths
}

// generateTargetBuilds generates build statements for a single target within a variant
func generateTargetBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, target config.Target, tc toolchain.Toolchain, emitSharedRules bool, targetModuleOutputs map[string]map[string]string, external map[string]plan.ExternalDependency) ([]string, error) {
	if target.Type == "interface_library" && len(target.HeaderUnits) == 0 {
		return nil, nil
	}
	if target.Type == "custom" {
		if emitSharedRules {
			*file = append(*file, ninja.Build{
				Rule: "custom", In: target.Inputs, InOrderOnly: targetDependencyOutputs(opts.Config, target, opts.BuildDir, variant, opts.Platform), Out: target.Outputs,
				Vars: ninja.Vars{{Key: "target", Val: target.Name}, {Key: "variant", Val: variant}, {Key: "platform", Val: opts.Platform.String()}},
			})
		}
		return target.Outputs, nil
	}
	var err error
	target, err = plan.PrepareUnityTarget(target, opts.BuildDir, variant)
	if err != nil {
		return nil, err
	}
	targetPlan := plan.ForTarget(opts.Config, target, variantConfig, opts.BuildDir, variant, opts.Platform)
	target = targetPlan.Target
	buildCfg, usage := targetPlan.Flags, targetPlan.Usage
	dependencyPlan, err := plan.ResolveDependencies(opts.Config, target, opts.BuildDir, variant, opts.Platform, external)
	if err != nil {
		return nil, fmt.Errorf("target %q: %w", target.Name, err)
	}
	target.Defines = append(target.Defines, dependencyPlan.Usage.Defines...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyPlan.Usage.CompilerFlags...)

	// Collect include paths
	includes := append(target.Includes, dependencyPlan.Usage.Includes...)

	var objects []string
	externalDependencies := externalDependencyOutputs(opts.Config, target.Depends, opts.BuildDir, variant, opts.Platform)
	buildDependencies := targetCustomOutputs(opts.Config, target)
	bmiDir := filepath.Join(opts.BuildDir, variant, target.Name, "modules")
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
	providedModules := make(map[string]string, len(headerOutputs)+len(modules.Provided))
	maps.Copy(providedModules, headerOutputs)
	maps.Copy(providedModules, modules.Provided)
	targetModuleOutputs[target.Name] = providedModules
	headerUnitBuilds := make([]string, 0, len(target.HeaderUnits))
	builtHeaderUnits, err := plan.DependencyModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	for _, unit := range target.HeaderUnits {
		name := plan.HeaderUnitName(unit.Name, unit.System)
		output := headerOutputs[name]
		arguments := plan.HeaderUnitArguments(tc, plan.HeaderUnitOptions{
			Source: unit.Path, Name: name, System: unit.System, Output: output,
			Includes: includes, SystemIncludes: target.SystemIncludes, Defines: target.Defines,
			Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
			ModuleFiles: builtHeaderUnits, ModuleMapper: modules.Mapper,
		})
		headerInputs := ninjaPaths(slices.Sorted(maps.Values(builtHeaderUnits)))
		ninjaOutput := ninjaPathLocal(output)
		statement := ninja.Build{
			Rule: "header_unit", Out: []string{ninjaOutput}, InImplicit: headerInputs,
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Vars:        ninja.Vars{{Key: "huflags", Val: ninjaResponseArguments(tc, arguments)}},
		}
		if !unit.System {
			statement.In = []string{ninjaPathLocal(unit.Path)}
		}
		*file = append(*file, statement)
		headerUnitBuilds = append(headerUnitBuilds, ninjaOutput)
		builtHeaderUnits[name] = output
	}
	sourcePlans := make(map[string]plan.Source, len(targetPlan.Sources))
	for _, source := range targetPlan.Sources {
		sourcePlans[source.Source] = source
	}

	for _, source := range modules.Sources {
		objPath := ninjaPathLocal(sourcePlans[source].Object)
		srcPath := ninjaPathLocal(source)
		objects = append(objects, objPath)
		compileIncludes := slices.Clone(includes)
		if tc.Name() == "msvc" {
			compileIncludes = append(compileIncludes, toolchainEnvironmentPaths(tc, "INCLUDE")...)
		}
		compileOpts := modules.ForSource(source, plan.CompileOptions{
			Source: source, Output: sourcePlans[source].Object, Includes: compileIncludes,
			SystemIncludes: target.SystemIncludes, Defines: target.Defines, Flags: buildCfg,
			Std: sourcePlans[source].Standard, TargetType: target.Type, Platform: opts.Platform,
			DependencyMode: plan.DependencyModeAll,
		})
		compileOpts = ninjaCompileOptions(compileOpts)
		invocation, err := plan.Compile(tc, compileOpts)
		if err != nil {
			return nil, err
		}
		rule := "cc"
		if toolchain.IsCXXSource(source) {
			rule = "cxx"
		}
		statement := ninja.Build{
			Rule:        rule,
			In:          []string{srcPath},
			InImplicit:  ninjaPaths(modules.Inputs(source)),
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Out:         []string{objPath},
			Vars:        ninja.Vars{{Key: "object", Val: objPath}, {Key: "args", Val: ninjaResponseArguments(tc, invocation.Arguments)}},
		}
		if invocation.DependencyFile != "" {
			statement.Vars = append(statement.Vars, ninja.Var{Key: "depfile", Val: invocation.DependencyFile})
		}
		if module, ok := modules.BySource[source]; ok {
			if output := modules.Outputs[module.Provides]; output != "" {
				ninjaOutput := ninjaPathLocal(output)
				if tc.Name() == "clang" && module.InternalPartition {
					partition := plan.CompileModulePartition(tc, compileOpts)
					*file = append(*file, ninja.Build{
						Rule: "module_partition", In: []string{srcPath}, InImplicit: statement.InImplicit,
						InOrderOnly: statement.InOrderOnly, Out: []string{ninjaOutput},
						Vars: ninja.Vars{{Key: "args", Val: ninjaResponseArguments(tc, partition.Arguments)}},
					})
					statement.InImplicit = append(statement.InImplicit, ninjaOutput)
				} else {
					statement.OutImplicit = []string{ninjaOutput}
				}
			}
		}
		*file = append(*file, statement)
	}

	// Link or archive
	outputPath := ninjaPathLocal(targetPlan.Output)
	dependencyInputs := ninjaArtifactPaths(dependencyPlan.Artifacts, opts.Platform)
	linkInputs := append(append([]string(nil), objects...), dependencyInputs...)

	switch target.Type {
	case "executable":
		ldflags := buildLinkerFlagsForNinja(target, append(usage.SysLibs, dependencyPlan.SystemLibraries...), buildCfg, opts.Platform, tc)
		ldflags = append(ldflags, dependencyPlan.Usage.LinkerFlags...)
		runtimeFlags, err := runtimeLibraryFlags(outputPath, dependencyPlan.SharedLibraryPaths, opts.Platform)
		if err != nil {
			return nil, err
		}
		ldflags = append(ldflags, runtimeFlags...)
		rule := "link_c"
		if dependencyPlan.UsesCXX {
			rule = "link"
		}
		*file = append(*file, ninja.Build{
			Rule: rule,
			In:   linkInputs,
			Out:  []string{outputPath},
			Vars: ninja.Vars{
				{Key: "ldflags", Val: strings.Join(ldflags, " ")},
			},
		})

	case "static_library":
		*file = append(*file, ninja.Build{
			Rule: "ar",
			In:   objects,
			Out:  []string{outputPath},
		})

	case "shared_library":
		ldflags := buildSharedLibLinkerFlags(target, append(usage.SysLibs, dependencyPlan.SystemLibraries...), buildCfg, opts.Platform, tc)
		ldflags = append(ldflags, dependencyPlan.Usage.LinkerFlags...)
		runtimeFlags, err := runtimeLibraryFlags(outputPath, dependencyPlan.SharedLibraryPaths, opts.Platform)
		if err != nil {
			return nil, err
		}
		ldflags = append(ldflags, runtimeFlags...)
		rule := "link_shared_c"
		if dependencyPlan.UsesCXX {
			rule = "link_shared"
		}
		statement := ninja.Build{
			Rule: rule,
			In:   linkInputs,
			Out:  []string{outputPath},
			Vars: ninja.Vars{
				{Key: "ldflags", Val: strings.Join(ldflags, " ")},
			},
		}
		addImportLibraryOutput(&statement, outputPath, opts.Platform)
		*file = append(*file, statement)
	}

	if target.Type == "interface_library" {
		return headerUnitBuilds, nil
	}
	return append([]string{outputPath}, headerUnitBuilds...), nil
}

func ninjaResponseArguments(tc toolchain.Toolchain, arguments []string) string {
	quoted := make([]string, len(arguments))
	quote := toolchain.QuoteGNUResponseFileArg
	if tc.Name() == "msvc" {
		quote = toolchain.QuoteResponseFileArg
	}
	for index, argument := range arguments {
		quoted[index] = strings.ReplaceAll(quote(argument), "$", "$$")
	}
	return strings.Join(quoted, " ")
}

func sourcesUseCXX(sources []string) bool {
	return slices.ContainsFunc(sources, toolchain.IsCXXSource)
}

func quoteMSVCValue(value string) string {
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func ninjaToolCommand(tc toolchain.Toolchain, tool string) string {
	command, args := toolchain.Command(tc, tool, nil)
	parts := append([]string{command}, args...)
	for i, part := range parts {
		part = strings.ReplaceAll(part, "$", "$$")
		if strings.ContainsAny(part, " \t\"") {
			part = `"` + strings.ReplaceAll(part, `"`, `\"`) + `"`
		}
		parts[i] = part
	}
	return strings.Join(parts, " ")
}

func toolchainEnvironmentPaths(tc toolchain.Toolchain, key string) []string {
	provider, ok := tc.(interface{ Environment() map[string]string })
	if !ok {
		return nil
	}
	var value string
	for name, candidate := range provider.Environment() {
		if strings.EqualFold(name, key) {
			value = candidate
			break
		}
	}
	var paths []string
	for path := range strings.SplitSeq(value, ";") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func appendMSVCLibraryPaths(flags []string, tc toolchain.Toolchain) []string {
	for _, key := range []string{"LIB", "LIBPATH"} {
		for _, path := range toolchainEnvironmentPaths(tc, key) {
			flags = append(flags, "/LIBPATH:"+quoteMSVCValue(NinjaPath(path)))
		}
	}
	return flags
}

// buildLinkerFlagsForNinja builds linker flags for executables
func buildLinkerFlagsForNinja(target config.Target, dependencySysLibs []string, buildCfg toolchain.Flags, platform toolchain.Platform, tc toolchain.Toolchain) []string {
	var flags []string
	if tc.Name() == "msvc" {
		flags = appendMSVCLibraryPaths(flags, tc)
	}

	// System libraries
	for _, sysLib := range append(target.SysLibs, dependencySysLibs...) {
		if flag := toolchain.SystemLibraryFlag(tc.Name(), platform, sysLib); flag != "" {
			flags = append(flags, flag)
		}
	}

	// Semantic linker flags
	semanticFlags := tc.LinkerFlags(buildCfg, []string{})
	flags = append(flags, semanticFlags...)

	return flags
}

// buildSharedLibLinkerFlags builds linker flags for shared libraries
func buildSharedLibLinkerFlags(target config.Target, dependencySysLibs []string, buildCfg toolchain.Flags, platform toolchain.Platform, tc toolchain.Toolchain) []string {
	var flags []string
	if tc.Name() == "msvc" {
		flags = appendMSVCLibraryPaths(flags, tc)
	}

	// Platform-specific shared library flags
	switch platform.OS {
	case "darwin":
		libName := "lib" + target.Name + ".dylib"
		flags = append(flags, "-install_name", "@rpath/"+libName)
	case "linux":
		libName := "lib" + target.Name + ".so"
		flags = append(flags, "-Wl,-soname,"+libName)
	}

	// System libraries
	for _, sysLib := range append(target.SysLibs, dependencySysLibs...) {
		if flag := toolchain.SystemLibraryFlag(tc.Name(), platform, sysLib); flag != "" {
			flags = append(flags, flag)
		}
	}

	// Semantic linker flags
	semanticFlags := tc.LinkerFlags(buildCfg, []string{})
	flags = append(flags, semanticFlags...)

	return flags
}

// ninjaPathLocal converts a path to use forward slashes (Ninja convention)
// Note: this is a local version; NinjaPath in common.go is exported for external use
func ninjaPathLocal(path string) string {
	path = NinjaPath(path)
	if len(path) > 1 && path[1] == ':' {
		path = path[:1] + "$:" + path[2:]
	}
	return path
}

func ninjaPaths(paths []string) []string {
	for index := range paths {
		paths[index] = ninjaPathLocal(paths[index])
	}
	return paths
}

func ninjaCompileOptions(opts plan.CompileOptions) plan.CompileOptions {
	opts.Source = NinjaPath(opts.Source)
	opts.Output = NinjaPath(opts.Output)
	for index, include := range opts.Includes {
		opts.Includes[index] = NinjaPath(include)
	}
	for index, include := range opts.SystemIncludes {
		opts.SystemIncludes[index] = NinjaPath(include)
	}
	if opts.ModuleOutput != "" {
		opts.ModuleOutput = NinjaPath(opts.ModuleOutput)
	}
	if opts.ModuleMapper != "" {
		opts.ModuleMapper = NinjaPath(opts.ModuleMapper)
	}
	for name, output := range opts.ModuleFiles {
		opts.ModuleFiles[name] = NinjaPath(output)
	}
	return opts
}

// outputPathForTarget returns the output path for a target
func outputPathForTarget(buildDir, variant, target, targetType string, platform toolchain.Platform) string {
	return plan.ArtifactPath(buildDir, variant, target, targetType, platform)
}

func outputNameForTarget(target, targetType string, platform toolchain.Platform) string {
	switch targetType {
	case "executable":
		return plan.ExecutableName(target, platform)
	case "static_library":
		return plan.StaticLibraryName(target, platform)
	case "shared_library":
		return plan.SharedLibraryName(target, platform)
	default:
		return target
	}
}
