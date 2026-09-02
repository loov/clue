// Package deps provides dependency management including fetching,
// caching, and building external dependencies.
package deps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cache manages the local cache of fetched dependencies
type Cache struct {
	baseDir string // project root directory
	depsDir string // path to .deps directory
	verbose bool
}

// DepMarker represents the .clue-dep marker file content
type DepMarker struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	FetchedAt time.Time `json:"fetched_at"`
	Ref       string    `json:"ref,omitempty"`  // for git dependencies
	URL       string    `json:"url,omitempty"`  // for tarball dependencies
	Path      string    `json:"path,omitempty"` // for vendored dependencies
}

// NewCache creates a new dependency cache manager
func NewCache(projectDir string, verbose bool) (*Cache, error) {
	depsDir := filepath.Join(projectDir, ".deps")

	// Create .deps directory if it doesn't exist
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create .deps directory: %w", err)
	}

	return &Cache{
		baseDir: projectDir,
		depsDir: depsDir,
		verbose: verbose,
	}, nil
}

// Has checks if a dependency is already fetched and cached
func (c *Cache) Has(dep Dependency) bool {
	cachePath := dep.CachePath(c.baseDir)

	switch dep.Type() {
	case "git":
		// For git deps, check if cache directory exists and has .git directory
		gitDir := filepath.Join(cachePath, ".git")
		info, err := os.Stat(gitDir)
		if err != nil {
			return false
		}
		return info.IsDir()

	case "tarball":
		// The directory may be left behind by an interrupted extraction.
		info, err := os.Stat(filepath.Join(cachePath, ".clue-dep"))
		if err != nil {
			return false
		}
		return !info.IsDir()

	case "vendored":
		// For vendored, the path is the original source location
		// Check if it exists (always "cached" if the path exists)
		info, err := os.Stat(cachePath)
		if err != nil {
			return false
		}
		return info.IsDir()

	default:
		return false
	}
}

// Path returns the local path where dependency sources are located
func (c *Cache) Path(dep Dependency) string {
	return dep.CachePath(c.baseDir)
}

// MarkFetched creates a marker file in the cache directory to track fetch metadata
func (c *Cache) MarkFetched(dep Dependency) error {
	cachePath := dep.CachePath(c.baseDir)
	markerPath := filepath.Join(cachePath, ".clue-dep")

	marker := DepMarker{
		Name:      dep.Name(),
		Type:      dep.Type(),
		FetchedAt: time.Now(),
	}

	// Add type-specific metadata
	switch d := dep.(type) {
	case *GitDependency:
		marker.Ref = d.Ref
		marker.URL = d.Repo
	case *TarballDependency:
		marker.URL = d.URL
	case *VendoredDependency:
		marker.Path = d.Path
	}

	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal marker: %w", err)
	}

	if err := os.WriteFile(markerPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write marker file: %w", err)
	}

	if c.verbose {
		fmt.Printf("Marked %s as fetched in cache\n", dep.Name())
	}

	return nil
}

// Clean removes the entire .deps directory
func (c *Cache) Clean() error {
	if _, err := os.Lstat(c.depsDir); err != nil {
		if os.IsNotExist(err) {
			if c.verbose {
				fmt.Println("Cache already clean (no .deps directory)")
			}
			return nil
		}
		return fmt.Errorf("failed to inspect .deps directory: %w", err)
	}

	if err := os.RemoveAll(c.depsDir); err != nil {
		return fmt.Errorf("failed to remove .deps directory: %w", err)
	}

	if c.verbose {
		fmt.Printf("Removed .deps directory: %s\n", c.depsDir)
	}

	return nil
}

// CleanDep removes a specific dependency from the cache
func (c *Cache) CleanDep(name string) error {
	sanitized := sanitizeName(name)
	found := false

	// Search in git/ subdirectory
	gitDir := filepath.Join(c.depsDir, "git")
	if entries, err := os.ReadDir(gitDir); err == nil {
		for _, entry := range entries {
			// Match entries that start with sanitized name followed by dash
			if entry.IsDir() && (entry.Name() == sanitized || strings.HasPrefix(entry.Name(), sanitized+"-")) {
				path := filepath.Join(gitDir, entry.Name())
				if err := os.RemoveAll(path); err != nil {
					return fmt.Errorf("failed to remove %s: %w", path, err)
				}
				if c.verbose {
					fmt.Printf("Removed cached dependency: %s\n", path)
				}
				found = true
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", gitDir, err)
	}

	// Search in tarball/ subdirectory
	tarballDir := filepath.Join(c.depsDir, "tarball")
	if entries, err := os.ReadDir(tarballDir); err == nil {
		for _, entry := range entries {
			// Match entries that start with sanitized name followed by dash
			if entry.IsDir() && (entry.Name() == sanitized || strings.HasPrefix(entry.Name(), sanitized+"-")) {
				path := filepath.Join(tarballDir, entry.Name())
				if err := os.RemoveAll(path); err != nil {
					return fmt.Errorf("failed to remove %s: %w", path, err)
				}
				if c.verbose {
					fmt.Printf("Removed cached dependency: %s\n", path)
				}
				found = true
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", tarballDir, err)
	}

	if !found {
		return fmt.Errorf("dependency %q not found in cache", name)
	}

	return nil
}
