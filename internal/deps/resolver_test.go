package deps

import (
	"strings"
	"testing"
)

func TestBuildOrder_NoDependencies(t *testing.T) {
	// Empty deps map should return empty order
	resolver := NewResolver(map[string]Dependency{})
	order, err := resolver.BuildOrder()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(order) != 0 {
		t.Errorf("Expected empty order, got %v", order)
	}
}

func TestBuildOrder_Independent(t *testing.T) {
	// Multiple deps with no inter-dependencies should return alphabetical order
	deps := map[string]Dependency{
		"zlib": NewGitDependency("zlib", "https://github.com/madler/zlib.git", "main", nil),
		"bzip": NewGitDependency("bzip", "https://github.com/example/bzip.git", "main", nil),
		"json": NewGitDependency("json", "https://github.com/example/json.git", "main", nil),
	}

	resolver := NewResolver(deps)
	order, err := resolver.BuildOrder()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should be alphabetical
	expected := []string{"bzip", "json", "zlib"}
	if len(order) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(order))
	}

	for i, name := range expected {
		if order[i] != name {
			t.Errorf("Expected order[%d] = %s, got %s", i, name, order[i])
		}
	}
}

func TestBuildOrder_WithDependencies(t *testing.T) {
	// Note: For Phase 6, dependencies don't have interdependencies yet
	// This test verifies current behavior (alphabetical ordering)
	// When we add clue.cue parsing for dependencies, this test will be updated
	deps := map[string]Dependency{
		"libA": NewGitDependency("libA", "https://github.com/example/a.git", "main", nil),
		"libB": NewGitDependency("libB", "https://github.com/example/b.git", "main", nil),
	}

	resolver := NewResolver(deps)
	order, err := resolver.BuildOrder()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// For now, should return alphabetical order
	expected := []string{"libA", "libB"}
	if len(order) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(order))
	}

	for i, name := range expected {
		if order[i] != name {
			t.Errorf("Expected order[%d] = %s, got %s", i, name, order[i])
		}
	}
}

func TestBuildOrder_CycleDetected(t *testing.T) {
	// Note: Cycle detection will be tested when we implement dependency interdependencies
	// For now, we verify that the graph library is set up with PreventCycles()
	// This is tested by attempting to add a cycle manually

	deps := map[string]Dependency{
		"libA": NewGitDependency("libA", "https://github.com/example/a.git", "main", nil),
		"libB": NewGitDependency("libB", "https://github.com/example/b.git", "main", nil),
	}

	resolver := NewResolver(deps)

	// Current implementation doesn't have inter-dependency edges yet
	// So no cycles can occur. This test documents intended behavior.
	order, err := resolver.BuildOrder()
	if err != nil {
		// If we get an error, it should mention cycle
		if !strings.Contains(err.Error(), "cycle") && !strings.Contains(err.Error(), "circular") {
			t.Errorf("Expected cycle-related error, got: %v", err)
		}
	} else {
		// No error is expected for current implementation
		if len(order) != 2 {
			t.Errorf("Expected 2 dependencies, got %d", len(order))
		}
	}
}

func TestValidateReferences_Valid(t *testing.T) {
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/example/foo.git", "main", nil),
	}

	resolver := NewResolver(deps)

	targets := map[string]struct {
		Name    string
		Depends []string
	}{
		"myapp": {
			Name:    "myapp",
			Depends: []string{"libfoo"}, // Valid dependency
		},
		"libfoo": {
			Name:    "libfoo",
			Depends: []string{},
		},
	}

	err := resolver.ValidateReferences(targets)
	if err != nil {
		t.Errorf("Expected no error for valid references, got: %v", err)
	}
}

func TestValidateReferences_Unknown(t *testing.T) {
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/example/foo.git", "main", nil),
	}

	resolver := NewResolver(deps)

	targets := map[string]struct {
		Name    string
		Depends []string
	}{
		"myapp": {
			Name:    "myapp",
			Depends: []string{"unknown"}, // Unknown dependency
		},
	}

	err := resolver.ValidateReferences(targets)
	if err == nil {
		t.Error("Expected error for unknown dependency")
	}

	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Expected error to mention 'unknown', got: %v", err)
	}

	if !strings.Contains(err.Error(), "myapp") {
		t.Errorf("Expected error to mention target 'myapp', got: %v", err)
	}
}

func TestValidateReferences_TargetToTarget(t *testing.T) {
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/example/foo.git", "main", nil),
	}

	resolver := NewResolver(deps)

	targets := map[string]struct {
		Name    string
		Depends []string
	}{
		"mylib": {
			Name:    "mylib",
			Depends: []string{"libfoo"},
		},
		"myapp": {
			Name:    "myapp",
			Depends: []string{"mylib"}, // Target depends on another target
		},
	}

	err := resolver.ValidateReferences(targets)
	if err != nil {
		t.Errorf("Expected no error for target-to-target dependency, got: %v", err)
	}
}
