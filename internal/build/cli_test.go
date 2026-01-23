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

	// Build clue if not already built
	clueCmd := exec.Command("go", "build", "-o", "clue_test_bin", "./cmd/clue")
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
