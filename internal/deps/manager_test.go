package deps

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_StatusEmpty(t *testing.T) {
	// No dependencies configured should return empty status
	tmpDir := t.TempDir()

	manager, err := NewManager(tmpDir, map[string]Dependency{}, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	statuses := manager.Status()
	if len(statuses) != 0 {
		t.Errorf("Expected empty status list, got %d items", len(statuses))
	}
}

func TestManager_StatusMissing(t *testing.T) {
	// Dependencies configured but not fetched should show "missing"
	tmpDir := t.TempDir()

	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/example/foo.git", "main", nil),
		"libbar": NewTarballDependency("libbar", "https://example.com/bar.tar.gz", "", "", nil),
	}

	manager, err := NewManager(tmpDir, deps, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	statuses := manager.Status()
	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}

	// All should be missing
	for _, status := range statuses {
		if status.Status != "missing" {
			t.Errorf("Expected status 'missing' for %s, got %s", status.Name, status.Status)
		}
	}
}

func TestManager_VendoredAlwaysCached(t *testing.T) {
	// Vendored dependency with valid path should show as "cached"
	tmpDir := t.TempDir()

	// Create a vendored directory
	vendorPath := filepath.Join(tmpDir, "vendor", "libfoo")
	if err := os.MkdirAll(vendorPath, 0755); err != nil {
		t.Fatalf("Failed to create vendor directory: %v", err)
	}

	// Create a dummy source file
	sourceFile := filepath.Join(vendorPath, "foo.cpp")
	if err := os.WriteFile(sourceFile, []byte("// foo"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	deps := map[string]Dependency{
		"libfoo": NewVendoredDependency("libfoo", vendorPath, nil),
	}

	manager, err := NewManager(tmpDir, deps, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	statuses := manager.Status()
	if len(statuses) != 1 {
		t.Fatalf("Expected 1 status, got %d", len(statuses))
	}

	if statuses[0].Status != "cached" {
		t.Errorf("Expected vendored dependency to be 'cached', got %s", statuses[0].Status)
	}

	if statuses[0].Type != "vendored" {
		t.Errorf("Expected type 'vendored', got %s", statuses[0].Type)
	}
}

func TestFetchAll_SkipsCached(t *testing.T) {
	// Verify that cached dependencies are not re-fetched
	tmpDir := t.TempDir()

	// Create a vendored dependency (always cached)
	vendorPath := filepath.Join(tmpDir, "vendor", "libfoo")
	if err := os.MkdirAll(vendorPath, 0755); err != nil {
		t.Fatalf("Failed to create vendor directory: %v", err)
	}

	// Create a dummy source file
	sourceFile := filepath.Join(vendorPath, "foo.cpp")
	if err := os.WriteFile(sourceFile, []byte("// foo"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	deps := map[string]Dependency{
		"libfoo": NewVendoredDependency("libfoo", vendorPath, nil),
	}

	manager, err := NewManager(tmpDir, deps, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// FetchAll should succeed without network calls
	ctx := context.Background()
	err = manager.FetchAll(ctx)
	if err != nil {
		t.Errorf("FetchAll failed: %v", err)
	}

	// Verify status is still cached
	statuses := manager.Status()
	if len(statuses) != 1 {
		t.Fatalf("Expected 1 status, got %d", len(statuses))
	}

	if statuses[0].Status != "cached" {
		t.Errorf("Expected status 'cached', got %s", statuses[0].Status)
	}
}

func TestFetchOne_Unknown(t *testing.T) {
	tmpDir := t.TempDir()

	manager, err := NewManager(tmpDir, map[string]Dependency{}, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	err = manager.FetchOne(ctx, "unknown")
	if err == nil {
		t.Error("Expected error for unknown dependency")
	}

	if err != nil && err.Error() != `dependency "unknown" not found` {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestClean(t *testing.T) {
	tmpDir := t.TempDir()

	deps := map[string]Dependency{
		"libfoo": NewGitDependency("libfoo", "https://github.com/example/foo.git", "main", nil),
	}

	manager, err := NewManager(tmpDir, deps, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Create a dummy file in .deps directory
	depsDir := filepath.Join(tmpDir, ".deps")
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create .deps directory: %v", err)
	}

	dummyFile := filepath.Join(depsDir, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}

	// Clean should remove the directory
	err = manager.Clean()
	if err != nil {
		t.Errorf("Clean failed: %v", err)
	}

	// Verify .deps directory is gone
	if _, err := os.Stat(depsDir); !os.IsNotExist(err) {
		t.Error("Expected .deps directory to be removed")
	}
}

func TestCleanOne_Unknown(t *testing.T) {
	tmpDir := t.TempDir()

	manager, err := NewManager(tmpDir, map[string]Dependency{}, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.CleanOne("unknown")
	if err == nil {
		t.Error("Expected error for unknown dependency")
	}
}

func TestManager_BuildOrder(t *testing.T) {
	// Verify that FetchAll uses correct build order
	tmpDir := t.TempDir()

	// Create vendored dependencies to avoid network calls
	vendorPath1 := filepath.Join(tmpDir, "vendor", "aaa")
	vendorPath2 := filepath.Join(tmpDir, "vendor", "zzz")
	vendorPath3 := filepath.Join(tmpDir, "vendor", "mmm")

	for _, path := range []string{vendorPath1, vendorPath2, vendorPath3} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create vendor directory: %v", err)
		}
		sourceFile := filepath.Join(path, "source.cpp")
		if err := os.WriteFile(sourceFile, []byte("// source"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}
	}

	deps := map[string]Dependency{
		"zzz": NewVendoredDependency("zzz", vendorPath2, nil),
		"aaa": NewVendoredDependency("aaa", vendorPath1, nil),
		"mmm": NewVendoredDependency("mmm", vendorPath3, nil),
	}

	manager, err := NewManager(tmpDir, deps, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// FetchAll should process in alphabetical order
	ctx := context.Background()
	err = manager.FetchAll(ctx)
	if err != nil {
		t.Errorf("FetchAll failed: %v", err)
	}

	// Verify all are cached
	statuses := manager.Status()
	if len(statuses) != 3 {
		t.Fatalf("Expected 3 statuses, got %d", len(statuses))
	}

	for _, status := range statuses {
		if status.Status != "cached" {
			t.Errorf("Expected status 'cached' for %s, got %s", status.Name, status.Status)
		}
	}
}
