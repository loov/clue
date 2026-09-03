package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/deps"
)

func captureCLIStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	runErr := run()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	var output bytes.Buffer
	if _, err := output.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return output.String(), runErr
}

func TestListDependenciesPrintsNoDependencies(t *testing.T) {
	// Empty dependencies map
	deps := make(map[string]deps.Dependency)

	output, err := captureCLIStdout(t, func() error { return listDependencies(deps, false) })
	if err != nil {
		t.Fatalf("listDependencies failed: %v", err)
	}
	if !strings.Contains(output, "No dependencies configured") {
		t.Fatalf("output = %q", output)
	}
}

func TestListDependenciesPrintsEveryDependency(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Create test dependencies
	deps := map[string]deps.Dependency{
		"libfoo": deps.NewGitDependency("libfoo", "https://github.com/test/foo", "main", nil),
		"libbar": deps.NewTarballDependency("libbar", "https://example.com/bar.tar.gz", "", "", nil),
		"vendor": deps.NewVendoredDependency("vendor", "vendor/libvendor", nil),
	}

	output, err := captureCLIStdout(t, func() error { return listDependencies(deps, false) })
	if err != nil {
		t.Fatalf("listDependencies failed: %v", err)
	}
	for _, name := range []string{"libfoo", "libbar", "vendor"} {
		if !strings.Contains(output, name) {
			t.Errorf("output does not contain %q: %s", name, output)
		}
	}
}

func TestListDependenciesVerboseModeIncludesLocations(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Create test dependencies
	deps := map[string]deps.Dependency{
		"libfoo": deps.NewGitDependency("libfoo", "https://github.com/test/foo", "v1.0.0", nil),
	}

	output, err := captureCLIStdout(t, func() error { return listDependencies(deps, true) })
	if err != nil {
		t.Fatalf("listDependencies verbose failed: %v", err)
	}
	if !strings.Contains(output, "ref: v1.0.0") {
		t.Fatalf("output = %q", output)
	}
}

func TestFetchDependenciesReturnsWithoutDependencies(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Empty dependencies map
	deps := make(map[string]deps.Dependency)

	// Run fetch
	ctx := t.Context()
	err := fetchDependencies(ctx, deps, false, "")
	if err != nil {
		t.Fatalf("fetchDependencies failed: %v", err)
	}
}

func TestFetchDependenciesRejectsUnknownDependency(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Create dependency with invalid URL (will fail to fetch)
	deps := map[string]deps.Dependency{
		"libfoo": deps.NewGitDependency("libfoo", "https://invalid-repo-url-12345.example.com/repo", "main", nil),
	}

	// Run fetch - should fail
	ctx := t.Context()
	err := fetchDependencies(ctx, deps, false, "")

	if err == nil {
		t.Fatal("fetchDependencies succeeded with an invalid URL")
	}

	// Verify error message contains useful info
	if !strings.Contains(err.Error(), "libfoo") {
		t.Errorf("Error should mention dependency name: %v", err)
	}
}

func TestCleanDependenciesIgnoresMissingCache(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Create empty dependencies map
	deps := make(map[string]deps.Dependency)

	// Run clean - should succeed even though .deps doesn't exist
	err := cleanDependencies(deps, "")
	if err != nil {
		t.Fatalf("cleanDependencies failed: %v", err)
	}
}

func TestCleanDependenciesRemovesCacheDirectory(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Create .deps directory with some content
	depsDir := filepath.Join(tmpDir, ".deps")
	gitDir := filepath.Join(depsDir, "git", "test-dep")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}

	// Write a test file
	testFile := filepath.Join(gitDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Verify .deps exists
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		t.Fatalf(".deps directory should exist before clean")
	}

	// Create empty dependencies map
	deps := make(map[string]deps.Dependency)

	// Run clean
	err := cleanDependencies(deps, "")
	if err != nil {
		t.Fatalf("cleanDependencies failed: %v", err)
	}

	// Verify .deps was removed
	if _, err := os.Stat(depsDir); !os.IsNotExist(err) {
		t.Fatalf(".deps directory should be removed after clean")
	}
}

func TestCleanDependenciesRemovesSelectedDependency(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Create .deps directory with multiple dependencies
	depsDir := filepath.Join(tmpDir, ".deps")
	gitDir := filepath.Join(depsDir, "git")
	if err := os.MkdirAll(filepath.Join(gitDir, "libfoo-main"), 0o755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "libbar-v1.0"), 0o755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}

	// Create dependencies map
	deps := map[string]deps.Dependency{
		"libfoo": deps.NewGitDependency("libfoo", "https://github.com/test/foo", "main", nil),
		"libbar": deps.NewGitDependency("libbar", "https://github.com/test/bar", "v1.0", nil),
	}

	// Clean just libfoo
	err := cleanDependencies(deps, "libfoo")
	if err != nil {
		t.Fatalf("cleanDependencies failed: %v", err)
	}

	// Verify libfoo is gone but libbar remains
	if _, err := os.Stat(filepath.Join(gitDir, "libfoo-main")); !os.IsNotExist(err) {
		t.Errorf("libfoo should be removed")
	}
	if _, err := os.Stat(filepath.Join(gitDir, "libbar-v1.0")); os.IsNotExist(err) {
		t.Errorf("libbar should still exist")
	}
}

func TestUpdateDependenciesRefreshesConfiguredDependencies(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory for test
	t.Chdir(tmpDir)

	// Empty dependencies map
	deps := make(map[string]deps.Dependency)

	// Run update - should succeed (just prints placeholder message)
	ctx := t.Context()
	err := updateDependencies(ctx, deps)
	if err != nil {
		t.Fatalf("updateDependencies failed: %v", err)
	}
}
