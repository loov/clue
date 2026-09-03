package fetch

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/loov/clue/internal/deps"
)

// Manager coordinates dependency fetching and caching.
type Manager struct {
	projectDir      string
	dependencies    map[string]deps.Dependency
	cache           *cache
	gitFetcher      *gitFetcher
	tarballFetcher  *tarballFetcher
	vendoredFetcher *vendoredFetcher
	resolver        *deps.Resolver
	verbose         bool
	lock            *lockFile
}

// Options configures a Manager.
type Options struct {
	Verbose bool
}

// Status describes a dependency's local state.
type Status struct {
	Name     string
	Type     string // "git", "tarball", "vendored", "pkg_config"
	Status   string // "cached", "missing", "system"
	Location string // Local path
	Ref      string // For git: branch/tag/commit
}

// NewManager creates a new dependency manager
func NewManager(projectDir string, dependencies map[string]deps.Dependency, opts Options) (*Manager, error) {
	// Initialize cache
	cache, err := newCache(projectDir, opts.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize fetchers
	gitFetcher := newGitFetcher(opts.Verbose)
	tarballFetcher := newTarballFetcher(opts.Verbose)
	vendoredFetcher := newVendoredFetcher(projectDir, opts.Verbose)

	// Initialize resolver
	resolver := deps.NewResolver(dependencies)
	lock, err := loadLockFile(projectDir)
	if err != nil {
		return nil, err
	}

	return &Manager{
		projectDir:      projectDir,
		dependencies:    dependencies,
		cache:           cache,
		gitFetcher:      gitFetcher,
		tarballFetcher:  tarballFetcher,
		vendoredFetcher: vendoredFetcher,
		resolver:        resolver,
		verbose:         opts.Verbose,
		lock:            lock,
	}, nil
}

// FetchAll fetches all dependencies in build order
func (m *Manager) FetchAll(ctx context.Context) error {
	order, err := m.resolver.BuildOrder()
	if err != nil {
		return fmt.Errorf("failed to determine build order: %w", err)
	}

	if len(order) == 0 {
		if m.verbose {
			fmt.Println("No dependencies to fetch")
		}
		return nil
	}

	fmt.Println("Fetching dependencies...")

	for i, name := range order {
		dep := m.dependencies[name]
		if dep == nil {
			return fmt.Errorf("dependency %q not found", name)
		}
		if dep.Type() == "pkg_config" {
			if m.verbose {
				fmt.Printf("  [%d/%d] Using system package %s\n", i+1, len(order), dep.(*deps.PkgConfigDependency).Package)
			}
			continue
		}

		// Check cache
		cached, err := m.cacheReady(dep, true)
		if err != nil {
			return err
		}
		if cached {
			fmt.Printf("  [%d/%d] Using cached %s\n", i+1, len(order), name)
			continue
		}

		// Fetch based on type
		cachePath := m.cache.path(dep)
		if dep.Type() == "git" || dep.Type() == "tarball" {
			if err := os.RemoveAll(cachePath); err != nil {
				return fmt.Errorf("remove incomplete cache for %q: %w", name, err)
			}
		}
		var fetchErr error

		switch dep.Type() {
		case "git":
			gitDep := dep.(*deps.GitDependency)
			fmt.Printf("  [%d/%d] Cloning %s (git:%s)\n", i+1, len(order), name, gitDep.Ref)
			ref, err := m.lock.gitRef(gitDep)
			if err != nil {
				return err
			}
			fetchErr = m.gitFetcher.fetchRef(ctx, gitDep, ref, cachePath)

		case "tarball":
			tarballDep := dep.(*deps.TarballDependency)
			fmt.Printf("  [%d/%d] Downloading %s.tar.gz\n", i+1, len(order), name)
			fetchErr = m.tarballFetcher.fetch(ctx, tarballDep, cachePath)

		case "vendored":
			fmt.Printf("  [%d/%d] Validating vendored %s\n", i+1, len(order), name)
			fetchErr = m.vendoredFetcher.fetch(ctx, dep, cachePath)

		default:
			return fmt.Errorf("unsupported dependency type %q for %s", dep.Type(), name)
		}

		// Fail-fast on error
		if fetchErr != nil {
			return fmt.Errorf("failed to fetch %s: %w", name, fetchErr)
		}

		// Mark as fetched
		if err := m.cache.markFetched(dep); err != nil {
			return fmt.Errorf("failed to mark %s as fetched: %w", name, err)
		}
		if err := m.recordLock(dep); err != nil {
			return err
		}
	}

	fmt.Println("All dependencies ready")
	return nil
}

// FetchOne fetches a single dependency by name
func (m *Manager) FetchOne(ctx context.Context, name string) error {
	dep := m.dependencies[name]
	if dep == nil {
		return fmt.Errorf("dependency %q not found", name)
	}
	if dep.Type() == "pkg_config" {
		return nil
	}

	// Check cache
	cached, err := m.cacheReady(dep, true)
	if err != nil {
		return err
	}
	if cached {
		if m.verbose {
			fmt.Printf("Using cached %s\n", name)
		}
		return nil
	}

	// Fetch based on type
	cachePath := m.cache.path(dep)
	if dep.Type() == "git" || dep.Type() == "tarball" {
		if err := os.RemoveAll(cachePath); err != nil {
			return fmt.Errorf("remove incomplete cache for %q: %w", name, err)
		}
	}
	var fetchErr error

	fmt.Printf("Fetching %s...\n", name)

	switch dep.Type() {
	case "git":
		gitDep := dep.(*deps.GitDependency)
		if m.verbose {
			fmt.Printf("Cloning from %s (ref: %s)\n", gitDep.Repo, gitDep.Ref)
		}
		ref, err := m.lock.gitRef(gitDep)
		if err != nil {
			return err
		}
		fetchErr = m.gitFetcher.fetchRef(ctx, gitDep, ref, cachePath)

	case "tarball":
		tarballDep := dep.(*deps.TarballDependency)
		if m.verbose {
			fmt.Printf("Downloading from %s\n", tarballDep.URL)
		}
		fetchErr = m.tarballFetcher.fetch(ctx, tarballDep, cachePath)

	case "vendored":
		vendoredDep := dep.(*deps.VendoredDependency)
		if m.verbose {
			fmt.Printf("Validating at %s\n", vendoredDep.Path)
		}
		fetchErr = m.vendoredFetcher.fetch(ctx, dep, cachePath)

	default:
		return fmt.Errorf("unsupported dependency type %q", dep.Type())
	}

	if fetchErr != nil {
		return fetchErr
	}

	// Mark as fetched
	if err := m.cache.markFetched(dep); err != nil {
		return fmt.Errorf("failed to mark as fetched: %w", err)
	}
	if err := m.recordLock(dep); err != nil {
		return err
	}

	fmt.Printf("Dependency %s ready\n", name)
	return nil
}

// UpdateAll resolves configured Git refs again and rewrites their lock entries.
func (m *Manager) UpdateAll(ctx context.Context) error {
	names := slices.Sorted(maps.Keys(m.dependencies))

	updated := 0
	for _, name := range names {
		dep := m.dependencies[name]
		if dep.Type() != "git" {
			continue
		}
		delete(m.lock.Dependencies, name)
		if err := os.RemoveAll(m.cache.path(dep)); err != nil {
			return fmt.Errorf("remove cached dependency %q: %w", name, err)
		}
		if err := m.FetchOne(ctx, name); err != nil {
			return err
		}
		fmt.Printf("Updated %s\n", name)
		updated++
	}
	if updated == 0 {
		fmt.Println("All dependencies are up to date")
	}
	return nil
}

// Status returns the status of all dependencies
func (m *Manager) Status() []Status {
	statuses := make([]Status, 0, len(m.dependencies))

	for name, dep := range m.dependencies {
		status := Status{
			Name:     name,
			Type:     dep.Type(),
			Location: m.cache.path(dep),
		}

		// Determine if cached or missing
		if dep.Type() == "pkg_config" {
			status.Status = "system"
			status.Location = dep.(*deps.PkgConfigDependency).Package
		} else if cached, _ := m.cacheReady(dep, false); cached {
			status.Status = "cached"
		} else {
			status.Status = "missing"
		}

		// Add type-specific info
		switch d := dep.(type) {
		case *deps.GitDependency:
			status.Ref = d.Ref
		case *deps.TarballDependency:
			status.Ref = d.URL
		case *deps.VendoredDependency:
			status.Ref = d.Path
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func (m *Manager) cacheReady(dep deps.Dependency, record bool) (bool, error) {
	if !m.cache.has(dep) {
		return false, nil
	}
	entry, locked := m.lock.Dependencies[dep.Name()]
	if !locked {
		if record {
			if err := m.recordLock(dep); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	switch configured := dep.(type) {
	case *deps.GitDependency:
		if _, err := m.lock.gitRef(configured); err != nil {
			return false, err
		}
		commit, err := gitCommit(m.cache.path(dep))
		return err == nil && commit == entry.Commit, err
	case *deps.TarballDependency:
		if entry.Type != dep.Type() || entry.URL != configured.URL || entry.Checksum != configured.Checksum {
			return false, fmt.Errorf("%s entry for %q does not match clue.cue; run 'clue deps update'", lockFileName, dep.Name())
		}
	}
	return true, nil
}

func (m *Manager) recordLock(dep deps.Dependency) error {
	var entry lockEntry
	switch configured := dep.(type) {
	case *deps.GitDependency:
		commit, err := gitCommit(m.cache.path(dep))
		if err != nil {
			return fmt.Errorf("resolve commit for %q: %w", dep.Name(), err)
		}
		entry = lockEntry{Type: dep.Type(), URL: configured.Repo, Ref: configured.Ref, Commit: commit}
	case *deps.TarballDependency:
		entry = lockEntry{Type: dep.Type(), URL: configured.URL, Checksum: configured.Checksum}
	default:
		return nil
	}
	if m.lock.Dependencies[dep.Name()] == entry {
		return nil
	}
	m.lock.Dependencies[dep.Name()] = entry
	return m.lock.save(m.projectDir)
}

// Clean removes the entire .deps directory
func (m *Manager) Clean() error {
	return m.cache.clean()
}

// CleanOne removes a specific dependency from cache
func (m *Manager) CleanOne(name string) error {
	if m.dependencies[name] == nil {
		return fmt.Errorf("dependency %q not found", name)
	}
	return m.cache.cleanDep(name)
}
