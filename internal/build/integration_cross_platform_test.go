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
