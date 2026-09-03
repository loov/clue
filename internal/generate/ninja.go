// Package generate provides build file generation for various build systems.
package generate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Duncaen/go-ninja"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/buildpath"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

// NinjaOptions holds options for generating build.ninja
type NinjaOptions struct {
	Config     *config.Config
	Variants   []string           // Variants to include (e.g., ["debug", "release"])
	BuildDir   string             // e.g., ".build"
	OutputPath string             // Output file path (default: build.ninja)
	Toolchain  string             // "clang" or "gcc"
	Platform   toolchain.Platform // Target platform
}

// defaultTarget is a custom Node for the ninja default statement
type defaultTarget struct {
	targets []string
}

func (d defaultTarget) WriteTo(w io.Writer) (int64, error) {
	line := "default " + strings.Join(d.targets, " ") + "\n"
	written, err := io.WriteString(w, line)
	return int64(written), err
}

func (d defaultTarget) RequiredVersion() ninja.Version {
	return ninja.Version(0)
}

// Ninja creates a build.ninja file for the project
func Ninja(opts NinjaOptions) error {
	if opts.OutputPath == "" {
		opts.OutputPath = "build.ninja"
	}
	var buf bytes.Buffer
	if err := WriteNinjaTo(&buf, opts); err != nil {
		return err
	}
	return writeIfChanged(opts.OutputPath, buf.Bytes())
}

// generateVariantBuilds generates build statements for a single variant
func generateVariantBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, targetOrder []string, tc toolchain.Toolchain, emitSharedRules bool) ([]string, error) {
	outputs, err := generateDependencyBuilds(file, opts, variant, variantConfig, tc, emitSharedRules)
	if err != nil {
		return nil, err
	}

	targetModules := make(map[string]map[string]string)
	for _, targetName := range targetOrder {
		target, exists := opts.Config.Targets[targetName]
		if !exists {
			continue
		}

		// Generate build statements for this target
		targetOutputs, err := generateTargetBuilds(file, opts, variant, variantConfig, target, tc, emitSharedRules, targetModules)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, targetOutputs...)
	}

	return outputs, nil
}

func generateDependencyBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, tc toolchain.Toolchain, emitFetchRules bool) ([]string, error) {
	names := make([]string, 0, len(opts.Config.Dependencies))
	for name := range opts.Config.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)

	var outputs []string
	for _, name := range names {
		dep := opts.Config.Dependencies[name]
		if _, ok := dep.(*deps.PkgConfigDependency); ok {
			continue
		}
		depPath := dep.CachePath(".")
		resolved, err := build.ResolveDepConfig(dep, depPath)
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", name, err)
		}
		sources := resolved.Sources
		if resolved.Type == "external_static" || resolved.Type == "external_shared" {
			output := ninjaPathLocal(dependencyOutputPath(opts.BuildDir, variant, dep, opts.Platform))
			if emitFetchRules {
				*file = append(*file, ninja.Build{
					Rule: "external_dep", Out: []string{output}, InOrderOnly: []string{"force_external"},
					Vars: ninja.Vars{{Key: "dep", Val: name}, {Key: "variant", Val: variant}, {Key: "platform", Val: opts.Platform.String()}},
				})
			}
			outputs = append(outputs, output)
			continue
		}
		if resolved.Type == "header_only" || resolved.Type == "prebuilt_static" || resolved.Type == "prebuilt_shared" {
			continue
		}
		if len(sources) == 0 {
			return nil, fmt.Errorf("dependency %q has no source files; run 'clue deps fetch' before generating Ninja", name)
		}
		dependencyUsage, err := dependencyCompileUsage(dep, opts.Config, tc)
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", name, err)
		}
		includes := append(resolved.Includes, dependencyUsage.Includes...)
		depTarget := config.Target{Name: name, Defines: append(resolved.Defines, dependencyUsage.Defines...)}
		buildCfg := toolchain.Config{Optimize: variantConfig.Optimization, Warnings: "default"}
		buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyUsage.CompilerFlags...)
		if buildCfg.Optimize == "" {
			buildCfg.Optimize = "none"
		}
		objectNames := buildpath.ObjectNames(sources)
		objects := make([]string, 0, len(sources))
		dependencyOutputs := externalDependencyOutputs(opts.Config, resolved.Depends, opts.BuildDir, variant, opts.Platform)
		sourcePaths := make([]string, 0, len(sources))
		for _, source := range sources {
			sourcePaths = append(sourcePaths, ninjaPathLocal(filepath.Join(depPath, source)))
		}
		if emitFetchRules {
			*file = append(*file, ninja.Build{
				Rule: "fetch_dep", Out: sourcePaths, InImplicit: []string{"clue.cue"},
				Vars: ninja.Vars{{Key: "dep", Val: name}},
			})
		}
		for _, source := range sources {
			compilerFlags := buildCompilerFlagsForNinja(opts.Config, depTarget, buildCfg, includes, tc, source)
			if resolved.Type == "shared_library" && tc.Name() != "msvc" {
				compilerFlags = append(compilerFlags, "-fPIC")
			}
			srcPath := filepath.Join(depPath, source)
			objPath := ninjaPathLocal(depObjectPath(opts.BuildDir, variant, name, objectNames[source]))
			rule, flagKey := "cc", "cflags"
			if isCPlusPlusFile(source) {
				rule, flagKey = "cxx", "cxxflags"
			}
			*file = append(*file, ninja.Build{
				Rule: rule, In: []string{ninjaPathLocal(srcPath)}, InOrderOnly: dependencyOutputs, Out: []string{objPath},
				Vars: ninja.Vars{
					{Key: "source", Val: ninjaPathLocal(srcPath)},
					{Key: "object", Val: objPath},
					{Key: flagKey, Val: strings.Join(compilerFlags, " ")},
				},
			})
			objects = append(objects, objPath)
		}

		output := ninjaPathLocal(dependencyOutputPath(opts.BuildDir, variant, dep, opts.Platform))
		if resolved.Type == "shared_library" {
			dependencyInputs, sharedPaths := externalDependencyLinkInputs(opts.Config, resolved.Depends, opts.BuildDir, variant, opts.Platform)
			inputs := append(objects, dependencyInputs...)
			ldflags := buildSharedLibLinkerFlags(depTarget, nil, buildCfg, opts.Platform, tc)
			ldflags = append(ldflags, dependencyUsage.LinkerFlags...)
			ldflags = append(ldflags, runtimeLibraryFlags(output, sharedPaths, opts.Platform)...)
			rule := "link_shared_c"
			if sourcesUseCXX(sources) {
				rule = "link_shared"
			}
			statement := ninja.Build{
				Rule: rule, In: inputs, Out: []string{output},
				Vars: ninja.Vars{{Key: "ldflags", Val: strings.Join(ldflags, " ")}},
			}
			addImportLibraryOutput(&statement, output, opts.Platform)
			*file = append(*file, statement)
		} else {
			*file = append(*file, ninja.Build{Rule: "ar", In: objects, Out: []string{output}})
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func dependencyTargetType(dep deps.Dependency) string {
	if _, ok := dep.(*deps.PkgConfigDependency); ok {
		return "header_only"
	}
	if buildConfig := dep.InlineBuild(); buildConfig != nil && buildConfig.Type != "" {
		switch buildConfig.Type {
		case "prebuilt_static":
			return "static_library"
		case "prebuilt_shared":
			return "shared_library"
		case "external_static":
			return "static_library"
		case "external_shared":
			return "shared_library"
		default:
			return buildConfig.Type
		}
	}
	if resolved, err := build.ResolveDepConfig(dep, dep.CachePath(".")); err == nil && resolved.Type != "" {
		return resolved.Type
	}
	return "static_library"
}

func dependencyDepends(dep deps.Dependency) []string {
	if buildConfig := dep.InlineBuild(); buildConfig != nil {
		return buildConfig.Depends
	}
	resolved, _ := build.ResolveDepConfig(dep, dep.CachePath("."))
	return resolved.Depends
}

func dependencyOutputPath(buildDir, variant string, dep deps.Dependency, platform toolchain.Platform) string {
	buildConfig := dep.InlineBuild()
	if buildConfig != nil && buildConfig.Library != "" {
		return filepath.Join(dep.CachePath("."), buildConfig.Library)
	}
	if dependencyTargetType(dep) == "header_only" {
		return ""
	}
	return filepath.Join(buildDir, variant, "deps", dep.Name(), "lib",
		outputNameForTarget(dep.Name(), dependencyTargetType(dep), platform))
}

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

func dependencyIncludePath(dep deps.Dependency) string {
	root := dep.CachePath(".")
	buildConfig := dep.InlineBuild()
	if buildConfig != nil && len(buildConfig.Headers) > 0 {
		return filepath.Dir(root)
	}
	if buildConfig != nil && len(buildConfig.Includes) > 0 {
		return filepath.Join(root, buildConfig.Includes[0])
	}
	if info, err := os.Stat(filepath.Join(root, "include")); err == nil && info.IsDir() {
		return filepath.Join(root, "include")
	}
	return root
}

func dependencyCompileUsage(dep deps.Dependency, cfg *config.Config, tc toolchain.Toolchain) (deps.Usage, error) {
	var usage deps.Usage
	seen := make(map[string]bool)
	var visit func(deps.Dependency) error
	visit = func(current deps.Dependency) error {
		if seen[current.Name()] {
			return nil
		}
		seen[current.Name()] = true
		if pkg, ok := current.(*deps.PkgConfigDependency); ok {
			resolved, err := resolvePkgConfig(context.Background(), pkg, tc)
			if err != nil {
				return err
			}
			mergeDependencyUsage(&usage, resolved)
			return nil
		}
		usage.Includes = append(usage.Includes, dependencyIncludePath(current))
		for _, name := range dependencyDepends(current) {
			if child, ok := cfg.Dependencies[name]; ok {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(dep); err != nil {
		return deps.Usage{}, err
	}
	return usage, nil
}

func targetDependencyUsage(cfg *config.Config, target config.Target, tc toolchain.Toolchain) (deps.Usage, error) {
	var usage deps.Usage
	seen := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		dep, ok := cfg.Dependencies[name]
		if !ok {
			if internal, exists := cfg.Targets[name]; exists {
				for _, child := range internal.Depends {
					if err := visit(child); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if pkg, ok := dep.(*deps.PkgConfigDependency); ok {
			resolved, err := resolvePkgConfig(context.Background(), pkg, tc)
			if err != nil {
				return err
			}
			mergeDependencyUsage(&usage, resolved)
			return nil
		}
		usage.Includes = append(usage.Includes, dependencyIncludePath(dep))
		for _, child := range dependencyDepends(dep) {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range target.Depends {
		if err := visit(name); err != nil {
			return deps.Usage{}, err
		}
	}
	return usage, nil
}

func resolvePkgConfig(ctx context.Context, pkg *deps.PkgConfigDependency, tc toolchain.Toolchain) (deps.Usage, error) {
	return pkg.ResolveWithRunner(ctx, func(ctx context.Context, name string, args ...string) (string, error) {
		return build.ToolOutput(ctx, tc, ".", name, args...)
	})
}

func mergeDependencyUsage(dst *deps.Usage, src deps.Usage) {
	dst.Includes = append(dst.Includes, src.Includes...)
	dst.Defines = append(dst.Defines, src.Defines...)
	dst.CompilerFlags = append(dst.CompilerFlags, src.CompilerFlags...)
	dst.LinkerFlags = append(dst.LinkerFlags, src.LinkerFlags...)
}

func targetCustomOutputs(cfg *config.Config, target config.Target) []string {
	var outputs []string
	for _, name := range target.Depends {
		if dependency, ok := cfg.Targets[name]; ok && dependency.Type == "custom" {
			outputs = append(outputs, dependency.Outputs...)
		}
	}
	return outputs
}

func targetDependencyOutputs(cfg *config.Config, target config.Target, buildDir, variant string, platform toolchain.Platform) []string {
	var outputs []string
	for _, name := range target.Depends {
		if dependency, ok := cfg.Targets[name]; ok {
			switch dependency.Type {
			case "custom":
				outputs = append(outputs, dependency.Outputs...)
			case "interface_library":
				outputs = append(outputs, targetDependencyOutputs(cfg, dependency, buildDir, variant, platform)...)
			default:
				outputs = append(outputs, outputPathForTarget(buildDir, variant, dependency.Name, dependency.Type, platform))
			}
			continue
		}
		if dependency, ok := cfg.Dependencies[name]; ok {
			if output := dependencyOutputPath(buildDir, variant, dependency, platform); output != "" {
				outputs = append(outputs, output)
			}
		}
	}
	return outputs
}

func externalDependencyOutputs(cfg *config.Config, names []string, buildDir, variant string, platform toolchain.Platform) []string {
	var outputs []string
	for _, name := range names {
		if dep, ok := cfg.Dependencies[name]; ok {
			if output := dependencyOutputPath(buildDir, variant, dep, platform); output != "" {
				outputs = append(outputs, ninjaPathLocal(output))
			}
		}
	}
	return outputs
}

func externalDependencyLinkInputs(cfg *config.Config, names []string, buildDir, variant string, platform toolchain.Platform) ([]string, []string) {
	var inputs, sharedPaths []string
	seen := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		dep, ok := cfg.Dependencies[name]
		if !ok {
			return
		}
		if _, ok := dep.(*deps.PkgConfigDependency); ok {
			return
		}
		targetType := dependencyTargetType(dep)
		if rawOutput := dependencyOutputPath(buildDir, variant, dep, platform); rawOutput != "" {
			output := ninjaPathLocal(rawOutput)
			inputs = append(inputs, linkInputPath(output, targetType, platform))
			if targetType == "shared_library" {
				sharedPaths = append(sharedPaths, filepath.Dir(output))
			}
		}
		for _, child := range dependencyDepends(dep) {
			visit(child)
		}
	}
	for _, name := range names {
		visit(name)
	}
	return inputs, sharedPaths
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
func generateTargetBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, target config.Target, tc toolchain.Toolchain, emitSharedRules bool, targetModuleOutputs map[string]map[string]string) ([]string, error) {
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
	plan := build.PlanTarget(opts.Config, target, variantConfig, opts.BuildDir, variant, opts.Platform)
	buildCfg, usage := plan.Flags, plan.Usage
	dependencyUsage, err := targetDependencyUsage(opts.Config, target, tc)
	if err != nil {
		return nil, fmt.Errorf("target %q: %w", target.Name, err)
	}
	target.Defines = append(append(usage.Defines, dependencyUsage.Defines...), variantConfig.Defines...)
	target.SystemIncludes = usage.SystemIncludes
	if target.CStd == "" {
		target.CStd = usage.CStd
	}
	if target.CXXStd == "" {
		target.CXXStd = usage.CXXStd
	}
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyUsage.CompilerFlags...)

	// Collect include paths
	includes := append(usage.Includes, dependencyUsage.Includes...)

	var objects []string
	externalDependencies := externalDependencyOutputs(opts.Config, target.Depends, opts.BuildDir, variant, opts.Platform)
	buildDependencies := targetCustomOutputs(opts.Config, target)
	bmiDir := filepath.Join(opts.BuildDir, variant, target.Name, "modules")
	availableModules, err := dependencyTargetModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	headerOutputs := headerUnitOutputs(tc, target.HeaderUnits, bmiDir)
	for name, output := range headerOutputs {
		if _, exists := availableModules[name]; exists {
			return nil, fmt.Errorf("header unit %q is also provided by a dependency target", name)
		}
		availableModules[name] = output
	}
	modules, err := resolveTargetModules(tc, target.Sources, build.CompileOptions{
		Includes:       includes,
		SystemIncludes: target.SystemIncludes,
		Defines:        target.Defines,
		Flags:          buildCfg,
		Std:            config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
	}, bmiDir, availableModules)
	if err != nil {
		return nil, err
	}
	providedModules := make(map[string]string, len(headerOutputs)+len(modules.provided))
	for name, output := range headerOutputs {
		providedModules[name] = output
	}
	for name, output := range modules.provided {
		providedModules[name] = output
	}
	targetModuleOutputs[target.Name] = providedModules
	headerUnitBuilds := make([]string, 0, len(target.HeaderUnits))
	builtHeaderUnits, err := dependencyTargetModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	for _, unit := range target.HeaderUnits {
		name := build.HeaderUnitName(unit.Name, unit.System)
		output := headerOutputs[name]
		arguments := build.HeaderUnitArguments(tc, build.HeaderUnitOptions{
			Source: unit.Path, Name: name, System: unit.System, Output: output,
			Includes: includes, SystemIncludes: target.SystemIncludes, Defines: target.Defines,
			Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
			ModuleFiles: builtHeaderUnits, ModuleMapper: modules.mapper,
		})
		headerInputs := make([]string, 0, len(builtHeaderUnits))
		for _, input := range builtHeaderUnits {
			headerInputs = append(headerInputs, input)
		}
		sort.Strings(headerInputs)
		statement := ninja.Build{
			Rule: "header_unit", Out: []string{output}, InImplicit: headerInputs,
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Vars:        ninja.Vars{{Key: "huflags", Val: ninjaResponseArguments(arguments)}},
		}
		if !unit.System {
			statement.In = []string{ninjaPathLocal(unit.Path)}
		}
		*file = append(*file, statement)
		headerUnitBuilds = append(headerUnitBuilds, output)
		builtHeaderUnits[name] = output
	}
	sourcePlans := make(map[string]build.SourcePlan, len(plan.Sources))
	for _, source := range plan.Sources {
		sourcePlans[source.Source] = source
	}

	for _, source := range modules.ordered {
		compilerFlags := buildCompilerFlagsForNinja(opts.Config, target, buildCfg, includes, tc, source)
		if _, moduleAware := modules.bySource[source]; moduleAware && config.CompileStandard(opts.Config.Toolchain, target, usage, source) == "" {
			if tc.Name() == "msvc" {
				compilerFlags = append(compilerFlags, "/std:c++20")
			} else {
				compilerFlags = append(compilerFlags, "-std=c++20")
			}
		}
		if target.Type == "shared_library" && tc.Name() != "msvc" {
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

		statement := ninja.Build{
			Rule:        rule,
			In:          []string{srcPath},
			InImplicit:  modules.inputs(source),
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Out:         []string{objPath},
			Vars: ninja.Vars{
				{Key: "source", Val: srcPath},
				{Key: "object", Val: objPath},
				{Key: flagKey, Val: strings.Join(append(append([]string(nil), compilerFlags...), modules.flags(source)...), " ")},
			},
		}
		if module, ok := modules.bySource[source]; ok {
			if output := modules.outputs[module.Provides]; output != "" {
				statement.OutImplicit = []string{output}
				if tc.Name() == "clang" && module.InternalPartition {
					statement.Rule = "module_partition"
					statement.Vars = append(statement.Vars, ninja.Var{Key: "bmi", Val: output})
				}
			}
		}
		*file = append(*file, statement)
	}

	// Link or archive
	outputPath := ninjaPathLocal(plan.Output)
	dependencyInputs, dependencySysLibs, sharedLibraryPaths := targetLinkDependencies(opts.Config, target, opts.BuildDir, variant, opts.Platform)
	linkInputs := append(append([]string(nil), objects...), dependencyInputs...)

	switch target.Type {
	case "executable":
		ldflags := buildLinkerFlagsForNinja(target, append(usage.SysLibs, dependencySysLibs...), buildCfg, tc)
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

func ninjaResponseArguments(arguments []string) string {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = strings.ReplaceAll(build.QuoteResponseFileArg(argument), "$", "$$")
	}
	return strings.Join(quoted, " ")
}

func sourcesUseCXX(sources []string) bool {
	for _, source := range sources {
		if toolchain.IsCXXSource(source) {
			return true
		}
	}
	return false
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
			for _, child := range internal.Depends {
				if visit(child) {
					return true
				}
			}
			return false
		}
		dependency, ok := cfg.Dependencies[name]
		if !ok {
			return false
		}
		resolved, err := build.ResolveDepConfig(dependency, dependency.CachePath("."))
		if err != nil {
			return false
		}
		if sourcesUseCXX(resolved.Sources) {
			return true
		}
		for _, child := range resolved.Depends {
			if visit(child) {
				return true
			}
		}
		return false
	}
	for _, name := range target.Depends {
		if visit(name) {
			return true
		}
	}
	return false
}

// buildCompilerFlagsForNinja builds compiler flags for Ninja output
func buildCompilerFlagsForNinja(cfg *config.Config, target config.Target, buildCfg toolchain.Config, includes []string, tc toolchain.Toolchain, source string) []string {
	var flags []string
	msvc := tc.Name() == "msvc"

	// Language standard
	if std := config.CompileStandard(cfg.Toolchain, target, config.Usage{}, source); std != "" {
		if msvc {
			flags = append(flags, "/std:"+build.TranslateStdForMSVC(std))
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
		path := ninjaPathLocal(inc)
		if msvc {
			path = quoteMSVCValue(path)
		}
		flags = append(flags, prefix+path)
	}
	for _, inc := range target.SystemIncludes {
		path := ninjaPathLocal(inc)
		if msvc {
			flags = append(flags, "/external:I"+quoteMSVCValue(path))
		} else {
			flags = append(flags, "-isystem", path)
		}
	}
	if msvc {
		for _, include := range toolchainEnvironmentPaths(tc, "INCLUDE") {
			flags = append(flags, "/I"+quoteMSVCValue(ninjaPathLocal(include)))
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
	command, args := build.ToolchainCommand(tc, tool, nil)
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
			flags = append(flags, "/LIBPATH:"+quoteMSVCValue(ninjaPathLocal(path)))
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
func buildLinkerFlagsForNinja(target config.Target, dependencySysLibs []string, buildCfg toolchain.Config, tc toolchain.Toolchain) []string {
	var flags []string
	if tc.Name() == "msvc" {
		flags = appendMSVCLibraryPaths(flags, tc)
	}

	// System libraries
	for _, sysLib := range append(target.SysLibs, dependencySysLibs...) {
		if tc.Name() == "msvc" {
			if translated := build.TranslateSysLibForMSVC(sysLib); translated != "" {
				flags = append(flags, translated)
			}
		} else {
			flags = append(flags, "-l"+sysLib)
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
		if tc.Name() == "msvc" {
			if translated := build.TranslateSysLibForMSVC(sysLib); translated != "" {
				flags = append(flags, translated)
			}
		} else {
			flags = append(flags, "-l"+sysLib)
		}
	}

	// Semantic linker flags
	semanticFlags := tc.LinkerFlags(buildCfg, []string{})
	flags = append(flags, semanticFlags...)

	return flags
}

// Helper functions

// ninjaPathLocal converts a path to use forward slashes (Ninja convention)
// Note: this is a local version; NinjaPath in common.go is exported for external use
func ninjaPathLocal(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// outputPathForTarget returns the output path for a target
func outputPathForTarget(buildDir, variant, target, targetType string, platform toolchain.Platform) string {
	return build.ArtifactPath(buildDir, variant, target, targetType, platform)
}

func outputNameForTarget(target, targetType string, platform toolchain.Platform) string {
	switch targetType {
	case "executable":
		return build.ExecutableName(target, platform)
	case "static_library":
		return build.StaticLibraryName(target, platform)
	case "shared_library":
		return build.SharedLibraryName(target, platform)
	default:
		return target
	}
}

// Note: isCPlusPlusFile is defined in compdb.go and shared between both generators

// writeIfChanged writes content to file only if it differs from existing content
func writeIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil // No change needed
	}

	return os.WriteFile(path, content, 0o644)
}

func addNinjaRules(file *ninja.File, msvc bool) {
	if msvc {
		*file = append(*file,
			ninja.Rule{
				Name: "cc", Command: `"$cc" $cflags /c "$source" /Fo"$object"`, Deps: ninja.DepsMSVC,
				MSVCDepsPrefix: "Note: including file:", Description: "CC $out",
			},
			ninja.Rule{
				Name: "cxx", Command: `"$cxx" $cxxflags /c "$source" /Fo"$object"`, Deps: ninja.DepsMSVC,
				MSVCDepsPrefix: "Note: including file:", Description: "CXX $out",
			},
			ninja.Rule{Name: "link", Command: `"$link" $in /OUT:"$out" $ldflags`, Description: "LINK $out"},
			ninja.Rule{Name: "link_shared", Command: `"$link" /DLL $in /OUT:"$out" /IMPLIB:"$implib" $ldflags`, Description: "LINK_SHARED $out"},
			ninja.Rule{Name: "ar", Command: `"$ar" /nologo /OUT:"$out" $in`, Description: "LIB $out"},
			ninja.Rule{Name: "header_unit", Command: `"$cxx" @$out.rsp`, Rspfile: "$out.rsp", RspfileContent: "$huflags", Description: "HEADER_UNIT $out"},
		)
	} else {
		*file = append(*file,
			ninja.Rule{
				Name: "cc", Command: "$cc @$object.rsp", Rspfile: "$object.rsp", RspfileContent: "-MD -MF $object.d $cflags -c $source -o $object",
				Depfile: "$object.d", Deps: ninja.DepsGCC, Description: "CC $out",
			},
			ninja.Rule{
				Name: "cxx", Command: "$cxx @$object.rsp", Rspfile: "$object.rsp", RspfileContent: "-MD -MF $object.d $cxxflags -c $source -o $object",
				Depfile: "$object.d", Deps: ninja.DepsGCC, Description: "CXX $out",
			},
			ninja.Rule{
				Name: "module_partition", Command: `$cxx @$object.rsp -x c++-module --precompile "$source" -o "$bmi" && $cxx @$object.rsp -MD -MF $object.d -c "$source" -o $object`,
				Rspfile: "$object.rsp", RspfileContent: "$cxxflags", Depfile: "$object.d", Deps: ninja.DepsGCC, Description: "CXX_MODULE_PARTITION $out",
			},
			ninja.Rule{Name: "link", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "$in -o $out $ldflags", Description: "LINK $out"},
			ninja.Rule{Name: "link_c", Command: "$cc @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "$in -o $out $ldflags", Description: "LINK $out"},
			ninja.Rule{Name: "link_shared", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "-shared $in -o $out $ldflags", Description: "LINK_SHARED $out"},
			ninja.Rule{Name: "link_shared_c", Command: "$cc @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "-shared $in -o $out $ldflags", Description: "LINK_SHARED $out"},
			ninja.Rule{Name: "ar", Command: "$ar crs $out $in", Description: "AR $out"},
			ninja.Rule{Name: "header_unit", Command: "$cxx @$out.rsp", Rspfile: "$out.rsp", RspfileContent: "$huflags", Description: "HEADER_UNIT $out"},
		)
	}
	*file = append(*file, ninja.Rule{
		Name: "fetch_dep", Command: "$clue deps fetch $dep", Description: "FETCH $dep",
	}, ninja.Rule{
		Name: "external_dep", Command: "$clue -quiet -variant $variant -target $platform deps build $dep", Description: "EXTERNAL $dep", Restat: true,
	}, ninja.Rule{
		Name: "custom", Command: "$clue -variant $variant -target $platform build $target", Description: "CUSTOM $target",
	})
}

// WriteNinjaTo writes Ninja file content to a writer (for testing)
func WriteNinjaTo(w io.Writer, opts NinjaOptions) error {
	// Set defaults
	if opts.BuildDir == "" {
		opts.BuildDir = ".build"
	}
	if opts.Toolchain == "" {
		opts.Toolchain = opts.Config.Toolchain.Compiler
	}
	if opts.Platform.OS == "" {
		opts.Platform = toolchain.HostPlatform()
	}
	if len(opts.Variants) == 0 {
		for name := range opts.Config.Variants {
			opts.Variants = append(opts.Variants, name)
		}
		if len(opts.Variants) == 0 {
			opts.Variants = []string{"debug"}
		}
		sort.Strings(opts.Variants)
	}

	// Discover toolchain
	settings := opts.Config.Toolchain
	settings.Compiler = opts.Toolchain
	toolchain, err := build.NewConfiguredToolchain(settings, opts.Platform, ".")
	if err != nil {
		return err
	}

	file := ninja.File{}

	// Header comment
	file = append(file, ninja.Comment{Lines: []string{"Generated by clue. Regenerate with: clue generate ninja"}})

	// Variables
	file = append(file, ninja.Comment{Lines: []string{"Build configuration"}})
	file = append(file, ninja.Var{Key: "builddir", Val: ".ninja_build"})
	file = append(file, ninja.Var{Key: "cc", Val: ninjaToolCommand(toolchain, toolchain.CC())})
	file = append(file, ninja.Var{Key: "cxx", Val: ninjaToolCommand(toolchain, toolchain.CXX())})
	file = append(file, ninja.Var{Key: "ar", Val: ninjaToolCommand(toolchain, toolchain.AR())})
	file = append(file, ninja.Var{Key: "clue", Val: "clue"})
	if toolchain.Name() == "msvc" {
		linker := "link.exe"
		if dir := filepath.Dir(toolchain.CC()); dir != "." {
			linker = filepath.Join(dir, linker)
		}
		file = append(file, ninja.Var{Key: "link", Val: ninjaPathLocal(linker)})
	}

	// Rules
	file = append(file, ninja.Comment{Lines: []string{"Compilation rules"}})
	addNinjaRules(&file, toolchain.Name() == "msvc")
	file = append(file, ninja.Build{Rule: "phony", Out: []string{"force_external"}})

	targetOrder, err := config.GetBuildOrder(opts.Config)
	if err != nil {
		return err
	}
	variantOutputs := make(map[string][]string)

	for variantIndex, variant := range opts.Variants {
		variantConfig, exists := opts.Config.Variants[variant]
		if !exists && len(opts.Config.Variants) > 0 {
			return fmt.Errorf("variant %q not found", variant)
		}
		file = append(file, ninja.Comment{Lines: []string{"Variant: " + variant}})
		outputs, err := generateVariantBuilds(&file, opts, variant, variantConfig, targetOrder, toolchain, variantIndex == 0)
		if err != nil {
			return err
		}
		variantOutputs[variant] = outputs
	}

	// Phony targets
	file = append(file, ninja.Comment{Lines: []string{"Phony targets"}})
	for _, variant := range opts.Variants {
		if outputs, ok := variantOutputs[variant]; ok && len(outputs) > 0 {
			file = append(file, ninja.Build{
				Rule: "phony",
				In:   outputs,
				Out:  []string{variant},
			})
		}
	}

	if len(opts.Variants) > 0 {
		file = append(file, defaultTarget{targets: []string{opts.Variants[0]}})
	}

	_, err = file.WriteTo(w)
	return err
}
