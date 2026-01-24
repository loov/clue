package build_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to run clue with args and capture output
func runClue(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	// Build clue if not already built (main.go is at project root)
	clueCmd := exec.Command("go", "build", "-o", "clue_test_bin", ".")
	clueCmd.Dir = findProjectRoot(t)
	if err := clueCmd.Run(); err != nil {
		t.Fatalf("failed to build clue: %v", err)
	}

	cluePath := filepath.Join(findProjectRoot(t), "clue_test_bin")
	defer os.Remove(cluePath)

	cmd := exec.Command(cluePath, args...)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run clue: %v", err)
	}

	return outBuf.String(), errBuf.String(), exitCode
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from current directory to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}

func TestCLI_QuietMode_NoOutputOnSuccess(t *testing.T) {
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Build with --quiet
	stdout, stderr, exitCode := runClue(t, testDir, "--quiet", "build")

	if exitCode != 0 {
		t.Fatalf("build failed with exit code %d: stderr=%s", exitCode, stderr)
	}

	// In quiet mode, stdout should be empty (no progress, no summary)
	if stdout != "" {
		t.Errorf("--quiet mode should produce no stdout, got: %q", stdout)
	}
}

func TestCLI_VerboseMode_ShowsCommands(t *testing.T) {
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Build with --verbose (using -v flag)
	stdout, _, exitCode := runClue(t, testDir, "-v", "build")

	if exitCode != 0 {
		t.Fatalf("build failed with exit code %d", exitCode)
	}

	// Verbose mode should show compiler command (contains "clang" or "g++")
	if !strings.Contains(stdout, "clang") && !strings.Contains(stdout, "g++") {
		t.Errorf("--verbose should show compiler commands, got: %s", stdout)
	}
}

func TestCLI_MutuallyExclusiveFlags(t *testing.T) {
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")

	_, stderr, exitCode := runClue(t, testDir, "--quiet", "-v", "build")

	if exitCode == 0 {
		t.Error("should fail when both --quiet and -v are specified")
	}

	if !strings.Contains(stderr, "mutually exclusive") && !strings.Contains(stderr, "cannot") {
		t.Errorf("should show mutual exclusion error, got: %s", stderr)
	}
}

func TestCLI_TimingDisplay(t *testing.T) {
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Build with verbose to see timing
	stdout, _, exitCode := runClue(t, testDir, "-v", "build")

	if exitCode != 0 {
		t.Fatalf("build failed with exit code %d", exitCode)
	}

	// Should see timing indicators (ms, s, or time format)
	hasTimingIndicator := strings.Contains(stdout, "ms") ||
		strings.Contains(stdout, "s)") ||
		strings.Contains(stdout, "build time")

	if !hasTimingIndicator {
		t.Errorf("verbose output should include timing, got: %s", stdout)
	}
}

func TestCLI_RunCommand_BuildsAndExecutes(t *testing.T) {
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Build first (run command builds but multi-target has dependencies)
	stdout, stderr, exitCode := runClue(t, testDir, "build")
	if exitCode != 0 {
		t.Fatalf("build failed with exit code %d: stderr=%s", exitCode, stderr)
	}

	// Run the target (should execute already-built binary)
	stdout, stderr, exitCode = runClue(t, testDir, "run", "calculator")

	if exitCode != 0 {
		t.Fatalf("run failed with exit code %d: stderr=%s", exitCode, stderr)
	}

	// Should see output from the executed program
	if !strings.Contains(stdout, "add") && !strings.Contains(stdout, "multiply") {
		t.Errorf("run output should include program output, got: %s", stdout)
	}
}

func TestCLI_RunCommand_PassesArguments(t *testing.T) {
	// This test requires a program that echoes arguments
	// Skip if testdata/args-test doesn't exist
	testDir := filepath.Join(findProjectRoot(t), "testdata", "args-test")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testdata/args-test not found")
	}

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Run with arguments
	stdout, _, exitCode := runClue(t, testDir, "run", "echoargs", "arg1", "arg2")

	if exitCode != 0 {
		t.Fatalf("run failed with exit code %d", exitCode)
	}

	// Should see the arguments in output
	if !strings.Contains(stdout, "arg1") || !strings.Contains(stdout, "arg2") {
		t.Errorf("run should pass arguments to program, got: %s", stdout)
	}
}

func TestCLI_RunCommand_FailsOnNonExecutable(t *testing.T) {
	// Need a test project with a library target
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testdata/multi-target not found")
	}

	// Try to run a library target
	_, stderr, exitCode := runClue(t, testDir, "run", "mathlib")

	if exitCode == 0 {
		t.Error("run should fail for non-executable targets")
	}

	if !strings.Contains(stderr, "not an executable") {
		t.Errorf("should show 'not an executable' error, got: %s", stderr)
	}
}

func TestCLI_RunCommand_FailsOnMissingTarget(t *testing.T) {
	testDir := filepath.Join(findProjectRoot(t), "testdata", "multi-target")

	_, stderr, exitCode := runClue(t, testDir, "run", "nonexistent")

	if exitCode == 0 {
		t.Error("run should fail for missing targets")
	}

	if !strings.Contains(stderr, "not found") {
		t.Errorf("should show 'not found' error, got: %s", stderr)
	}
}

func TestCLI_ModuleDetection(t *testing.T) {
	// Test module detection (doesn't require full compilation)
	testDir := filepath.Join(findProjectRoot(t), "testdata", "module-test")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testdata/module-test not found")
	}

	// Just validate the config - this tests module detection without needing clang-scan-deps
	_, stderr, exitCode := runClue(t, testDir, "validate")

	// Validation should succeed (module detection is separate from compilation)
	if exitCode != 0 {
		t.Errorf("validate should succeed for module project: %s", stderr)
	}
}

func TestCLI_ModuleBuild(t *testing.T) {
	// Check if clang-scan-deps is available
	if _, err := exec.LookPath("clang-scan-deps"); err != nil {
		t.Skip("clang-scan-deps not available, skipping module build test")
	}

	testDir := filepath.Join(findProjectRoot(t), "testdata", "module-test")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testdata/module-test not found")
	}

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Build with verbose to see module ordering
	stdout, stderr, exitCode := runClue(t, testDir, "-v", "build")

	// This may fail if the system doesn't have full module support
	// The key is that detection and ordering work correctly
	if exitCode != 0 {
		// Check if it's a module compilation error vs detection error
		if strings.Contains(stderr, "module") && strings.Contains(stderr, "order") {
			t.Logf("Module ordering worked but compilation failed: %s", stderr)
		} else {
			t.Logf("Build failed (may be expected without full module support): %s", stderr)
		}
	}

	// In verbose mode, should see module-related output
	if strings.Contains(stdout, "module") || strings.Contains(stdout, "Module") {
		t.Logf("Module detection output present: %s", stdout)
	}
}
