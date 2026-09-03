package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// compilerAvailable checks if a compiler is available in PATH
func compilerAvailable(compiler string) bool {
	_, err := exec.LookPath(compiler)
	return err == nil
}

// crossCompilerAvailable checks if a cross-compiler is available for the target platform
func crossCompilerAvailable(target toolchain.Platform) bool {
	// Discover what the cross-compiler would be named
	toolchain, err := NewToolchain("gcc", target)
	if err != nil {
		return false
	}

	// Check if the C compiler exists
	_, err = exec.LookPath(toolchain.CC())
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
	err := os.WriteFile(mainPath, []byte(mainSource), 0o644)
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
	err = os.WriteFile(configPath, []byte(cueConfig), 0o644)
	if err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	return configPath
}

// TestHostPlatform_ReportsSupportedRuntimeTarget verifies that HostPlatform() returns a valid platform
func TestHostPlatform_ReportsSupportedRuntimeTarget(t *testing.T) {
	host := toolchain.HostPlatform()

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
	if !toolchain.IsSupportedTarget(host) {
		t.Errorf("host platform %s should be a supported target", host)
	}

	t.Logf("Detected host platform: %s", host)
}

// TestPlatformAgnosticConfig_BuildsOnHost verifies that a platform-agnostic config can build successfully
// This test verifies Success Criterion 1: Same config works on different platforms
func TestPlatformAgnosticConfig_BuildsOnHost(t *testing.T) {
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
	builder, err := NewBuilder(toolchainName, toolchain.HostPlatform(), VerbosityNormal, 1, false)
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
	ctx := t.Context()
	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !result.Success {
		t.Error("build should succeed with platform-agnostic config")
	}

	// Verify output artifact exists
	execPath := filepath.Join(dir, "build", "debug", "bin", plan.ExecutableName("crossplatform", toolchain.HostPlatform()))
	if _, err := os.Stat(execPath); err != nil {
		t.Errorf("executable should exist: %v", err)
	}

	t.Logf("Successfully built platform-agnostic config on %s using %s", toolchain.HostPlatform(), toolchainName)
}

// TestCrossCompilation_UsesTargetTripletTools verifies cross-compilation toolchain discovery
// This test verifies Success Criterion 2: Cross-compilation uses correct toolchain
func TestCrossCompilation_UsesTargetTripletTools(t *testing.T) {
	// Test cross-compilation to linux-arm64 (if on amd64) or linux-amd64 (if on arm64)
	host := toolchain.HostPlatform()
	var targetPlatform toolchain.Platform

	// Choose a cross-compilation target based on host
	if host.OS == "linux" && host.Arch == "amd64" {
		targetPlatform = toolchain.Platform{OS: "linux", Arch: "arm64"}
	} else if host.OS == "linux" && host.Arch == "arm64" {
		targetPlatform = toolchain.Platform{OS: "linux", Arch: "amd64"}
	} else {
		t.Skip("cross-compilation test requires linux host (amd64 or arm64)")
	}

	// Skip if cross-compiler not available
	if !crossCompilerAvailable(targetPlatform) {
		t.Skipf("cross-compiler not available for %s", targetPlatform)
	}

	// Parse target string
	parsed, err := toolchain.ParseTarget(targetPlatform.String())
	if err != nil {
		t.Fatalf("ParseTarget failed: %v", err)
	}
	if parsed.String() != targetPlatform.String() {
		t.Errorf("ParseTarget returned %s, expected %s", parsed, targetPlatform)
	}

	// Discover toolchain for cross-compilation
	toolchain, err := NewToolchain("gcc", targetPlatform)
	if err != nil {
		t.Fatalf("NewToolchain failed: %v", err)
	}

	// Verify toolchain has correct GNU triplet prefix
	expectedPrefix := ""
	switch targetPlatform.Arch {
	case "arm64":
		expectedPrefix = "aarch64-linux-gnu"
	case "amd64":
		expectedPrefix = "x86_64-linux-gnu"
	}

	if !strings.Contains(toolchain.CC(), expectedPrefix) {
		t.Errorf("CC compiler %s should contain %s for cross-compilation", toolchain.CC(), expectedPrefix)
	}
	if !strings.Contains(toolchain.AR(), expectedPrefix) {
		t.Errorf("AR archiver %s should contain %s for cross-compilation", toolchain.AR(), expectedPrefix)
	}

	t.Logf("Cross-compilation toolchain for %s: CC=%s, CXX=%s, AR=%s",
		targetPlatform, toolchain.CC(), toolchain.CXX(), toolchain.AR())
}

// TestCrossCompilation_RejectsUnavailableCompiler verifies upfront validation of cross-compiler availability
func TestCrossCompilation_RejectsUnavailableCompiler(t *testing.T) {
	// Test with an unavailable cross-compiler (darwin from linux)
	host := toolchain.HostPlatform()
	if host.OS != "linux" {
		t.Skip("cross-compilation validation test requires linux host")
	}

	// Try to build for darwin-arm64 (cross-compiler unlikely to be installed)
	targetPlatform := toolchain.Platform{OS: "darwin", Arch: "arm64"}

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

// TestCrossCompilerNaming_MapsGNUTriplets verifies GNU triplet prefix mapping
func TestCrossCompilerNaming_MapsGNUTriplets(t *testing.T) {
	tests := []struct {
		platform       toolchain.Platform
		expectedPrefix string
	}{
		{toolchain.Platform{OS: "linux", Arch: "arm64"}, "aarch64-linux-gnu-"},
		{toolchain.Platform{OS: "linux", Arch: "amd64"}, "x86_64-linux-gnu-"},
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

// TestSemanticFlagMapping_TranslatesCompilerOptions verifies semantic flag translation to compiler-specific flags
// This test verifies Success Criterion 3: Semantic flags map correctly
func TestSemanticFlagMapping_TranslatesCompilerOptions(t *testing.T) {
	tests := []struct {
		name      string
		config    toolchain.Config
		toolchain string
		expected  []string
	}{
		{
			name:      "optimization fast",
			config:    toolchain.Config{Optimize: "fast"},
			toolchain: "gcc",
			expected:  []string{"-O2"},
		},
		{
			name:      "optimization size",
			config:    toolchain.Config{Optimize: "size"},
			toolchain: "clang",
			expected:  []string{"-Os"},
		},
		{
			name:      "warnings strict",
			config:    toolchain.Config{Warnings: "strict"},
			toolchain: "gcc",
			expected:  []string{"-Wall", "-Wextra"},
		},
		{
			name:      "debug full",
			config:    toolchain.Config{Debug: "full"},
			toolchain: "clang",
			expected:  []string{"-g"},
		},
		{
			name:      "sanitizer address",
			config:    toolchain.Config{Sanitizers: []string{"address"}},
			toolchain: "clang",
			expected:  []string{"-fsanitize=address"},
		},
		{
			name:      "sanitizer undefined",
			config:    toolchain.Config{Sanitizers: []string{"undefined"}},
			toolchain: "gcc",
			expected:  []string{"-fsanitize=undefined"},
		},
		{
			name:      "lto enabled",
			config:    toolchain.Config{LTO: true},
			toolchain: "clang",
			expected:  []string{"-flto"},
		},
		{
			name:      "pic enabled",
			config:    toolchain.Config{PIC: true},
			toolchain: "gcc",
			expected:  []string{"-fPIC"},
		},
		{
			name:      "coverage clang",
			config:    toolchain.Config{Coverage: true},
			toolchain: "clang",
			expected:  []string{"-fprofile-instr-generate", "-fcoverage-mapping"},
		},
		{
			name:      "coverage gcc",
			config:    toolchain.Config{Coverage: true},
			toolchain: "gcc",
			expected:  []string{"-fprofile-arcs", "-ftest-coverage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewToolchain(tt.toolchain, toolchain.HostPlatform())
			if err != nil {
				t.Fatalf("NewToolchain failed: %v", err)
			}
			flags := tc.CompilerFlags(tt.config)

			// Verify all expected flags are present
			for _, expectedFlag := range tt.expected {
				found := slices.Contains(flags, expectedFlag)
				if !found {
					t.Errorf("expected flag %s not found in %v", expectedFlag, flags)
				}
			}
		})
	}
}

// TestSemanticFlagMapping_TranslatesLinkerOptions verifies semantic flag translation for linker
func TestSemanticFlagMapping_TranslatesLinkerOptions(t *testing.T) {
	tests := []struct {
		name      string
		config    toolchain.Config
		toolchain string
		expected  []string
	}{
		{
			name:      "debug full in linker",
			config:    toolchain.Config{Debug: "full"},
			toolchain: "gcc",
			expected:  []string{"-g"},
		},
		{
			name:      "sanitizer address in linker",
			config:    toolchain.Config{Sanitizers: []string{"address"}},
			toolchain: "clang",
			expected:  []string{"-fsanitize=address"},
		},
		{
			name:      "lto in linker",
			config:    toolchain.Config{LTO: true},
			toolchain: "gcc",
			expected:  []string{"-flto"},
		},
		{
			name:      "coverage clang in linker",
			config:    toolchain.Config{Coverage: true},
			toolchain: "clang",
			expected:  []string{"-fprofile-instr-generate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewToolchain(tt.toolchain, toolchain.HostPlatform())
			if err != nil {
				t.Fatalf("NewToolchain failed: %v", err)
			}
			flags := tc.LinkerFlags(tt.config, []string{})

			// Verify all expected flags are present
			for _, expectedFlag := range tt.expected {
				found := slices.Contains(flags, expectedFlag)
				if !found {
					t.Errorf("expected linker flag %s not found in %v", expectedFlag, flags)
				}
			}
		})
	}
}

// TestSharedLibraryExtension_UsesPlatformSuffix verifies platform-specific shared library extensions
// This test verifies Success Criterion 4: Platform-specific file extensions
func TestSharedLibraryExtension_UsesPlatformSuffix(t *testing.T) {
	tests := []struct {
		platform toolchain.Platform
		expected string
	}{
		{toolchain.Platform{OS: "linux", Arch: "amd64"}, ".so"},
		{toolchain.Platform{OS: "linux", Arch: "arm64"}, ".so"},
		{toolchain.Platform{OS: "darwin", Arch: "amd64"}, ".dylib"},
		{toolchain.Platform{OS: "darwin", Arch: "arm64"}, ".dylib"},
		{toolchain.Platform{OS: "windows", Arch: "amd64"}, ".dll"},
	}

	for _, tt := range tests {
		t.Run(tt.platform.String(), func(t *testing.T) {
			ext := plan.SharedLibraryExtension(tt.platform)
			if ext != tt.expected {
				t.Errorf("SharedLibraryExtension(%s) = %s, want %s",
					tt.platform, ext, tt.expected)
			}
		})
	}
}

// TestOutputPath_UsesPlatformSuffix verifies that Builder.OutputPath uses correct extensions
func TestOutputPath_UsesPlatformSuffix(t *testing.T) {
	tests := []struct {
		name         string
		platform     toolchain.Platform
		targetType   string
		expectedExt  string
		expectedPath string // path fragment to verify
	}{
		{
			name:         "linux shared library",
			platform:     toolchain.Platform{OS: "linux", Arch: "amd64"},
			targetType:   "shared_library",
			expectedExt:  ".so",
			expectedPath: filepath.Join("lib", "libmylib.so"),
		},
		{
			name:         "darwin shared library",
			platform:     toolchain.Platform{OS: "darwin", Arch: "arm64"},
			targetType:   "shared_library",
			expectedExt:  ".dylib",
			expectedPath: filepath.Join("lib", "libmylib.dylib"),
		},
		{
			name:         "linux static library",
			platform:     toolchain.Platform{OS: "linux", Arch: "amd64"},
			targetType:   "static_library",
			expectedExt:  ".a",
			expectedPath: filepath.Join("lib", "libmylib.a"),
		},
		{
			name:         "darwin static library",
			platform:     toolchain.Platform{OS: "darwin", Arch: "amd64"},
			targetType:   "static_library",
			expectedExt:  ".a",
			expectedPath: filepath.Join("lib", "libmylib.a"),
		},
		{
			name:         "linux executable",
			platform:     toolchain.Platform{OS: "linux", Arch: "arm64"},
			targetType:   "executable",
			expectedExt:  "",
			expectedPath: filepath.Join("bin", "mylib"),
		},
		{
			name:         "windows shared library",
			platform:     toolchain.Platform{OS: "windows", Arch: "amd64"},
			targetType:   "shared_library",
			expectedExt:  ".dll",
			expectedPath: filepath.Join("lib", "mylib.dll"),
		},
		{
			name:         "windows static library",
			platform:     toolchain.Platform{OS: "windows", Arch: "amd64"},
			targetType:   "static_library",
			expectedExt:  ".lib",
			expectedPath: filepath.Join("lib", "mylib.lib"),
		},
		{
			name:         "windows executable",
			platform:     toolchain.Platform{OS: "windows", Arch: "amd64"},
			targetType:   "executable",
			expectedExt:  ".exe",
			expectedPath: filepath.Join("bin", "mylib.exe"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &Builder{target: tt.platform}
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
