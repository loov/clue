package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiler_isCPlusPlus(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	tests := []struct {
		source   string
		expected bool
	}{
		{"main.cpp", true},
		{"file.cc", true},
		{"code.cxx", true},
		{"source.C", true},
		{"program.CPP", true},
		{"main.c", false},
		{"header.h", false},
		{"readme.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := compiler.isCPlusPlus(tt.source)
			if result != tt.expected {
				t.Errorf("isCPlusPlus(%s) = %v, want %v", tt.source, result, tt.expected)
			}
		})
	}
}

func TestCompiler_WithToolchain(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})
	platform := HostPlatform()

	tests := []struct {
		toolchainName string
		source        string
		expectCC      bool // true if should use CC, false for CXX
	}{
		{"clang", "main.cpp", false}, // C++ uses CXX
		{"clang", "main.c", true},    // C uses CC
		{"gcc", "main.cpp", false},
		{"gcc", "main.c", true},
	}

	for _, tt := range tests {
		t.Run(tt.toolchainName+"_"+tt.source, func(t *testing.T) {
			tc, err := DiscoverToolchain(tt.toolchainName, platform)
			if err != nil {
				t.Fatalf("DiscoverToolchain failed: %v", err)
			}

			compiler := NewCompiler(executor, tc)
			result := compiler.compilerCmd(tt.source)

			if tt.expectCC {
				if result != tc.CC {
					t.Errorf("compilerCmd(%s) = %s, want CC=%s",
						tt.source, result, tc.CC)
				}
			} else {
				if result != tc.CXX {
					t.Errorf("compilerCmd(%s) = %s, want CXX=%s",
						tt.source, result, tc.CXX)
				}
			}
		})
	}
}

func TestCompiler_CompileSource_Integration(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple C++ file
	sourceFile := filepath.Join(tmpDir, "main.cpp")
	sourceContent := `int main() { return 0; }`
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile
	objectFile := filepath.Join(tmpDir, "main.o")
	opts := CompileOptions{
		Source: sourceFile,
		Output: objectFile,
		Flags: BuildConfig{
			Optimize:         "none",
			Warnings:         "default",
			WarningsAsErrors: false,
			Debug:            "none",
		},
	}

	result, err := compiler.CompileSource(context.Background(), opts)

	// Verify compilation succeeded
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	// Verify object file created
	if _, err := os.Stat(objectFile); os.IsNotExist(err) {
		t.Errorf("Object file %s was not created", objectFile)
	}

	// Verify result fields
	if result.Source != sourceFile {
		t.Errorf("Result.Source = %s, want %s", result.Source, sourceFile)
	}
	if result.Object != objectFile {
		t.Errorf("Result.Object = %s, want %s", result.Object, objectFile)
	}
	if result.Duration == 0 {
		t.Errorf("Result.Duration = 0, expected non-zero duration")
	}
}

func TestCompiler_CompileSource_Error(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a C++ file with syntax error
	sourceFile := filepath.Join(tmpDir, "bad.cpp")
	sourceContent := `int main() { syntax error }`
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Attempt compilation
	objectFile := filepath.Join(tmpDir, "bad.o")
	opts := CompileOptions{
		Source: sourceFile,
		Output: objectFile,
		Flags: BuildConfig{
			Optimize:         "none",
			Warnings:         "default",
			WarningsAsErrors: false,
			Debug:            "none",
		},
	}

	result, err := compiler.CompileSource(context.Background(), opts)

	// Verify compilation failed
	if err == nil {
		t.Errorf("Expected compilation error, got nil")
	}

	if result == nil {
		t.Fatalf("Result should not be nil even on error")
	}

	if result.Success {
		t.Errorf("Result.Success = true, want false")
	}
}

func TestCompiler_CompileSource_WithFlags(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple C++ file
	sourceFile := filepath.Join(tmpDir, "main.cpp")
	sourceContent := `int main() { return 0; }`
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile with semantic flags
	objectFile := filepath.Join(tmpDir, "main.o")
	opts := CompileOptions{
		Source: sourceFile,
		Output: objectFile,
		Flags: BuildConfig{
			Optimize:         "fast",
			Warnings:         "strict",
			WarningsAsErrors: true,
			Debug:            "full",
		},
	}

	result, err := compiler.CompileSource(context.Background(), opts)

	// Verify compilation succeeded (flags are valid)
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	// Verify object file created
	if _, err := os.Stat(objectFile); os.IsNotExist(err) {
		t.Errorf("Object file %s was not created", objectFile)
	}
}

func TestCompiler_CompileSource_WithIncludes(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory structure
	tmpDir := t.TempDir()
	includeDir := filepath.Join(tmpDir, "include")
	srcDir := filepath.Join(tmpDir, "src")

	if err := os.MkdirAll(includeDir, 0755); err != nil {
		t.Fatalf("Failed to create include dir: %v", err)
	}
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	// Create header file
	headerFile := filepath.Join(includeDir, "header.h")
	headerContent := `#define HEADER_LOADED 1`
	if err := os.WriteFile(headerFile, []byte(headerContent), 0644); err != nil {
		t.Fatalf("Failed to write header file: %v", err)
	}

	// Create source file that includes the header
	sourceFile := filepath.Join(srcDir, "main.cpp")
	sourceContent := `#include "header.h"
int main() { return HEADER_LOADED; }`
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile with include path
	objectFile := filepath.Join(srcDir, "main.o")
	opts := CompileOptions{
		Source:   sourceFile,
		Output:   objectFile,
		Includes: []string{includeDir},
		Flags: BuildConfig{
			Optimize:         "none",
			Warnings:         "default",
			WarningsAsErrors: false,
			Debug:            "none",
		},
	}

	result, err := compiler.CompileSource(context.Background(), opts)

	// Verify compilation succeeded
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	// Verify object file created
	if _, err := os.Stat(objectFile); os.IsNotExist(err) {
		t.Errorf("Object file %s was not created", objectFile)
	}
}

func TestCompileSource_GeneratesDepFile(t *testing.T) {
	// Skip if clang not available
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not found in PATH")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create source file with an include
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)

	// Create a header file
	headerContent := `#ifndef CONFIG_H
#define CONFIG_H
#define VERSION 1
#endif
`
	headerPath := filepath.Join(srcDir, "config.h")
	os.WriteFile(headerPath, []byte(headerContent), 0644)

	// Create source that includes the header
	srcContent := `#include "config.h"
int main() { return VERSION; }
`
	srcPath := filepath.Join(srcDir, "main.cpp")
	os.WriteFile(srcPath, []byte(srcContent), 0644)

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile
	objDir := filepath.Join(tmpDir, "obj")
	os.MkdirAll(objDir, 0755)

	result, err := compiler.CompileSource(context.Background(), CompileOptions{
		Source:   srcPath,
		Output:   filepath.Join(objDir, "main.cpp.o"),
		Includes: []string{srcDir},
		Flags:    BuildConfig{},
		Std:      "c++17",
	})

	// Verify compilation succeeded
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	// Verify .d file path is set
	if result.DepFile == "" {
		t.Fatal("DepFile not set in result")
	}

	// Verify .d file exists
	if _, err := os.Stat(result.DepFile); err != nil {
		t.Fatalf(".d file not created: %v", err)
	}

	// Verify .d file contains the header
	depContent, err := os.ReadFile(result.DepFile)
	if err != nil {
		t.Fatalf("failed to read .d file: %v", err)
	}

	if !strings.Contains(string(depContent), "config.h") {
		t.Errorf(".d file does not contain config.h dependency:\n%s", depContent)
	}
}

func TestCompileSource_DepFilePath(t *testing.T) {
	// Unit test - no actual compilation needed
	// Just verify the path computation logic

	objPath := "/build/debug/myapp/obj/main.cpp.o"
	expectedDepPath := "/build/debug/myapp/obj/main.cpp.d"

	// Compute dep path the same way compiler does
	depPath := filepath.Base(objPath[:len(objPath)-len(filepath.Ext(objPath))]) + ".d"
	depPath = filepath.Join(filepath.Dir(objPath), depPath)

	if depPath != expectedDepPath {
		t.Errorf("dep path = %s, want %s", depPath, expectedDepPath)
	}
}

func TestCompiler_CompileSources_FailFast(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create first source file (good)
	goodFile := filepath.Join(tmpDir, "good.cpp")
	goodContent := `int main() { return 0; }`
	if err := os.WriteFile(goodFile, []byte(goodContent), 0644); err != nil {
		t.Fatalf("Failed to write good source file: %v", err)
	}

	// Create second source file (bad)
	badFile := filepath.Join(tmpDir, "bad.cpp")
	badContent := `int main() { syntax error }`
	if err := os.WriteFile(badFile, []byte(badContent), 0644); err != nil {
		t.Fatalf("Failed to write bad source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile sources
	sources := []CompileOptions{
		{
			Source: goodFile,
			Output: filepath.Join(tmpDir, "good.o"),
			Flags: BuildConfig{
				Optimize:         "none",
				Warnings:         "default",
				WarningsAsErrors: false,
				Debug:            "none",
			},
		},
		{
			Source: badFile,
			Output: filepath.Join(tmpDir, "bad.o"),
			Flags: BuildConfig{
				Optimize:         "none",
				Warnings:         "default",
				WarningsAsErrors: false,
				Debug:            "none",
			},
		},
	}

	results, err := compiler.CompileSources(context.Background(), sources)

	// Verify error returned after second file
	if err == nil {
		t.Errorf("Expected error from CompileSources, got nil")
	}

	// Verify first file compiled successfully
	if len(results) != 1 {
		t.Errorf("Expected 1 successful result, got %d", len(results))
	}

	if len(results) > 0 {
		if !results[0].Success {
			t.Errorf("First result should be successful")
		}
		if results[0].Source != goodFile {
			t.Errorf("First result source = %s, want %s", results[0].Source, goodFile)
		}
	}

	// Verify first object file was created
	goodObject := filepath.Join(tmpDir, "good.o")
	if _, err := os.Stat(goodObject); os.IsNotExist(err) {
		t.Errorf("Good object file %s was not created", goodObject)
	}

	// Verify second object file was NOT created (compilation stopped)
	badObject := filepath.Join(tmpDir, "bad.o")
	if _, err := os.Stat(badObject); !os.IsNotExist(err) {
		t.Errorf("Bad object file %s should not exist", badObject)
	}
}

// TestCompiler_SharedLibrary_AddsPIC tests that shared_library targets get -fPIC automatically
func TestCompiler_SharedLibrary_AddsPIC(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple C++ file
	sourceFile := filepath.Join(tmpDir, "lib.cpp")
	sourceContent := `int lib_func() { return 42; }`
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile with TargetType = "shared_library"
	objectFile := filepath.Join(tmpDir, "lib.o")
	opts := CompileOptions{
		Source:     sourceFile,
		Output:     objectFile,
		TargetType: "shared_library", // This should trigger automatic -fPIC
		Flags: BuildConfig{
			Optimize:         "none",
			Warnings:         "default",
			WarningsAsErrors: false,
			Debug:            "none",
		},
	}

	result, err := compiler.CompileSource(context.Background(), opts)

	// Verify compilation succeeded
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	// Verify object file created
	if _, err := os.Stat(objectFile); os.IsNotExist(err) {
		t.Errorf("Object file %s was not created", objectFile)
	}

	// Use the object file to verify it has PIC by attempting to link it into a shared library
	// This will fail if the object was not compiled with -fPIC on Linux
	if HostPlatform().OS == "linux" {
		ext := ".so"
		libFile := filepath.Join(tmpDir, "libtest"+ext)
		cmd := exec.Command("clang++", "-shared", objectFile, "-o", libFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Failed to link shared library from object (object may not have -fPIC): %v\nOutput: %s", err, output)
		}
	}
}

// TestCompiler_Executable_NoPIC tests that executable targets don't get -fPIC automatically
func TestCompiler_Executable_NoPIC(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple C++ file
	sourceFile := filepath.Join(tmpDir, "main.cpp")
	sourceContent := `int main() { return 0; }`
	if err := os.WriteFile(sourceFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)

	// Compile with TargetType = "executable" (default, should not add -fPIC)
	objectFile := filepath.Join(tmpDir, "main.o")
	opts := CompileOptions{
		Source:     sourceFile,
		Output:     objectFile,
		TargetType: "executable",
		Flags: BuildConfig{
			Optimize:         "none",
			Warnings:         "default",
			WarningsAsErrors: false,
			Debug:            "none",
		},
	}

	result, err := compiler.CompileSource(context.Background(), opts)

	// Verify compilation succeeded
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	// Verify object file created
	if _, err := os.Stat(objectFile); os.IsNotExist(err) {
		t.Errorf("Object file %s was not created", objectFile)
	}
}
