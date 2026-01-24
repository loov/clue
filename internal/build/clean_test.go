package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClean_VariantOnly(t *testing.T) {
	// Setup: Create temp directory structure
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, ".build")

	// Create build/debug/myapp/main.o and build/debug/bin/myapp
	debugDir := filepath.Join(buildDir, "debug")
	if err := os.MkdirAll(filepath.Join(debugDir, "myapp"), 0o755); err != nil {
		t.Fatalf("Failed to create debug/myapp dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(debugDir, "bin"), 0o755); err != nil {
		t.Fatalf("Failed to create debug/bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(debugDir, "myapp", "main.o"), []byte("obj"), 0o644); err != nil {
		t.Fatalf("Failed to create main.o: %v", err)
	}
	if err := os.WriteFile(filepath.Join(debugDir, "bin", "myapp"), []byte("exe"), 0o755); err != nil {
		t.Fatalf("Failed to create myapp: %v", err)
	}

	// Create build/release/myapp/main.o
	releaseDir := filepath.Join(buildDir, "release")
	if err := os.MkdirAll(filepath.Join(releaseDir, "myapp"), 0o755); err != nil {
		t.Fatalf("Failed to create release/myapp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "myapp", "main.o"), []byte("obj"), 0o644); err != nil {
		t.Fatalf("Failed to create release main.o: %v", err)
	}

	// Test: Clean debug variant
	result, err := Clean(CleanOptions{
		BuildDir: buildDir,
		Variant:  "debug",
		All:      false,
	})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if !result.Existed {
		t.Error("Expected result.Existed to be true")
	}

	expectedPath := filepath.Join(buildDir, "debug")
	if result.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, result.Path)
	}

	// Verify build/debug/ is removed
	if _, err := os.Stat(debugDir); !os.IsNotExist(err) {
		t.Error("Expected build/debug/ to be removed")
	}

	// Verify build/release/ still exists
	if _, err := os.Stat(releaseDir); err != nil {
		t.Errorf("Expected build/release/ to still exist: %v", err)
	}

	// Verify result string
	resultStr := result.String()
	if !strings.Contains(resultStr, "Cleaned:") {
		t.Errorf("Expected result string to contain 'Cleaned:', got: %s", resultStr)
	}
}

func TestClean_All(t *testing.T) {
	// Setup: Create temp directory structure
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, ".build")

	// Create build/debug/ and build/release/
	debugDir := filepath.Join(buildDir, "debug")
	releaseDir := filepath.Join(buildDir, "release")
	if err := os.MkdirAll(filepath.Join(debugDir, "myapp"), 0o755); err != nil {
		t.Fatalf("Failed to create debug dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(releaseDir, "myapp"), 0o755); err != nil {
		t.Fatalf("Failed to create release dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(debugDir, "myapp", "main.o"), []byte("obj"), 0o644); err != nil {
		t.Fatalf("Failed to create debug main.o: %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "myapp", "main.o"), []byte("obj"), 0o644); err != nil {
		t.Fatalf("Failed to create release main.o: %v", err)
	}

	// Test: Clean all
	result, err := Clean(CleanOptions{
		BuildDir: buildDir,
		Variant:  "", // Should be ignored when All is true
		All:      true,
	})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if !result.Existed {
		t.Error("Expected result.Existed to be true")
	}

	if result.Path != buildDir {
		t.Errorf("Expected path %s, got %s", buildDir, result.Path)
	}

	// Verify entire build/ directory is removed
	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Error("Expected entire build/ directory to be removed")
	}

	// Verify result string
	resultStr := result.String()
	if !strings.Contains(resultStr, "Cleaned:") {
		t.Errorf("Expected result string to contain 'Cleaned:', got: %s", resultStr)
	}
}

func TestClean_NonExistent(t *testing.T) {
	// Test: Clean non-existent directory
	tmpDir := t.TempDir()
	nonExistentBuild := filepath.Join(tmpDir, "nonexistent_build_xyz")

	result, err := Clean(CleanOptions{
		BuildDir: nonExistentBuild,
		Variant:  "debug",
		All:      false,
	})
	if err != nil {
		t.Fatalf("Clean should not error on non-existent directory: %v", err)
	}

	if result.Existed {
		t.Error("Expected result.Existed to be false")
	}

	// Verify result string contains "not found"
	resultStr := result.String()
	if !strings.Contains(resultStr, "not found") {
		t.Errorf("Expected result string to contain 'not found', got: %s", resultStr)
	}
}

func TestClean_EmptyVariant(t *testing.T) {
	// Test: Clean with empty variant and All=false should return error
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, ".build")

	_, err := Clean(CleanOptions{
		BuildDir: buildDir,
		Variant:  "",
		All:      false,
	})

	if err == nil {
		t.Fatal("Expected error when Variant is empty and All is false")
	}

	if !strings.Contains(err.Error(), "variant must be specified") {
		t.Errorf("Expected error to mention variant requirement, got: %v", err)
	}
}
