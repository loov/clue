package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

type targetModules struct {
	ordered   []string
	bySource  map[string]build.ModuleDependency
	outputs   map[string]string
	provided  map[string]string
	inherited map[string]string
	mapper    string
	toolchain toolchain.Toolchain
}

func resolveTargetModules(tc toolchain.Toolchain, sources []string, opts build.CompileOptions, bmiDir string, available map[string]string) (targetModules, error) {
	hasModuleExtension := false
	allSourcesExist := true
	for _, source := range sources {
		hasModuleExtension = hasModuleExtension || build.IsModuleExtension(source)
		if _, err := os.Stat(source); err != nil {
			allSourcesExist = false
		}
	}
	if !hasModuleExtension && !allSourcesExist {
		return targetModules{ordered: sources, outputs: available, toolchain: tc}, nil
	}
	dependencies, err := build.ScanModuleDependencies(tc, sources, opts)
	if err != nil {
		return targetModules{}, err
	}
	ordered, err := build.OrderModuleCompilationWithProviders(dependencies, available)
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
		ordered:   ordered,
		bySource:  make(map[string]build.ModuleDependency, len(dependencies)),
		outputs:   make(map[string]string, len(available)+len(dependencies)),
		provided:  make(map[string]string, len(dependencies)),
		inherited: make(map[string]string, len(available)),
		toolchain: tc,
	}
	for name, output := range available {
		modules.outputs[name] = output
		modules.inherited[name] = output
	}
	for _, dependency := range dependencies {
		modules.bySource[dependency.Source] = dependency
		if dependency.Provides != "" {
			if _, exists := modules.outputs[dependency.Provides]; exists {
				return targetModules{}, fmt.Errorf("module %q is also provided by a dependency target", dependency.Provides)
			}
			output := build.ModuleOutputPathFor(tc, bmiDir, dependency.Provides)
			modules.outputs[dependency.Provides] = output
			modules.provided[dependency.Provides] = output
		}
	}
	if tc.Name() == "gcc" && (len(modules.outputs) > 0 || len(dependencies) > 0) {
		modules.mapper = filepath.Join(bmiDir, "modules.mapper")
		if err := build.WriteModuleMapper(modules.mapper, modules.outputs); err != nil {
			return targetModules{}, err
		}
	}
	return modules, nil
}

func (m targetModules) flags(source string) []string {
	module, ok := m.bySource[source]
	if !ok {
		return nil
	}
	requiredOutputs := make(map[string]string, len(m.inherited)+len(module.Requires))
	for name, output := range m.inherited {
		requiredOutputs[name] = output
	}
	for _, required := range module.Requires {
		if output := m.outputs[required]; output != "" {
			requiredOutputs[required] = output
		}
	}
	return build.ModuleCompileFlags(m.toolchain, module, m.outputs[module.Provides], requiredOutputs, m.mapper)
}

func (m targetModules) inputs(source string) []string {
	module, ok := m.bySource[source]
	if !ok {
		return nil
	}
	seen := make(map[string]bool, len(m.inherited)+len(module.Requires))
	inputs := make([]string, 0, len(m.inherited)+len(module.Requires))
	for _, output := range m.inherited {
		if !seen[output] {
			seen[output] = true
			inputs = append(inputs, output)
		}
	}
	for _, required := range module.Requires {
		if output := m.outputs[required]; output != "" && !seen[output] {
			seen[output] = true
			inputs = append(inputs, output)
		}
	}
	sort.Strings(inputs)
	return inputs
}

func headerUnitOutputs(tc toolchain.Toolchain, units []config.HeaderUnit, bmiDir string) map[string]string {
	outputs := make(map[string]string, len(units))
	for _, unit := range units {
		name := build.HeaderUnitName(unit.Name, unit.System)
		outputs[name] = build.ModuleOutputPathFor(tc, bmiDir, name)
	}
	return outputs
}

func dependencyTargetModuleOutputs(cfg *config.Config, target config.Target, targets map[string]map[string]string) (map[string]string, error) {
	outputs := make(map[string]string)
	seen := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		dependency, ok := cfg.Targets[name]
		if !ok {
			return nil
		}
		for module, output := range targets[name] {
			if previous, exists := outputs[module]; exists && previous != output {
				return fmt.Errorf("module %q is provided by multiple dependency targets", module)
			}
			outputs[module] = output
		}
		for _, child := range dependency.Depends {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, dependency := range target.Depends {
		if err := visit(dependency); err != nil {
			return nil, err
		}
	}
	return outputs, nil
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
