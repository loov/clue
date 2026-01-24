package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// createTestTarGz creates a tar.gz archive in memory with the given files
func createTestTarGz(t *testing.T, files map[string]string) string {
	t.Helper()

	// Create temp file for archive
	tmpFile, err := os.CreateTemp("", "test-*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpName := tmpFile.Name()

	// Create gzip writer
	gzw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gzw)

	// Add files to archive
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if content == "" {
			// Directory
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
		} else {
			hdr.Typeflag = tar.TypeReg
		}

		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if content != "" {
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatalf("failed to write tar content: %v", err)
			}
		}
	}

	// Close writers
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	return tmpName
}

// createTestTarGzWithSymlink creates a tar.gz with a symlink entry
func createTestTarGzWithSymlink(t *testing.T) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test-symlink-*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpName := tmpFile.Name()

	gzw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gzw)

	// Add a regular file
	hdr := &tar.Header{
		Name:     "file.txt",
		Mode:     0o644,
		Size:     5,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	// Add a symlink
	symlinkHdr := &tar.Header{
		Name:     "link.txt",
		Mode:     0o644,
		Typeflag: tar.TypeSymlink,
		Linkname: "file.txt",
	}
	if err := tw.WriteHeader(symlinkHdr); err != nil {
		t.Fatalf("failed to write symlink header: %v", err)
	}

	tw.Close()
	gzw.Close()
	tmpFile.Close()

	return tmpName
}

func TestExtractTarGz_Normal(t *testing.T) {
	// Create test archive
	files := map[string]string{
		"file1.txt":         "content1",
		"dir/":              "",
		"dir/file2.txt":     "content2",
		"dir/sub/":          "",
		"dir/sub/file3.txt": "content3",
	}
	archivePath := createTestTarGz(t, files)
	defer os.Remove(archivePath)

	// Create temp directory for extraction
	targetDir := t.TempDir()

	// Extract
	if err := ExtractTarGz(archivePath, targetDir); err != nil {
		t.Fatalf("ExtractTarGz failed: %v", err)
	}

	// Verify files exist
	file1 := filepath.Join(targetDir, "file1.txt")
	if _, err := os.Stat(file1); err != nil {
		t.Errorf("file1.txt not extracted: %v", err)
	}
	content, _ := os.ReadFile(file1)
	if string(content) != "content1" {
		t.Errorf("file1.txt has wrong content: got %q, want %q", content, "content1")
	}

	file2 := filepath.Join(targetDir, "dir", "file2.txt")
	if _, err := os.Stat(file2); err != nil {
		t.Errorf("dir/file2.txt not extracted: %v", err)
	}

	file3 := filepath.Join(targetDir, "dir", "sub", "file3.txt")
	if _, err := os.Stat(file3); err != nil {
		t.Errorf("dir/sub/file3.txt not extracted: %v", err)
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	// Create malicious archive with path traversal
	files := map[string]string{
		"../../../etc/passwd": "malicious",
	}
	archivePath := createTestTarGz(t, files)
	defer os.Remove(archivePath)

	targetDir := t.TempDir()

	// Extract should fail
	err := ExtractTarGz(archivePath, targetDir)
	if err == nil {
		t.Fatal("ExtractTarGz should have failed on path traversal")
	}
	if err.Error() != "path traversal detected: ../../../etc/passwd" {
		t.Errorf("wrong error message: got %q", err.Error())
	}

	// Verify no files created outside target
	etcPasswd := "/etc/passwd"
	originalContent, _ := os.ReadFile(etcPasswd)
	if bytes.Contains(originalContent, []byte("malicious")) {
		t.Error("malicious content written to /etc/passwd")
	}
}

func TestExtractTarGz_AbsolutePath(t *testing.T) {
	// Create archive with absolute path
	files := map[string]string{
		"/tmp/malicious.txt": "bad",
	}
	archivePath := createTestTarGz(t, files)
	defer os.Remove(archivePath)

	targetDir := t.TempDir()

	// Extract should fail
	err := ExtractTarGz(archivePath, targetDir)
	if err == nil {
		t.Fatal("ExtractTarGz should have failed on absolute path")
	}
	if err.Error() != "path traversal detected: /tmp/malicious.txt" {
		t.Errorf("wrong error message: got %q", err.Error())
	}
}

func TestExtractTarGz_SymlinkIgnored(t *testing.T) {
	// Create archive with symlink
	archivePath := createTestTarGzWithSymlink(t)
	defer os.Remove(archivePath)

	targetDir := t.TempDir()

	// Extract should succeed (symlinks are skipped)
	if err := ExtractTarGz(archivePath, targetDir); err != nil {
		t.Fatalf("ExtractTarGz failed: %v", err)
	}

	// Verify regular file exists
	file1 := filepath.Join(targetDir, "file.txt")
	if _, err := os.Stat(file1); err != nil {
		t.Errorf("file.txt not extracted: %v", err)
	}

	// Verify symlink was NOT created
	link := filepath.Join(targetDir, "link.txt")
	if _, err := os.Stat(link); err == nil {
		t.Error("symlink should not have been created")
	}
}

func TestStripPrefix(t *testing.T) {
	// Create test directory structure
	targetDir := t.TempDir()
	prefix := "mylib-1.0.0"
	prefixDir := filepath.Join(targetDir, prefix)

	// Create files in prefix directory
	if err := os.MkdirAll(filepath.Join(prefixDir, "subdir"), 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prefixDir, "file1.txt"), []byte("content1"), 0o644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prefixDir, "subdir", "file2.txt"), []byte("content2"), 0o644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	// Strip prefix
	if err := StripPrefix(targetDir, prefix); err != nil {
		t.Fatalf("StripPrefix failed: %v", err)
	}

	// Verify files moved to root
	file1 := filepath.Join(targetDir, "file1.txt")
	if _, err := os.Stat(file1); err != nil {
		t.Errorf("file1.txt not moved to root: %v", err)
	}

	file2 := filepath.Join(targetDir, "subdir", "file2.txt")
	if _, err := os.Stat(file2); err != nil {
		t.Errorf("subdir/file2.txt not moved: %v", err)
	}

	// Verify prefix directory removed
	if _, err := os.Stat(prefixDir); err == nil {
		t.Error("prefix directory should have been removed")
	}
}

func TestDetectArchiveType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"file.tar.gz", "tar.gz"},
		{"file.tgz", "tar.gz"},
		{"file.TGZ", "tar.gz"},
		{"file.TAR.GZ", "tar.gz"},
		{"file.zip", "zip"},
		{"file.ZIP", "zip"},
		{"file.tar.bz2", ""},
		{"file.unknown", ""},
		{"file.txt", ""},
		{"https://example.com/archive.tar.gz", "tar.gz"},
		{"https://example.com/archive.zip", "zip"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := DetectArchiveType(tt.path)
			if result != tt.expected {
				t.Errorf("DetectArchiveType(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractZip_Normal(t *testing.T) {
	// Create test zip archive
	tmpFile, err := os.CreateTemp("", "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(tmpFile)

	// Add files
	files := map[string]string{
		"file1.txt":         "content1",
		"dir/file2.txt":     "content2",
		"dir/sub/file3.txt": "content3",
	}

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content: %v", err)
		}
	}

	zw.Close()
	tmpFile.Close()

	// Extract
	targetDir := t.TempDir()
	if err := ExtractZip(tmpName, targetDir); err != nil {
		t.Fatalf("ExtractZip failed: %v", err)
	}

	// Verify files
	file1 := filepath.Join(targetDir, "file1.txt")
	content, _ := os.ReadFile(file1)
	if string(content) != "content1" {
		t.Errorf("file1.txt has wrong content: got %q, want %q", content, "content1")
	}

	file3 := filepath.Join(targetDir, "dir", "sub", "file3.txt")
	if _, err := os.Stat(file3); err != nil {
		t.Errorf("dir/sub/file3.txt not extracted: %v", err)
	}
}

func TestExtractZip_PathTraversal(t *testing.T) {
	// Create malicious zip
	tmpFile, err := os.CreateTemp("", "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	zw := zip.NewWriter(tmpFile)
	w, _ := zw.Create("../../../etc/passwd")
	_, _ = w.Write([]byte("malicious"))
	zw.Close()
	tmpFile.Close()

	// Extract should fail
	targetDir := t.TempDir()
	err = ExtractZip(tmpName, targetDir)
	if err == nil {
		t.Fatal("ExtractZip should have failed on path traversal")
	}
	if err.Error() != "path traversal detected: ../../../etc/passwd" {
		t.Errorf("wrong error message: got %q", err.Error())
	}
}
