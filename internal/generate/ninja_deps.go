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

func generateDependencyBuilds(ctx context.Context, file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, tc toolchain.Toolchain, emitFetchRules bool, external map[string]plan.ExternalDependency) ([]string, error) {
	names := slices.Sorted(maps.Keys(opts.Config.Dependencies))

	var outputs []string
	for _, name := range names {
		dep := opts.Config.Dependencies[name]
		if _, ok := dep.(*deps.PkgConfigDependency); ok {
			continue
		}
		depPath := dep.CachePath(".")
		resolved, err := deps.ResolveBuildConfig(dep, depPath)
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
		dependencyUsage, err := dependencyCompileUsage(ctx, dep, opts.Config, tc)
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", name, err)
		}
		includes := append(resolved.Includes, dependencyUsage.Includes...)
		depTarget := config.Target{Name: name, Defines: append(resolved.Defines, dependencyUsage.Defines...)}
		buildCfg := toolchain.Flags{Optimize: variantConfig.Optimization, Warnings: "default"}
		buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyUsage.CompilerFlags...)
		if buildCfg.Optimize == "" {
			buildCfg.Optimize = "none"
		}
		objectNames := plan.ObjectNames(sources)
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
			srcPath := filepath.Join(depPath, source)
			objPath := depObjectPath(opts.BuildDir, variant, name, objectNames[source])
			compileIncludes := slices.Clone(includes)
			if tc.Name() == "msvc" {
				compileIncludes = append(compileIncludes, toolchainEnvironmentPaths(tc, "INCLUDE")...)
			}
			compileOpts := ninjaCompileOptions(plan.CompileOptions{
				Source: srcPath, Output: objPath, Includes: compileIncludes, Defines: depTarget.Defines,
				Flags: buildCfg, Std: opts.Config.Toolchain.Standard(source), TargetType: resolved.Type,
				Platform: opts.Platform, DependencyMode: plan.DependencyModeAll,
			})
			invocation, err := plan.Compile(tc, compileOpts)
			if err != nil {
				return nil, err
			}
			rule := "cc"
			if toolchain.IsCXXSource(source) {
				rule = "cxx"
			}
			statement := ninja.Build{
				Rule: rule, In: []string{compileOpts.Source}, InOrderOnly: dependencyOutputs, Out: []string{compileOpts.Output},
				Vars: ninja.Vars{{Key: "object", Val: compileOpts.Output}, {Key: "args", Val: ninjaResponseArguments(tc, invocation.Arguments)}},
			}
			if invocation.DependencyFile != "" {
				statement.Vars = append(statement.Vars, ninja.Var{Key: "depfile", Val: invocation.DependencyFile})
			}
			*file = append(*file, statement)
			objects = append(objects, compileOpts.Output)
		}

		output := ninjaPathLocal(dependencyOutputPath(opts.BuildDir, variant, dep, opts.Platform))
		if resolved.Type == "shared_library" {
			dependencyPlan, err := plan.ResolveDependencies(opts.Config, config.Target{Depends: resolved.Depends}, opts.BuildDir, variant, opts.Platform, external)
			if err != nil {
				return nil, err
			}
			dependencyInputs := ninjaArtifactPaths(dependencyPlan.Artifacts, opts.Platform)
			inputs := append(objects, dependencyInputs...)
			ldflags := buildSharedLibLinkerFlags(depTarget, nil, buildCfg, opts.Platform, tc)
			ldflags = append(ldflags, dependencyUsage.LinkerFlags...)
			runtimeFlags, err := runtimeLibraryFlags(output, dependencyPlan.SharedLibraryPaths, opts.Platform)
			if err != nil {
				return nil, err
			}
			ldflags = append(ldflags, runtimeFlags...)
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
	if resolved, err := deps.ResolveBuildConfig(dep, dep.CachePath(".")); err == nil && resolved.Type != "" {
		return resolved.Type
	}
	return "static_library"
}

func dependencyDepends(dep deps.Dependency) []string {
	if buildConfig := dep.InlineBuild(); buildConfig != nil {
		return buildConfig.Depends
	}
	resolved, _ := deps.ResolveBuildConfig(dep, dep.CachePath("."))
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

func dependencyCompileUsage(ctx context.Context, dep deps.Dependency, cfg *config.Config, tc toolchain.Toolchain) (deps.Usage, error) {
	var usage deps.Usage
	seen := make(map[string]bool)
	var visit func(deps.Dependency) error
	visit = func(current deps.Dependency) error {
		if seen[current.Name()] {
			return nil
		}
		seen[current.Name()] = true
		if pkg, ok := current.(*deps.PkgConfigDependency); ok {
			resolved, err := resolvePkgConfig(ctx, pkg, tc)
			if err != nil {
				return err
			}
			mergeDependencyUsage(&usage, resolved)
			return nil
		}
		usage.Includes = append(usage.Includes, deps.IncludePath(current, current.CachePath(".")))
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

func resolvePkgConfig(ctx context.Context, pkg *deps.PkgConfigDependency, tc toolchain.Toolchain) (deps.Usage, error) {
	return pkg.ResolveWithRunner(ctx, func(ctx context.Context, name string, args ...string) (string, error) {
		return toolchain.Output(ctx, tc, ".", name, args...)
	})
}

func mergeDependencyUsage(dst *deps.Usage, src deps.Usage) {
	dst.Includes = append(dst.Includes, src.Includes...)
	dst.Defines = append(dst.Defines, src.Defines...)
	dst.CompilerFlags = append(dst.CompilerFlags, src.CompilerFlags...)
	dst.LinkerFlags = append(dst.LinkerFlags, src.LinkerFlags...)
}

func resolveExternalDependencies(ctx context.Context, cfg *config.Config, tc toolchain.Toolchain, buildDir, variant string, platform toolchain.Platform) (map[string]plan.ExternalDependency, error) {
	resolved := make(map[string]plan.ExternalDependency, len(cfg.Dependencies))
	var roots []string
	for _, name := range slices.Sorted(maps.Keys(cfg.Dependencies)) {
		dependency := cfg.Dependencies[name]
		if _, ok := dependency.(*deps.PkgConfigDependency); ok {
			continue
		}
		build, err := deps.ResolveBuildConfig(dependency, dependency.CachePath("."))
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", name, err)
		}
		resolved[name] = plan.ExternalDependency{
			Name: name, Type: build.Type, Output: dependencyOutputPath(buildDir, variant, dependency, platform),
			Include: deps.IncludePath(dependency, dependency.CachePath(".")), Depends: build.Depends,
			RequiresCXX: sourcesUseCXX(build.Sources),
		}
		roots = append(roots, name)
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Targets)) {
		target := cfg.Targets[name]
		roots = append(roots, target.Depends...)
	}
	seen := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		if dependency, ok := resolved[name]; ok {
			for _, child := range dependency.Depends {
				if err := visit(child); err != nil {
					return err
				}
			}
			return nil
		}
		if target, ok := cfg.Targets[name]; ok {
			for _, child := range target.Depends {
				if err := visit(child); err != nil {
					return err
				}
			}
			return nil
		}
		dependency, ok := cfg.Dependencies[name]
		if !ok {
			return nil
		}
		pkg, ok := dependency.(*deps.PkgConfigDependency)
		if !ok {
			return nil
		}
		usage, err := resolvePkgConfig(ctx, pkg, tc)
		if err != nil {
			return fmt.Errorf("dependency %q: %w", name, err)
		}
		resolved[name] = plan.ExternalDependency{Name: name, Type: "pkg_config", Usage: usage}
		return nil
	}
	for _, name := range roots {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return resolved, nil
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
