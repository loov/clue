package fetch

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/loov/clue/internal/deps"
	"golang.org/x/sync/errgroup"
)

// Manager coordinates dependency fetching and caching.
type Manager struct {
	projectDir      string
	dependencies    map[string]deps.Dependency
	cache           *cache
	gitFetcher      *gitFetcher
	tarballFetcher  *tarballFetcher
	vendoredFetcher *vendoredFetcher
	verbose         bool
	quiet           bool       // no progress output, only errors
	lockMu          sync.Mutex // guards lock; FetchAll fetches concurrently
	lock            *lockFile
}

// Options configures a Manager.
type Options struct {
	Verbose bool
	Quiet   bool // suppress progress output
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
	// "<dependency>:<target>" entries share their dependency's checkout, so
	// only that dependency is fetched, locked and listed.
	checkouts := make(map[string]deps.Dependency, len(dependencies))
	for name, dependency := range dependencies {
		if _, ok := dependency.(*deps.TargetDependency); !ok {
			checkouts[name] = dependency
		}
	}
	dependencies = checkouts

	// Initialize cache
	cache, err := newCache(projectDir, opts.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize fetchers
	gitFetcher := newGitFetcher(opts.Verbose)
	tarballFetcher := newTarballFetcher(opts.Verbose)
	vendoredFetcher := newVendoredFetcher(projectDir, opts.Verbose)

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
		verbose:         opts.Verbose,
		quiet:           opts.Quiet,
		lock:            lock,
	}, nil
}

// FetchAll fetches independent checkouts. Build dependencies are resolved by
// the builder, after every checkout's description is available.
func (m *Manager) FetchAll(ctx context.Context) error {
	order := slices.Sorted(maps.Keys(m.dependencies))

	if len(order) == 0 {
		if m.verbose {
			fmt.Println("No dependencies to fetch")
		}
		return nil
	}

	m.progress("Fetching dependencies...")

	// Fetches are independent of each other; the build order only numbers them.
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i, name := range order {
		group.Go(func() error { return m.fetchListed(ctx, i, len(order), name) })
	}
	if err := group.Wait(); err != nil {
		return err
	}

	m.progress("All dependencies ready")
	return nil
}

func (m *Manager) progress(message string) {
	if !m.quiet {
		fmt.Println(message)
	}
}

func (m *Manager) progressf(format string, args ...any) {
	if !m.quiet {
		fmt.Printf(format, args...)
	}
}

// fetchListed fetches dependency name, number i of count in FetchAll's listing.
func (m *Manager) fetchListed(ctx context.Context, i, count int, name string) error {
	dep := m.dependencies[name]
	if dep == nil {
		return fmt.Errorf("dependency %q not found", name)
	}
	if dep.Type() == "pkg_config" {
		if m.verbose {
			fmt.Printf("  [%d/%d] Using system package %s\n", i+1, count, dep.(*deps.PkgConfigDependency).Package)
		}
		return nil
	}

	// Check cache
	cached, err := m.cacheReady(dep, true)
	if err != nil {
		return err
	}
	if cached {
		m.progressf("  [%d/%d] Using cached %s\n", i+1, count, name)
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

	switch dep.Type() {
	case "git":
		gitDep := dep.(*deps.GitDependency)
		m.progressf("  [%d/%d] Cloning %s (git:%s)\n", i+1, count, name, gitDep.Ref)
		m.lockMu.Lock()
		ref, err := m.lock.gitRef(gitDep)
		m.lockMu.Unlock()
		if err != nil {
			return err
		}
		fetchErr = m.gitFetcher.fetchRef(ctx, gitDep, ref, cachePath)

	case "tarball":
		tarballDep := dep.(*deps.TarballDependency)
		m.progressf("  [%d/%d] Downloading %s.tar.gz\n", i+1, count, name)
		fetchErr = m.tarballFetcher.fetch(ctx, tarballDep, cachePath)

	case "vendored":
		m.progressf("  [%d/%d] Validating vendored %s\n", i+1, count, name)
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
	return nil
}

// FetchOne fetches a single dependency by name
func (m *Manager) FetchOne(ctx context.Context, name string) error {
	name, _, _ = strings.Cut(name, ":") // a dependency's target is fetched with the dependency
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

	m.progressf("Fetching %s...\n", name)

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

	m.progressf("Dependency %s ready\n", name)
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
		m.progressf("Updated %s\n", name)
		updated++
	}
	if updated == 0 {
		m.progress("All dependencies are up to date")
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
	m.lockMu.Lock()
	entry, locked := m.lock.Dependencies[dep.Name()]
	m.lockMu.Unlock()
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
		m.lockMu.Lock()
		_, err := m.lock.gitRef(configured)
		m.lockMu.Unlock()
		if err != nil {
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

// PruneLock removes lock entries of dependencies that are no longer
// configured. Call it once every dependency, including those that fetched
// dependencies declare, has been fetched.
func (m *Manager) PruneLock() error {
	m.lockMu.Lock()
	defer m.lockMu.Unlock()
	pruned := false
	for name := range m.lock.Dependencies {
		if _, configured := m.dependencies[name]; !configured {
			delete(m.lock.Dependencies, name)
			pruned = true
		}
	}
	if !pruned {
		return nil
	}
	return m.lock.save(m.projectDir)
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
	m.lockMu.Lock()
	defer m.lockMu.Unlock()
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
