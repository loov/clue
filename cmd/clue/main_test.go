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
