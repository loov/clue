package deps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunList_Empty(t *testing.T) {
	// Empty dependencies map
	deps := make(map[string]Dependency)

	// Capture output
	// Note: RunList prints to stdout, so we can't easily capture it
	// For now, just verify it doesn't error
	err := RunList(deps, false)
	if err != nil {
		t.Fatalf("RunList failed: %v", err)
	}
}

func TestRunList_WithDeps(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create test dependencies
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/test/foo", "main", nil),
		"libbar": NewTarballDependency("libbar", "https://example.com/bar.tar.gz", "", "", nil),
		"vendor": NewVendoredDependency("vendor", "vendor/libvendor", nil),
	}

	// Run list
	err = RunList(deps, false)
	if err != nil {
		t.Fatalf("RunList failed: %v", err)
	}

	// Success if no error
}

func TestRunList_Verbose(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create test dependencies
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/test/foo", "v1.0.0", nil),
	}

	// Run list with verbose
	err = RunList(deps, true)
	if err != nil {
		t.Fatalf("RunList verbose failed: %v", err)
	}
}

func TestRunFetch_NoDeps(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Empty dependencies map
	deps := make(map[string]Dependency)

	// Run fetch
	ctx := context.Background()
	err = RunFetch(ctx, deps, FetchOptions{
		Verbose: false,
		CIMode:  false,
		Name:    "",
	})

	if err != nil {
		t.Fatalf("RunFetch failed: %v", err)
	}
}

func TestRunFetch_InvalidDependency(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create dependency with invalid URL (will fail to fetch)
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://invalid-repo-url-12345.example.com/repo", "main", nil),
	}

	// Run fetch - should fail
	ctx := context.Background()
	err = RunFetch(ctx, deps, FetchOptions{
		Verbose: false,
		CIMode:  false,
		Name:    "",
	})

	if err == nil {
		t.Fatalf("Expected RunFetch to fail with invalid URL")
	}

	// Verify error message contains useful info
	if !strings.Contains(err.Error(), "libfoo") {
		t.Errorf("Error should mention dependency name: %v", err)
	}
}

func TestRunClean_NotExists(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create empty dependencies map
	deps := make(map[string]Dependency)

	// Run clean - should succeed even though .deps doesn't exist
	err = RunClean(deps, "")
	if err != nil {
		t.Fatalf("RunClean failed: %v", err)
	}
}

func TestRunClean_RemovesDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .deps directory with some content
	depsDir := filepath.Join(tmpDir, ".deps")
	gitDir := filepath.Join(depsDir, "git", "test-dep")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}

	// Write a test file
	testFile := filepath.Join(gitDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Verify .deps exists
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		t.Fatalf(".deps directory should exist before clean")
	}

	// Create empty dependencies map
	deps := make(map[string]Dependency)

	// Run clean
	err = RunClean(deps, "")
	if err != nil {
		t.Fatalf("RunClean failed: %v", err)
	}

	// Verify .deps was removed
	if _, err := os.Stat(depsDir); !os.IsNotExist(err) {
		t.Fatalf(".deps directory should be removed after clean")
	}
}

func TestRunClean_SingleDependency(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .deps directory with multiple dependencies
	depsDir := filepath.Join(tmpDir, ".deps")
	gitDir := filepath.Join(depsDir, "git")
	if err := os.MkdirAll(filepath.Join(gitDir, "libfoo-main"), 0755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "libbar-v1.0"), 0755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}

	// Create dependencies map
	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/test/foo", "main", nil),
		"libbar": NewGitDependency("libbar", "https://github.com/test/bar", "v1.0", nil),
	}

	// Clean just libfoo
	err = RunClean(deps, "libfoo")
	if err != nil {
		t.Fatalf("RunClean failed: %v", err)
	}

	// Verify libfoo is gone but libbar remains
	if _, err := os.Stat(filepath.Join(gitDir, "libfoo-main")); !os.IsNotExist(err) {
		t.Errorf("libfoo should be removed")
	}
	if _, err := os.Stat(filepath.Join(gitDir, "libbar-v1.0")); os.IsNotExist(err) {
		t.Errorf("libbar should still exist")
	}
}

func TestRunUpdate(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "clue-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Empty dependencies map
	deps := make(map[string]Dependency)

	// Run update - should succeed (just prints placeholder message)
	ctx := context.Background()
	err = RunUpdate(ctx, deps)
	if err != nil {
		t.Fatalf("RunUpdate failed: %v", err)
	}
}
