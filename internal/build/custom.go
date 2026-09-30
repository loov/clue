package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
)

func (b *Builder) buildCustomTarget(ctx context.Context, opts Options, target config.Target) (*TargetResult, error) {
	start := time.Now()
	if opts.Config != nil {
		expanded, err := plan.ExpandCustomTarget(opts.Config, target, opts.BuildDir, opts.Variant, b.target)
		if err != nil {
			return nil, err
		}
		target = expanded
	}
	fingerprint, err := linkFingerprint(b.toolchain, target.Command[0],
		struct {
			Command         []string
			WorkDir, Stdout string
		}{target.Command, target.WorkDir, target.Stdout}, target.Inputs)
	if err != nil {
		return nil, fmt.Errorf("fingerprinting custom target %q: %w", target.Name, err)
	}
	// The fingerprint lives in the build directory, not beside the outputs,
	// which may be inside a bundle or another directory the command owns.
	stamp := filepath.Join(opts.BuildDir, opts.Variant, "custom", target.Name+".clue-link")
	previous, err := os.ReadFile(stamp)
	current := err == nil && bytes.Equal(previous, fingerprint)
	for _, output := range target.Outputs {
		if _, err := os.Stat(output); err != nil {
			current = false
		}
	}
	if !opts.ForceRebuild && current {
		return customTargetResult(target, start), nil
	}
	for _, output := range target.Outputs {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return nil, fmt.Errorf("creating output directory for custom target %q: %w", target.Name, err)
		}
	}
	if err := b.runCustomCommand(ctx, target); err != nil {
		return nil, fmt.Errorf("running custom target %q: %w", target.Name, err)
	}
	for _, output := range target.Outputs {
		if _, err := os.Stat(output); err != nil {
			return nil, fmt.Errorf("custom target %q did not produce %q", target.Name, output)
		}
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return nil, fmt.Errorf("caching custom target %q: %w", target.Name, err)
	}
	if err := os.WriteFile(stamp, fingerprint, 0o644); err != nil {
		return nil, fmt.Errorf("caching custom target %q: %w", target.Name, err)
	}
	return customTargetResult(target, start), nil
}

func customTargetResult(target config.Target, start time.Time) *TargetResult {
	return &TargetResult{
		Name: target.Name, Type: target.Type, Output: target.Outputs[0],
		Duration: time.Since(start), Success: true,
	}
}

// runCustomCommand runs a custom target's command in its working directory,
// writing its standard output to target.Stdout when set. The file is replaced
// only when the command succeeds.
func (b *Builder) runCustomCommand(ctx context.Context, target config.Target) error {
	config := b.executor.config
	if target.WorkDir != "" {
		if err := os.MkdirAll(target.WorkDir, 0o755); err != nil {
			return err
		}
		config.WorkDir = target.WorkDir
	}
	var output *os.File
	if target.Stdout != "" {
		var err error
		output, err = os.CreateTemp(filepath.Dir(target.Stdout), ".clue-stdout-*")
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(output.Name()) }()
		config.Stdout = output
	}
	_, err := newExecutor(config).RunCommand(ctx, target.Command[0], target.Command[1:]...)
	if output == nil {
		return err
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(output.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(output.Name(), target.Stdout)
}
