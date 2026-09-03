package main

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/deps/fetch"
	"github.com/zeebo/clingy"
)

type depsCommand struct {
	options             *options
	action              string
	takesName, optional bool
	name                string
}

func (c *depsCommand) Setup(params clingy.Parameters) {
	if !c.takesName {
		return
	}
	description := "dependency to " + c.action
	if c.optional {
		name := params.Arg("name", description, clingy.Optional).(*string)
		if name != nil {
			c.name = *name
		}
		return
	}
	c.name = params.Arg("name", description).(string)
}

func (c *depsCommand) Execute(context.Context) error {
	return result(runDeps(c.options.dir, c.options.target, c.options.verbose, c.action, c.name))
}

func runDeps(dir, target string, verbose bool, subCmd, name string) int {
	// Load config
	verbosity := build.VerbosityNormal
	if verbose {
		verbosity = build.VerbosityVerbose
	}
	cfg, variant, platform, err := loadConfig(dir, "", target, verbosity)
	if err != nil {
		printError(err)
		return 1
	}

	// Setup context for operations
	ctx := context.Background()

	switch subCmd {
	case "list":
		if err := listDependencies(cfg.Dependencies, verbose); err != nil {
			printError(err)
			return 1
		}

	case "fetch":
		if err := fetchDependencies(ctx, cfg.Dependencies, verbose, name); err != nil {
			printError(err)
			return 1
		}

	case "build":
		builder, err := build.NewConfiguredBuilder(cfg.Toolchain, platform, dir, verbosity, 1, false)
		if err != nil {
			printError(err)
			return 1
		}
		if err := builder.BuildDependency(ctx, build.Options{
			Config: cfg, Variant: variant, BuildDir: cfg.BuildDir, Verbosity: verbosity,
		}, name); err != nil {
			printError(err)
			return 1
		}
	case "clean":
		if err := cleanDependencies(cfg.Dependencies, name); err != nil {
			printError(err)
			return 1
		}

	case "update":
		if err := updateDependencies(ctx, cfg.Dependencies); err != nil {
			printError(err)
			return 1
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown deps subcommand: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "Available subcommands: list, fetch, build, clean, update")
		return 1
	}

	return 0
}

// listDependencies lists all dependencies and their status.
func listDependencies(dependencies map[string]deps.Dependency, verbose bool) error {
	if len(dependencies) == 0 {
		fmt.Println("No dependencies configured")
		return nil
	}

	// Create manager
	mgr, err := fetch.NewManager(".", dependencies, fetch.Options{
		Verbose: verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Get status
	statuses := mgr.Status()

	// Sort by name for consistent output
	slices.SortFunc(statuses, func(a, b fetch.Status) int {
		return cmp.Compare(a.Name, b.Name)
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

// fetchDependencies fetches one dependency, or all dependencies when name is empty.
func fetchDependencies(ctx context.Context, dependencies map[string]deps.Dependency, verbose bool, name string) error {
	if len(dependencies) == 0 {
		fmt.Println("No dependencies to fetch")
		return nil
	}

	// Create manager
	mgr, err := fetch.NewManager(".", dependencies, fetch.Options{
		Verbose: verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Fetch single or all
	if name != "" {
		return mgr.FetchOne(ctx, name)
	}

	// Get initial status for summary
	initialStatuses := mgr.Status()
	cachedCount := 0
	for _, status := range initialStatuses {
		if status.Status != "missing" {
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

// cleanDependencies removes dependency cache entries.
func cleanDependencies(dependencies map[string]deps.Dependency, name string) error {
	// Create manager (verbose=false for clean)
	mgr, err := fetch.NewManager(".", dependencies, fetch.Options{
		Verbose: false,
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

// updateDependencies checks out the latest commit of configured Git branches.
func updateDependencies(ctx context.Context, dependencies map[string]deps.Dependency) error {
	mgr, err := fetch.NewManager(".", dependencies, fetch.Options{})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}
	return mgr.UpdateAll(ctx)
}
