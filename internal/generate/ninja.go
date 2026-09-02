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
func generateVariantBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, targetOrder []string, tc toolchain.Toolchain) []string {
	var outputs []string

	for _, targetName := range targetOrder {
		target, exists := opts.Config.Targets[targetName]
		if !exists {
			continue
		}

		// Generate build statements for this target
		targetOutputs := generateTargetBuilds(file, opts, variant, variantConfig, target, tc)
		outputs = append(outputs, targetOutputs...)
	}

	return outputs
}

// generateTargetBuilds generates build statements for a single target within a variant
func generateTargetBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, target config.Target, _ toolchain.Toolchain) []string {
	// Build configuration for flags
	buildCfg := targetToBuildConfig(target, variantConfig)
	target.Defines = append(append([]string(nil), target.Defines...), variantConfig.Defines...)

	// Collect include paths
	includes := target.Includes

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
			Rule: rule,
			In:   []string{srcPath},
			Out:  []string{objPath},
			Vars: ninja.Vars{
				{Key: flagKey, Val: flags},
			},
		})
	}

	// Link or archive
	outputPath := ninjaPathLocal(outputPathForTarget(opts.BuildDir, variant, target.Name, target.Type, opts.Platform))
	dependencyInputs, dependencySysLibs := targetLinkDependencies(opts.Config, target, opts.BuildDir, variant, opts.Platform)
	linkInputs := append(append([]string(nil), objects...), dependencyInputs...)

	switch target.Type {
	case "executable":
		ldflags := buildLinkerFlagsForNinja(target, dependencySysLibs, buildCfg, opts.Platform, opts.Toolchain)
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

func targetLinkDependencies(cfg *config.Config, target config.Target, buildDir, variant string, platform toolchain.Platform) ([]string, []string) {
	var inputs, sysLibs []string
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
			return
		}
		if dep.Type == "static_library" || dep.Type == "shared_library" {
			inputs = append(inputs, ninjaPathLocal(outputPathForTarget(buildDir, variant, dep.Name, dep.Type, platform)))
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
	return inputs, sysLibs
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
	switch targetType {
	case "executable":
		return filepath.Join(buildDir, variant, "bin", target)
	case "static_library":
		return filepath.Join(buildDir, variant, "lib", "lib"+target+".a")
	case "shared_library":
		ext := build.SharedLibraryExtension(platform)
		return filepath.Join(buildDir, variant, "lib", "lib"+target+ext)
	default:
		return filepath.Join(buildDir, variant, "bin", target)
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

	targetOrder := getSortedTargetNames(opts.Config)
	variantOutputs := make(map[string][]string)

	for _, variant := range opts.Variants {
		variantConfig, exists := opts.Config.Variants[variant]
		if !exists && len(opts.Config.Variants) > 0 {
			return fmt.Errorf("variant %q not found", variant)
		}
		file = append(file, ninja.Comment{Lines: []string{"Variant: " + variant}})
		outputs := generateVariantBuilds(&file, opts, variant, variantConfig, targetOrder, toolchain)
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
