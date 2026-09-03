package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dominikbraun/graph"
)

// ComputeBuildOrder returns targets in dependency order (dependencies first).
func ComputeBuildOrder(cfg *Config) ([]string, error) {
	g := graph.New(graph.StringHash, graph.Directed(), graph.PreventCycles())
	names := slices.Sorted(maps.Keys(cfg.Targets))
	for _, name := range names {
		if err := g.AddVertex(name); err != nil {
			return nil, fmt.Errorf("add target %q: %w", name, err)
		}
	}

	for _, name := range names {
		for _, dependency := range cfg.Targets[name].Depends {
			if _, ok := cfg.Targets[dependency]; ok {
				if err := g.AddEdge(dependency, name); err != nil {
					if errors.Is(err, graph.ErrEdgeCreatesCycle) {
						return nil, fmt.Errorf("cyclic dependency detected: %s -> %s", name, dependency)
					}
					return nil, fmt.Errorf("add dependency %s -> %s: %w", name, dependency, err)
				}
				continue
			}
			if _, ok := cfg.Dependencies[dependency]; !ok {
				return nil, fmt.Errorf("target %q depends on unknown target %q", name, dependency)
			}
		}
	}

	order, err := graph.StableTopologicalSort(g, func(a, b string) bool { return a < b })
	if err != nil {
		return nil, fmt.Errorf("compute build order: %w", err)
	}
	return order, nil
}
