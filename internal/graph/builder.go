package graph

import (
	"errors"
	"fmt"

	"github.com/dominikbraun/graph"
)

var (
	ErrCyclicDependency = errors.New("cyclic dependency detected")
	ErrNodeNotFound     = errors.New("node not found")
)

// BuildGraph represents the dependency graph of build targets
type BuildGraph struct {
	g graph.Graph[string, Node]
}

// Builder constructs a BuildGraph from target definitions
type Builder struct {
	nodes map[string]Node
	edges [][2]string // [from, to] pairs (from depends on to)
}

// NewBuilder creates a new graph builder
func NewBuilder() *Builder {
	return &Builder{
		nodes: make(map[string]Node),
		edges: make([][2]string, 0),
	}
}

// AddNode adds a build target node
func (b *Builder) AddNode(n Node) error {
	if _, exists := b.nodes[n.ID]; exists {
		return fmt.Errorf("duplicate node: %s", n.ID)
	}
	b.nodes[n.ID] = n
	return nil
}

// AddDependency records that 'from' depends on 'to'
func (b *Builder) AddDependency(from, to string) {
	b.edges = append(b.edges, [2]string{from, to})
}

// Build constructs the graph, detecting cycles
func (b *Builder) Build() (*BuildGraph, error) {
	// Create directed graph with cycle prevention
	g := graph.New(NodeID,
		graph.Directed(),
		graph.PreventCycles(), // Rejects edges that would create cycles
	)

	// Add all vertices first
	for _, node := range b.nodes {
		if err := g.AddVertex(node); err != nil {
			return nil, fmt.Errorf("failed to add node %s: %w", node.ID, err)
		}
	}

	// Add edges (dependencies)
	for _, edge := range b.edges {
		from, to := edge[0], edge[1]

		// Validate nodes exist
		if _, exists := b.nodes[from]; !exists {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, from)
		}
		if _, exists := b.nodes[to]; !exists {
			return nil, fmt.Errorf("%w: %s (referenced by %s)", ErrNodeNotFound, to, from)
		}

		// AddEdge direction: from -> to means "from depends on to"
		// So "to" must be built before "from"
		if err := g.AddEdge(to, from); err != nil {
			if errors.Is(err, graph.ErrEdgeCreatesCycle) {
				return nil, fmt.Errorf("%w: %s -> %s", ErrCyclicDependency, from, to)
			}
			return nil, fmt.Errorf("failed to add dependency %s -> %s: %w", from, to, err)
		}
	}

	return &BuildGraph{g: g}, nil
}

// TopologicalOrder returns nodes in build order (dependencies first)
func (bg *BuildGraph) TopologicalOrder() ([]string, error) {
	// Use stable sort for deterministic builds
	order, err := graph.StableTopologicalSort(bg.g, func(a, b string) bool {
		return a < b // Lexical ordering for stability
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compute build order: %w", err)
	}
	return order, nil
}

// GetNode retrieves a node by ID
func (bg *BuildGraph) GetNode(id string) (Node, error) {
	node, err := bg.g.Vertex(id)
	if err != nil {
		return Node{}, fmt.Errorf("%w: %s", ErrNodeNotFound, id)
	}
	return node, nil
}

// Dependencies returns the direct dependencies of a node
func (bg *BuildGraph) Dependencies(id string) ([]string, error) {
	// In our graph, edges point from dependency to dependent
	// So predecessors of 'id' are its dependencies
	adj, err := bg.g.AdjacencyMap()
	if err != nil {
		return nil, err
	}

	var deps []string
	for source, targets := range adj {
		for target := range targets {
			if target == id {
				deps = append(deps, source)
			}
		}
	}
	return deps, nil
}
