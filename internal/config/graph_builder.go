package config

import (
	"fmt"

	"github.com/loov/clue/internal/graph"
)

// BuildGraphFromConfig constructs a dependency graph from parsed configuration
func BuildGraphFromConfig(cfg *Config) (*graph.BuildGraph, error) {
	builder := graph.NewBuilder()

	// Add all targets as nodes
	for name, target := range cfg.Targets {
		nodeType := targetTypeToNodeType(target.Type)
		node := graph.Node{
			ID:   name,
			Type: nodeType,
			Path: "", // Will be set during build phase
			Metadata: map[string]any{
				"sources":  target.Sources,
				"headers":  target.Headers,
				"includes": target.Includes,
				"defines":  target.Defines,
				"flags":    target.Flags,
			},
		}
		if err := builder.AddNode(node); err != nil {
			return nil, fmt.Errorf("failed to add target %q: %w", name, err)
		}
	}

	// Add dependency edges
	for name, target := range cfg.Targets {
		for _, dep := range target.Depends {
			// Check if it's a target dependency (not an external dependency)
			if _, exists := cfg.Targets[dep]; exists {
				// Target-to-target dependency - add edge to build graph
				builder.AddDependency(name, dep)
			} else if _, exists := cfg.Dependencies[dep]; exists {
				// External dependency - skip adding to graph
				// External dependencies are handled separately in the builder
			} else {
				// Unknown dependency
				return nil, fmt.Errorf("target %q depends on unknown target %q", name, dep)
			}
		}
	}

	// Build the graph (will detect cycles)
	g, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	return g, nil
}

// targetTypeToNodeType converts config target type to graph node type
func targetTypeToNodeType(targetType string) graph.NodeType {
	switch targetType {
	case "executable":
		return graph.NodeTypeExecutable
	case "static_library":
		return graph.NodeTypeStatic
	case "shared_library":
		return graph.NodeTypeShared
	default:
		return graph.NodeTypeExecutable // Default fallback
	}
}

// GetBuildOrder returns targets in dependency order (dependencies first)
func GetBuildOrder(cfg *Config) ([]string, error) {
	g, err := BuildGraphFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return g.TopologicalOrder()
}
