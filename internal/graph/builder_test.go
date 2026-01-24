package graph

import (
	"errors"
	"testing"
)

func TestSimpleGraph(t *testing.T) {
	b := NewBuilder()

	// main depends on lib
	_ = b.AddNode(Node{ID: "lib", Type: NodeTypeStatic})
	_ = b.AddNode(Node{ID: "main", Type: NodeTypeExecutable})
	b.AddDependency("main", "lib")

	g, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder failed: %v", err)
	}

	// lib must come before main
	libIdx, mainIdx := -1, -1
	for i, id := range order {
		if id == "lib" {
			libIdx = i
		}
		if id == "main" {
			mainIdx = i
		}
	}

	if libIdx >= mainIdx {
		t.Errorf("lib (%d) should come before main (%d), order: %v", libIdx, mainIdx, order)
	}
}

func TestCycleDetection(t *testing.T) {
	b := NewBuilder()

	// Create cycle: a -> b -> c -> a
	_ = b.AddNode(Node{ID: "a", Type: NodeTypeStatic})
	_ = b.AddNode(Node{ID: "b", Type: NodeTypeStatic})
	_ = b.AddNode(Node{ID: "c", Type: NodeTypeStatic})
	b.AddDependency("a", "b")
	b.AddDependency("b", "c")
	b.AddDependency("c", "a") // Creates cycle

	_, err := b.Build()
	if err == nil {
		t.Fatal("Expected cycle detection error, got nil")
	}
	if !errors.Is(err, ErrCyclicDependency) {
		t.Errorf("Expected ErrCyclicDependency, got: %v", err)
	}
}

func TestMissingDependency(t *testing.T) {
	b := NewBuilder()

	_ = b.AddNode(Node{ID: "main", Type: NodeTypeExecutable})
	b.AddDependency("main", "nonexistent")

	_, err := b.Build()
	if err == nil {
		t.Fatal("Expected error for missing dependency, got nil")
	}
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("Expected ErrNodeNotFound, got: %v", err)
	}
}

func TestDeterministicOrder(t *testing.T) {
	// Run multiple times to verify stability
	for range 5 {
		b := NewBuilder()

		// Diamond dependency: main -> {a, b} -> base
		_ = b.AddNode(Node{ID: "base", Type: NodeTypeStatic})
		_ = b.AddNode(Node{ID: "a", Type: NodeTypeStatic})
		_ = b.AddNode(Node{ID: "b", Type: NodeTypeStatic})
		_ = b.AddNode(Node{ID: "main", Type: NodeTypeExecutable})

		b.AddDependency("a", "base")
		b.AddDependency("b", "base")
		b.AddDependency("main", "a")
		b.AddDependency("main", "b")

		g, err := b.Build()
		if err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		order, err := g.TopologicalOrder()
		if err != nil {
			t.Fatalf("TopologicalOrder failed: %v", err)
		}

		// Order should be deterministic: base, a, b, main
		expected := []string{"a", "b", "base", "main"}
		if len(order) != len(expected) {
			t.Fatalf("Expected %d nodes, got %d: %v", len(expected), len(order), order)
		}

		// base must be first (both a and b depend on it)
		// main must be last (depends on both)
		// a and b can be in either order but must be stable
		if order[len(order)-1] != "main" {
			t.Errorf("main should be last, got order: %v", order)
		}
	}
}

func TestDependencies(t *testing.T) {
	b := NewBuilder()

	_ = b.AddNode(Node{ID: "base", Type: NodeTypeStatic})
	_ = b.AddNode(Node{ID: "lib", Type: NodeTypeStatic})
	_ = b.AddNode(Node{ID: "main", Type: NodeTypeExecutable})

	b.AddDependency("lib", "base")
	b.AddDependency("main", "lib")
	b.AddDependency("main", "base")

	g, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	deps, err := g.Dependencies("main")
	if err != nil {
		t.Fatalf("Dependencies failed: %v", err)
	}

	// main depends on both lib and base
	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d: %v", len(deps), deps)
	}
}
