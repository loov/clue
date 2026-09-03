package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to run clue with args and capture output
func runClue(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	// Create temp directory for test binary (auto-cleaned by Go test framework)
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "clue_test_bin")

	// Build clue in temp directory (main.go is at project root)
	clueCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	clueCmd.Dir = "."
	if err := clueCmd.Run(); err != nil {
		t.Fatalf("failed to build clue: %v", err)
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run clue: %v", err)
	}

	return outBuf.String(), errBuf.String(), exitCode
}

func TestCLI_HelpListsCommandsAndGlobalFlags(t *testing.T) {
	stdout, stderr, exitCode := runClue(t, ".", "--help")
	if exitCode != 0 {
		t.Fatalf("help failed with exit code %d: %s", exitCode, stderr)
	}
	for _, text := range []string{"Available commands:", "build", "deps", "generate", "--variant", "--jobs"} {
		if !strings.Contains(stdout, text) {
			t.Errorf("help output does not contain %q:\n%s", text, stdout)
		}
	}
}

func TestCLI_QuietMode_NoOutputOnSuccess(t *testing.T) {
	testDir := filepath.Join("testdata", "multi-target")

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
	testDir := filepath.Join("testdata", "multi-target")

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

func TestCLI_RejectsMutuallyExclusiveFlags(t *testing.T) {
	testDir := filepath.Join("testdata", "multi-target")

	_, stderr, exitCode := runClue(t, testDir, "--quiet", "-v", "build")

	if exitCode == 0 {
		t.Error("should fail when both --quiet and -v are specified")
	}

	if !strings.Contains(stderr, "mutually exclusive") && !strings.Contains(stderr, "cannot") {
		t.Errorf("should show mutual exclusion error, got: %s", stderr)
	}
}

func TestCLI_AcceptsFlagsAfterCommand(t *testing.T) {
	testDir := filepath.Join("testdata", "sample")

	stdout, stderr, exitCode := runClue(t, ".", "validate", "-dir", testDir, "-variant", "release")
	if exitCode != 0 {
		t.Fatalf("validate failed with exit code %d: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "Applied variant: release") {
		t.Errorf("post-command variant flag was not applied:\n%s", stdout)
	}
}

func TestCLI_DirBuildsFromProjectDirectory(t *testing.T) {
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	projectDir := t.TempDir()
	config := `name: "dir-test"
version: "1.0.0"
buildDir: "output"
toolchain: {
	compiler: "clang"
	std: "c++17"
}
targets: app: {
	name: "app"
	type: "executable"
	sources: ["main.cpp"]
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.cpp"), []byte("int main() { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, exitCode := runClue(t, t.TempDir(), "-dir", projectDir, "build")
	if exitCode != 0 {
		t.Fatalf("build failed with exit code %d: %s", exitCode, stderr)
	}
	artifact := filepath.Join(projectDir, "output", "debug", "bin", "app")
	if _, err := os.Stat(artifact); err != nil {
		t.Errorf("artifact was not written below the project directory: %v", err)
	}

	_, stderr, exitCode = runClue(t, t.TempDir(), "-dir", projectDir, "clean", "-all")
	if exitCode != 0 {
		t.Fatalf("clean failed with exit code %d: %s", exitCode, stderr)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "output")); !os.IsNotExist(err) {
		t.Errorf("configured build directory was not cleaned, stat error: %v", err)
	}
}

func TestCLI_BuildRejectsUnknownTarget(t *testing.T) {
	testDir := filepath.Join("testdata", "sample")

	_, stderr, exitCode := runClue(t, testDir, "build", "missing")
	if exitCode == 0 {
		t.Fatal("build accepted an unknown target")
	}
	if !strings.Contains(stderr, `target "missing" not found`) {
		t.Errorf("unexpected error: %s", stderr)
	}
}

func TestCLI_VerboseModeShowsCompilationTimes(t *testing.T) {
	testDir := filepath.Join("testdata", "multi-target")

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
	testDir := filepath.Join("testdata", "multi-target")

	// Clean first
	runClue(t, testDir, "--all", "clean")

	// Build first (run command builds but multi-target has dependencies)
	_, stderr, exitCode := runClue(t, testDir, "build")
	if exitCode != 0 {
		t.Fatalf("build failed with exit code %d: stderr=%s", exitCode, stderr)
	}

	// Run the target (should execute already-built binary)
	stdout, stderr, exitCode := runClue(t, testDir, "run", "calculator")

	if exitCode != 0 {
		t.Fatalf("run failed with exit code %d: stderr=%s", exitCode, stderr)
	}

	// Should see output from the executed program
	if !strings.Contains(stdout, "add") && !strings.Contains(stdout, "multiply") {
		t.Errorf("run output should include program output, got: %s", stdout)
	}
}

func TestCLI_RunCommand_PassesArguments(t *testing.T) {
	testDir := filepath.Join("testdata", "args-test")

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
	testDir := filepath.Join("testdata", "multi-target")
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
	testDir := filepath.Join("testdata", "multi-target")

	_, stderr, exitCode := runClue(t, testDir, "run", "nonexistent")

	if exitCode == 0 {
		t.Error("run should fail for missing targets")
	}

	if !strings.Contains(stderr, "not found") {
		t.Errorf("should show 'not found' error, got: %s", stderr)
	}
}

func TestCLI_BuildDetectsModuleInterfaces(t *testing.T) {
	// Test module detection (doesn't require full compilation)
	testDir := filepath.Join("testdata", "module-test")
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

func TestCLI_BuildCompilesModuleConsumer(t *testing.T) {
	// Check if clang-scan-deps is available
	if _, err := exec.LookPath("clang-scan-deps"); err != nil {
		t.Skip("clang-scan-deps not available, skipping module build test")
	}

	testDir := filepath.Join("testdata", "module-test")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testdata/module-test not found")
	}

	// Clean first
	runClue(t, testDir, "--all", "clean")

	stdout, stderr, exitCode := runClue(t, testDir, "-v", "run", "moduletest")
	if exitCode != 0 {
		t.Fatalf("module build failed with exit code %d: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "Module compilation order:") {
		t.Errorf("module compilation order missing from output: %s", stdout)
	}
	if !strings.Contains(stdout, "Hello from module!") {
		t.Errorf("module executable output missing: %s", stdout)
	}
}
