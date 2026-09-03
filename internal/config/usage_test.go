package config

import (
	"slices"
	"testing"
)

func TestTargetUsesCXXThroughInternalDependency(t *testing.T) {
	cfg := &Config{Targets: map[string]Target{
		"core": {Name: "core", Sources: []string{"core.cpp"}},
		"app":  {Name: "app", Sources: []string{"main.c"}, Depends: []string{"core"}},
	}}
	if !TargetUsesCXX(cfg, cfg.Targets["app"]) {
		t.Fatal("C target depending on C++ library must use the C++ linker")
	}
	cfg.Targets["core"] = Target{Name: "core", Sources: []string{"core.c"}}
	if TargetUsesCXX(cfg, cfg.Targets["app"]) {
		t.Fatal("pure C graph must not use the C++ linker")
	}
}

func TestCompileUsage_PropagatesOnlyPublicRequirements(t *testing.T) {
	cfg := &Config{Targets: map[string]Target{
		"base": {
			Name: "base", Includes: []string{"base/private"}, Defines: []string{"BASE_PRIVATE"},
			Public: Usage{
				Includes: []string{"base/include"}, SystemIncludes: []string{"vendor/include"}, Defines: []string{"BASE_PUBLIC"},
				CompilerFlags: []string{"-pthread"}, LinkerFlags: []string{"-Wl,--as-needed"}, SysLibs: []string{"dl"}, CXXStd: "c++20",
			},
		},
		"middle": {
			Name: "middle", Depends: []string{"base"}, Includes: []string{"middle/private"},
			Public: Usage{Includes: []string{"middle/include"}, Defines: []string{"MIDDLE_PUBLIC"}},
		},
	}}
	target := Target{
		Name: "app", Depends: []string{"middle"}, Includes: []string{"app/private"},
		Public: Usage{Includes: []string{"app/include"}},
	}

	got := CompileUsage(cfg, target)
	want := Usage{
		Includes:       []string{"app/private", "app/include", "middle/include", "base/include"},
		SystemIncludes: []string{"vendor/include"}, Defines: []string{"MIDDLE_PUBLIC", "BASE_PUBLIC"},
		CompilerFlags: []string{"-pthread"}, LinkerFlags: []string{"-Wl,--as-needed"}, SysLibs: []string{"dl"}, CXXStd: "c++20",
	}
	if !slices.Equal(want.Includes, got.Includes) || !slices.Equal(want.SystemIncludes, got.SystemIncludes) ||
		!slices.Equal(want.Defines, got.Defines) || !slices.Equal(want.CompilerFlags, got.CompilerFlags) ||
		!slices.Equal(want.LinkerFlags, got.LinkerFlags) || !slices.Equal(want.SysLibs, got.SysLibs) || want.CXXStd != got.CXXStd {
		t.Fatalf("CompileUsage() = %+v, want %+v", got, want)
	}
}

func TestCompileStandardUsesTargetThenPublicRequirement(t *testing.T) {
	project := Toolchain{CStd: "c11", CXXStd: "c++17"}
	usage := Usage{CStd: "c17", CXXStd: "c++20"}
	if got := CompileStandard(project, Target{}, usage, "main.cpp"); got != "c++20" {
		t.Fatalf("inherited C++ standard = %q", got)
	}
	if got := CompileStandard(project, Target{CXXStd: "c++23"}, usage, "main.cpp"); got != "c++23" {
		t.Fatalf("target C++ standard = %q", got)
	}
}
