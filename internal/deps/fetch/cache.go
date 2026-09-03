// Package fetch downloads and caches external dependencies.
package fetch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/loov/clue/internal/deps"
)

// cache manages the local cache of fetched dependencies.
type cache struct {
	baseDir string // project root directory
	depsDir string // path to .deps directory
	verbose bool
}

// depMarker represents the .clue-dep marker file content.
type depMarker struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	FetchedAt time.Time `json:"fetched_at"`
	Ref       string    `json:"ref,omitzero"`  // for git dependencies
	URL       string    `json:"url,omitzero"`  // for tarball dependencies
	Path      string    `json:"path,omitzero"` // for vendored dependencies
	Checksum  string    `json:"checksum,omitzero"`
}

// newCache creates a dependency cache manager.
func newCache(projectDir string, verbose bool) (*cache, error) {
	depsDir := filepath.Join(projectDir, ".deps")

	// Create .deps directory if it doesn't exist
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create .deps directory: %w", err)
	}

	return &cache{
		baseDir: projectDir,
		depsDir: depsDir,
		verbose: verbose,
	}, nil
}

// has reports whether a dependency is fetched and cached.
func (c *cache) has(dep deps.Dependency) bool {
	cachePath := dep.CachePath(c.baseDir)

	switch dep.Type() {
	case "git":
		gitInfo, err := os.Stat(filepath.Join(cachePath, ".git"))
		if err != nil {
			return false
		}
		return gitInfo.IsDir() && c.markerMatches(dep)

	case "tarball":
		// The directory may be left behind by an interrupted extraction.
		return c.markerMatches(dep)

	case "vendored":
		// For vendored, the path is the original source location
		// Check if it exists (always "cached" if the path exists)
		info, err := os.Stat(cachePath)
		if err != nil {
			return false
		}
		return info.IsDir()

	case "pkg_config":
		return true

	default:
		return false
	}
}

func (c *cache) markerMatches(dep deps.Dependency) bool {
	data, err := os.ReadFile(filepath.Join(dep.CachePath(c.baseDir), ".clue-dep"))
	if err != nil {
		return false
	}
	var marker depMarker
	if json.Unmarshal(data, &marker) != nil || marker.Name != dep.Name() || marker.Type != dep.Type() {
		return false
	}
	switch configured := dep.(type) {
	case *deps.GitDependency:
		return marker.URL == configured.Repo && marker.Ref == configured.Ref
	case *deps.TarballDependency:
		return marker.URL == configured.URL && marker.Checksum == configured.Checksum
	default:
		return true
	}
}

// path returns the local path where dependency sources are located.
func (c *cache) path(dep deps.Dependency) string {
	return dep.CachePath(c.baseDir)
}

// markFetched records fetch metadata in the cache directory.
func (c *cache) markFetched(dep deps.Dependency) error {
	cachePath := dep.CachePath(c.baseDir)
	markerPath := filepath.Join(cachePath, ".clue-dep")

	marker := depMarker{
		Name:      dep.Name(),
		Type:      dep.Type(),
		FetchedAt: time.Now(),
	}

	// Add type-specific metadata
	switch d := dep.(type) {
	case *deps.GitDependency:
		marker.Ref = d.Ref
		marker.URL = d.Repo
	case *deps.TarballDependency:
		marker.URL = d.URL
		marker.Checksum = d.Checksum
	case *deps.VendoredDependency:
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

// clean removes the entire .deps directory.
func (c *cache) clean() error {
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

// cleanDep removes a specific dependency from the cache.
func (c *cache) cleanDep(name string) error {
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(name, "_")
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
