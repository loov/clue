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
	sources   []string
	bySource  map[string]moduleDependency
	outputs   map[string]string
	provided  map[string]string
	inherited map[string]string
	mapper    string
}

// CompilationOrder returns sources with module providers before consumers.
func (m Modules) CompilationOrder() []string { return m.sources }

// ModuleSourceCount returns the number of sources that declare or import modules.
func (m Modules) ModuleSourceCount() int { return len(m.bySource) }

// ProvidedModules returns the BMIs produced by this target.
func (m Modules) ProvidedModules() map[string]string {
	provided := make(map[string]string, len(m.provided))
	maps.Copy(provided, m.provided)
	return provided
}

// MapperPath returns the GCC module mapper path, if one is needed.
func (m Modules) MapperPath() string { return m.mapper }

// ResolveModules scans sources and resolves their module outputs and dependencies.
func ResolveModules(tc toolchain.Toolchain, sources []string, bmiDir string, available map[string]string) (Modules, error) {
	hasModuleExtension := false
	allSourcesExist := true
	for _, source := range sources {
		hasModuleExtension = hasModuleExtension || isModuleExtension(source)
		if _, err := os.Stat(source); err != nil {
			allSourcesExist = false
		}
	}
	if !hasModuleExtension && !allSourcesExist {
		return Modules{sources: sources, outputs: available}, nil
	}
	dependencies, err := scanModuleDependencies(sources)
	if err != nil {
		return Modules{}, err
	}
	ordered, err := orderModuleCompilation(dependencies, available)
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
		sources:   ordered,
		bySource:  make(map[string]moduleDependency, len(dependencies)),
		outputs:   make(map[string]string, len(available)+len(dependencies)),
		provided:  make(map[string]string, len(dependencies)),
		inherited: make(map[string]string, len(available)),
	}
	for name, output := range available {
		modules.outputs[name] = output
		modules.inherited[name] = output
	}
	for _, dependency := range dependencies {
		modules.bySource[dependency.Source] = dependency
		if dependency.Provides != "" {
			if _, exists := modules.outputs[dependency.Provides]; exists {
				return Modules{}, fmt.Errorf("module %q is also provided by a dependency target", dependency.Provides)
			}
			output := moduleOutputPath(tc, bmiDir, dependency.Provides)
			modules.outputs[dependency.Provides] = output
			modules.provided[dependency.Provides] = output
		}
	}
	if tc.Name() == "gcc" && (len(modules.outputs) > 0 || len(dependencies) > 0) {
		modules.mapper = filepath.Join(bmiDir, "modules.mapper")
		if err := writeModuleMapper(modules.mapper, modules.outputs); err != nil {
			return Modules{}, err
		}
	}
	return modules, nil
}

// ForSource adds source-specific module inputs and outputs to opts.
func (m Modules) ForSource(source string, opts CompileOptions) CompileOptions {
	module, ok := m.bySource[source]
	if !ok {
		return opts
	}
	opts.ModuleAware = true
	opts.ModuleOutput = m.outputs[module.Provides]
	opts.ModuleName = module.Provides
	opts.ModuleMapper = m.mapper
	opts.InternalPartition = module.InternalPartition
	opts.ModuleFiles = make(map[string]string, len(m.inherited)+len(module.Requires))
	maps.Copy(opts.ModuleFiles, m.inherited)
	for _, required := range module.Requires {
		if output := m.outputs[required]; output != "" {
			opts.ModuleFiles[required] = output
		}
	}
	return opts
}

// Inputs returns the BMI files required before source can compile.
func (m Modules) Inputs(source string) []string {
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
	slices.Sort(inputs)
	return inputs
}

// HeaderUnitOutputs maps header-unit import names to their BMI output paths.
func HeaderUnitOutputs(tc toolchain.Toolchain, units []config.HeaderUnit, bmiDir string) map[string]string {
	outputs := make(map[string]string, len(units))
	for _, unit := range units {
		name := HeaderUnitName(unit.Name, unit.System)
		outputs[name] = moduleOutputPath(tc, bmiDir, name)
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
