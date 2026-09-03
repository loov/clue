package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestComputeFileHash_IsStableAndContentSensitive(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	t.Run("computes consistent hash for file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.cpp")
		content := []byte("int main() {}")
		if err := os.WriteFile(testFile, content, 0o644); err != nil {
			t.Fatal(err)
		}

		hash1, err := ComputeFileHash(testFile)
		if err != nil {
			t.Fatalf("ComputeFileHash failed: %v", err)
		}

		hash2, err := ComputeFileHash(testFile)
		if err != nil {
			t.Fatalf("ComputeFileHash failed: %v", err)
		}

		if hash1 != hash2 {
			t.Errorf("hash not consistent: %s != %s", hash1, hash2)
		}

		if hash1 == "" {
			t.Error("hash is empty")
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		_, err := ComputeFileHash(filepath.Join(tmpDir, "nonexistent.cpp"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		file1 := filepath.Join(tmpDir, "file1.cpp")
		file2 := filepath.Join(tmpDir, "file2.cpp")

		if err := os.WriteFile(file1, []byte("int main() {}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file2, []byte("int foo() {}"), 0o644); err != nil {
			t.Fatal(err)
		}

		hash1, err := ComputeFileHash(file1)
		if err != nil {
			t.Fatal(err)
		}

		hash2, err := ComputeFileHash(file2)
		if err != nil {
			t.Fatal(err)
		}

		if hash1 == hash2 {
			t.Error("different content should produce different hashes")
		}
	})
}

func TestComputeCompilerIdentity_ReportsFileMetadataAndMissingFiles(t *testing.T) {
	// Use go binary as test compiler (known to exist)
	goBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("returns compiler identity", func(t *testing.T) {
		identity, err := toolchain.ComputeCompilerIdentity(goBinary)
		if err != nil {
			t.Fatalf("GetCompilerIdentity failed: %v", err)
		}

		if identity.Path != goBinary {
			t.Errorf("path: got %q, want %q", identity.Path, goBinary)
		}

		if identity.Mtime == 0 {
			t.Error("mtime should not be zero")
		}

		if identity.Size == 0 {
			t.Error("size should not be zero")
		}
	})

	t.Run("returns error for nonexistent compiler", func(t *testing.T) {
		_, err := toolchain.ComputeCompilerIdentity("/nonexistent/compiler")
		if err == nil {
			t.Error("expected error for nonexistent compiler")
		}
	})
}
