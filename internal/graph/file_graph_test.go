package graph

import (
	"strings"
	"testing"
)

func TestBuildFileGraph_CreatesCompileAndLinkNodes(t *testing.T) {
	builder := NewFileGraphBuilder()

	targets := []TargetFiles{
		{
			Name:    "myapp",
			Type:    "executable",
			Sources: []string{"main.cpp", "utils.cpp"},
		},
	}

	g, err := builder.BuildFileGraph(".build", targets)
	if err != nil {
		t.Fatalf("BuildFileGraph failed: %v", err)
	}

	// Should have: 2 sources + 2 compile cmds + 2 objects + 1 link cmd + 1 output = 8 nodes
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder failed: %v", err)
	}

	// Verify node types appear in correct order
	// Sources should come before compile commands
	// Compile commands should come before objects
	// Objects should come before link command
	// Link command should come before output

	sourceIdx := -1
	compileIdx := -1
	objectIdx := -1
	linkIdx := -1
	outputIdx := -1

	for i, id := range order {
		if strings.HasPrefix(id, "src:") && sourceIdx == -1 {
			sourceIdx = i
		}
		if strings.HasPrefix(id, "cmd:compile:") && compileIdx == -1 {
			compileIdx = i
		}
		if strings.HasPrefix(id, "obj:") && objectIdx == -1 {
			objectIdx = i
		}
		if strings.HasPrefix(id, "cmd:link:") {
			linkIdx = i
		}
		if strings.HasPrefix(id, "out:") {
			outputIdx = i
		}
	}

	if sourceIdx > compileIdx {
		t.Error("source nodes should come before compile commands")
	}
	if compileIdx > objectIdx {
		t.Error("compile commands should come before object nodes")
	}
	if objectIdx > linkIdx {
		t.Error("object nodes should come before link command")
	}
	if linkIdx > outputIdx {
		t.Error("link command should come before output")
	}
}

func TestBuildFileGraph_ConnectsTargetDependencies(t *testing.T) {
	builder := NewFileGraphBuilder()

	targets := []TargetFiles{
		{
			Name:    "libmath",
			Type:    "static_library",
			Sources: []string{"math.cpp"},
		},
		{
			Name:    "myapp",
			Type:    "executable",
			Sources: []string{"main.cpp"},
			Depends: []string{"libmath"},
		},
	}

	g, err := builder.BuildFileGraph(".build", targets)
	if err != nil {
		t.Fatalf("BuildFileGraph failed: %v", err)
	}

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder failed: %v", err)
	}

	// libmath output must come before myapp link command
	libmathOutIdx := -1
	myappLinkIdx := -1

	for i, id := range order {
		if id == "out:libmath" {
			libmathOutIdx = i
		}
		if id == "cmd:link:myapp" {
			myappLinkIdx = i
		}
	}

	if libmathOutIdx == -1 {
		t.Error("libmath output node not found")
	}
	if myappLinkIdx == -1 {
		t.Error("myapp link command not found")
	}
	if libmathOutIdx > myappLinkIdx {
		t.Errorf("libmath output (idx %d) should come before myapp link (idx %d)", libmathOutIdx, myappLinkIdx)
	}
}

func TestBuildFileGraph_RejectsTargetCycle(t *testing.T) {
	builder := NewFileGraphBuilder()

	// Create circular dependency: A depends on B, B depends on A
	targets := []TargetFiles{
		{
			Name:    "libA",
			Type:    "static_library",
			Sources: []string{"a.cpp"},
			Depends: []string{"libB"},
		},
		{
			Name:    "libB",
			Type:    "static_library",
			Sources: []string{"b.cpp"},
			Depends: []string{"libA"},
		},
	}

	_, err := builder.BuildFileGraph(".build", targets)
	if err == nil {
		t.Error("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cyclic") {
		t.Errorf("error should mention cycle: %v", err)
	}
}

func TestObjectPath_PlacesObjectUnderTargetDirectory(t *testing.T) {
	builder := NewFileGraphBuilder()

	tests := []struct {
		target, source, buildDir, expected string
	}{
		{"myapp", "main.cpp", ".build", ".build/myapp/main.o"},
		{"myapp", "src/utils.cpp", ".build", ".build/myapp/utils.o"},
		{"lib", "math.c", ".build", ".build/lib/math.o"},
	}

	for _, tt := range tests {
		got := builder.objectPath(tt.target, tt.source, tt.buildDir)
		if got != tt.expected {
			t.Errorf("objectPath(%q, %q, %q) = %q, want %q",
				tt.target, tt.source, tt.buildDir, got, tt.expected)
		}
	}
}

func TestOutputPath_UsesTypeSpecificArtifactDirectory(t *testing.T) {
	builder := NewFileGraphBuilder()

	tests := []struct {
		target, targetType, buildDir, expected string
	}{
		{"myapp", "executable", ".build", ".build/myapp"},
		{"math", "static_library", ".build", ".build/libmath.a"},
		{"utils", "shared_library", ".build", ".build/libutils.so"},
	}

	for _, tt := range tests {
		got := builder.outputPath(tt.target, tt.targetType, tt.buildDir)
		if got != tt.expected {
			t.Errorf("outputPath(%q, %q, %q) = %q, want %q",
				tt.target, tt.targetType, tt.buildDir, got, tt.expected)
		}
	}
}
