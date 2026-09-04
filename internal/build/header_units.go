package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// CompileHeaderUnit builds one header unit BMI.
func (c *Compiler) CompileHeaderUnit(ctx context.Context, opts plan.HeaderUnitOptions) (resultErr error) {
	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return fmt.Errorf("create header-unit output directory: %w", err)
	}
	arguments := plan.HeaderUnitArguments(c.toolchain, opts)
	responseFile := toolchain.MaybeUseGNUResponseFileIn
	if isMSVC(c.toolchain) {
		responseFile = toolchain.MaybeUseResponseFileIn
	}
	args, cleanupPath, err := responseFile(filepath.Dir(opts.Output), arguments)
	if err != nil {
		return fmt.Errorf("create header-unit response file: %w", err)
	}
	if cleanupPath != "" {
		defer func() { resultErr = errors.Join(resultErr, os.Remove(cleanupPath)) }()
	}
	if _, err = c.executor.RunCommand(ctx, c.toolchain.CXX(), args...); err != nil {
		return fmt.Errorf("compile header unit %s: %w", opts.Name, err)
	}
	if _, err = os.Stat(opts.Output); err != nil {
		return fmt.Errorf("compiler did not produce header unit %s at %s: %w", opts.Name, opts.Output, err)
	}
	return nil
}
