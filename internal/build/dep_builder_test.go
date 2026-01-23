package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/deps"
)

// TestDepBuilder_InlineConfig tests building a dependency with inline configuration
func TestDepBuilder_InlineConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files
	sourcePath := filepath.Join(tmpDir, "libfoo")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write a simple C++ source file
	mainCpp := filepath.Join(sourcePath, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte(`
int add(int a, int b) {
    return a + b;
}
`), 0644); err != nil {
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
	toolchain, err := DiscoverToolchain("clang", HostPlatform())
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
		Variant:  "debug",
		Platform: HostPlatform(),
		BuildDir: buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts)
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
}

// TestDepBuilder_ClueConfig tests building a dependency with clue.cue configuration
func TestDepBuilder_ClueConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files
	sourcePath := filepath.Join(tmpDir, "libbar")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write a simple C++ source file
	barCpp := filepath.Join(sourcePath, "bar.cpp")
	if err := os.WriteFile(barCpp, []byte(`
int multiply(int a, int b) {
    return a * b;
}
`), 0644); err != nil {
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
	if err := os.WriteFile(clueCue, []byte(cueContent), 0644); err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	// Create dependency without inline config (should use clue.cue)
	dep := deps.NewVendoredDependency("libbar", sourcePath, nil)

	// Create builder components
	toolchain, err := DiscoverToolchain("clang", HostPlatform())
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
		Variant:  "release",
		Platform: HostPlatform(),
		BuildDir: buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts)
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
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write a simple C++ source file
	mainCpp := filepath.Join(sourcePath, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte(`int test() { return 42; }`), 0644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create dependency without inline config and no clue.cue
	dep := deps.NewVendoredDependency("libnone", sourcePath, nil)

	// Create builder components
	toolchain, err := DiscoverToolchain("clang", HostPlatform())
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
		Variant:  "debug",
		Platform: HostPlatform(),
		BuildDir: buildDir,
		Verbosity: VerbosityNormal,
	}

	// Should fail with error about missing configuration
	_, err = depBuilder.BuildDep(context.Background(), dep, sourcePath, opts)
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
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Write multiple C++ source files
	for _, name := range []string{"a.cpp", "b.cpp", "c.cpp"} {
		file := filepath.Join(sourcePath, name)
		if err := os.WriteFile(file, []byte(`int test() { return 1; }`), 0644); err != nil {
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
	toolchain, err := DiscoverToolchain("clang", HostPlatform())
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
		Variant:  "debug",
		Platform: HostPlatform(),
		BuildDir: buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts)
	if err != nil {
		t.Fatalf("BuildDep failed: %v", err)
	}

	// Verify all 3 files were compiled
	if result.SourceCount != 3 {
		t.Errorf("expected 3 source files (glob expansion), got %d", result.SourceCount)
	}
}

// TestDepBuilder_IncludePath tests include path determination
func TestDepBuilder_IncludePath(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create source files with include directory
	sourcePath := filepath.Join(tmpDir, "libinc")
	includeDir := filepath.Join(sourcePath, "include")
	if err := os.MkdirAll(includeDir, 0755); err != nil {
		t.Fatalf("failed to create include directory: %v", err)
	}

	// Write a header file
	headerFile := filepath.Join(includeDir, "test.h")
	if err := os.WriteFile(headerFile, []byte("#pragma once\nint test();"), 0644); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}

	// Write a simple C++ source file
	mainCpp := filepath.Join(sourcePath, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte(`
#include "test.h"
int test() { return 42; }
`), 0644); err != nil {
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
	toolchain, err := DiscoverToolchain("clang", HostPlatform())
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
		Variant:  "debug",
		Platform: HostPlatform(),
		BuildDir: buildDir,
		Verbosity: VerbosityNormal,
	}

	result, err := depBuilder.BuildDep(context.Background(), dep, sourcePath, opts)
	if err != nil {
		t.Fatalf("BuildDep failed: %v", err)
	}

	// Verify include path points to include directory
	if result.IncludePath != includeDir {
		t.Errorf("expected include path %q, got %q", includeDir, result.IncludePath)
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
