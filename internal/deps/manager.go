package deps

import (
	"context"
	"fmt"
	"os"
	"sort"
)

// Manager coordinates dependency fetching and building
type Manager struct {
	projectDir      string
	cache           *Cache
	gitFetcher      *GitFetcher
	tarballFetcher  *TarballFetcher
	vendoredFetcher *VendoredFetcher
	resolver        *Resolver
	verbose         bool
	lock            *LockFile
}

// ManagerOptions configures the manager
type ManagerOptions struct {
	Verbose bool
}

// DepStatus represents the status of a dependency
type DepStatus struct {
	Name     string
	Type     string // "git", "tarball", "vendored", "pkg_config"
	Status   string // "cached", "missing", "system"
	Location string // Local path
	Ref      string // For git: branch/tag/commit
}

// NewManager creates a new dependency manager
func NewManager(projectDir string, deps map[string]Dependency, opts ManagerOptions) (*Manager, error) {
	// Initialize cache
	cache, err := NewCache(projectDir, opts.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize fetchers
	gitFetcher := NewGitFetcher(opts.Verbose)
	tarballFetcher := NewTarballFetcher(opts.Verbose)
	vendoredFetcher := NewVendoredFetcher(projectDir, opts.Verbose)

	// Initialize resolver
	resolver := NewResolver(deps)
	lock, err := loadLockFile(projectDir)
	if err != nil {
		return nil, err
	}

	return &Manager{
		projectDir:      projectDir,
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
		dep := m.resolver.dependencies[name]
		if dep == nil {
			return fmt.Errorf("dependency %q not found", name)
		}
		if dep.Type() == "pkg_config" {
			if m.verbose {
				fmt.Printf("  [%d/%d] Using system package %s\n", i+1, len(order), dep.(*PkgConfigDependency).Package)
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
		cachePath := m.cache.Path(dep)
		if dep.Type() == "git" || dep.Type() == "tarball" {
			if err := os.RemoveAll(cachePath); err != nil {
				return fmt.Errorf("remove incomplete cache for %q: %w", name, err)
			}
		}
		var fetchErr error

		switch dep.Type() {
		case "git":
			gitDep := dep.(*GitDependency)
			fmt.Printf("  [%d/%d] Cloning %s (git:%s)\n", i+1, len(order), name, gitDep.Ref)
			ref, err := m.lock.gitRef(gitDep)
			if err != nil {
				return err
			}
			fetchErr = m.gitFetcher.FetchRef(ctx, gitDep, ref, cachePath)

		case "tarball":
			tarballDep := dep.(*TarballDependency)
			fmt.Printf("  [%d/%d] Downloading %s.tar.gz\n", i+1, len(order), name)
			fetchErr = m.tarballFetcher.Fetch(ctx, tarballDep, cachePath)

		case "vendored":
			fmt.Printf("  [%d/%d] Validating vendored %s\n", i+1, len(order), name)
			fetchErr = m.vendoredFetcher.Fetch(ctx, dep, cachePath)

		default:
			return fmt.Errorf("unsupported dependency type %q for %s", dep.Type(), name)
		}

		// Fail-fast on error
		if fetchErr != nil {
			return fmt.Errorf("failed to fetch %s: %w", name, fetchErr)
		}

		// Mark as fetched
		if err := m.cache.MarkFetched(dep); err != nil {
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
	dep := m.resolver.dependencies[name]
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
	cachePath := m.cache.Path(dep)
	if dep.Type() == "git" || dep.Type() == "tarball" {
		if err := os.RemoveAll(cachePath); err != nil {
			return fmt.Errorf("remove incomplete cache for %q: %w", name, err)
		}
	}
	var fetchErr error

	fmt.Printf("Fetching %s...\n", name)

	switch dep.Type() {
	case "git":
		gitDep := dep.(*GitDependency)
		if m.verbose {
			fmt.Printf("Cloning from %s (ref: %s)\n", gitDep.Repo, gitDep.Ref)
		}
		ref, err := m.lock.gitRef(gitDep)
		if err != nil {
			return err
		}
		fetchErr = m.gitFetcher.FetchRef(ctx, gitDep, ref, cachePath)

	case "tarball":
		tarballDep := dep.(*TarballDependency)
		if m.verbose {
			fmt.Printf("Downloading from %s\n", tarballDep.URL)
		}
		fetchErr = m.tarballFetcher.Fetch(ctx, tarballDep, cachePath)

	case "vendored":
		vendoredDep := dep.(*VendoredDependency)
		if m.verbose {
			fmt.Printf("Validating at %s\n", vendoredDep.Path)
		}
		fetchErr = m.vendoredFetcher.Fetch(ctx, dep, cachePath)

	default:
		return fmt.Errorf("unsupported dependency type %q", dep.Type())
	}

	if fetchErr != nil {
		return fetchErr
	}

	// Mark as fetched
	if err := m.cache.MarkFetched(dep); err != nil {
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
	names := make([]string, 0, len(m.resolver.dependencies))
	for name := range m.resolver.dependencies {
		names = append(names, name)
	}
	sort.Strings(names)

	updated := 0
	for _, name := range names {
		dep := m.resolver.dependencies[name]
		if dep.Type() != "git" {
			continue
		}
		delete(m.lock.Dependencies, name)
		if err := os.RemoveAll(m.cache.Path(dep)); err != nil {
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
func (m *Manager) Status() []DepStatus {
	statuses := make([]DepStatus, 0, len(m.resolver.dependencies))

	for name, dep := range m.resolver.dependencies {
		status := DepStatus{
			Name:     name,
			Type:     dep.Type(),
			Location: m.cache.Path(dep),
		}

		// Determine if cached or missing
		if dep.Type() == "pkg_config" {
			status.Status = "system"
			status.Location = dep.(*PkgConfigDependency).Package
		} else if cached, _ := m.cacheReady(dep, false); cached {
			status.Status = "cached"
		} else {
			status.Status = "missing"
		}

		// Add type-specific info
		switch d := dep.(type) {
		case *GitDependency:
			status.Ref = d.Ref
		case *TarballDependency:
			status.Ref = d.URL
		case *VendoredDependency:
			status.Ref = d.Path
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func (m *Manager) cacheReady(dep Dependency, record bool) (bool, error) {
	if !m.cache.Has(dep) {
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
	case *GitDependency:
		if _, err := m.lock.gitRef(configured); err != nil {
			return false, err
		}
		commit, err := gitCommit(m.cache.Path(dep))
		return err == nil && commit == entry.Commit, err
	case *TarballDependency:
		if entry.Type != dep.Type() || entry.URL != configured.URL || entry.Checksum != configured.Checksum {
			return false, fmt.Errorf("%s entry for %q does not match clue.cue; run 'clue deps update'", LockFileName, dep.Name())
		}
	}
	return true, nil
}

func (m *Manager) recordLock(dep Dependency) error {
	var entry LockEntry
	switch configured := dep.(type) {
	case *GitDependency:
		commit, err := gitCommit(m.cache.Path(dep))
		if err != nil {
			return fmt.Errorf("resolve commit for %q: %w", dep.Name(), err)
		}
		entry = LockEntry{Type: dep.Type(), URL: configured.Repo, Ref: configured.Ref, Commit: commit}
	case *TarballDependency:
		entry = LockEntry{Type: dep.Type(), URL: configured.URL, Checksum: configured.Checksum}
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
	return m.cache.Clean()
}

// CleanOne removes a specific dependency from cache
func (m *Manager) CleanOne(name string) error {
	if m.resolver.dependencies[name] == nil {
		return fmt.Errorf("dependency %q not found", name)
	}
	return m.cache.CleanDep(name)
}
