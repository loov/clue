package build_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/testclue"
	"github.com/loov/clue/internal/toolchain"
)

// TestJSONExample_BuildsAndRuns tests the json-example testdata project
func TestJSONExample_BuildsAndRuns(t *testing.T) {
	testclue.SkipIfNoClangPP(t)

	projectDir, err := filepath.Abs("../../testdata/json-example")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Verify project exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Skip("testdata/json-example not found")
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Build
	buildDir := ".build"
	t.Cleanup(func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Errorf("remove build directory: %v", err)
		}
	})

	builder, err := build.NewBuilder("clang", toolchain.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	opts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityNormal,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Build was not successful")
	}

	// Run executable
	exePath := filepath.Join(buildDir, "debug", "bin", "json-example")
	cmd := exec.Command(exePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run executable: %v\nOutput: %s", err, output)
	}

	// Validate output
	outputStr := string(output)
	expectedStrings := []string{
		"Parsed name: example-project",
		"Version: 1",
		"Items count: 2",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Output missing expected string %q\nGot: %s", expected, outputStr)
		}
	}
}

// TestCatch2Example_BuildsAndRuns tests the catch2-example testdata project
func TestCatch2Example_BuildsAndRuns(t *testing.T) {
	testclue.SkipIfNoClangPP(t)

	projectDir, err := filepath.Abs("../../testdata/catch2-example")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Skip("testdata/catch2-example not found")
	}

	t.Chdir(projectDir)

	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	buildDir := ".build"
	t.Cleanup(func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Errorf("remove build directory: %v", err)
		}
	})

	builder, err := build.NewBuilder("clang", toolchain.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	opts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityNormal,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Build was not successful")
	}

	// Run test executable - Catch2 returns 0 if all tests pass
	exePath := filepath.Join(buildDir, "debug", "bin", "catch2-tests")
	cmd := exec.Command(exePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Tests failed (non-zero exit): %v\nOutput: %s", err, output)
	}

	// Verify Catch2 output indicates tests ran
	outputStr := string(output)
	expectedStrings := []string{
		"test case",
		"passed",
	}
	foundAny := false
	for _, expected := range expectedStrings {
		if strings.Contains(strings.ToLower(outputStr), expected) {
			foundAny = true
			break
		}
	}
	if !foundAny {
		t.Logf("Catch2 output: %s", outputStr)
	}
}

// TestMultiDependencyExample_BuildsAndRuns tests the multi-deps-example testdata project
func TestMultiDependencyExample_BuildsAndRuns(t *testing.T) {
	testclue.SkipIfNoClangPP(t)

	projectDir, err := filepath.Abs("../../testdata/multi-deps-example")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Skip("testdata/multi-deps-example not found")
	}

	t.Chdir(projectDir)

	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	buildDir := ".build"
	t.Cleanup(func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Errorf("remove build directory: %v", err)
		}
	})

	builder, err := build.NewBuilder("clang", toolchain.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	opts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityNormal,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Build was not successful")
	}

	// Run executable
	exePath := filepath.Join(buildDir, "debug", "bin", "multi-deps-example")
	cmd := exec.Command(exePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run executable: %v\nOutput: %s", err, output)
	}

	// Validate output - verify both libraries contributed
	outputStr := string(output)
	expectedStrings := []string{
		"10 + 5 = 15",         // stringutils using simplemath
		"7 * 8 = 56",          // stringutils using simplemath
		"9^2 = 81",            // stringutils using simplemath::square
		"add(100, 200) = 300", // direct simplemath usage
		"Multi-deps example complete!",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Output missing expected string %q\nGot: %s", expected, outputStr)
		}
	}
}
