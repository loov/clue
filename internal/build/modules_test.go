package build

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
)

func TestModuleCompileFlags_MapEachToolchain(t *testing.T) {
	clangToolchain := clang.New("clang", "clang++", "llvm-ar", toolchain.Platform{OS: "linux", Arch: "amd64"})
	gcc, _ := NewToolchain("gcc", toolchain.HostPlatform())
	tests := []struct {
		name   string
		tc     toolchain.Toolchain
		output string
		mapper string
		want   []string
	}{
		{"clang", clangToolchain, "math.pcm", "", []string{"-fcxx-modules", "-fmodule-output=math.pcm", "-fmodule-file=base=base.pcm", "-fmodule-file=vector.pcm"}},
		{"gcc", gcc, "math.gcm", "modules.mapper", []string{"-fmodules-ts", "-x", "c++", "-fmodule-mapper=modules.mapper"}},
		{"msvc", newTestMSVCToolchain(), "math.ifc", "", []string{"/interface", "/ifcOutput", "/reference", "/headerUnit:angle"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := plan.ModuleCompileFlags(test.tc, plan.ModuleDependency{Source: "math.cppm", IsModule: true, Provides: "math"}, test.output, map[string]string{
				"base": "base.pcm", "<vector>": "vector.pcm",
			}, test.mapper)
			for _, want := range test.want {
				if !slices.Contains(flags, want) {
					t.Errorf("flags %q missing %q", flags, want)
				}
			}
		})
	}
}

func TestModuleCompileFlags_ClangOmitsCXXModulesSwitchOnWindows(t *testing.T) {
	tc := clang.New("clang", "clang++", "llvm-ar", toolchain.Platform{OS: "windows", Arch: "amd64"})
	flags := plan.ModuleCompileFlags(tc, plan.ModuleDependency{UsesModules: true}, "math.pcm", nil, "")
	if slices.Contains(flags, "-fcxx-modules") {
		t.Errorf("Windows Clang module flags = %q", flags)
	}
}

func TestMSVCModuleFlags_IncludePartitionAndHeaderUnitSwitches(t *testing.T) {
	tc := newTestMSVCToolchain()
	partition := plan.ModuleCompileFlags(tc, plan.ModuleDependency{InternalPartition: true}, "math-detail.ifc", nil, "")
	for _, want := range []string{"/internalPartition", "/ifcOutput", "math-detail.ifc"} {
		if !slices.Contains(partition, want) {
			t.Errorf("partition flags %q missing %q", partition, want)
		}
	}
	header := plan.HeaderUnitArguments(tc, plan.HeaderUnitOptions{
		Source: "vector", Name: "<vector>", System: true, Output: "vector.ifc",
	})
	for _, want := range []string{"/exportHeader", "/headerName:angle", "/ifcOutput", "vector.ifc"} {
		if !slices.Contains(header, want) {
			t.Errorf("header-unit flags %q missing %q", header, want)
		}
	}
}

func TestScanModuleDependencies_PartitionsAndHeaderUnits(t *testing.T) {
	dir := t.TempDir()
	partition := filepath.Join(dir, "math-detail.cppm")
	primary := filepath.Join(dir, "math.cppm")
	if err := os.WriteFile(partition, []byte("module math:detail;\nimport <vector>;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary, []byte("export module math;\nexport import :detail;\nimport \"numbers.hpp\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dependencies, err := plan.ScanModuleDependencies([]string{primary, partition})
	if err != nil {
		t.Fatal(err)
	}
	if got := dependencies[0]; got.Provides != "math" || !slices.Contains(got.Requires, "math:detail") || !slices.Contains(got.Requires, `"numbers.hpp"`) {
		t.Fatalf("primary dependency = %#v", got)
	}
	if got := dependencies[1]; got.Provides != "math:detail" || !got.InternalPartition || !slices.Contains(got.Requires, "<vector>") {
		t.Fatalf("partition dependency = %#v", got)
	}
}

func TestOrderModuleCompilation_AcceptsDependencyTargetProvider(t *testing.T) {
	order, err := plan.OrderModuleCompilationWithProviders([]plan.ModuleDependency{{
		Source: "main.cpp", Requires: []string{"math"},
	}}, map[string]string{"math": "math.pcm"})
	if err != nil || len(order) != 1 || order[0] != "main.cpp" {
		t.Fatalf("order = %v, err = %v", order, err)
	}
}

func TestDetectModuleSources_RecognizesModuleExtensions(t *testing.T) {
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

	moduleSources, err := plan.DetectModuleSources(files)
	if err != nil {
		t.Fatalf("DetectModuleSources failed: %v", err)
	}

	// Should detect .cppm and .ixx as modules
	if len(moduleSources) != 2 {
		t.Errorf("expected 2 module sources, got %d", len(moduleSources))
	}
}

func TestDetectModuleSources_RecognizesImportStatements(t *testing.T) {
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

	moduleSources, err := plan.DetectModuleSources([]string{moduleFile, regularFile})
	if err != nil {
		t.Fatalf("DetectModuleSources failed: %v", err)
	}

	if len(moduleSources) != 1 {
		t.Errorf("expected 1 module source, got %d", len(moduleSources))
	}
}

func TestDetectModuleSources_RecognizesNamedModuleImports(t *testing.T) {
	moduleFile := filepath.Join(t.TempDir(), "main.cpp")
	if err := os.WriteFile(moduleFile, []byte("import hello;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleSources, err := plan.DetectModuleSources([]string{moduleFile})
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleSources) != 1 || moduleSources[0] != moduleFile {
		t.Fatalf("DetectModuleSources() = %v, want [%s]", moduleSources, moduleFile)
	}
}

func TestOrderModuleCompilation_PlacesProvidersBeforeConsumers(t *testing.T) {
	// Module A provides "modA"
	// Module B provides "modB", requires "modA"
	// Module C provides "modC", requires "modB"
	deps := []plan.ModuleDependency{
		{Source: "c.cpp", IsModule: true, Provides: "modC", Requires: []string{"modB"}},
		{Source: "a.cpp", IsModule: true, Provides: "modA", Requires: nil},
		{Source: "b.cpp", IsModule: true, Provides: "modB", Requires: []string{"modA"}},
	}

	order, err := plan.OrderModuleCompilation(deps)
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

func TestOrderModuleCompilation_RejectsCycle(t *testing.T) {
	deps := []plan.ModuleDependency{
		{Source: "a.cpp", IsModule: true, Provides: "modA", Requires: []string{"modB"}},
		{Source: "b.cpp", IsModule: true, Provides: "modB", Requires: []string{"modA"}},
	}

	_, err := plan.OrderModuleCompilation(deps)
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

func TestOrderModuleCompilation_RejectsMissingProvider(t *testing.T) {
	deps := []plan.ModuleDependency{
		{Source: "a.cpp", IsModule: true, Provides: "modA", Requires: []string{"nonexistent"}},
	}

	_, err := plan.OrderModuleCompilation(deps)
	if err == nil {
		t.Error("expected error for missing module")
	}
}

func TestIsModuleExtension_RecognizesSupportedSuffixes(t *testing.T) {
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
		if got := plan.IsModuleExtension(tc.path); got != tc.expected {
			t.Errorf("IsModuleExtension(%q) = %v, want %v", tc.path, got, tc.expected)
		}
	}
}
