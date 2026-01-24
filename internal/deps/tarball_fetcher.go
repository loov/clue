package deps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// TarballFetcher downloads and extracts tarball dependencies
type TarballFetcher struct {
	verbose bool
	ciMode  bool
}

// NewTarballFetcher creates a new tarball fetcher
func NewTarballFetcher(verbose, ciMode bool) *TarballFetcher {
	return &TarballFetcher{
		verbose: verbose,
		ciMode:  ciMode,
	}
}

// Fetch downloads, verifies, and extracts a tarball dependency
func (f *TarballFetcher) Fetch(ctx context.Context, dep *TarballDependency, targetPath string) error {
	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "clue-dep-*.tar")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFileName := tmpFile.Name()
	defer os.Remove(tmpFileName)

	// Download with checksum computation
	if f.verbose {
		fmt.Printf("Downloading %s...\n", dep.URL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", dep.URL, nil)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to create request for %s: %w", dep.URL, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download %s: %w", dep.URL, err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("failed to download %s: HTTP %d", dep.URL, resp.StatusCode)
	}

	// Compute checksum while downloading
	h := sha256.New()
	w := io.MultiWriter(tmpFile, h)

	bytesWritten, err := io.Copy(w, resp.Body)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download %s: %w", dep.URL, err)
	}
	tmpFile.Close()

	actualChecksum := hex.EncodeToString(h.Sum(nil))

	if f.verbose {
		fmt.Printf("Download complete (%d bytes)\n", bytesWritten)
	}

	// Checksum verification
	if dep.Checksum != "" {
		if actualChecksum != dep.Checksum {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
				dep.Name(), dep.Checksum, actualChecksum)
		}
		if f.verbose {
			fmt.Printf("Checksum verified: %s\n", actualChecksum)
		}
	} else {
		// Missing checksum
		if f.ciMode {
			return fmt.Errorf("dependency %s missing checksum (required in CI mode)", dep.Name())
		}
		// Warning in non-CI mode
		fmt.Printf("Warning: %s has no checksum verification\n", dep.Name())
	}

	// Detect archive type and extract
	if f.verbose {
		fmt.Printf("Extracting to %s...\n", targetPath)
	}

	// Create target directory
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetPath, err)
	}

	archiveType := DetectArchiveType(dep.URL)
	switch archiveType {
	case "tar.gz":
		if err := ExtractTarGz(tmpFileName, targetPath); err != nil {
			os.RemoveAll(targetPath)
			return fmt.Errorf("failed to extract tar.gz: %w", err)
		}
	case "zip":
		if err := ExtractZip(tmpFileName, targetPath); err != nil {
			os.RemoveAll(targetPath)
			return fmt.Errorf("failed to extract zip: %w", err)
		}
	default:
		os.RemoveAll(targetPath)
		return fmt.Errorf("unsupported archive format: %s", dep.URL)
	}

	// Handle stripPrefix if specified
	if dep.StripPrefix != "" {
		if f.verbose {
			fmt.Printf("Stripping prefix: %s\n", dep.StripPrefix)
		}
		if err := StripPrefix(targetPath, dep.StripPrefix); err != nil {
			os.RemoveAll(targetPath)
			return fmt.Errorf("failed to strip prefix %s: %w", dep.StripPrefix, err)
		}
	}

	// Count extracted files for verbose output
	if f.verbose {
		fileCount := 0
		// Walk errors are non-fatal for file counting (ignore walk errors)
		_ = filepath.Walk(targetPath, func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})
		fmt.Printf("Extracted %d files\n", fileCount)
	}

	return nil
}
