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

// TestDepsList_Integration tests the deps list command with a real project
func TestDepsList_Integration(t *testing.T) {
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

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run deps list
	err = deps.RunList(cfg.Dependencies, false)

	w.Close()
	os.Stdout = oldStdout
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	output := buf.String()

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

// TestDepsFetch_Integration tests the deps fetch command with vendored dependency
func TestDepsFetch_Integration(t *testing.T) {
	// Setup: use testdata/deps-project
	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Change to project directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run deps fetch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = deps.RunFetch(ctx, cfg.Dependencies, deps.FetchOptions{
		Verbose: false,
		CIMode:  false,
	})

	w.Close()
	os.Stdout = oldStdout
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	output := buf.String()

	if err != nil {
		t.Errorf("RunFetch failed: %v", err)
	}

	// For vendored deps, fetch should show validation/cached status
	if !strings.Contains(output, "libmath") {
		t.Logf("Output: %s", output)
		t.Error("Expected output to mention libmath")
	}
}

// TestDepsClean_Integration tests the deps clean command
func TestDepsClean_Integration(t *testing.T) {
	// Create a temporary .deps directory with content
	tempDir := t.TempDir()
	depsDir := filepath.Join(tempDir, ".deps")
	gitDir := filepath.Join(depsDir, "git")
	tarballDir := filepath.Join(depsDir, "tarball")

	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create git dir: %v", err)
	}
	if err := os.MkdirAll(tarballDir, 0755); err != nil {
		t.Fatalf("Failed to create tarball dir: %v", err)
	}

	// Create some dummy files
	testFile := filepath.Join(gitDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		t.Fatal(".deps directory should exist before clean")
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a dummy dependency map (RunClean needs it for manager creation)
	dummyDeps := make(map[string]deps.Dependency)

	// Change to temp directory so manager uses correct path
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Run deps clean
	err := deps.RunClean(dummyDeps, "")

	w.Close()
	os.Stdout = oldStdout
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if err != nil {
		t.Errorf("RunClean failed: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(depsDir); !os.IsNotExist(err) {
		t.Error(".deps directory should be removed after clean")
	}
}

// TestBuildWithDeps_Integration tests the complete workflow end-to-end
func TestBuildWithDeps_Integration(t *testing.T) {
	// This test is essentially the same as TestSuccessCriteria1_VendoredDependency
	// but focuses on the complete workflow aspect

	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Change to project directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	buildDir := ".build"
	defer os.RemoveAll(buildDir)

	// Step 1: Verify dependencies are listed correctly
	var listBuf bytes.Buffer
	oldStdout := os.Stdout
	r1, w1, _ := os.Pipe()
	os.Stdout = w1

	err = deps.RunList(cfg.Dependencies, false)

	w1.Close()
	os.Stdout = oldStdout
	if _, err := listBuf.ReadFrom(r1); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	listOutput := listBuf.String()

	if err != nil {
		t.Errorf("RunList failed: %v", err)
	}

	if !strings.Contains(listOutput, "libmath") {
		t.Error("Dependency list should contain libmath")
	}

	// Step 2: Verify fetch works
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = deps.RunFetch(ctx, cfg.Dependencies, deps.FetchOptions{
		Verbose: false,
		CIMode:  false,
	})

	if err != nil {
		t.Errorf("Fetch should succeed: %v", err)
	}

	// Step 3: Verify we can build (covered by other tests)
	// Step 4: Verify executable runs correctly (covered by other tests)

	// This test verifies the workflow integrates correctly
	t.Log("Complete workflow test passed: list → fetch → (build tested elsewhere)")
}
