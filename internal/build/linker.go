package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

type linkResult struct {
	Output    string
	ImportLib string
	Duration  time.Duration
	Success   bool
}

type linker struct {
	executor  *executor
	toolchain toolchain.Toolchain
	target    toolchain.Platform
}

func newLinker(executor *executor, tc toolchain.Toolchain, target toolchain.Platform) *linker {
	return &linker{executor: executor, toolchain: tc, target: target}
}

func isMSVC(tc toolchain.Toolchain) bool {
	return tc.Name() == "msvc"
}

func (l *linker) LinkExecutable(ctx context.Context, opts plan.LinkOptions) (*linkResult, error) {
	if err := createOutputDirectory(opts.Output); err != nil {
		return nil, err
	}
	return l.execute(ctx, plan.Link(l.toolchain, l.target, opts), opts.Output, "linker")
}

func (l *linker) CreateStaticLibrary(ctx context.Context, opts plan.ArchiveOptions) (*linkResult, error) {
	if err := createOutputDirectory(opts.Output); err != nil {
		return nil, err
	}
	// Archivers update existing files in place, retaining removed members.
	if err := os.Remove(opts.Output); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to replace static library: %w", err)
	}
	return l.execute(ctx, plan.Archive(l.toolchain, opts), opts.Output, "archiver")
}

func (l *linker) LinkSharedLibrary(ctx context.Context, opts plan.SharedLibraryOptions) (*linkResult, error) {
	if err := createOutputDirectory(opts.Output); err != nil {
		return nil, err
	}
	return l.execute(ctx, plan.LinkShared(l.toolchain, l.target, opts), opts.Output, "linker")
}

func createOutputDirectory(output string) error {
	directory := filepath.Dir(output)
	if directory == "" || directory == "." {
		return nil
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return nil
}

func (l *linker) execute(ctx context.Context, invocation plan.Invocation, output, operation string) (_ *linkResult, resultErr error) {
	args := invocation.Arguments
	cleanupPath := ""
	if operation != "archiver" || isMSVC(l.toolchain) {
		responseFile := toolchain.MaybeUseGNUResponseFileIn
		if isMSVC(l.toolchain) {
			responseFile = toolchain.MaybeUseResponseFileIn
		}
		var err error
		args, cleanupPath, err = responseFile(filepath.Dir(output), invocation.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to create response file: %w", err)
		}
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}

	start := time.Now()
	result, err := l.executor.RunCommand(ctx, invocation.Tool, args...)
	if err != nil {
		failure := fmt.Errorf("%s failed: %w", operation, err)
		if isMSVC(l.toolchain) && operation == "linker" {
			failure = fmt.Errorf("%w\nCommand: %s %s", failure, invocation.Tool, strings.Join(invocation.Arguments, " "))
		}
		return &linkResult{Output: output, ImportLib: invocation.ImportLibrary, Duration: time.Since(start)}, failure
	}
	return &linkResult{
		Output: output, ImportLib: invocation.ImportLibrary, Duration: result.Duration, Success: true,
	}, nil
}
