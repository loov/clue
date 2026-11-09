package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps/fetch"
	"github.com/loov/clue/internal/discovery"
	"github.com/loov/clue/internal/generate"
	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/clingy"
)

type generateCommand struct {
	options *options
	format  string
	out     string
}

func (c *generateCommand) Setup(params clingy.Parameters) {
	if c.format != "schema" {
		c.out = params.Flag("out", "directory to write the files to (default: the project directory)", "").(string)
	}
}

func (c *generateCommand) Execute(ctx context.Context) error {
	return result(runGenerate(ctx, c.options.dir, c.out, c.options.variant, c.options.target, c.format))
}

// runGenerate writes the files subCmd names into out, or into the project
// directory dir when out is empty.
func runGenerate(ctx context.Context, dir, out, variant, target, subCmd string) int {
	if subCmd == "schema" {
		path, err := config.WriteSchemaPackage(dir)
		if err != nil {
			printError(err)
			return 1
		}
		fmt.Printf("Generated: %s\n", path)
		return 0
	}
	targetPlatform, err := parseTargetPlatform(target)
	if err != nil {
		printError(err)
		return 1
	}

	// Load config without applying variant - generators handle variants internally
	cfg, err := discovery.LoadOrDiscoverForTarget(dir, targetPlatform)
	if err != nil {
		printError(err)
		return 1
	}

	// Resolve environment variables (but don't apply variant)
	env, err := config.ResolveEnvVars(cfg)
	if err != nil {
		printError(err)
		return 1
	}
	if len(env.Variables) > 0 {
		cfg, err = config.ApplyEnvVars(cfg, env)
		if err != nil {
			printError(err)
			return 1
		}
	}

	// Determine selected variant for compile-commands
	selectedVariant := config.SelectConfigVariant(cfg, variant)

	if out == "" {
		out = dir
	} else if err := os.MkdirAll(out, 0o755); err != nil {
		printError(err)
		return 1
	}
	switch subCmd {
	case "ninja":
		return generateNinja(ctx, out, cfg, targetPlatform)
	case "compile-commands":
		return generateCompileCommands(ctx, out, cfg, selectedVariant, targetPlatform)
	case "all":
		if ret := generateNinja(ctx, out, cfg, targetPlatform); ret != 0 {
			return ret
		}
		return generateCompileCommands(ctx, out, cfg, selectedVariant, targetPlatform)
	default:
		fmt.Fprintf(os.Stderr, "Unknown generate subcommand: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "Available subcommands: ninja, compile-commands, all")
		return 1
	}
}

func generateNinja(ctx context.Context, out string, cfg *config.Config, platform toolchain.Platform) int {
	// A changed patch selects a new checkout. Materialize it (and any newly
	// declared dependencies) before the generator expands source globs.
	if err := build.FetchDependencies(ctx, cfg, fetch.Options{Quiet: true}); err != nil {
		printError(err)
		return 1
	}
	// Collect all variant names
	variants := slices.Sorted(maps.Keys(cfg.Variants))
	// If no variants defined, use "debug" as default
	if len(variants) == 0 {
		variants = []string{"debug"}
	}

	// A cross build gets its own Ninja file: build.<os-arch>.ninja.
	outputPath := filepath.Join(out, "build.ninja")
	if platform.IsCrossCompile() {
		outputPath = filepath.Join(out, "build."+platform.String()+".ninja")
	}
	regenerate := []string{"-target", platform.String(), "generate", "ninja"}
	if out != "." {
		regenerate = append(regenerate, "-out", out)
	}
	err := generate.Ninja(ctx, generate.NinjaOptions{
		Config:   cfg,
		Variants: variants,
		// Ninja keeps its outputs apart from "clue build", whose cache it does
		// not share, so switching between them does not rebuild everything.
		BuildDir:   filepath.Join(cfg.BuildDir, "ninja"),
		OutputPath: outputPath,
		Toolchain:  cfg.Toolchain.Compiler,
		Platform:   platform,
		Regenerate: regenerate,
	})
	if err != nil {
		if ctx.Err() != nil {
			return 1
		}
		printError(err)
		return 1
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return 0
}

func generateCompileCommands(ctx context.Context, out string, cfg *config.Config, variant string, platform toolchain.Platform) int {
	outputPath := filepath.Join(out, "compile_commands.json")
	err := generate.CompileCommands(ctx, generate.CompDBOptions{
		Config:     cfg,
		Variant:    variant,
		BuildDir:   cfg.BuildDir,
		OutputPath: outputPath,
		Toolchain:  cfg.Toolchain.Compiler,
		Platform:   platform,
	})
	if err != nil {
		if ctx.Err() != nil {
			return 1
		}
		printError(err)
		return 1
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return 0
}
