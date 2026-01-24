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

	dir := t.TempDir()

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
	builder, err := NewBuilder(toolchainName, HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	opts := Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  filepath.Join(dir, "build"),
		Verbosity: VerbosityNormal,
		Jobs:      1,
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
	switch targetPlatform.Arch {
	case "arm64":
		expectedPrefix = "aarch64-linux-gnu"
	case "amd64":
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
	_, err := NewBuilder("clang", targetPlatform, VerbosityNormal, 1, false)

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

// TestSemanticFlagMapping verifies semantic flag translation to compiler-specific flags
// This test verifies Success Criterion 3: Semantic flags map correctly
func TestSemanticFlagMapping(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		toolchain string
		expected  []string
	}{
		{
			name:      "optimization fast",
			config:    Config{Optimize: "fast"},
			toolchain: "gcc",
			expected:  []string{"-O2"},
		},
		{
			name:      "optimization size",
			config:    Config{Optimize: "size"},
			toolchain: "clang",
			expected:  []string{"-Os"},
		},
		{
			name:      "warnings strict",
			config:    Config{Warnings: "strict"},
			toolchain: "gcc",
			expected:  []string{"-Wall", "-Wextra"},
		},
		{
			name:      "debug full",
			config:    Config{Debug: "full"},
			toolchain: "clang",
			expected:  []string{"-g"},
		},
		{
			name:      "sanitizer address",
			config:    Config{Sanitizers: []string{"address"}},
			toolchain: "clang",
			expected:  []string{"-fsanitize=address"},
		},
		{
			name:      "sanitizer undefined",
			config:    Config{Sanitizers: []string{"undefined"}},
			toolchain: "gcc",
			expected:  []string{"-fsanitize=undefined"},
		},
		{
			name:      "lto enabled",
			config:    Config{LTO: true},
			toolchain: "clang",
			expected:  []string{"-flto"},
		},
		{
			name:      "pic enabled",
			config:    Config{PIC: true},
			toolchain: "gcc",
			expected:  []string{"-fPIC"},
		},
		{
			name:      "coverage clang",
			config:    Config{Coverage: true},
			toolchain: "clang",
			expected:  []string{"-fprofile-instr-generate", "-fcoverage-mapping"},
		},
		{
			name:      "coverage gcc",
			config:    Config{Coverage: true},
			toolchain: "gcc",
			expected:  []string{"-fprofile-arcs", "-ftest-coverage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := CompilerFlagsWithToolchain(tt.config, tt.toolchain)

			// Verify all expected flags are present
			for _, expectedFlag := range tt.expected {
				found := false
				for _, flag := range flags {
					if flag == expectedFlag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected flag %s not found in %v", expectedFlag, flags)
				}
			}
		})
	}
}

// TestSemanticFlagMapping_Linker verifies semantic flag translation for linker
func TestSemanticFlagMapping_Linker(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		toolchain string
		expected  []string
	}{
		{
			name:      "debug full in linker",
			config:    Config{Debug: "full"},
			toolchain: "gcc",
			expected:  []string{"-g"},
		},
		{
			name:      "sanitizer address in linker",
			config:    Config{Sanitizers: []string{"address"}},
			toolchain: "clang",
			expected:  []string{"-fsanitize=address"},
		},
		{
			name:      "lto in linker",
			config:    Config{LTO: true},
			toolchain: "gcc",
			expected:  []string{"-flto"},
		},
		{
			name:      "coverage clang in linker",
			config:    Config{Coverage: true},
			toolchain: "clang",
			expected:  []string{"-fprofile-instr-generate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := LinkerFlagsWithToolchain(tt.config, []string{}, tt.toolchain)

			// Verify all expected flags are present
			for _, expectedFlag := range tt.expected {
				found := false
				for _, flag := range flags {
					if flag == expectedFlag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected linker flag %s not found in %v", expectedFlag, flags)
				}
			}
		})
	}
}

// TestPlatformSpecificExtensions verifies platform-specific shared library extensions
// This test verifies Success Criterion 4: Platform-specific file extensions
func TestPlatformSpecificExtensions(t *testing.T) {
	tests := []struct {
		platform Platform
		expected string
	}{
		{Platform{OS: "linux", Arch: "amd64"}, ".so"},
		{Platform{OS: "linux", Arch: "arm64"}, ".so"},
		{Platform{OS: "darwin", Arch: "amd64"}, ".dylib"},
		{Platform{OS: "darwin", Arch: "arm64"}, ".dylib"},
		{Platform{OS: "windows", Arch: "amd64"}, ".dll"},
	}

	for _, tt := range tests {
		t.Run(tt.platform.String(), func(t *testing.T) {
			ext := SharedLibraryExtension(tt.platform)
			if ext != tt.expected {
				t.Errorf("SharedLibraryExtension(%s) = %s, want %s",
					tt.platform, ext, tt.expected)
			}
		})
	}
}

// TestOutputPathExtensions verifies that Builder.OutputPath uses correct extensions
func TestOutputPathExtensions(t *testing.T) {
	tests := []struct {
		name         string
		platform     Platform
		targetType   string
		expectedExt  string
		expectedPath string // path fragment to verify
	}{
		{
			name:         "linux shared library",
			platform:     Platform{OS: "linux", Arch: "amd64"},
			targetType:   "shared_library",
			expectedExt:  ".so",
			expectedPath: "lib/libmylib.so",
		},
		{
			name:         "darwin shared library",
			platform:     Platform{OS: "darwin", Arch: "arm64"},
			targetType:   "shared_library",
			expectedExt:  ".dylib",
			expectedPath: "lib/libmylib.dylib",
		},
		{
			name:         "linux static library",
			platform:     Platform{OS: "linux", Arch: "amd64"},
			targetType:   "static_library",
			expectedExt:  ".a",
			expectedPath: "lib/libmylib.a",
		},
		{
			name:         "darwin static library",
			platform:     Platform{OS: "darwin", Arch: "amd64"},
			targetType:   "static_library",
			expectedExt:  ".a",
			expectedPath: "lib/libmylib.a",
		},
		{
			name:         "linux executable",
			platform:     Platform{OS: "linux", Arch: "arm64"},
			targetType:   "executable",
			expectedExt:  "",
			expectedPath: "bin/mylib",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create builder with target platform
			builder, err := NewBuilder("clang", tt.platform, VerbosityNormal, 1, false)
			if err != nil {
				// If toolchain discovery fails, skip (cross-compiler may not be available)
				t.Skipf("toolchain not available for %s: %v", tt.platform, err)
			}

			// Get output path
			outputPath := builder.OutputPath("/build", "debug", "mylib", tt.targetType)

			// Verify extension
			if tt.expectedExt != "" {
				if !strings.HasSuffix(outputPath, tt.expectedExt) {
					t.Errorf("OutputPath should end with %s, got %s", tt.expectedExt, outputPath)
				}
			}

			// Verify path contains expected fragment
			if !strings.Contains(outputPath, tt.expectedPath) {
				t.Errorf("OutputPath should contain %s, got %s", tt.expectedPath, outputPath)
			}

			t.Logf("OutputPath for %s %s: %s", tt.platform, tt.targetType, outputPath)
		})
	}
}
