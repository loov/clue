package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommand(t *testing.T) {
	// Build the binary first
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	// Find testdata directory
	testdataDir := filepath.Join("..", "..", "testdata", "sample")

	// Test validate command (flags before command for Go's flag package)
	cmd = exec.Command(binary, "-dir", testdataDir, "-v", "validate")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, out)
	}

	output := string(out)

	// Check expected output
	if !strings.Contains(output, "sample-project") {
		t.Errorf("Expected 'sample-project' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Configuration valid") {
		t.Errorf("Expected 'Configuration valid' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "utils") {
		t.Errorf("Expected 'utils' target in output, got:\n%s", output)
	}
}

func TestValidateWithVariant(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	testdataDir := filepath.Join("..", "..", "testdata", "sample")

	// Test with release variant (flags before command)
	cmd = exec.Command(binary, "-dir", testdataDir, "-variant", "release", "-v", "validate")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate with variant failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "release") {
		t.Errorf("Expected 'release' variant in output, got:\n%s", output)
	}
}

func TestValidateInvalidConfig(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	// Create invalid config (empty sources is invalid)
	invalidDir := t.TempDir()
	invalidConfig := `{
	"name": "invalid",
	"targets": {
		"broken": {
			"name": "broken",
			"type": "invalid_type",
			"sources": []
		}
	}
}`
	if err := os.WriteFile(filepath.Join(invalidDir, "clue.cue"), []byte(invalidConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Should fail validation
	cmd = exec.Command(binary, "-dir", invalidDir, "validate")
	out, _ := cmd.CombinedOutput()

	output := string(out)
	// Should show error (don't check exact message, just that there's an error indicator)
	if !strings.Contains(strings.ToLower(output), "error") {
		t.Errorf("Expected error in output for invalid config, got:\n%s", output)
	}
}

func TestVersionFlag(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	cmd = exec.Command(binary, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "clue version") {
		t.Errorf("Expected version output, got: %s", out)
	}
}

func TestValidateNoConfigError(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	// Create empty directory (no clue.cue)
	emptyDir := t.TempDir()

	// Should fail with "no CUE configuration files found"
	cmd = exec.Command(binary, "-dir", emptyDir, "validate")
	out, _ := cmd.CombinedOutput()

	output := string(out)
	if !strings.Contains(output, "no CUE configuration files found") {
		t.Errorf("Expected 'no CUE configuration files found' in output, got:\n%s", output)
	}
}

func TestCycleDetectionError(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	// Create config with cyclic dependencies
	cycleDir := t.TempDir()
	cycleConfig := `{
	"name": "cyclic",
	"targets": {
		"a": {
			"name": "a",
			"type": "static_library",
			"sources": ["a.cpp"],
			"depends": ["b"]
		},
		"b": {
			"name": "b",
			"type": "static_library",
			"sources": ["b.cpp"],
			"depends": ["a"]
		}
	}
}`
	if err := os.WriteFile(filepath.Join(cycleDir, "clue.cue"), []byte(cycleConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Should fail with cycle detection error
	cmd = exec.Command(binary, "-dir", cycleDir, "validate")
	out, _ := cmd.CombinedOutput()

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "cyclic") {
		t.Errorf("Expected 'cyclic' error in output, got:\n%s", output)
	}
}

func TestBuild_MultiTarget(t *testing.T) {
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	// Get absolute path to testdata
	testdataDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "multi-target"))
	if err != nil {
		t.Fatal(err)
	}

	// Clean first
	exec.Command(binary, "-dir", testdataDir, "clean", "--all").Run()

	// Build the project - must run from project dir due to relative paths
	cmd = exec.Command(binary, "build")
	cmd.Dir = testdataDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Built:") {
		t.Errorf("Expected 'Built:' in output, got:\n%s", output)
	}

	// Check that artifacts exist
	libPath := filepath.Join(testdataDir, "build", "debug", "lib", "libmathlib.a")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Errorf("Expected library at %s, but it doesn't exist", libPath)
	}

	exePath := filepath.Join(testdataDir, "build", "debug", "bin", "calculator")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Errorf("Expected executable at %s, but it doesn't exist", exePath)
	}

	// Run the executable
	cmd = exec.Command(exePath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Executable failed: %v\n%s", err, out)
	}

	output = string(out)
	// Check expected output from calculator
	if !strings.Contains(output, "add(2,3) = 5") {
		t.Errorf("Expected calculation output, got:\n%s", output)
	}
	if !strings.Contains(output, "multiply(4,5) = 20") {
		t.Errorf("Expected multiplication output, got:\n%s", output)
	}
	if !strings.Contains(output, "sqrt(16) = 4") {
		t.Errorf("Expected sqrt output, got:\n%s", output)
	}
}

func TestBuild_Verbose(t *testing.T) {
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	testdataDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "multi-target"))
	if err != nil {
		t.Fatal(err)
	}

	// Clean first
	exec.Command(binary, "clean", "--all").CombinedOutput()

	// Build with verbose flag
	cmd = exec.Command(binary, "-v", "build")
	cmd.Dir = testdataDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	output := string(out)
	// Should contain compiler command
	if !strings.Contains(output, "clang++") {
		t.Errorf("Expected 'clang++' in verbose output, got:\n%s", output)
	}
}

func TestClean_AfterBuild(t *testing.T) {
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	testdataDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "multi-target"))
	if err != nil {
		t.Fatal(err)
	}

	// Build first
	cmd = exec.Command(binary, "build")
	cmd.Dir = testdataDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	// Clean debug artifacts
	cmd = exec.Command(binary, "clean")
	cmd.Dir = testdataDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Clean failed: %v\n%s", err, out)
	}

	// Verify debug dir is removed
	debugDir := filepath.Join(testdataDir, "build", "debug")
	if _, err := os.Stat(debugDir); !os.IsNotExist(err) {
		t.Errorf("Expected debug dir to be removed, but it still exists")
	}

	// Clean all
	cmd = exec.Command(binary, "clean", "--all")
	cmd.Dir = testdataDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Clean all failed: %v\n%s", err, out)
	}

	// Verify build dir is removed or empty
	buildDir := filepath.Join(testdataDir, "build")
	entries, err := os.ReadDir(buildDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("Expected build dir to be empty after clean --all, but found %d entries", len(entries))
	}
}

func TestBuild_SysLibs(t *testing.T) {
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "clue")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, out)
	}

	testdataDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "syslibs-test"))
	if err != nil {
		t.Fatal(err)
	}

	// Clean first
	exec.Command(binary, "clean", "--all").Run()

	// Build with verbose flag to see linker command
	cmd = exec.Command(binary, "-v", "build")
	cmd.Dir = testdataDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\nOutput:\n%s", err, output)
	}

	// Verify -lm appears in linker command
	if !strings.Contains(string(output), "-lm") {
		t.Errorf("expected linker command to contain -lm, got:\n%s", output)
	}

	// Verify executable was created and runs
	exePath := filepath.Join(testdataDir, "build", "debug", "bin", "mathtest")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Fatalf("executable not found at %s", exePath)
	}

	// Run the executable
	runCmd := exec.Command(exePath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executable failed: %v\nOutput:\n%s", err, runOutput)
	}

	// Verify output contains expected result
	if !strings.Contains(string(runOutput), "sqrt(2.0) = 1.41") {
		t.Errorf("unexpected output: %s", runOutput)
	}
}
