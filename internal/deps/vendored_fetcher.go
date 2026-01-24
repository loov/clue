package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// VendoredFetcher validates local vendored dependencies
type VendoredFetcher struct {
	projectDir string
	verbose    bool
}

// NewVendoredFetcher creates a new vendored fetcher
func NewVendoredFetcher(projectDir string, verbose bool) *VendoredFetcher {
	return &VendoredFetcher{
		projectDir: projectDir,
		verbose:    verbose,
	}
}

// Fetch validates that the vendored dependency exists at the specified path
// For vendored dependencies, "fetching" means validation only - no actual copying
func (f *VendoredFetcher) Fetch(_ context.Context, dep Dependency, _ string) error {
	vendoredDep, ok := dep.(*VendoredDependency)
	if !ok {
		return fmt.Errorf("expected VendoredDependency, got %T", dep)
	}

	if f.verbose {
		fmt.Printf("Validating vendored dependency %s at %s...\n", vendoredDep.Name(), vendoredDep.Path)
	}

	// Resolve relative path to absolute
	absPath := vendoredDep.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(f.projectDir, vendoredDep.Path)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("vendored dependency %s not found at %s", vendoredDep.Name(), absPath)
		}
		return fmt.Errorf("failed to stat vendored dependency %s: %w", vendoredDep.Name(), err)
	}

	if !info.IsDir() {
		return fmt.Errorf("vendored dependency %s path is not a directory: %s", vendoredDep.Name(), absPath)
	}

	// Count source files for reporting
	sourceCount := 0
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".c" || ext == ".cpp" || ext == ".cc" || ext == ".cxx" || ext == ".h" || ext == ".hpp" {
				sourceCount++
			}
		}
		return nil
	})
	if err != nil {
		// Don't fail on walk errors, just report with unknown count
		if f.verbose {
			fmt.Printf("Found vendored dependency %s\n", vendoredDep.Name())
		}
		return nil
	}

	if f.verbose {
		fmt.Printf("Found vendored dependency %s (%d source files)\n", vendoredDep.Name(), sourceCount)
	}

	// Check for build configuration
	hasClueConfig := false
	hasInlineConfig := vendoredDep.BuildConfig != nil

	clueConfigPath := filepath.Join(absPath, "clue.cue")
	if _, err := os.Stat(clueConfigPath); err == nil {
		hasClueConfig = true
	}

	// Warn if no build configuration
	if !hasClueConfig && !hasInlineConfig {
		if f.verbose {
			fmt.Printf("Warning: %s has no build configuration (no clue.cue or inline config)\n", vendoredDep.Name())
		}
	}

	return nil
}

// Path returns the resolved absolute path for the vendored dependency
func (f *VendoredFetcher) Path(dep *VendoredDependency) string {
	absPath := dep.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(f.projectDir, dep.Path)
	}
	return absPath
}
