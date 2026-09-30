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
	fingerprint, err := linkFingerprint(b.toolchain, target.Command[0], target.Command, target.Inputs)
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
	if _, err := b.executor.RunCommand(ctx, target.Command[0], target.Command[1:]...); err != nil {
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
