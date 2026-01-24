package deps

import (
	"context"
	"fmt"
	"sort"
)

// FetchOptions configures the fetch operation
type FetchOptions struct {
	Verbose bool
	CIMode  bool
	Name    string // Optional: fetch specific dependency
}

// RunList lists all dependencies and their status
func RunList(deps map[string]Dependency, verbose bool) error {
	if len(deps) == 0 {
		fmt.Println("No dependencies configured")
		return nil
	}

	// Create manager
	mgr, err := NewManager(".", deps, ManagerOptions{
		Verbose: verbose,
		CIMode:  false,
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Get status
	statuses := mgr.Status()

	// Sort by name for consistent output
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	// Print table
	fmt.Println("Dependencies:")
	fmt.Printf("  %-20s %-12s %-12s %s\n", "NAME", "TYPE", "STATUS", "LOCATION")

	for _, status := range statuses {
		location := status.Location
		if status.Status == "missing" {
			location = "-"
		}

		fmt.Printf("  %-20s %-12s %-12s %s\n",
			status.Name,
			status.Type,
			status.Status,
			location,
		)

		// Show additional info in verbose mode
		if verbose && status.Ref != "" {
			switch status.Type {
			case "git":
				fmt.Printf("    ref: %s\n", status.Ref)
			case "tarball":
				fmt.Printf("    url: %s\n", status.Ref)
			case "vendored":
				fmt.Printf("    path: %s\n", status.Ref)
			}
		}
	}

	return nil
}

// RunFetch fetches dependencies
func RunFetch(ctx context.Context, deps map[string]Dependency, opts FetchOptions) error {
	if len(deps) == 0 {
		fmt.Println("No dependencies to fetch")
		return nil
	}

	// Create manager
	mgr, err := NewManager(".", deps, ManagerOptions{
		Verbose: opts.Verbose,
		CIMode:  opts.CIMode,
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Fetch single or all
	if opts.Name != "" {
		return mgr.FetchOne(ctx, opts.Name)
	}

	// Get initial status for summary
	initialStatuses := mgr.Status()
	cachedCount := 0
	for _, status := range initialStatuses {
		if status.Status == "cached" {
			cachedCount++
		}
	}

	// Fetch all
	if err := mgr.FetchAll(ctx); err != nil {
		return err
	}

	// Calculate summary
	downloadedCount := len(initialStatuses) - cachedCount
	if downloadedCount > 0 {
		fmt.Printf("\nFetched %d dependencies (%d cached, %d downloaded)\n",
			len(initialStatuses), cachedCount, downloadedCount)
	}

	return nil
}

// RunClean removes dependency cache
func RunClean(deps map[string]Dependency, name string) error {
	// Create manager (verbose=false for clean)
	mgr, err := NewManager(".", deps, ManagerOptions{
		Verbose: false,
		CIMode:  false,
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Clean specific or all
	if name != "" {
		if err := mgr.CleanOne(name); err != nil {
			return err
		}
		fmt.Printf("Removed %s from cache\n", name)
		return nil
	}

	// Clean all
	if err := mgr.Clean(); err != nil {
		return err
	}

	fmt.Println("Cleaned dependency cache")
	return nil
}

// RunUpdate checks for dependency updates (placeholder for Phase 6)
func RunUpdate(_ context.Context, _ map[string]Dependency) error {
	fmt.Println("Dependency update checking not yet implemented")
	return nil
}
