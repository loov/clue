package deps

import (
	"context"
	"fmt"
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

	return &Manager{
		projectDir:      projectDir,
		cache:           cache,
		gitFetcher:      gitFetcher,
		tarballFetcher:  tarballFetcher,
		vendoredFetcher: vendoredFetcher,
		resolver:        resolver,
		verbose:         opts.Verbose,
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
		if m.cache.Has(dep) {
			fmt.Printf("  [%d/%d] Using cached %s\n", i+1, len(order), name)
			continue
		}

		// Fetch based on type
		cachePath := m.cache.Path(dep)
		var fetchErr error

		switch dep.Type() {
		case "git":
			gitDep := dep.(*GitDependency)
			fmt.Printf("  [%d/%d] Cloning %s (git:%s)\n", i+1, len(order), name, gitDep.Ref)
			fetchErr = m.gitFetcher.Fetch(ctx, dep, cachePath)

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
	if m.cache.Has(dep) {
		if m.verbose {
			fmt.Printf("Using cached %s\n", name)
		}
		return nil
	}

	// Fetch based on type
	cachePath := m.cache.Path(dep)
	var fetchErr error

	fmt.Printf("Fetching %s...\n", name)

	switch dep.Type() {
	case "git":
		gitDep := dep.(*GitDependency)
		if m.verbose {
			fmt.Printf("Cloning from %s (ref: %s)\n", gitDep.Repo, gitDep.Ref)
		}
		fetchErr = m.gitFetcher.Fetch(ctx, dep, cachePath)

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

	fmt.Printf("Dependency %s ready\n", name)
	return nil
}

// UpdateAll fetches missing Git dependencies and fast-forwards cached branches.
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
		if !m.cache.Has(dep) {
			if err := m.FetchOne(ctx, name); err != nil {
				return err
			}
			updated++
			continue
		}
		changed, err := m.gitFetcher.Update(ctx, m.cache.Path(dep))
		if err != nil {
			return fmt.Errorf("failed to update %s: %w", name, err)
		}
		if changed {
			if err := m.cache.MarkFetched(dep); err != nil {
				return err
			}
			fmt.Printf("Updated %s\n", name)
			updated++
		}
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
		} else if m.cache.Has(dep) {
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
