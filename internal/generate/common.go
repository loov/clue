package generate

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

type targetModules struct {
	ordered  []string
	bySource map[string]build.ModuleDependency
	outputs  map[string]string
}

func resolveTargetModules(tc toolchain.Toolchain, sources []string, opts build.CompileOptions, bmiDir string) (targetModules, error) {
	hasModuleExtension := false
	allSourcesExist := true
	for _, source := range sources {
		hasModuleExtension = hasModuleExtension || build.IsModuleExtension(source)
		if _, err := os.Stat(source); err != nil {
			allSourcesExist = false
		}
	}
	if !hasModuleExtension && !allSourcesExist {
		return targetModules{ordered: sources}, nil
	}
	dependencies, err := build.ScanModuleDependencies(tc, sources, opts)
	if err != nil {
		return targetModules{}, err
	}
	if len(dependencies) == 0 {
		return targetModules{ordered: sources}, nil
	}
	ordered, err := build.OrderModuleCompilation(dependencies)
	if err != nil {
		return targetModules{}, err
	}
	seen := make(map[string]bool, len(ordered))
	for _, source := range ordered {
		seen[source] = true
	}
	for _, source := range sources {
		if !seen[source] {
			ordered = append(ordered, source)
		}
	}
	modules := targetModules{
		ordered:  ordered,
		bySource: make(map[string]build.ModuleDependency, len(dependencies)),
		outputs:  make(map[string]string, len(dependencies)),
	}
	for _, dependency := range dependencies {
		modules.bySource[dependency.Source] = dependency
		if dependency.Provides != "" {
			modules.outputs[dependency.Provides] = build.ModuleOutputPath(bmiDir, dependency.Provides)
		}
	}
	return modules, nil
}

func (m targetModules) flags(source string) []string {
	module, ok := m.bySource[source]
	if !ok {
		return nil
	}
	var flags []string
	if output := m.outputs[module.Provides]; output != "" {
		flags = append(flags, "-fmodule-output="+output)
	}
	requires := append([]string(nil), module.Requires...)
	sort.Strings(requires)
	for _, required := range requires {
		if output := m.outputs[required]; output != "" {
			flags = append(flags, "-fmodule-file="+required+"="+output)
		}
	}
	return flags
}

func (m targetModules) inputs(source string) []string {
	module, ok := m.bySource[source]
	if !ok {
		return nil
	}
	var inputs []string
	for _, required := range module.Requires {
		if output := m.outputs[required]; output != "" {
			inputs = append(inputs, output)
		}
	}
	sort.Strings(inputs)
	return inputs
}

// AbsPath returns the absolute path, panicking on error (for generation where paths must be valid)
func AbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		// Paths should already be validated at config load time
		panic("invalid path: " + path)
	}
	return abs
}

// NinjaPath normalizes a path for Ninja files (forward slashes on all platforms)
func NinjaPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// objectPath returns the path for a named object file
// Shared between compdb.go and ninja.go generators
func objectPath(buildDir, variant, targetName, objectName string) string {
	return filepath.Join(buildDir, variant, targetName, "obj", objectName)
}

// targetToBuildConfig converts config.Target and config.Variant to toolchain.Config
// Shared between compdb.go and ninja.go generators
func targetToBuildConfig(target config.Target, variant config.Variant) toolchain.Config {
	cfg := toolchain.Config{
		Optimize:         variant.Optimization,
		Warnings:         "default",
		WarningsAsErrors: true,
		Debug:            "none",
		RawCompiler:      target.Flags.Compiler,
		RawLinker:        target.Flags.Linker,
		Sanitizers:       append([]string(nil), target.Sanitizers...),
	}
	if target.LTO != nil {
		cfg.LTO = *target.LTO
	}
	if target.PIC != nil {
		cfg.PIC = *target.PIC
	}
	if target.Coverage != nil {
		cfg.Coverage = *target.Coverage
	}

	// Apply target-specific semantic flags
	if target.Optimize != "" {
		cfg.Optimize = target.Optimize
	}
	if target.Warnings != "" {
		cfg.Warnings = target.Warnings
	}
	if target.Debug != "" {
		cfg.Debug = target.Debug
	}
	if target.WarningsAsErrors != nil {
		cfg.WarningsAsErrors = *target.WarningsAsErrors
	}

	// Apply variant debug info
	if variant.DebugInfoSet || variant.DebugInfo {
		if variant.DebugInfo {
			cfg.Debug = "full"
		} else {
			cfg.Debug = "none"
		}
	}
	if variant.Sanitizers != nil {
		cfg.Sanitizers = append([]string(nil), variant.Sanitizers...)
	}
	if variant.LTO != nil {
		cfg.LTO = *variant.LTO
	}
	if variant.PIC != nil {
		cfg.PIC = *variant.PIC
	}
	if variant.Coverage != nil {
		cfg.Coverage = *variant.Coverage
	}

	// Merge variant raw flags
	cfg.RawCompiler = append(cfg.RawCompiler, variant.Flags.Compiler...)
	cfg.RawLinker = append(cfg.RawLinker, variant.Flags.Linker...)

	return cfg
}
