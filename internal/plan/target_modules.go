package plan

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

// Modules describes module compilation order and BMI relationships for a target.
type Modules struct {
	Sources   []string
	BySource  map[string]ModuleDependency
	Outputs   map[string]string
	Provided  map[string]string
	Inherited map[string]string
	Mapper    string
	Toolchain toolchain.Toolchain
}

// ResolveModules scans sources and resolves their module outputs and dependencies.
func ResolveModules(tc toolchain.Toolchain, sources []string, bmiDir string, available map[string]string) (Modules, error) {
	hasModuleExtension := false
	allSourcesExist := true
	for _, source := range sources {
		hasModuleExtension = hasModuleExtension || IsModuleExtension(source)
		if _, err := os.Stat(source); err != nil {
			allSourcesExist = false
		}
	}
	if !hasModuleExtension && !allSourcesExist {
		return Modules{Sources: sources, Outputs: available, Toolchain: tc}, nil
	}
	dependencies, err := ScanModuleDependencies(sources)
	if err != nil {
		return Modules{}, err
	}
	ordered, err := OrderModuleCompilationWithProviders(dependencies, available)
	if err != nil {
		return Modules{}, err
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
	modules := Modules{
		Sources:   ordered,
		BySource:  make(map[string]ModuleDependency, len(dependencies)),
		Outputs:   make(map[string]string, len(available)+len(dependencies)),
		Provided:  make(map[string]string, len(dependencies)),
		Inherited: make(map[string]string, len(available)),
		Toolchain: tc,
	}
	for name, output := range available {
		modules.Outputs[name] = output
		modules.Inherited[name] = output
	}
	for _, dependency := range dependencies {
		modules.BySource[dependency.Source] = dependency
		if dependency.Provides != "" {
			if _, exists := modules.Outputs[dependency.Provides]; exists {
				return Modules{}, fmt.Errorf("module %q is also provided by a dependency target", dependency.Provides)
			}
			output := ModuleOutputPathFor(tc, bmiDir, dependency.Provides)
			modules.Outputs[dependency.Provides] = output
			modules.Provided[dependency.Provides] = output
		}
	}
	if tc.Name() == "gcc" && (len(modules.Outputs) > 0 || len(dependencies) > 0) {
		modules.Mapper = filepath.Join(bmiDir, "modules.mapper")
		if err := WriteModuleMapper(modules.Mapper, modules.Outputs); err != nil {
			return Modules{}, err
		}
	}
	return modules, nil
}

// Flags returns the compiler module flags for source.
func (m Modules) Flags(source string) []string {
	module, ok := m.BySource[source]
	if !ok {
		return nil
	}
	requiredOutputs := make(map[string]string, len(m.Inherited)+len(module.Requires))
	maps.Copy(requiredOutputs, m.Inherited)
	for _, required := range module.Requires {
		if output := m.Outputs[required]; output != "" {
			requiredOutputs[required] = output
		}
	}
	return ModuleCompileFlags(m.Toolchain, module, m.Outputs[module.Provides], requiredOutputs, m.Mapper)
}

// Inputs returns the BMI files required before source can compile.
func (m Modules) Inputs(source string) []string {
	module, ok := m.BySource[source]
	if !ok {
		return nil
	}
	seen := make(map[string]bool, len(m.Inherited)+len(module.Requires))
	inputs := make([]string, 0, len(m.Inherited)+len(module.Requires))
	for _, output := range m.Inherited {
		if !seen[output] {
			seen[output] = true
			inputs = append(inputs, output)
		}
	}
	for _, required := range module.Requires {
		if output := m.Outputs[required]; output != "" && !seen[output] {
			seen[output] = true
			inputs = append(inputs, output)
		}
	}
	slices.Sort(inputs)
	return inputs
}

// HeaderUnitOutputs maps header-unit import names to their BMI output paths.
func HeaderUnitOutputs(tc toolchain.Toolchain, units []config.HeaderUnit, bmiDir string) map[string]string {
	outputs := make(map[string]string, len(units))
	for _, unit := range units {
		name := HeaderUnitName(unit.Name, unit.System)
		outputs[name] = ModuleOutputPathFor(tc, bmiDir, name)
	}
	return outputs
}

// DependencyModuleOutputs collects BMIs exported by a target's dependency graph.
func DependencyModuleOutputs(cfg *config.Config, target config.Target, targets map[string]map[string]string) (map[string]string, error) {
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
