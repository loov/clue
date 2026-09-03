package main

import (
	"context"
	"fmt"
	"os"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/deps"
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
		if err := deps.RunList(cfg.Dependencies, verbose); err != nil {
			printError(err)
			return 1
		}

	case "fetch":
		if err := deps.RunFetch(ctx, cfg.Dependencies, deps.FetchOptions{
			Verbose: verbose,
			Name:    name,
		}); err != nil {
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
		if err := deps.RunClean(cfg.Dependencies, name); err != nil {
			printError(err)
			return 1
		}

	case "update":
		if err := deps.RunUpdate(ctx, cfg.Dependencies); err != nil {
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
