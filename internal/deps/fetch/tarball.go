package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/deps"
)

const maxTarballBytes int64 = 1 << 30

var tarballHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// tarballFetcher downloads and extracts tarball dependencies.
type tarballFetcher struct {
	verbose bool
}

// newTarballFetcher creates a tarball fetcher.
func newTarballFetcher(verbose bool) *tarballFetcher {
	return &tarballFetcher{verbose: verbose}
}

// fetch downloads, verifies, and extracts a tarball dependency.
func (f *tarballFetcher) fetch(ctx context.Context, dep *deps.TarballDependency, targetPath string) (resultErr error) {
	if err := dep.Validate(); err != nil {
		return err
	}
	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "clue-dep-*.tar")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFileName := tmpFile.Name()
	tmpClosed := false
	defer func() {
		if !tmpClosed {
			resultErr = errors.Join(resultErr, tmpFile.Close())
		}
		resultErr = errors.Join(resultErr, os.Remove(tmpFileName))
	}()

	// Download with checksum computation
	if f.verbose {
		fmt.Printf("Downloading %s...\n", dep.URL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", dep.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", dep.URL, err)
	}

	resp, err := tarballHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", dep.URL, err)
	}
	bodyClosed := false
	defer func() {
		if !bodyClosed {
			resultErr = errors.Join(resultErr, resp.Body.Close())
		}
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: HTTP %d", dep.URL, resp.StatusCode)
	}
	if resp.ContentLength > maxTarballBytes {
		return fmt.Errorf("failed to download %s: archive exceeds %d byte limit", dep.URL, maxTarballBytes)
	}

	// Compute checksum while downloading
	h := sha256.New()
	w := io.MultiWriter(tmpFile, h)

	bytesWritten, copyErr := io.Copy(w, io.LimitReader(resp.Body, maxTarballBytes+1))
	bodyErr := resp.Body.Close()
	bodyClosed = true
	if err := errors.Join(copyErr, bodyErr); err != nil {
		return fmt.Errorf("failed to download %s: %w", dep.URL, err)
	}
	if err := tmpFile.Close(); err != nil {
		tmpClosed = true
		return fmt.Errorf("failed to close download for %s: %w", dep.URL, err)
	}
	tmpClosed = true
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

	archiveType := detectArchiveType(dep.URL)
	switch archiveType {
	case "tar.gz":
		if err := extractTarGz(tmpFileName, targetPath); err != nil {
			return errors.Join(fmt.Errorf("failed to extract tar.gz: %w", err), os.RemoveAll(targetPath))
		}
	case "zip":
		if err := extractZip(tmpFileName, targetPath); err != nil {
			return errors.Join(fmt.Errorf("failed to extract zip: %w", err), os.RemoveAll(targetPath))
		}
	default:
		return errors.Join(fmt.Errorf("unsupported archive format: %s", dep.URL), os.RemoveAll(targetPath))
	}

	// Handle stripPrefix if specified
	if dep.StripPrefix != "" {
		if f.verbose {
			fmt.Printf("Stripping prefix: %s\n", dep.StripPrefix)
		}
		if err := stripPrefix(targetPath, dep.StripPrefix); err != nil {
			return errors.Join(fmt.Errorf("failed to strip prefix %s: %w", dep.StripPrefix, err), os.RemoveAll(targetPath))
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
