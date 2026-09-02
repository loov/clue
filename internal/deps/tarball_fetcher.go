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
	"time"
)

const maxTarballBytes int64 = 1 << 30

var tarballHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// TarballFetcher downloads and extracts tarball dependencies
type TarballFetcher struct {
	verbose bool
}

// NewTarballFetcher creates a new tarball fetcher
func NewTarballFetcher(verbose bool) *TarballFetcher {
	return &TarballFetcher{verbose: verbose}
}

// Fetch downloads, verifies, and extracts a tarball dependency
func (f *TarballFetcher) Fetch(ctx context.Context, dep *TarballDependency, targetPath string) error {
	if err := dep.Validate(); err != nil {
		return err
	}
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

	resp, err := tarballHTTPClient.Do(req)
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
	if resp.ContentLength > maxTarballBytes {
		tmpFile.Close()
		return fmt.Errorf("failed to download %s: archive exceeds %d byte limit", dep.URL, maxTarballBytes)
	}

	// Compute checksum while downloading
	h := sha256.New()
	w := io.MultiWriter(tmpFile, h)

	bytesWritten, err := io.Copy(w, io.LimitReader(resp.Body, maxTarballBytes+1))
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download %s: %w", dep.URL, err)
	}
	tmpFile.Close()
	if bytesWritten > maxTarballBytes {
		return fmt.Errorf("failed to download %s: archive exceeds %d byte limit", dep.URL, maxTarballBytes)
	}

	actualChecksum := hex.EncodeToString(h.Sum(nil))

	if f.verbose {
		fmt.Printf("Download complete (%d bytes)\n", bytesWritten)
	}

	if actualChecksum != dep.Checksum {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
			dep.Name(), dep.Checksum, actualChecksum)
	}
	if f.verbose {
		fmt.Printf("Checksum verified: %s\n", actualChecksum)
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
