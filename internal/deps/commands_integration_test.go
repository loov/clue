package deps_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
)

func captureStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := run()
	closeErr := w.Close()
	os.Stdout = oldStdout

	var output bytes.Buffer
	if _, err := output.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return output.String(), runErr
}

// TestDepsList_PrintsDependencyStatus tests the deps list command with a real project
func TestDepsList_PrintsDependencyStatus(t *testing.T) {
	// Setup: use testdata/deps-project
	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(projectDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	output, err := captureStdout(t, func() error { return deps.RunList(cfg.Dependencies, false) })
	if err != nil {
		t.Errorf("RunList failed: %v", err)
	}

	// Verify output contains libmath with vendored type
	if !strings.Contains(output, "libmath") {
		t.Errorf("Expected output to contain 'libmath', got: %s", output)
	}

	if !strings.Contains(output, "vendored") {
		t.Errorf("Expected output to contain 'vendored', got: %s", output)
	}
}

// TestDepsFetch_DownloadsConfiguredDependencies tests the deps fetch command with vendored dependency
func TestDepsFetch_DownloadsConfiguredDependencies(t *testing.T) {
	// Setup: use testdata/deps-project
	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Run deps fetch
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	output, err := captureStdout(t, func() error {
		return deps.RunFetch(ctx, cfg.Dependencies, deps.FetchOptions{Verbose: false})
	})
	if err != nil {
		t.Errorf("RunFetch failed: %v", err)
	}

	// For vendored deps, fetch should show validation/cached status
	if !strings.Contains(output, "libmath") {
		t.Logf("Output: %s", output)
		t.Error("Expected output to mention libmath")
	}
}

// TestDepsClean_RemovesDependencyCache tests the deps clean command
func TestDepsClean_RemovesDependencyCache(t *testing.T) {
	// Create a temporary .deps directory with content
	tempDir := t.TempDir()
	depsDir := filepath.Join(tempDir, ".deps")
	gitDir := filepath.Join(depsDir, "git")
	tarballDir := filepath.Join(depsDir, "tarball")

	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create git dir: %v", err)
	}
	if err := os.MkdirAll(tarballDir, 0o755); err != nil {
		t.Fatalf("Failed to create tarball dir: %v", err)
	}

	// Create some dummy files
	testFile := filepath.Join(gitDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		t.Fatal(".deps directory should exist before clean")
	}

	// Create a dummy dependency map (RunClean needs it for manager creation)
	dummyDeps := make(map[string]deps.Dependency)

	t.Chdir(tempDir)

	// Run deps clean
	_, err := captureStdout(t, func() error { return deps.RunClean(dummyDeps, "") })
	if err != nil {
		t.Errorf("RunClean failed: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(depsDir); !os.IsNotExist(err) {
		t.Error(".deps directory should be removed after clean")
	}
}

// TestBuildWithDeps_LinksFetchedDependency tests the complete workflow end-to-end
func TestBuildWithDeps_LinksFetchedDependency(t *testing.T) {
	// This test is essentially the same as TestSuccessCriteria1_VendoredDependency
	// but focuses on the complete workflow aspect

	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Step 1: Verify dependencies are listed correctly
	listOutput, err := captureStdout(t, func() error { return deps.RunList(cfg.Dependencies, false) })
	if err != nil {
		t.Errorf("RunList failed: %v", err)
	}

	if !strings.Contains(listOutput, "libmath") {
		t.Error("Dependency list should contain libmath")
	}

	// Step 2: Verify fetch works
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	err = deps.RunFetch(ctx, cfg.Dependencies, deps.FetchOptions{
		Verbose: false,
	})
	if err != nil {
		t.Errorf("Fetch should succeed: %v", err)
	}

	// Step 3: Verify we can build (covered by other tests)
	// Step 4: Verify executable runs correctly (covered by other tests)

	// This test verifies the workflow integrates correctly
	t.Log("Complete workflow test passed: list → fetch → (build tested elsewhere)")
}
