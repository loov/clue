package generate

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Duncaen/go-ninja"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
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

func runtimeLibraryFlags(output string, paths []string, platform toolchain.Platform) []string {
	var flags []string
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		relative, err := filepath.Rel(filepath.Dir(output), path)
		if err != nil {
			continue
		}
		switch platform.OS {
		case "darwin":
			flags = append(flags, "-Wl,-rpath,@loader_path/"+filepath.ToSlash(relative))
		case "linux":
			flags = append(flags, "-Wl,-rpath,$$ORIGIN/"+filepath.ToSlash(relative))
		}
	}
	return flags
}

// generateTargetBuilds generates build statements for a single target within a variant
func generateTargetBuilds(ctx context.Context, file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, target config.Target, tc toolchain.Toolchain, emitSharedRules bool, targetModuleOutputs map[string]map[string]string) ([]string, error) {
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
	dependencyUsage, err := targetDependencyUsage(ctx, opts.Config, target, tc)
	if err != nil {
		return nil, fmt.Errorf("target %q: %w", target.Name, err)
	}
	target.Defines = append(target.Defines, dependencyUsage.Defines...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyUsage.CompilerFlags...)

	// Collect include paths
	includes := append(target.Includes, dependencyUsage.Includes...)

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
		compilerFlags := buildCompilerFlagsForNinja(opts.Config, target, buildCfg, includes, tc, source)
		if _, moduleAware := modules.BySource[source]; moduleAware && config.CompileStandard(opts.Config.Toolchain, target, usage, source) == "" {
			if tc.Name() == "msvc" {
				compilerFlags = append(compilerFlags, "/std:c++20")
			} else {
				compilerFlags = append(compilerFlags, "-std=c++20")
			}
		}
		if target.Type == "shared_library" && opts.Platform.OS != "windows" && tc.Name() != "msvc" {
			compilerFlags = append(compilerFlags, "-fPIC")
		}
		// Determine object path
		objPath := ninjaPathLocal(sourcePlans[source].Object)
		srcPath := ninjaPathLocal(source)

		objects = append(objects, objPath)

		// Determine rule based on source extension
		rule := "cc"
		flagKey := "cflags"
		if isCPlusPlusFile(source) {
			rule = "cxx"
			flagKey = "cxxflags"
		}

		moduleFlags := modules.Flags(source)
		for index := range moduleFlags {
			moduleFlags[index] = strings.ReplaceAll(moduleFlags[index], `\`, "/")
		}
		statement := ninja.Build{
			Rule:        rule,
			In:          []string{srcPath},
			InImplicit:  ninjaPaths(modules.Inputs(source)),
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Out:         []string{objPath},
			Vars: ninja.Vars{
				{Key: "source", Val: srcPath},
				{Key: "object", Val: objPath},
				{Key: flagKey, Val: strings.Join(append(append([]string(nil), compilerFlags...), moduleFlags...), " ")},
			},
		}
		if module, ok := modules.BySource[source]; ok {
			if output := modules.Outputs[module.Provides]; output != "" {
				ninjaOutput := ninjaPathLocal(output)
				if tc.Name() == "clang" && module.InternalPartition {
					partitionVars := append(slices.Clone(statement.Vars), ninja.Var{Key: "bmi", Val: ninjaOutput})
					*file = append(*file, ninja.Build{
						Rule: "module_partition", In: []string{srcPath}, InImplicit: statement.InImplicit,
						InOrderOnly: statement.InOrderOnly, Out: []string{ninjaOutput}, Vars: partitionVars,
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
	dependencyInputs, dependencySysLibs, sharedLibraryPaths := targetLinkDependencies(opts.Config, target, opts.BuildDir, variant, opts.Platform)
	linkInputs := append(append([]string(nil), objects...), dependencyInputs...)

	switch target.Type {
	case "executable":
		ldflags := buildLinkerFlagsForNinja(target, append(usage.SysLibs, dependencySysLibs...), buildCfg, opts.Platform, tc)
		ldflags = append(ldflags, dependencyUsage.LinkerFlags...)
		ldflags = append(ldflags, runtimeLibraryFlags(outputPath, sharedLibraryPaths, opts.Platform)...)
		rule := "link_c"
		if targetUsesCXXForNinja(opts.Config, target) {
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
		ldflags := buildSharedLibLinkerFlags(target, append(usage.SysLibs, dependencySysLibs...), buildCfg, opts.Platform, tc)
		ldflags = append(ldflags, dependencyUsage.LinkerFlags...)
		ldflags = append(ldflags, runtimeLibraryFlags(outputPath, sharedLibraryPaths, opts.Platform)...)
		rule := "link_shared_c"
		if targetUsesCXXForNinja(opts.Config, target) {
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

func targetUsesCXXForNinja(cfg *config.Config, target config.Target) bool {
	if config.TargetUsesCXX(cfg, target) {
		return true
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		if internal, ok := cfg.Targets[name]; ok {
			if sourcesUseCXX(internal.Sources) {
				return true
			}
			return slices.ContainsFunc(internal.Depends, visit)
		}
		dependency, ok := cfg.Dependencies[name]
		if !ok {
			return false
		}
		resolved, err := deps.ResolveBuildConfig(dependency, dependency.CachePath("."))
		if err != nil {
			return false
		}
		if sourcesUseCXX(resolved.Sources) {
			return true
		}
		return slices.ContainsFunc(resolved.Depends, visit)
	}
	return slices.ContainsFunc(target.Depends, visit)
}

// buildCompilerFlagsForNinja builds compiler flags for Ninja output
func buildCompilerFlagsForNinja(cfg *config.Config, target config.Target, buildCfg toolchain.Config, includes []string, tc toolchain.Toolchain, source string) []string {
	var flags []string
	msvc := tc.Name() == "msvc"

	// Language standard
	if std := config.CompileStandard(cfg.Toolchain, target, config.Usage{}, source); std != "" {
		if msvc {
			flags = append(flags, "/std:"+plan.TranslateStdForMSVC(std))
		} else {
			flags = append(flags, "-std="+std)
		}
	}

	// Include paths
	for _, inc := range includes {
		prefix := "-I"
		if msvc {
			prefix = "/I"
		}
		path := NinjaPath(inc)
		if msvc {
			path = quoteMSVCValue(path)
		}
		flags = append(flags, prefix+path)
	}
	for _, inc := range target.SystemIncludes {
		path := NinjaPath(inc)
		if msvc {
			flags = append(flags, "/external:I"+quoteMSVCValue(path))
		} else {
			flags = append(flags, "-isystem", path)
		}
	}
	if msvc {
		for _, include := range toolchainEnvironmentPaths(tc, "INCLUDE") {
			flags = append(flags, "/I"+quoteMSVCValue(NinjaPath(include)))
		}
	}

	// Defines
	for _, def := range target.Defines {
		prefix := "-D"
		if msvc {
			prefix = "/D"
		}
		flags = append(flags, prefix+def)
	}

	// Semantic flags from build package
	semanticFlags := tc.CompilerFlags(buildCfg)
	flags = append(flags, semanticFlags...)

	return flags
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

func targetLinkDependencies(cfg *config.Config, target config.Target, buildDir, variant string, platform toolchain.Platform) ([]string, []string, []string) {
	var inputs, sysLibs, sharedPaths []string
	seenTargets := map[string]bool{}
	seenSysLibs := map[string]bool{}
	for _, sysLib := range target.SysLibs {
		seenSysLibs[sysLib] = true
	}

	var visit func(string)
	visit = func(name string) {
		if seenTargets[name] {
			return
		}
		seenTargets[name] = true

		dep, ok := cfg.Targets[name]
		if !ok {
			if external, ok := cfg.Dependencies[name]; ok {
				if _, ok := external.(*deps.PkgConfigDependency); ok {
					return
				}
				targetType := dependencyTargetType(external)
				if rawOutput := dependencyOutputPath(buildDir, variant, external, platform); rawOutput != "" {
					output := ninjaPathLocal(rawOutput)
					inputs = append(inputs, linkInputPath(output, targetType, platform))
					if targetType == "shared_library" {
						sharedPaths = append(sharedPaths, filepath.Dir(output))
					}
				}
				for _, child := range dependencyDepends(external) {
					visit(child)
				}
			}
			return
		}
		if dep.Type == "static_library" || dep.Type == "shared_library" {
			output := ninjaPathLocal(outputPathForTarget(buildDir, variant, dep.Name, dep.Type, platform))
			inputs = append(inputs, linkInputPath(output, dep.Type, platform))
			if dep.Type == "shared_library" {
				sharedPaths = append(sharedPaths, filepath.Dir(output))
			}
		}
		for _, sysLib := range dep.SysLibs {
			if !seenSysLibs[sysLib] {
				seenSysLibs[sysLib] = true
				sysLibs = append(sysLibs, sysLib)
			}
		}
		for _, child := range dep.Depends {
			visit(child)
		}
	}
	for _, name := range target.Depends {
		visit(name)
	}
	return inputs, sysLibs, sharedPaths
}

// buildLinkerFlagsForNinja builds linker flags for executables
func buildLinkerFlagsForNinja(target config.Target, dependencySysLibs []string, buildCfg toolchain.Config, platform toolchain.Platform, tc toolchain.Toolchain) []string {
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
func buildSharedLibLinkerFlags(target config.Target, dependencySysLibs []string, buildCfg toolchain.Config, platform toolchain.Platform, tc toolchain.Toolchain) []string {
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
