package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/loov/clue/internal/config"
)

// RunOptions configures the run operation
type RunOptions struct {
	Config    *config.Config
	Variant   string
	BuildDir  string
	Target    string    // Target name to run
	Args      []string  // Arguments to pass to executable
	Verbosity Verbosity // For build output
	Jobs      int       // Parallel jobs for build
}

// RunResult contains the result of running an executable
type RunResult struct {
	ExitCode int
	Output   string // Path to executed binary
}

// RunTarget builds a target and executes it
// Returns error if target doesn't exist, isn't executable, or build fails
func RunTarget(ctx context.Context, opts RunOptions) (*RunResult, error) {
	// 1. Validate target exists and is executable type
	target, ok := opts.Config.Targets[opts.Target]
	if !ok {
		return nil, fmt.Errorf("target %q not found in configuration", opts.Target)
	}
	if target.Type != "executable" {
		return nil, fmt.Errorf("target %q is a %s, not an executable", opts.Target, target.Type)
	}

	// 2. Build the target first
	platform := HostPlatform() // Run always uses host platform
	builder, err := NewConfiguredBuilder(opts.Config.Toolchain, platform, ".", opts.Verbosity, opts.Jobs, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}

	buildOpts := Options{
		Config:    opts.Config,
		Variant:   opts.Variant,
		BuildDir:  opts.BuildDir,
		Verbosity: opts.Verbosity,
		Targets:   []string{opts.Target},
		Jobs:      opts.Jobs,
	}

	result, err := builder.Build(ctx, buildOpts)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// 3. Find executable path from build result
	var execPath string
	for _, tr := range result.Targets {
		if tr.Name == opts.Target {
			execPath = tr.Output
			break
		}
	}
	if execPath == "" {
		return nil, fmt.Errorf("build succeeded but no output found for target %q", opts.Target)
	}

	// 4. Execute binary in current working directory
	cmd := exec.CommandContext(ctx, execPath, opts.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	// cmd.Dir defaults to current directory - this is what we want

	err = cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run executable: %w", err)
		}
	}

	return &RunResult{
		ExitCode: exitCode,
		Output:   execPath,
	}, nil
}
