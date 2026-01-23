package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
)

// compilerAvailable checks if a compiler is available in PATH
func compilerAvailable(compiler string) bool {
	_, err := exec.LookPath(compiler)
	return err == nil
}

// crossCompilerAvailable checks if a cross-compiler is available for the target platform
func crossCompilerAvailable(target Platform) bool {
	// Discover what the cross-compiler would be named
	toolchain, err := DiscoverToolchain("gcc", target)
	if err != nil {
		return false
	}

	// Check if the C compiler exists
	_, err = exec.LookPath(toolchain.CC)
	return err == nil
}

// createCrossPlatformTestProject creates a minimal C++ project for cross-platform testing
func createCrossPlatformTestProject(t *testing.T, dir string) string {
	t.Helper()

	// Create simple C++ hello world
	mainSource := `#include <iostream>

int main() {
    std::cout << "Hello from cross-platform test" << std::endl;
    return 0;
}
`
	mainPath := filepath.Join(dir, "main.cpp")
	err := os.WriteFile(mainPath, []byte(mainSource), 0644)
	if err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create minimal CUE config that works on any platform
	cueConfig := fmt.Sprintf(`name: "crossplatform"
version: "1.0.0"

toolchain: {
    compiler: "clang"
    std: "c++17"
}

targets: {
    crossplatform: {
        name: "crossplatform"
        type: "executable"
        sources: [%q]
    }
}
`, mainPath)

	configPath := filepath.Join(dir, "clue.cue")
	err = os.WriteFile(configPath, []byte(cueConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	return configPath
}

// TestPlatformDetection verifies that HostPlatform() returns a valid platform
func TestPlatformDetection(t *testing.T) {
	host := HostPlatform()

	// Verify the platform string is in expected format (os-arch)
	platformStr := host.String()
	parts := strings.Split(platformStr, "-")
	if len(parts) != 2 {
		t.Errorf("expected platform format 'os-arch', got %s", platformStr)
	}

	// Verify OS and Arch fields are populated
	if host.OS == "" {
		t.Error("HostPlatform() returned empty OS")
	}
	if host.Arch == "" {
		t.Error("HostPlatform() returned empty Arch")
	}

	// Verify current platform is supported
	if !IsSupportedTarget(host) {
		t.Errorf("host platform %s should be a supported target", host)
	}

	t.Logf("Detected host platform: %s", host)
}

// TestSameConfigMultiplePlatforms verifies that a platform-agnostic config can build successfully
// This test verifies Success Criterion 1: Same config works on different platforms
func TestSameConfigMultiplePlatforms(t *testing.T) {
	// Skip if no C++ compiler available
	if !compilerAvailable("clang++") && !compilerAvailable("g++") {
		t.Skip("no C++ compiler available (clang++ or g++)")
	}

	dir, err := os.MkdirTemp("", "clue-cross-platform-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create test project with platform-agnostic config
	createCrossPlatformTestProject(t, dir)

	// Load config
	loader := config.NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Determine which compiler to use
	toolchainName := "clang"
	if !compilerAvailable("clang++") {
		toolchainName = "gcc"
	}

	// Build with native toolchain for current platform
	builder, err := NewBuilder(toolchainName, HostPlatform(), false, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	opts := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(dir, "build"),
		Verbose:  false,
		Jobs:     1,
	}

	// Build
	ctx := context.Background()
	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !result.Success {
		t.Error("build should succeed with platform-agnostic config")
	}

	// Verify output artifact exists
	execPath := filepath.Join(dir, "build", "debug", "bin", "crossplatform")
	if _, err := os.Stat(execPath); err != nil {
		t.Errorf("executable should exist: %v", err)
	}

	t.Logf("Successfully built platform-agnostic config on %s using %s", HostPlatform(), toolchainName)
}

// TestCrossCompilationTarget verifies cross-compilation toolchain discovery
// This test verifies Success Criterion 2: Cross-compilation uses correct toolchain
func TestCrossCompilationTarget(t *testing.T) {
	// Test cross-compilation to linux-arm64 (if on amd64) or linux-amd64 (if on arm64)
	host := HostPlatform()
	var targetPlatform Platform

	// Choose a cross-compilation target based on host
	if host.OS == "linux" && host.Arch == "amd64" {
		targetPlatform = Platform{OS: "linux", Arch: "arm64"}
	} else if host.OS == "linux" && host.Arch == "arm64" {
		targetPlatform = Platform{OS: "linux", Arch: "amd64"}
	} else {
		t.Skip("cross-compilation test requires linux host (amd64 or arm64)")
	}

	// Skip if cross-compiler not available
	if !crossCompilerAvailable(targetPlatform) {
		t.Skipf("cross-compiler not available for %s", targetPlatform)
	}

	// Parse target string
	parsed, err := ParseTarget(targetPlatform.String())
	if err != nil {
		t.Fatalf("ParseTarget failed: %v", err)
	}
	if parsed.String() != targetPlatform.String() {
		t.Errorf("ParseTarget returned %s, expected %s", parsed, targetPlatform)
	}

	// Discover toolchain for cross-compilation
	toolchain, err := DiscoverToolchain("gcc", targetPlatform)
	if err != nil {
		t.Fatalf("DiscoverToolchain failed: %v", err)
	}

	// Verify toolchain has correct GNU triplet prefix
	expectedPrefix := ""
	if targetPlatform.Arch == "arm64" {
		expectedPrefix = "aarch64-linux-gnu"
	} else if targetPlatform.Arch == "amd64" {
		expectedPrefix = "x86_64-linux-gnu"
	}

	if !strings.Contains(toolchain.CC, expectedPrefix) {
		t.Errorf("CC compiler %s should contain %s for cross-compilation", toolchain.CC, expectedPrefix)
	}
	if !strings.Contains(toolchain.AR, expectedPrefix) {
		t.Errorf("AR archiver %s should contain %s for cross-compilation", toolchain.AR, expectedPrefix)
	}

	t.Logf("Cross-compilation toolchain for %s: CC=%s, CXX=%s, AR=%s",
		targetPlatform, toolchain.CC, toolchain.CXX, toolchain.AR)
}

// TestCrossCompilationValidation verifies upfront validation of cross-compiler availability
func TestCrossCompilationValidation(t *testing.T) {
	// Test with an unavailable cross-compiler (darwin from linux)
	host := HostPlatform()
	if host.OS != "linux" {
		t.Skip("cross-compilation validation test requires linux host")
	}

	// Try to build for darwin-arm64 (cross-compiler unlikely to be installed)
	targetPlatform := Platform{OS: "darwin", Arch: "arm64"}

	// Create builder - should fail during toolchain validation
	_, err := NewBuilder("clang", targetPlatform, false, 1, false)

	// Should get an error about missing cross-compiler
	if err == nil {
		// If no error, the cross-compiler might actually be installed (unusual but possible)
		t.Skip("darwin cross-compiler is installed, cannot test validation failure")
	}

	// Verify error message is clear
	errMsg := err.Error()
	if !strings.Contains(errMsg, "compiler not found") && !strings.Contains(errMsg, "toolchain") {
		t.Errorf("expected clear error about missing compiler, got: %s", errMsg)
	}

	t.Logf("Cross-compilation validation correctly failed: %v", err)
}

// TestCrossCompilerNaming verifies GNU triplet prefix mapping
func TestCrossCompilerNaming(t *testing.T) {
	tests := []struct {
		platform       Platform
		expectedPrefix string
	}{
		{Platform{OS: "linux", Arch: "arm64"}, "aarch64-linux-gnu-"},
		{Platform{OS: "linux", Arch: "amd64"}, "x86_64-linux-gnu-"},
	}

	for _, tt := range tests {
		t.Run(tt.platform.String(), func(t *testing.T) {
			// Use the internal gnuTripletPrefix function to test mapping
			prefix := gnuTripletPrefix(tt.platform)
			if prefix != tt.expectedPrefix {
				t.Errorf("gnuTripletPrefix(%s) = %s, want %s",
					tt.platform, prefix, tt.expectedPrefix)
			}
		})
	}
}
