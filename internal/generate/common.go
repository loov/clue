package generate

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/loov/clue/internal/build"
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
