package deps

import (
	"fmt"
	"sort"

	"github.com/dominikbraun/graph"
)

// Resolver determines dependency build order using topological sort
type Resolver struct {
	dependencies map[string]Dependency
}

// NewResolver creates a new dependency resolver
func NewResolver(deps map[string]Dependency) *Resolver {
	return &Resolver{
		dependencies: deps,
	}
}

// BuildOrder returns dependencies in build order using topological sort
// Dependencies without interdependencies are returned in alphabetical order for reproducibility
func (r *Resolver) BuildOrder() ([]string, error) {
	if len(r.dependencies) == 0 {
		return []string{}, nil
	}

	// Create graph with cycle prevention
	g := graph.New(graph.StringHash, graph.PreventCycles())

	// Add vertex for each dependency
	for name := range r.dependencies {
		if err := g.AddVertex(name); err != nil {
			return nil, fmt.Errorf("failed to add dependency %q to graph: %w", name, err)
		}
	}

	// Check for interdependencies by examining each dependency's clue.cue
	// Note: For Phase 6, we don't recursively resolve transitive dependencies.
	// Dependencies can only depend on other *configured* dependencies.
	hasEdges := false
	for name, dep := range r.dependencies {
		// TODO: Parse clue.cue from dep.CachePath() to find depends field
		// For now, dependencies don't depend on each other (most common case)
		// This will be implemented when we add clue.cue parsing to dependencies
		_ = name
		_ = dep
	}

	// If no edges (no interdependencies), return alphabetical order
	if !hasEdges {
		names := make([]string, 0, len(r.dependencies))
		for name := range r.dependencies {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, nil
	}

	// Perform stable topological sort for deterministic order
	order, err := graph.StableTopologicalSort(g, func(a, b string) bool {
		return a < b // Lexical order
	})
	if err != nil {
		// Format cycle error message if cycle detected
		return nil, formatCycleError(err)
	}

	return order, nil
}

// ValidateReferences checks that every target's depends array only references valid items
func (r *Resolver) ValidateReferences(targets map[string]struct {
	Name    string
	Depends []string
}) error {
	// Get all valid target names
	validTargets := make(map[string]bool)
	for _, target := range targets {
		validTargets[target.Name] = true
	}

	// Check each target's dependencies
	for _, target := range targets {
		for _, dep := range target.Depends {
			// Check if dep references another target or a dependency
			if !validTargets[dep] && r.dependencies[dep] == nil {
				return fmt.Errorf("target %q depends on unknown %q", target.Name, dep)
			}
		}
	}

	return nil
}

// formatCycleError extracts cycle information from graph error
func formatCycleError(err error) error {
	// The graph library's error message contains cycle information
	// Format: "edge would create a cycle"
	// We enhance this with a more helpful message
	return fmt.Errorf("circular dependency detected: %w", err)
}
