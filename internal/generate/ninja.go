// Package generate provides build file generation for various build systems.
package generate

import (
	"bytes"
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
func generateVariantBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, targetOrder []string, tc toolchain.Toolchain, emitFetchRules bool) ([]string, error) {
	outputs, err := generateDependencyBuilds(file, opts, variant, variantConfig, emitFetchRules)
	if err != nil {
		return nil, err
	}

	for _, targetName := range targetOrder {
		target, exists := opts.Config.Targets[targetName]
		if !exists {
			continue
		}

		// Generate build statements for this target
		targetOutputs := generateTargetBuilds(file, opts, variant, variantConfig, target, tc)
		outputs = append(outputs, targetOutputs...)
	}

	return outputs, nil
}

func generateDependencyBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, emitFetchRules bool) ([]string, error) {
	names := make([]string, 0, len(opts.Config.Dependencies))
	for name := range opts.Config.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)

	var outputs []string
	for _, name := range names {
		dep := opts.Config.Dependencies[name]
		depPath := dep.CachePath(".")
		resolved, err := build.ResolveDepConfig(dep, depPath)
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", name, err)
		}
		sources := resolved.Sources
		if len(sources) == 0 {
			return nil, fmt.Errorf("dependency %q has no source files; run 'clue deps fetch' before generating Ninja", name)
		}
		includes := dependencyCompileIncludes(dep, opts.Config)
		includes = append(resolved.Includes, includes...)
		depTarget := config.Target{Name: name, Defines: resolved.Defines}
		buildCfg := toolchain.Config{Optimize: variantConfig.Optimization, Warnings: "default"}
		if buildCfg.Optimize == "" {
			buildCfg.Optimize = "none"
		}
		cflags := buildCompilerFlagsForNinja(opts.Config, depTarget, buildCfg, includes, false, opts.Toolchain, opts.Platform)
		cxxflags := buildCompilerFlagsForNinja(opts.Config, depTarget, buildCfg, includes, true, opts.Toolchain, opts.Platform)
		if resolved.Type == "shared_library" {
			cflags = append(cflags, "-fPIC")
			cxxflags = append(cxxflags, "-fPIC")
		}

		objectNames := buildpath.ObjectNames(sources)
		objects := make([]string, 0, len(sources))
		var dependencyOutputs []string
		if buildConfig := dep.InlineBuild(); buildConfig != nil {
			dependencyOutputs = externalDependencyOutputs(opts.Config, buildConfig.Depends, opts.BuildDir, variant, opts.Platform)
		}
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
			objPath := ninjaPathLocal(depObjectPath(opts.BuildDir, variant, name, objectNames[source]))
			rule, flagKey, flags := "cc", "cflags", cflags
			if isCPlusPlusFile(source) {
				rule, flagKey, flags = "cxx", "cxxflags", cxxflags
			}
			*file = append(*file, ninja.Build{
				Rule: rule, In: []string{ninjaPathLocal(srcPath)}, InOrderOnly: dependencyOutputs, Out: []string{objPath},
				Vars: ninja.Vars{{Key: flagKey, Val: strings.Join(flags, " ")}},
			})
			objects = append(objects, objPath)
		}

		output := ninjaPathLocal(dependencyOutputPath(opts.BuildDir, variant, dep, opts.Platform))
		if resolved.Type == "shared_library" {
			dependencyInputs, sharedPaths := externalDependencyLinkInputs(opts.Config, resolved.Depends, opts.BuildDir, variant, opts.Platform)
			inputs := append(objects, dependencyInputs...)
			ldflags := buildSharedLibLinkerFlags(depTarget, nil, buildCfg, opts.Platform, opts.Toolchain)
			ldflags = append(ldflags, runtimeLibraryFlags(output, sharedPaths, opts.Platform)...)
			*file = append(*file, ninja.Build{
				Rule: "link_shared", In: inputs, Out: []string{output},
				Vars: ninja.Vars{{Key: "ldflags", Val: strings.Join(ldflags, " ")}},
			})
		} else {
			*file = append(*file, ninja.Build{Rule: "ar", In: objects, Out: []string{output}})
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func dependencyTargetType(dep deps.Dependency) string {
	if buildConfig := dep.InlineBuild(); buildConfig != nil && buildConfig.Type != "" {
		return buildConfig.Type
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
	return filepath.Join(buildDir, variant, "deps", dep.Name(), "lib",
		outputNameForTarget(dep.Name(), dependencyTargetType(dep), platform))
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

func dependencyCompileIncludes(dep deps.Dependency, cfg *config.Config) []string {
	buildConfig := dep.InlineBuild()
	var includes []string
	includes = append(includes, dependencyIncludePath(dep))
	if buildConfig != nil {
		for _, name := range buildConfig.Depends {
			if child, ok := cfg.Dependencies[name]; ok {
				includes = append(includes, dependencyIncludePath(child))
			}
		}
	}
	return includes
}

func targetDependencyIncludes(cfg *config.Config, target config.Target) []string {
	var includes []string
	for _, name := range target.Depends {
		if dep, ok := cfg.Dependencies[name]; ok {
			includes = append(includes, dependencyIncludePath(dep))
		}
	}
	return includes
}

func externalDependencyOutputs(cfg *config.Config, names []string, buildDir, variant string, platform toolchain.Platform) []string {
	var outputs []string
	for _, name := range names {
		if dep, ok := cfg.Dependencies[name]; ok {
			outputs = append(outputs, ninjaPathLocal(dependencyOutputPath(buildDir, variant, dep, platform)))
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
		output := ninjaPathLocal(dependencyOutputPath(buildDir, variant, dep, platform))
		inputs = append(inputs, output)
		if dependencyTargetType(dep) == "shared_library" {
			sharedPaths = append(sharedPaths, filepath.Dir(output))
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
func generateTargetBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, target config.Target, _ toolchain.Toolchain) []string {
	// Build configuration for flags
	buildCfg := targetToBuildConfig(target, variantConfig)
	target.Defines = append(append([]string(nil), target.Defines...), variantConfig.Defines...)

	// Collect include paths
	includes := append(append([]string(nil), target.Includes...), targetDependencyIncludes(opts.Config, target)...)

	// Build compiler flags
	cflags := buildCompilerFlagsForNinja(opts.Config, target, buildCfg, includes, false, opts.Toolchain, opts.Platform)
	cxxflags := buildCompilerFlagsForNinja(opts.Config, target, buildCfg, includes, true, opts.Toolchain, opts.Platform)

	// Add -fPIC for shared libraries
	if target.Type == "shared_library" {
		cflags = append(cflags, "-fPIC")
		cxxflags = append(cxxflags, "-fPIC")
	}

	var objects []string
	objectNames := buildpath.ObjectNames(target.Sources)
	externalDependencies := externalDependencyOutputs(opts.Config, target.Depends, opts.BuildDir, variant, opts.Platform)

	for _, source := range target.Sources {
		// Determine object path
		objPath := ninjaPathLocal(objectPath(opts.BuildDir, variant, target.Name, objectNames[source]))
		srcPath := ninjaPathLocal(source)

		objects = append(objects, objPath)

		// Determine rule based on source extension
		rule := "cc"
		flags := strings.Join(cflags, " ")
		flagKey := "cflags"
		if isCPlusPlusFile(source) {
			rule = "cxx"
			flags = strings.Join(cxxflags, " ")
			flagKey = "cxxflags"
		}

		*file = append(*file, ninja.Build{
			Rule:        rule,
			In:          []string{srcPath},
			InOrderOnly: externalDependencies,
			Out:         []string{objPath},
			Vars: ninja.Vars{
				{Key: flagKey, Val: flags},
			},
		})
	}

	// Link or archive
	outputPath := ninjaPathLocal(outputPathForTarget(opts.BuildDir, variant, target.Name, target.Type, opts.Platform))
	dependencyInputs, dependencySysLibs, sharedLibraryPaths := targetLinkDependencies(opts.Config, target, opts.BuildDir, variant, opts.Platform)
	linkInputs := append(append([]string(nil), objects...), dependencyInputs...)

	switch target.Type {
	case "executable":
		ldflags := buildLinkerFlagsForNinja(target, dependencySysLibs, buildCfg, opts.Platform, opts.Toolchain)
		ldflags = append(ldflags, runtimeLibraryFlags(outputPath, sharedLibraryPaths, opts.Platform)...)
		*file = append(*file, ninja.Build{
			Rule: "link",
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
		ldflags := buildSharedLibLinkerFlags(target, dependencySysLibs, buildCfg, opts.Platform, opts.Toolchain)
		ldflags = append(ldflags, runtimeLibraryFlags(outputPath, sharedLibraryPaths, opts.Platform)...)
		*file = append(*file, ninja.Build{
			Rule: "link_shared",
			In:   linkInputs,
			Out:  []string{outputPath},
			Vars: ninja.Vars{
				{Key: "ldflags", Val: strings.Join(ldflags, " ")},
			},
		})
	}

	return []string{outputPath}
}

// Note: targetToBuildConfig is defined in compdb.go and shared between both generators
// Note: objectPath is defined in compdb.go and shared between both generators

// buildCompilerFlagsForNinja builds compiler flags for Ninja output
func buildCompilerFlagsForNinja(cfg *config.Config, target config.Target, buildCfg toolchain.Config, includes []string, _ bool, toolchainName string, platform toolchain.Platform) []string {
	var flags []string

	// Language standard
	if cfg.Toolchain.Std != "" {
		flags = append(flags, "-std="+cfg.Toolchain.Std)
	}

	// Include paths
	for _, inc := range includes {
		flags = append(flags, "-I"+ninjaPathLocal(inc))
	}

	// Defines
	for _, def := range target.Defines {
		flags = append(flags, "-D"+def)
	}

	// Semantic flags from build package
	tc, err := build.NewToolchain(toolchainName, platform)
	if err != nil {
		// Fallback to gcc if toolchain creation fails
		tc, _ = build.NewToolchain("gcc", platform)
	}
	semanticFlags := tc.CompilerFlags(buildCfg)
	flags = append(flags, semanticFlags...)

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
				output := ninjaPathLocal(dependencyOutputPath(buildDir, variant, external, platform))
				inputs = append(inputs, output)
				if dependencyTargetType(external) == "shared_library" {
					sharedPaths = append(sharedPaths, filepath.Dir(output))
				}
				for _, child := range dependencyDepends(external) {
					visit(child)
				}
			}
			return
		}
		if dep.Type == "static_library" || dep.Type == "shared_library" {
			output := ninjaPathLocal(outputPathForTarget(buildDir, variant, dep.Name, dep.Type, platform))
			inputs = append(inputs, output)
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
func buildLinkerFlagsForNinja(target config.Target, dependencySysLibs []string, buildCfg toolchain.Config, platform toolchain.Platform, toolchainName string) []string {
	var flags []string

	// System libraries
	for _, sysLib := range append(target.SysLibs, dependencySysLibs...) {
		flags = append(flags, "-l"+sysLib)
	}

	// Semantic linker flags
	tc, err := build.NewToolchain(toolchainName, platform)
	if err != nil {
		// Fallback to gcc if toolchain creation fails
		tc, _ = build.NewToolchain("gcc", platform)
	}
	semanticFlags := tc.LinkerFlags(buildCfg, []string{})
	flags = append(flags, semanticFlags...)

	return flags
}

// buildSharedLibLinkerFlags builds linker flags for shared libraries
func buildSharedLibLinkerFlags(target config.Target, dependencySysLibs []string, buildCfg toolchain.Config, platform toolchain.Platform, toolchainName string) []string {
	var flags []string

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
		flags = append(flags, "-l"+sysLib)
	}

	// Semantic linker flags
	tc, err := build.NewToolchain(toolchainName, platform)
	if err != nil {
		// Fallback to gcc if toolchain creation fails
		tc, _ = build.NewToolchain("gcc", platform)
	}
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
	return filepath.Join(buildDir, variant, outputDirectoryForTarget(targetType), outputNameForTarget(target, targetType, platform))
}

func outputDirectoryForTarget(targetType string) string {
	if targetType == "executable" {
		return "bin"
	}
	return "lib"
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

// getSortedTargetNames returns target names in deterministic order
func getSortedTargetNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// writeIfChanged writes content to file only if it differs from existing content
func writeIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil // No change needed
	}

	return os.WriteFile(path, content, 0o644)
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
	toolchain, err := build.NewToolchain(opts.Toolchain, opts.Platform)
	if err != nil {
		return err
	}

	file := ninja.File{}

	// Header comment
	file = append(file, ninja.Comment{Lines: []string{"Generated by clue. Regenerate with: clue generate ninja"}})

	// Variables
	file = append(file, ninja.Comment{Lines: []string{"Build configuration"}})
	file = append(file, ninja.Var{Key: "builddir", Val: ".ninja_build"})
	file = append(file, ninja.Var{Key: "cc", Val: toolchain.CC()})
	file = append(file, ninja.Var{Key: "cxx", Val: toolchain.CXX()})
	file = append(file, ninja.Var{Key: "ar", Val: toolchain.AR()})
	file = append(file, ninja.Var{Key: "clue", Val: "clue"})

	// Rules
	file = append(file, ninja.Comment{Lines: []string{"Compilation rules"}})
	file = append(file, ninja.Rule{
		Name:        "cc",
		Command:     "$cc -MD -MF $out.d $cflags -c $in -o $out",
		Depfile:     "$out.d",
		Deps:        ninja.DepsGCC,
		Description: "CC $out",
	})
	file = append(file, ninja.Rule{
		Name:        "cxx",
		Command:     "$cxx -MD -MF $out.d $cxxflags -c $in -o $out",
		Depfile:     "$out.d",
		Deps:        ninja.DepsGCC,
		Description: "CXX $out",
	})
	file = append(file, ninja.Rule{
		Name:        "link",
		Command:     "$cxx $in -o $out $ldflags",
		Description: "LINK $out",
	})
	file = append(file, ninja.Rule{
		Name:        "link_shared",
		Command:     "$cxx -shared $in -o $out $ldflags",
		Description: "LINK_SHARED $out",
	})
	file = append(file, ninja.Rule{
		Name:        "ar",
		Command:     "$ar crs $out $in",
		Description: "AR $out",
	})
	file = append(file, ninja.Rule{
		Name:        "fetch_dep",
		Command:     "$clue deps fetch $dep",
		Description: "FETCH $dep",
	})

	targetOrder := getSortedTargetNames(opts.Config)
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
