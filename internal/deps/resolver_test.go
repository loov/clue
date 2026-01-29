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
	// libB depends on libA, so libA should be built first
	deps := map[string]Dependency{
		"libA": NewGitDependency("libA", "https://github.com/example/a.git", "main", nil),
		"libB": NewGitDependency("libB", "https://github.com/example/b.git", "main", &InlineConfig{
			Sources: []string{"b.cpp"},
			Depends: []string{"libA"},
		}),
	}

	resolver := NewResolver(deps)
	order, err := resolver.BuildOrder()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// libA should come before libB
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

func TestBuildOrder_WithDependencies_ReverseName(t *testing.T) {
	// alpha depends on zeta, so zeta should be built first (reverse of alphabetical)
	deps := map[string]Dependency{
		"alpha": NewVendoredDependency("alpha", "vendor/alpha", &InlineConfig{
			Sources: []string{"alpha.cpp"},
			Depends: []string{"zeta"},
		}),
		"zeta": NewVendoredDependency("zeta", "vendor/zeta", &InlineConfig{
			Sources: []string{"zeta.cpp"},
		}),
	}

	resolver := NewResolver(deps)
	order, err := resolver.BuildOrder()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// zeta should come before alpha (reverse of alphabetical)
	expected := []string{"zeta", "alpha"}
	if len(order) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(order))
	}

	for i, name := range expected {
		if order[i] != name {
			t.Errorf("Expected order[%d] = %s, got %s", i, name, order[i])
		}
	}
}

func TestBuildOrder_UnknownDependency(t *testing.T) {
	// A dependency declares depends on a nonexistent dependency
	deps := map[string]Dependency{
		"libA": NewGitDependency("libA", "https://github.com/example/a.git", "main", &InlineConfig{
			Sources: []string{"a.cpp"},
			Depends: []string{"nonexistent"},
		}),
	}

	resolver := NewResolver(deps)
	_, err := resolver.BuildOrder()
	if err == nil {
		t.Error("Expected error for unknown dependency")
	}

	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Expected error to mention 'unknown', got: %v", err)
	}

	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("Expected error to mention 'nonexistent', got: %v", err)
	}
}

func TestBuildOrder_CyclicDependency(t *testing.T) {
	// Two dependencies that depend on each other create a cycle
	deps := map[string]Dependency{
		"libA": NewGitDependency("libA", "https://github.com/example/a.git", "main", &InlineConfig{
			Sources: []string{"a.cpp"},
			Depends: []string{"libB"},
		}),
		"libB": NewGitDependency("libB", "https://github.com/example/b.git", "main", &InlineConfig{
			Sources: []string{"b.cpp"},
			Depends: []string{"libA"},
		}),
	}

	resolver := NewResolver(deps)
	_, err := resolver.BuildOrder()
	if err == nil {
		t.Error("Expected error for cyclic dependency")
	}

	// Error should mention cycle or circular
	if !strings.Contains(err.Error(), "cycle") && !strings.Contains(err.Error(), "circular") {
		t.Errorf("Expected error to mention 'cycle' or 'circular', got: %v", err)
	}
}

func TestBuildOrder_CycleDetected(t *testing.T) {
	// Cycle detection is now functional - test with explicit cycle
	deps := map[string]Dependency{
		"libA": NewGitDependency("libA", "https://github.com/example/a.git", "main", &InlineConfig{
			Sources: []string{"a.cpp"},
			Depends: []string{"libB"},
		}),
		"libB": NewGitDependency("libB", "https://github.com/example/b.git", "main", &InlineConfig{
			Sources: []string{"b.cpp"},
			Depends: []string{"libA"},
		}),
	}

	resolver := NewResolver(deps)

	// Cycle detection is now functional
	_, err := resolver.BuildOrder()
	if err == nil {
		t.Error("Expected error for cycle detection")
	}

	// If we get an error, it should mention cycle
	if !strings.Contains(err.Error(), "cycle") && !strings.Contains(err.Error(), "circular") {
		t.Errorf("Expected cycle-related error, got: %v", err)
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
