package build

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestModuleScanArgsMatchCompilerConfiguration(t *testing.T) {
	t.Setenv("CXX", "configured-clang++")
	tc, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	compiler := NewCompiler(NewExecutor(ExecutorConfig{}), tc)
	args := compiler.moduleScanArgs("hello.cppm", CompileOptions{
		Includes: []string{"include"},
		Defines:  []string{"FEATURE=1"},
		Flags:    Config{RawCompiler: []string{"-fexperimental-library"}},
		Std:      "c++23",
	})

	for _, want := range []string{"configured-clang++", "-Iinclude", "-DFEATURE=1", "-fexperimental-library", "-std=c++23"} {
		if !slices.Contains(args, want) {
			t.Errorf("module scan arguments %q missing %q", args, want)
		}
	}
}

func TestDetectModuleSources_ByExtension(t *testing.T) {
	// Create temp files with module extensions
	tmpDir := t.TempDir()

	files := []string{
		filepath.Join(tmpDir, "module.cppm"),
		filepath.Join(tmpDir, "interface.ixx"),
		filepath.Join(tmpDir, "regular.cpp"),
	}

	for _, f := range files {
		if err := os.WriteFile(f, []byte("// empty"), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", f, err)
		}
	}

	moduleSources, err := DetectModuleSources(files)
	if err != nil {
		t.Fatalf("DetectModuleSources failed: %v", err)
	}

	// Should detect .cppm and .ixx as modules
	if len(moduleSources) != 2 {
		t.Errorf("expected 2 module sources, got %d", len(moduleSources))
	}
}

func TestDetectModuleSources_ByContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .cpp file with module content
	moduleFile := filepath.Join(tmpDir, "uses_module.cpp")
	if err := os.WriteFile(moduleFile, []byte("import std;\n\nint main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write module file: %v", err)
	}

	regularFile := filepath.Join(tmpDir, "regular.cpp")
	if err := os.WriteFile(regularFile, []byte("#include <iostream>\n\nint main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write regular file: %v", err)
	}

	moduleSources, err := DetectModuleSources([]string{moduleFile, regularFile})
	if err != nil {
		t.Fatalf("DetectModuleSources failed: %v", err)
	}

	if len(moduleSources) != 1 {
		t.Errorf("expected 1 module source, got %d", len(moduleSources))
	}
}

func TestDetectModuleSources_NamedModuleConsumer(t *testing.T) {
	moduleFile := filepath.Join(t.TempDir(), "main.cpp")
	if err := os.WriteFile(moduleFile, []byte("import hello;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleSources, err := DetectModuleSources([]string{moduleFile})
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleSources) != 1 || moduleSources[0] != moduleFile {
		t.Fatalf("DetectModuleSources() = %v, want [%s]", moduleSources, moduleFile)
	}
}

func TestOrderModuleCompilation(t *testing.T) {
	// Module A provides "modA"
	// Module B provides "modB", requires "modA"
	// Module C provides "modC", requires "modB"
	deps := []ModuleDependency{
		{Source: "c.cpp", IsModule: true, Provides: "modC", Requires: []string{"modB"}},
		{Source: "a.cpp", IsModule: true, Provides: "modA", Requires: nil},
		{Source: "b.cpp", IsModule: true, Provides: "modB", Requires: []string{"modA"}},
	}

	order, err := OrderModuleCompilation(deps)
	if err != nil {
		t.Fatalf("OrderModuleCompilation failed: %v", err)
	}

	// Expected order: a.cpp, b.cpp, c.cpp
	if len(order) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(order))
	}

	// Find indices
	indexOf := func(s string) int {
		for i, src := range order {
			if src == s {
				return i
			}
		}
		return -1
	}

	aIdx := indexOf("a.cpp")
	bIdx := indexOf("b.cpp")
	cIdx := indexOf("c.cpp")

	if aIdx >= bIdx {
		t.Errorf("a.cpp should come before b.cpp")
	}
	if bIdx >= cIdx {
		t.Errorf("b.cpp should come before c.cpp")
	}
}

func TestOrderModuleCompilation_CircularDependency(t *testing.T) {
	deps := []ModuleDependency{
		{Source: "a.cpp", IsModule: true, Provides: "modA", Requires: []string{"modB"}},
		{Source: "b.cpp", IsModule: true, Provides: "modB", Requires: []string{"modA"}},
	}

	_, err := OrderModuleCompilation(deps)
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

func TestOrderModuleCompilation_MissingModule(t *testing.T) {
	deps := []ModuleDependency{
		{Source: "a.cpp", IsModule: true, Provides: "modA", Requires: []string{"nonexistent"}},
	}

	_, err := OrderModuleCompilation(deps)
	if err == nil {
		t.Error("expected error for missing module")
	}
}

func TestIsModuleExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"module.cppm", true},
		{"interface.ixx", true},
		{"module.mpp", true},
		{"regular.cpp", false},
		{"header.hpp", false},
		{"source.cc", false},
	}

	for _, tc := range tests {
		if got := IsModuleExtension(tc.path); got != tc.expected {
			t.Errorf("IsModuleExtension(%q) = %v, want %v", tc.path, got, tc.expected)
		}
	}
}
