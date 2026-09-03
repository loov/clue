package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// HeaderUnitOptions describes one explicitly configured C++ header unit.
type HeaderUnitOptions struct {
	Source         string
	Name           string
	System         bool
	Output         string
	Includes       []string
	SystemIncludes []string
	Defines        []string
	Flags          Config
	Std            string
	ModuleFiles    map[string]string
	ModuleMapper   string
}

// HeaderUnitArguments returns compiler arguments shared by direct builds and generators.
func HeaderUnitArguments(tc Toolchain, opts HeaderUnitOptions) []string {
	standard := opts.Std
	if standard == "" {
		standard = "c++20"
	}
	var args []string
	if tc.Name() == "msvc" {
		args = append(args, tc.CompilerFlags(opts.Flags)...)
		args = append(args, "/std:"+TranslateStdForMSVC(standard))
		for _, include := range opts.Includes {
			args = append(args, "/I"+include)
		}
		for _, include := range opts.SystemIncludes {
			args = append(args, "/external:I"+include)
		}
		for _, define := range opts.Defines {
			args = append(args, "/D"+define)
		}
		args = append(args, ModuleCompileFlags(tc, ModuleDependency{}, "", opts.ModuleFiles, "")...)
		kind := "/headerName:quote"
		if opts.System {
			kind = "/headerName:angle"
		}
		return append(args, "/exportHeader", kind, opts.Source, "/ifcOutput", opts.Output)
	}

	args = append(args, "-std="+standard)
	for _, include := range opts.Includes {
		args = append(args, "-I"+include)
	}
	for _, include := range opts.SystemIncludes {
		args = append(args, "-isystem", include)
	}
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}
	args = append(args, tc.CompilerFlags(opts.Flags)...)
	args = append(args, ModuleCompileFlags(tc, ModuleDependency{UsesModules: true}, "", opts.ModuleFiles, opts.ModuleMapper)...)
	headerKind := "c++-user-header"
	if opts.System {
		headerKind = "c++-system-header"
	}
	if tc.Name() == "gcc" {
		return append(args, "-x", headerKind, opts.Source, "-c")
	}
	return append(args, "-x", headerKind, "--precompile", opts.Source, "-o", opts.Output)
}

// CompileHeaderUnit builds one header unit BMI.
func (c *Compiler) CompileHeaderUnit(ctx context.Context, opts HeaderUnitOptions) (resultErr error) {
	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return fmt.Errorf("create header-unit output directory: %w", err)
	}
	args, cleanupPath, err := MaybeUseResponseFileIn(filepath.Dir(opts.Output), HeaderUnitArguments(c.toolchain, opts))
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
