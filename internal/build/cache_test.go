package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFileHash(t *testing.T) {
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

func TestNormalizeFlags(t *testing.T) {
	// Get current working directory for testing relative path resolution
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		flags    []string
		expected []string
	}{
		{
			name:     "sorts flags alphabetically",
			flags:    []string{"-O2", "-Wall", "-g"},
			expected: []string{"-O2", "-Wall", "-g"},
		},
		{
			name:     "filters out verbose flag",
			flags:    []string{"--verbose", "-O2"},
			expected: []string{"-O2"},
		},
		{
			name:     "filters out color flag",
			flags:    []string{"--color", "-O2", "-Wall"},
			expected: []string{"-O2", "-Wall"},
		},
		{
			name:     "filters out progress flag",
			flags:    []string{"-O2", "--progress", "-Wall"},
			expected: []string{"-O2", "-Wall"},
		},
		{
			name:     "filters out -v flag",
			flags:    []string{"-v", "-O2"},
			expected: []string{"-O2"},
		},
		{
			name:     "resolves relative include paths",
			flags:    []string{"-O2", "-I./include"},
			expected: []string{"-I" + filepath.Join(cwd, "include"), "-O2"},
		},
		{
			name:     "preserves absolute include paths",
			flags:    []string{"-O2", "-I/usr/include"},
			expected: []string{"-I/usr/include", "-O2"},
		},
		{
			name:     "handles multiple include paths",
			flags:    []string{"-I./inc1", "-I./inc2", "-O2"},
			expected: []string{"-I" + filepath.Join(cwd, "inc1"), "-I" + filepath.Join(cwd, "inc2"), "-O2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeFlags(tt.flags)

			if len(result) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(result), len(tt.expected))
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("flag[%d]: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetCompilerIdentity(t *testing.T) {
	// Use go binary as test compiler (known to exist)
	goBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("returns compiler identity", func(t *testing.T) {
		identity, err := GetCompilerIdentity(goBinary)
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
		_, err := GetCompilerIdentity("/nonexistent/compiler")
		if err == nil {
			t.Error("expected error for nonexistent compiler")
		}
	})
}

func TestComputeCacheKey(t *testing.T) {
	key1 := CacheKey{
		SourceHash:   "abc123",
		DepsHash:     "def456",
		CompilerID:   CompilerIdentity{Path: "/usr/bin/clang", Mtime: 123456, Size: 1000},
		Flags:        []string{"-O2", "-Wall"},
		IncludePaths: []string{"/usr/include"},
	}

	key2 := key1 // Same key

	key3 := CacheKey{
		SourceHash:   "different",
		DepsHash:     "def456",
		CompilerID:   CompilerIdentity{Path: "/usr/bin/clang", Mtime: 123456, Size: 1000},
		Flags:        []string{"-O2", "-Wall"},
		IncludePaths: []string{"/usr/include"},
	}

	t.Run("same key produces same hash", func(t *testing.T) {
		hash1 := ComputeCacheKey(key1)
		hash2 := ComputeCacheKey(key2)

		if hash1 != hash2 {
			t.Errorf("same key should produce same hash: %s != %s", hash1, hash2)
		}

		if hash1 == "" {
			t.Error("hash should not be empty")
		}
	})

	t.Run("different key produces different hash", func(t *testing.T) {
		hash1 := ComputeCacheKey(key1)
		hash3 := ComputeCacheKey(key3)

		if hash1 == hash3 {
			t.Error("different keys should produce different hashes")
		}
	})

	t.Run("compiler identity affects hash", func(t *testing.T) {
		key4 := key1
		key4.CompilerID.Mtime = 999999

		hash1 := ComputeCacheKey(key1)
		hash4 := ComputeCacheKey(key4)

		if hash1 == hash4 {
			t.Error("different compiler mtime should produce different hash")
		}
	})

	t.Run("flags order matters", func(t *testing.T) {
		key5 := key1
		key5.Flags = []string{"-Wall", "-O2"} // Different order

		hash1 := ComputeCacheKey(key1)
		hash5 := ComputeCacheKey(key5)

		if hash1 == hash5 {
			t.Error("different flag order should produce different hash")
		}
	})
}
