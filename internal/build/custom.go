package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/config"
)

func (b *Builder) buildCustomTarget(ctx context.Context, opts Options, target config.Target) (*TargetResult, error) {
	start := time.Now()
	fingerprint, err := linkFingerprint(b.toolchain, target.Command[0], target.Command, target.Inputs)
	if err != nil {
		return nil, fmt.Errorf("fingerprinting custom target %q: %w", target.Name, err)
	}
	current := linkIsCurrent(target.Outputs[0], fingerprint)
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
	if err := storeLinkFingerprint(target.Outputs[0], fingerprint); err != nil {
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
