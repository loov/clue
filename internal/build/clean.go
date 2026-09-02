package build

import (
	"fmt"
	"os"
	"path/filepath"
)

// CleanOptions configures what to clean during a clean operation
type CleanOptions struct {
	BuildDir string // Build root directory (default: ".build")
	Variant  string // Variant to clean (e.g., "debug", "release")
	All      bool   // If true, remove entire BuildDir
}

// CleanResult contains information about what was cleaned
type CleanResult struct {
	Path    string // Path that was cleaned
	Existed bool   // Whether it existed before cleaning
}

// String returns a human-readable description of the clean result
func (r *CleanResult) String() string {
	if r.Existed {
		return fmt.Sprintf("Cleaned: %s", r.Path)
	}
	return fmt.Sprintf("Already clean: %s (not found)", r.Path)
}

// Clean removes build artifacts based on the provided options
func Clean(opts CleanOptions) (*CleanResult, error) {
	// Determine target path
	var targetPath string
	if opts.All {
		targetPath = opts.BuildDir
	} else {
		// If no variant specified, return error
		if opts.Variant == "" {
			return nil, fmt.Errorf("variant must be specified when not using --all")
		}
		if opts.Variant == "." || !filepath.IsLocal(opts.Variant) || filepath.Base(opts.Variant) != opts.Variant {
			return nil, fmt.Errorf("invalid variant %q", opts.Variant)
		}
		targetPath = filepath.Join(opts.BuildDir, opts.Variant)
	}

	// Check if path exists
	_, err := os.Stat(targetPath)
	existed := err == nil

	// Remove if it exists
	if existed {
		if err := os.RemoveAll(targetPath); err != nil {
			return nil, fmt.Errorf("failed to remove %s: %w", targetPath, err)
		}
	}

	return &CleanResult{
		Path:    targetPath,
		Existed: existed,
	}, nil
}
