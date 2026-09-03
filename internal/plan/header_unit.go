package plan

import (
	"github.com/loov/clue/internal/toolchain"
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
	Flags          toolchain.Config
	Std            string
	ModuleFiles    map[string]string
	ModuleMapper   string
}

// HeaderUnitArguments returns compiler arguments shared by direct builds and generators.
func HeaderUnitArguments(tc toolchain.Toolchain, opts HeaderUnitOptions) []string {
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
