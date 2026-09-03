package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/clingy"
)

type testCommand struct {
	options   *options
	selectors []string
}

func (c *testCommand) Setup(params clingy.Parameters) {
	c.selectors = params.Arg("selector", "test name or label", clingy.Repeated).([]string)
}

func (c *testCommand) Execute(context.Context) error {
	o := c.options
	return result(runTests(o.dir, o.variant, o.target, o.verbosity(), o.jobs, c.selectors))
}

func runTests(dir, variant, target string, verbosity build.Verbosity, jobs int, selectors []string) int {
	cfg, selectedVariant, platform, err := loadConfig(dir, variant, target, build.VerbosityQuiet)
	if err != nil {
		printError(err)
		return 1
	}
	if platform != toolchain.HostPlatform() {
		printError(fmt.Errorf("cannot run tests for non-host target %s", platform))
		return 1
	}
	targets, err := selectConfiguredTests(cfg, selectors)
	if err != nil {
		printError(err)
		return 1
	}
	if code := runBuild(dir, variant, target, verbosity, false, jobs, false, false, false, 10, targets); code != 0 {
		return code
	}

	cases := make([]build.TestCase, 0, len(targets))
	for _, name := range targets {
		configured := cfg.Targets[name].Test
		executable, err := filepath.Abs(filepath.Join(cfg.BuildDir, selectedVariant, "bin", plan.ExecutableName(name, platform)))
		if err != nil {
			printError(err)
			return 1
		}
		workingDirectory := "."
		if configured.WorkingDirectory != "" {
			workingDirectory = configured.WorkingDirectory
		}
		workingDirectory, err = filepath.Abs(workingDirectory)
		if err != nil {
			printError(err)
			return 1
		}
		cases = append(cases, build.TestCase{
			Name: name, Executable: executable, Args: configured.Args,
			Environment: configured.Environment, WorkingDirectory: workingDirectory,
		})
	}
	summary := build.RunTests(context.Background(), cases, resolvedJobs(jobs), verbosity)
	if summary.Failed > 0 {
		return 1
	}
	return 0
}

func selectConfiguredTests(cfg *config.Config, selectors []string) ([]string, error) {
	selected := make(map[string]bool)
	for _, selector := range selectors {
		matched := false
		for name, target := range cfg.Targets {
			if target.Test == nil {
				continue
			}
			if name == selector {
				selected[name], matched = true, true
				continue
			}
			for _, label := range target.Test.Labels {
				if label == selector {
					selected[name], matched = true, true
				}
			}
		}
		if !matched {
			return nil, fmt.Errorf("no test or label matches %q", selector)
		}
	}
	if len(selectors) == 0 {
		for name, target := range cfg.Targets {
			if target.Test != nil {
				selected[name] = true
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no tests configured")
	}
	return slices.Sorted(maps.Keys(selected)), nil
}
