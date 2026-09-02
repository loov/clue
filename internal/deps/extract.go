package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractTarGz extracts a tar.gz archive to the target directory
// Validates all paths to prevent directory traversal attacks
func ExtractTarGz(archivePath, targetDir string) error {
	// Open archive file
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", archivePath, err)
	}
	defer f.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for %s: %w", archivePath, err)
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Track extracted files for cleanup on error
	extractedFiles := []string{}

	// Extract each entry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Validate path with filepath.IsLocal (Go 1.20+)
		if !filepath.IsLocal(hdr.Name) {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("path traversal detected: %s", hdr.Name)
		}

		// Resolve full path
		fullPath := filepath.Join(targetDir, hdr.Name)

		// Additional security check: verify resolved path is within targetDir
		absTarget, err := filepath.Abs(fullPath)
		if err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to resolve absolute path for %s: %w", fullPath, err)
		}
		absDir, err := filepath.Abs(targetDir)
		if err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to resolve absolute path for target dir %s: %w", targetDir, err)
		}
		if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) && absTarget != absDir {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("path traversal detected: %s resolves outside target directory", hdr.Name)
		}

		// Handle entry based on type
		switch hdr.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(fullPath, 0o755); err != nil {
				// Cleanup on error
				for _, path := range extractedFiles {
					os.RemoveAll(path)
				}
				return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}
			extractedFiles = append(extractedFiles, fullPath)

		case tar.TypeReg:
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				// Cleanup on error
				for _, path := range extractedFiles {
					os.RemoveAll(path)
				}
				return fmt.Errorf("failed to create parent directory for %s: %w", fullPath, err)
			}

			// Create file
			outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				// Cleanup on error
				for _, path := range extractedFiles {
					os.RemoveAll(path)
				}
				return fmt.Errorf("failed to create file %s: %w", fullPath, err)
			}

			// Copy contents
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				// Cleanup on error
				for _, path := range extractedFiles {
					os.RemoveAll(path)
				}
				return fmt.Errorf("failed to write file %s: %w", fullPath, err)
			}

			outFile.Close()
			extractedFiles = append(extractedFiles, fullPath)

		case tar.TypeSymlink, tar.TypeLink:
			// Skip symlinks and hardlinks for security
			// Silent skip - these are rare in release tarballs

		default:
			// Skip other types (char devices, block devices, etc.)
			// These are security risks and not needed for source code
		}
	}

	return nil
}

// ExtractZip extracts a zip archive to the target directory
// Validates all paths to prevent directory traversal attacks
func ExtractZip(archivePath, targetDir string) error {
	// Open zip archive
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive %s: %w", archivePath, err)
	}
	defer r.Close()

	// Track extracted files for cleanup on error
	extractedFiles := []string{}

	// Extract each file
	for _, f := range r.File {
		// Validate path with filepath.IsLocal
		if !filepath.IsLocal(f.Name) {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("path traversal detected: %s", f.Name)
		}

		// Resolve full path
		fullPath := filepath.Join(targetDir, f.Name)

		// Additional security check: verify resolved path is within targetDir
		absTarget, err := filepath.Abs(fullPath)
		if err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to resolve absolute path for %s: %w", fullPath, err)
		}
		absDir, err := filepath.Abs(targetDir)
		if err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to resolve absolute path for target dir %s: %w", targetDir, err)
		}
		if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) && absTarget != absDir {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("path traversal detected: %s resolves outside target directory", f.Name)
		}

		// Handle directories
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fullPath, 0o755); err != nil {
				// Cleanup on error
				for _, path := range extractedFiles {
					os.RemoveAll(path)
				}
				return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}
			extractedFiles = append(extractedFiles, fullPath)
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to create parent directory for %s: %w", fullPath, err)
		}

		// Open file from zip
		srcFile, err := f.Open()
		if err != nil {
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to open file in zip %s: %w", f.Name, err)
		}

		// Create output file
		outFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			srcFile.Close()
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to create file %s: %w", fullPath, err)
		}

		// Copy contents
		if _, err := io.Copy(outFile, srcFile); err != nil {
			srcFile.Close()
			outFile.Close()
			// Cleanup on error
			for _, path := range extractedFiles {
				os.RemoveAll(path)
			}
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}

		srcFile.Close()
		outFile.Close()
		extractedFiles = append(extractedFiles, fullPath)
	}

	return nil
}

// StripPrefix moves contents from dir/prefix/* to dir/* and removes the prefix directory
// Common for GitHub release tarballs that extract to a versioned directory
func StripPrefix(dir, prefix string) error {
	if !filepath.IsLocal(prefix) {
		return fmt.Errorf("invalid strip prefix %q", prefix)
	}
	prefixPath := filepath.Join(dir, prefix)

	// Check if prefix directory exists
	if _, err := os.Stat(prefixPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("prefix directory %s does not exist", prefix)
		}
		return fmt.Errorf("failed to stat prefix directory %s: %w", prefix, err)
	}

	// Read all entries in prefix directory
	entries, err := os.ReadDir(prefixPath)
	if err != nil {
		return fmt.Errorf("failed to read prefix directory %s: %w", prefix, err)
	}

	// Move each entry to parent directory
	for _, entry := range entries {
		oldPath := filepath.Join(prefixPath, entry.Name())
		newPath := filepath.Join(dir, entry.Name())

		// Check if destination already exists
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("destination %s already exists, cannot strip prefix", newPath)
		}

		// Move the entry
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to move %s to %s: %w", oldPath, newPath, err)
		}
	}

	// Remove the now-empty prefix directory
	if err := os.Remove(prefixPath); err != nil {
		return fmt.Errorf("failed to remove prefix directory %s: %w", prefixPath, err)
	}

	return nil
}

// DetectArchiveType returns the archive type based on the file path
func DetectArchiveType(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return "tar.gz"
	}
	if strings.HasSuffix(lower, ".zip") {
		return "zip"
	}
	return ""
}
