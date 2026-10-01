package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// RunOptions configures the run operation
type RunOptions struct {
	Config    *config.Config
	Variant   string
	BuildDir  string
	Target    string             // Target name to run
	Args      []string           // Arguments to pass to executable
	Verbosity Verbosity          // For build output
	Jobs      int                // Parallel jobs for build
	Platform  toolchain.Platform // Target platform; the host when empty
}

// RunResult contains the result of running an executable
type RunResult struct {
	ExitCode int
	Output   string // Path to executed binary, or the task's command
}

// RunTarget builds an executable target and runs it, or builds the targets a
// task depends on and runs its command, with opts.Args appended.
// Returns error if target doesn't exist, can't be run, or build fails
func RunTarget(ctx context.Context, opts RunOptions) (*RunResult, error) {
	// 1. Validate target exists and can be run
	target, ok := opts.Config.Targets[opts.Target]
	if !ok {
		return nil, fmt.Errorf("target %q not found in configuration", opts.Target)
	}
	if target.Type != "executable" && target.Type != "task" {
		return nil, fmt.Errorf("target %q is a %s, not an executable or a task", opts.Target, target.Type)
	}

	// 2. Build the target first
	platform := opts.Platform
	if platform.OS == "" {
		platform = toolchain.HostPlatform()
	}
	var emulator []string
	// Host targets of a cross build run on the machine running clue.
	if target.Type == "executable" && (!target.Host || opts.Config.Host == nil) {
		var err error
		if emulator, err = Emulator(opts.Config, platform); err != nil {
			return nil, err
		}
	}
	var result *Result
	if target.Type == "executable" || len(target.Depends) > 0 {
		var err error
		result, err = Build(ctx, Options{
			Config:    opts.Config,
			Variant:   opts.Variant,
			BuildDir:  opts.BuildDir,
			Verbosity: opts.Verbosity,
			Targets:   []string{opts.Target},
			Jobs:      opts.Jobs,
		}, platform)
		if err != nil || !result.Success {
			return nil, fmt.Errorf("build failed: %w", err)
		}
	}

	// 3. Find the command: the task's, or the built executable
	var name, dir string
	var args []string
	if target.Type == "task" {
		expanded, err := plan.ExpandCustomTarget(opts.Config, target, opts.BuildDir, opts.Variant, platform)
		if err != nil {
			return nil, err
		}
		name, args, dir = expanded.Command[0], append(expanded.Command[1:], opts.Args...), expanded.WorkDir
	} else {
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
		name, args = EmulatedCommand(emulator, execPath, opts.Args)
	}

	// 4. Execute in the current working directory, or the task's
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run %s: %w", name, err)
		}
	}

	return &RunResult{
		ExitCode: exitCode,
		Output:   name,
	}, nil
}

// Emulator returns the command that runs programs built for platform:
// nothing for the host, toolchain.emulator for other targets.
func Emulator(cfg *config.Config, platform toolchain.Platform) ([]string, error) {
	if platform == toolchain.HostPlatform() {
		return nil, nil
	}
	if len(cfg.Toolchain.Emulator) == 0 {
		return nil, fmt.Errorf("cannot run programs for non-host target %s without toolchain.emulator", platform)
	}
	return cfg.Toolchain.Emulator, nil
}

// EmulatedCommand returns the command that runs program with args: through
// emulator when it is set, else directly.
func EmulatedCommand(emulator []string, program string, args []string) (string, []string) {
	if len(emulator) == 0 {
		return program, args
	}
	return emulator[0], append(append(append([]string(nil), emulator[1:]...), program), args...)
}
