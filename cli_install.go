package main

import (
	"context"
	"fmt"

	"github.com/loov/clue/internal/build"
	"github.com/zeebo/clingy"
)

type installCommand struct {
	options *options
	targets []string
}

func (c *installCommand) Setup(params clingy.Parameters) {
	c.targets = params.Arg("target", "target to install", clingy.Repeated).([]string)
}

func (c *installCommand) Execute(ctx context.Context) error {
	o := c.options
	return result(runInstall(ctx, o.dir, o.variant, o.target, o.prefix, o.destDir, o.verbosity(), o.jobs, c.targets))
}

func runInstall(ctx context.Context, dir, variant, target, prefix, destDir string, verbosity build.Verbosity, jobs int, targets []string) int {
	cfg, selectedVariant, platform, err := loadConfig(dir, variant, target, build.VerbosityQuiet)
	if err != nil {
		printError(err)
		return 1
	}
	targets, err = build.InstallTargets(cfg, targets)
	if err != nil {
		printError(err)
		return 1
	}
	if code := runBuild(ctx, dir, variant, target, verbosity, false, jobs, false, false, false, 10, targets); code != 0 {
		return code
	}
	if ctx.Err() != nil {
		return 1
	}
	result, err := build.Install(build.InstallOptions{
		Config: cfg, Variant: selectedVariant, Platform: platform,
		Prefix: prefix, DestDir: destDir, Targets: targets,
	})
	if err != nil {
		printError(err)
		return 1
	}
	if verbosity >= build.VerbosityNormal {
		fmt.Printf("Installed %d files to %s\n", len(result.Files), result.Root)
	}
	return 0
}
