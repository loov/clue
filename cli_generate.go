package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/generate"
	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/clingy"
)

type generateCommand struct {
	options *options
	format  string
}

func (*generateCommand) Setup(clingy.Parameters) {}

func (c *generateCommand) Execute(ctx context.Context) error {
	return result(runGenerate(ctx, c.options.dir, c.options.variant, c.options.target, c.format))
}

func runGenerate(ctx context.Context, dir, variant, target, subCmd string) int {
	targetPlatform, err := parseTargetPlatform(target)
	if err != nil {
		printError(err)
		return 1
	}

	// Load config without applying variant - generators handle variants internally
	loader := config.NewLoader()
	cfg, err := loader.LoadOrDiscoverForTarget(dir, targetPlatform)
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
	selector := config.NewVariantSelector()
	if variant != "" {
		selector.SetCLIFlag(variant)
	}
	selectedVariant := selector.Select()

	switch subCmd {
	case "ninja":
		return generateNinja(ctx, dir, cfg, targetPlatform)
	case "compile-commands":
		return generateCompileCommands(ctx, dir, cfg, selectedVariant, targetPlatform)
	case "all":
		if ret := generateNinja(ctx, dir, cfg, targetPlatform); ret != 0 {
			return ret
		}
		return generateCompileCommands(ctx, dir, cfg, selectedVariant, targetPlatform)
	default:
		fmt.Fprintf(os.Stderr, "Unknown generate subcommand: %s\n", subCmd)
		fmt.Fprintln(os.Stderr, "Available subcommands: ninja, compile-commands, all")
		return 1
	}
}

func generateNinja(ctx context.Context, dir string, cfg *config.Config, platform toolchain.Platform) int {
	// Collect all variant names
	variants := slices.Sorted(maps.Keys(cfg.Variants))
	// If no variants defined, use "debug" as default
	if len(variants) == 0 {
		variants = []string{"debug"}
	}

	outputPath := filepath.Join(dir, "build.ninja")
	err := generate.Ninja(ctx, generate.NinjaOptions{
		Config:     cfg,
		Variants:   variants,
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

func generateCompileCommands(ctx context.Context, dir string, cfg *config.Config, variant string, platform toolchain.Platform) int {
	outputPath := filepath.Join(dir, "compile_commands.json")
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
