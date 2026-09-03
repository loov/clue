package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/clingy"
)

type runCommand struct {
	options *options
	target  string
	args    []string
}

func (c *runCommand) Setup(params clingy.Parameters) {
	c.target = params.Arg("target", "executable target to run").(string)
	c.args = params.Arg("argument", "argument passed to the executable", clingy.Repeated).([]string)
}

func (c *runCommand) Execute(ctx context.Context) error {
	o := c.options
	return result(runRun(ctx, o.dir, o.variant, o.target, o.verbosity(), o.jobs, c.target, c.args))
}

func runRun(ctx context.Context, dir, variant, target string, verbosity build.Verbosity, jobs int, targetName string, execArgs []string) int {
	// Load configuration
	cfg, selectedVariant, platform, err := loadConfig(dir, variant, target, verbosity)
	if err != nil {
		printError(err)
		return 1
	}
	if platform != toolchain.HostPlatform() {
		printError(fmt.Errorf("cannot run executable for non-host target %s", platform))
		return 1
	}

	// Compute actual job count (same as runBuild)
	actualJobs := jobs
	if actualJobs == 0 {
		actualJobs = max(runtime.NumCPU()/2, 1)
	} else if actualJobs < 0 {
		actualJobs = runtime.NumCPU()
	}

	result, err := build.RunTarget(ctx, build.RunOptions{
		Config:    cfg,
		Variant:   selectedVariant,
		BuildDir:  cfg.BuildDir,
		Target:    targetName,
		Args:      execArgs,
		Verbosity: verbosity,
		Jobs:      actualJobs,
	})

	if ctx.Err() != nil {
		return 1
	}

	if err != nil {
		printError(err)
		return 1
	}

	return result.ExitCode
}
