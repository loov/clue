package build

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/deps"
)

// TestDepBuilder_InlineConfig tests building a dependency with inline configuration
func TestDepBuilder_InlineConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files
	sourcePath := filepath.Join(tmpDir, "libfoo")
	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write a simple C++ source file
	mainCpp := filepath.Join(sourcePath, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte(`
int add(int a, int b) {
    return a + b;
}
`), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create dependency with inline config
	inlineConfig := &deps.InlineConfig{
		Sources:  []string{"main.cpp"},
		Includes: []string{},
		Defines:  []string{},
		Type:     "static_library",
	}
	dep := deps.NewVendoredDependency("libfoo", sourcePath, inlineConfig)

	// Create builder components
	toolchain, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Skipf("clang not available: %v", err)
	}

	executor := NewExecutor(ExecutorConfig{Verbose: false, StreamOutput: false, WorkDir: ""})
	compiler := NewCompiler(executor, toolchain)
	linker := NewLinker(executor, toolchain, HostPlatform())

	depBuilder := NewDepBuilder(compiler, linker, toolchain, VerbosityNormal)

	// Build dependency
	buildDir := filepath.Join(tmpDir, ".build")
	depBuilder.cache, err = cache.NewManager(buildDir)
	if err != nil {
		t.Fatal(err)
	}
	opts := DepBuildOptions{
		Variant:   "debug",
		Platform:  HostPlatform(),
		BuildDir:  buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts, nil)
	if err != nil {
		t.Fatalf("BuildDep failed: %v", err)
	}

	// Verify result
	if result.Name != "libfoo" {
		t.Errorf("expected name 'libfoo', got %q", result.Name)
	}
	if result.SourceCount != 1 {
		t.Errorf("expected 1 source file, got %d", result.SourceCount)
	}

	// Verify library file exists
	if _, err := os.Stat(result.LibPath); err != nil {
		t.Errorf("library file not created: %v", err)
	}

	// Verify it's named correctly
	expectedLibName := "liblibfoo.a"
	if filepath.Base(result.LibPath) != expectedLibName {
		t.Errorf("expected library name %q, got %q", expectedLibName, filepath.Base(result.LibPath))
	}
	objectPath := filepath.Join(buildDir, "debug", "deps", "libfoo", "obj", "main.cpp.o")
	objectBefore, err := os.Stat(objectPath)
	if err != nil {
		t.Fatal(err)
	}
	libraryBefore, err := os.Stat(result.LibPath)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts, nil); err != nil {
		t.Fatal(err)
	}
	objectAfter, err := os.Stat(objectPath)
	if err != nil {
		t.Fatal(err)
	}
	libraryAfter, err := os.Stat(result.LibPath)
	if err != nil {
		t.Fatal(err)
	}
	if !objectAfter.ModTime().Equal(objectBefore.ModTime()) || !libraryAfter.ModTime().Equal(libraryBefore.ModTime()) {
		t.Error("unchanged dependency was rebuilt")
	}
}

func TestDepBuilder_HeaderOnlyDependencyNeedsNoCompiler(t *testing.T) {
	root := t.TempDir()
	include := filepath.Join(root, "include")
	if err := os.Mkdir(include, 0o755); err != nil {
		t.Fatal(err)
	}
	dep := deps.NewVendoredDependency("headers", root, &deps.InlineConfig{
		Type: "header_only", Includes: []string{"include"},
	})
	result, err := (&DepBuilder{}).BuildDep(t.Context(), dep, root, DepBuildOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "header_only" || result.LibPath != "" || result.IncludePath != include {
		t.Fatalf("header-only result = %+v", result)
	}
}

func TestDepBuilder_PrebuiltDependencyNeedsNoCompiler(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "libcustom.a")
	if err := os.WriteFile(library, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dep := deps.NewVendoredDependency("custom", root, &deps.InlineConfig{
		Type: "prebuilt_static", Library: "libcustom.a",
	})
	result, err := (&DepBuilder{}).BuildDep(t.Context(), dep, root, DepBuildOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "prebuilt_static" || result.LibPath != library {
		t.Fatalf("prebuilt result = %+v", result)
	}
}

func TestDepBuilder_ExternalDependencyRunsCommands(t *testing.T) {
	root := t.TempDir()
	generator := `package main
import "os"
func main() {
	if err := os.MkdirAll("build", 0755); err != nil { panic(err) }
	if err := os.WriteFile("build/custom.a", nil, 0644); err != nil { panic(err) }
}`
	if err := os.WriteFile(filepath.Join(root, "generate.go"), []byte(generator), 0o644); err != nil {
		t.Fatal(err)
	}
	dep := deps.NewVendoredDependency("custom", root, &deps.InlineConfig{
		Type: "external_static", Library: filepath.Join("build", "custom.a"),
		Commands: [][]string{{"go", "run", "generate.go"}},
	})
	result, err := (&DepBuilder{}).BuildDep(t.Context(), dep, root, DepBuildOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "external_static" || result.LibPath != filepath.Join(root, "build", "custom.a") {
		t.Fatalf("external result = %+v", result)
	}
}

func TestDepBuilder_SharedLibraryUsesProjectStandard(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib.cpp")
	if err := os.WriteFile(source, []byte(`
#if __cplusplus < 202002L
#error expected C++20
#endif
extern "C" int answer() { return 42; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	dep := deps.NewVendoredDependency("answer", root, &deps.InlineConfig{
		Sources: []string{"lib.cpp"}, Type: "shared_library",
	})
	toolchain, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Skipf("clang not available: %v", err)
	}
	executor := NewExecutor(ExecutorConfig{StreamOutput: false})
	builder := NewDepBuilder(
		NewCompiler(executor, toolchain), NewLinker(executor, toolchain, HostPlatform()),
		toolchain, VerbosityQuiet,
	)

	result, err := builder.BuildDep(context.Background(), dep, root, DepBuildOptions{
		Variant: "debug", Platform: HostPlatform(), BuildDir: filepath.Join(root, ".build"), Std: "c++20",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "shared_library" {
		t.Errorf("type = %q, want shared_library", result.Type)
	}
	want := "libanswer" + SharedLibraryExtension(HostPlatform())
	if filepath.Base(result.LibPath) != want {
		t.Errorf("library = %q, want %q", filepath.Base(result.LibPath), want)
	}
}

// TestDepBuilder_ClueConfig tests building a dependency with clue.cue configuration
func TestDepBuilder_ClueConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files
	sourcePath := filepath.Join(tmpDir, "libbar")
	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write a simple C++ source file
	barCpp := filepath.Join(sourcePath, "bar.cpp")
	if err := os.WriteFile(barCpp, []byte(`
int multiply(int a, int b) {
    return a * b;
}
`), 0o644); err != nil {
		t.Fatalf("failed to write bar.cpp: %v", err)
	}

	// Write clue.cue configuration
	clueCue := filepath.Join(sourcePath, "clue.cue")
	cueContent := `
targets: {
	libbar: {
		type: "static_library"
		sources: ["bar.cpp"]
	}
}
`
	if err := os.WriteFile(clueCue, []byte(cueContent), 0o644); err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	// Create dependency without inline config (should use clue.cue)
	dep := deps.NewVendoredDependency("libbar", sourcePath, nil)

	// Create builder components
	toolchain, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Skipf("clang not available: %v", err)
	}

	executor := NewExecutor(ExecutorConfig{Verbose: false, StreamOutput: false, WorkDir: ""})
	compiler := NewCompiler(executor, toolchain)
	linker := NewLinker(executor, toolchain, HostPlatform())

	depBuilder := NewDepBuilder(compiler, linker, toolchain, VerbosityNormal)

	// Build dependency
	buildDir := filepath.Join(tmpDir, ".build")
	opts := DepBuildOptions{
		Variant:   "release",
		Platform:  HostPlatform(),
		BuildDir:  buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts, nil)
	if err != nil {
		t.Fatalf("BuildDep failed: %v", err)
	}

	// Verify result
	if result.Name != "libbar" {
		t.Errorf("expected name 'libbar', got %q", result.Name)
	}
	if result.SourceCount != 1 {
		t.Errorf("expected 1 source file, got %d", result.SourceCount)
	}

	// Verify library file exists
	if _, err := os.Stat(result.LibPath); err != nil {
		t.Errorf("library file not created: %v", err)
	}
}

// TestDepBuilder_NoConfig tests error when no configuration is available
func TestDepBuilder_NoConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files but NO configuration
	sourcePath := filepath.Join(tmpDir, "libnone")
	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write a simple C++ source file
	mainCpp := filepath.Join(sourcePath, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte(`int test() { return 42; }`), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create dependency without inline config and no clue.cue
	dep := deps.NewVendoredDependency("libnone", sourcePath, nil)

	// Create builder components
	toolchain, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Skipf("clang not available: %v", err)
	}

	executor := NewExecutor(ExecutorConfig{Verbose: false, StreamOutput: false, WorkDir: ""})
	compiler := NewCompiler(executor, toolchain)
	linker := NewLinker(executor, toolchain, HostPlatform())

	depBuilder := NewDepBuilder(compiler, linker, toolchain, VerbosityNormal)

	// Build dependency
	buildDir := filepath.Join(tmpDir, ".build")
	opts := DepBuildOptions{
		Variant:   "debug",
		Platform:  HostPlatform(),
		BuildDir:  buildDir,
		Verbosity: VerbosityNormal,
	}

	// Should fail with error about missing configuration
	_, err = depBuilder.BuildDep(context.Background(), dep, sourcePath, opts, nil)
	if err == nil {
		t.Fatal("expected error for missing configuration, got nil")
	}

	// Verify error message mentions missing configuration
	errMsg := err.Error()
	if !containsString(errMsg, "no build configuration") {
		t.Errorf("expected error about missing configuration, got: %v", err)
	}
}

// TestDepBuilder_GlobSources tests glob expansion in source patterns
func TestDepBuilder_GlobSources(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files
	sourcePath := filepath.Join(tmpDir, "libglob")
	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write multiple C++ source files
	for _, name := range []string{"a.cpp", "b.cpp", "c.cpp"} {
		file := filepath.Join(sourcePath, name)
		if err := os.WriteFile(file, []byte(`int test() { return 1; }`), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	// Create dependency with glob pattern
	inlineConfig := &deps.InlineConfig{
		Sources:  []string{"*.cpp"},
		Includes: []string{},
		Defines:  []string{},
		Type:     "static_library",
	}
	dep := deps.NewVendoredDependency("libglob", sourcePath, inlineConfig)

	// Create builder components
	toolchain, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Skipf("clang not available: %v", err)
	}

	executor := NewExecutor(ExecutorConfig{Verbose: false, StreamOutput: false, WorkDir: ""})
	compiler := NewCompiler(executor, toolchain)
	linker := NewLinker(executor, toolchain, HostPlatform())

	depBuilder := NewDepBuilder(compiler, linker, toolchain, VerbosityNormal)

	// Build dependency
	buildDir := filepath.Join(tmpDir, ".build")
	opts := DepBuildOptions{
		Variant:   "debug",
		Platform:  HostPlatform(),
		BuildDir:  buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts, nil)
	if err != nil {
		t.Fatalf("BuildDep failed: %v", err)
	}

	// Verify all 3 files were compiled
	if result.SourceCount != 3 {
		t.Errorf("expected 3 source files (glob expansion), got %d", result.SourceCount)
	}
}

func TestResolveDepConfig_SelectsTargetAndItsInternalDependencies(t *testing.T) {
	root := t.TempDir()
	config := `targets: {
	base: {type: "static_library", sources: ["base.cpp"], public: includes: ["include"]}
	exported: {type: "static_library", sources: ["exported.cpp"], depends: ["base"]}
	unused: {type: "static_library", sources: ["unused.cpp"]}
}`
	if err := os.WriteFile(filepath.Join(root, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	dep := deps.NewVendoredDependency("package", root, nil)
	dep.TargetName = "exported"
	resolved, err := ResolveDepConfig(dep, root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(resolved.Sources, []string{"exported.cpp", "base.cpp"}) {
		t.Fatalf("sources = %v", resolved.Sources)
	}
	if !slices.Equal(resolved.Includes, []string{filepath.Join(root, "include")}) {
		t.Fatalf("includes = %v", resolved.Includes)
	}
}

func TestResolveDepConfig_RejectsAmbiguousTargets(t *testing.T) {
	root := t.TempDir()
	config := `targets: {
	first: {type: "static_library", sources: ["first.cpp"]}
	second: {type: "static_library", sources: ["second.cpp"]}
}`
	if err := os.WriteFile(filepath.Join(root, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveDepConfig(deps.NewVendoredDependency("package", root, nil), root)
	if err == nil || !strings.Contains(err.Error(), "multiple targets") {
		t.Fatalf("ResolveDepConfig() error = %v, want ambiguous-target error", err)
	}
}

// TestDepBuilder_IncludePath tests include path determination
func TestDepBuilder_IncludePath(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files with include directory
	sourcePath := filepath.Join(tmpDir, "libinc")
	includeDir := filepath.Join(sourcePath, "include")
	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		t.Fatalf("failed to create include directory: %v", err)
	}

	// Write a header file
	headerFile := filepath.Join(includeDir, "test.h")
	if err := os.WriteFile(headerFile, []byte("#pragma once\nint test();"), 0o644); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}

	// Write a simple C++ source file
	mainCpp := filepath.Join(sourcePath, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte(`
#include "test.h"
int test() { return 42; }
`), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create dependency with inline config (no explicit includes)
	inlineConfig := &deps.InlineConfig{
		Sources:  []string{"main.cpp"},
		Includes: []string{},
		Defines:  []string{},
		Type:     "static_library",
	}
	dep := deps.NewVendoredDependency("libinc", sourcePath, inlineConfig)

	// Create builder components
	toolchain, err := NewToolchain("clang", HostPlatform())
	if err != nil {
		t.Skipf("clang not available: %v", err)
	}

	executor := NewExecutor(ExecutorConfig{Verbose: false, StreamOutput: false, WorkDir: ""})
	compiler := NewCompiler(executor, toolchain)
	linker := NewLinker(executor, toolchain, HostPlatform())

	depBuilder := NewDepBuilder(compiler, linker, toolchain, VerbosityNormal)

	// Build dependency
	buildDir := filepath.Join(tmpDir, ".build")
	opts := DepBuildOptions{
		Variant:   "debug",
		Platform:  HostPlatform(),
		BuildDir:  buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts, nil)
	if err != nil {
		t.Fatalf("BuildDep failed: %v", err)
	}

	// Verify include path points to include directory
	if result.IncludePath != includeDir {
		t.Errorf("expected include path %q, got %q", includeDir, result.IncludePath)
	}
}

// TestDepBuilder_HeadersIncludePath tests include path determination with headers field
func TestDepBuilder_HeadersIncludePath(t *testing.T) {
	tests := []struct {
		name         string
		inlineConfig *deps.InlineConfig
		sourcePath   string
		expectedPath string
		description  string
	}{
		{
			name: "headers_set",
			inlineConfig: &deps.InlineConfig{
				Sources: []string{"math.cpp"},
				Headers: []string{"math.h"},
			},
			sourcePath:   "/tmp/test/vendor/simplemath",
			expectedPath: "/tmp/test/vendor",
			description:  "When headers is set, include path should be parent directory",
		},
		{
			name: "headers_empty_includes_set",
			inlineConfig: &deps.InlineConfig{
				Sources:  []string{"math.cpp"},
				Headers:  []string{},
				Includes: []string{"custom"},
			},
			sourcePath:   "/tmp/test/vendor/simplemath",
			expectedPath: "/tmp/test/vendor/simplemath/custom",
			description:  "When headers is empty but includes is set, use includes",
		},
		{
			name: "headers_nil_includes_set",
			inlineConfig: &deps.InlineConfig{
				Sources:  []string{"math.cpp"},
				Includes: []string{".."},
			},
			sourcePath:   "/tmp/test/vendor/simplemath",
			expectedPath: "/tmp/test/vendor",
			description:  "When headers is nil but includes is set, use includes (filepath.Join cleans ..)",
		},
		{
			name: "both_empty",
			inlineConfig: &deps.InlineConfig{
				Sources: []string{"math.cpp"},
			},
			sourcePath:   "/tmp/test/vendor/simplemath",
			expectedPath: "/tmp/test/vendor/simplemath",
			description:  "When neither is set, fallback to sourcePath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal DepBuilder (determineIncludePath doesn't use any fields)
			db := &DepBuilder{}

			// Create vendored dependency with the test inline config
			dep := deps.NewVendoredDependency("testdep", tt.sourcePath, tt.inlineConfig)

			// Call determineIncludePath
			result := db.determineIncludePath(dep, tt.sourcePath, nil)

			if result != tt.expectedPath {
				t.Errorf("%s: expected include path %q, got %q", tt.description, tt.expectedPath, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
