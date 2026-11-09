package plan

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestForTargetResolvesSharedBuildSemantics(t *testing.T) {
	debug := true
	cfg := &config.Config{
		Toolchain: config.Toolchain{CStd: "c17", CXXStd: "c++23"},
		Targets: map[string]config.Target{
			"base": {Name: "base", Type: "interface_library", Public: config.Usage{Includes: []string{"include"}, Defines: []string{"PUBLIC"}}},
		},
	}
	target := config.Target{Name: "app", Type: "executable", Sources: []string{"src/main.c", "src/main.cpp"}, Depends: []string{"base"}}
	variant := config.Variant{Name: "debug", DebugInfo: true, DebugInfoSet: true, LTO: &debug, Defines: []string{"DEBUG"}}
	targetPlan, err := ForTarget(cfg, target, variant, "build", "debug", toolchain.Platform{OS: "windows", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if targetPlan.Output != filepath.Join("build", "debug", "bin", "app.exe") || targetPlan.Flags.Debug != "full" || !targetPlan.Flags.LTO {
		t.Fatalf("plan = %+v", targetPlan)
	}
	if len(targetPlan.Sources) != 2 || targetPlan.Sources[0].Standard != "c17" || targetPlan.Sources[1].Standard != "c++23" {
		t.Fatalf("source plan = %+v", targetPlan.Sources)
	}
	if len(targetPlan.Usage.Includes) != 1 || targetPlan.Usage.Defines[0] != "PUBLIC" {
		t.Fatalf("usage = %+v", targetPlan.Usage)
	}
	if got := targetPlan.Target; len(got.Includes) != 1 || len(got.Defines) != 2 || got.Defines[1] != "DEBUG" || got.CStd != "c17" || got.CXXStd != "c++23" {
		t.Fatalf("resolved target = %+v", got)
	}
}

func TestForTarget_SourceFlags(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{}}
	target := config.Target{
		Name: "lib", Type: "static_library", Sources: []string{"src/a.cpp", "src/b.cpp", "other/c.cpp"},
		Flags:       config.Flags{Compiler: []string{"-fno-rtti"}},
		SourceFlags: map[string][]string{"src/*.cpp": {"-DSRC"}, "src/b.cpp": {"-frtti"}},
	}
	targetPlan, err := ForTarget(cfg, target, config.Variant{}, ".build", "debug", toolchain.Platform{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, source := range targetPlan.Sources {
		got[source.Source] = WithSourceFlags(targetPlan.Flags, source).RawCompiler
	}
	for source, want := range map[string][]string{
		"src/a.cpp": {"-fno-rtti", "-DSRC"}, "src/b.cpp": {"-fno-rtti", "-DSRC", "-frtti"}, "other/c.cpp": {"-fno-rtti"},
	} {
		if !slices.Equal(got[source], want) {
			t.Errorf("%s flags = %q, want %q", source, got[source], want)
		}
	}
}

func TestForTarget_LanguageFlags(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{}}
	target := config.Target{
		Name: "lib", Type: "static_library", Sources: []string{"a.c", "b.cpp", "c.S", "d.mm"},
		Flags: config.Flags{
			Compiler: []string{"-march=x86-64-v2"}, C: []string{"-Wall"}, CXX: []string{"-Wall", "-Wextra"}, ASM: []string{"-Wa,--noexecstack"},
		},
		SourceFlags: map[string][]string{"b.cpp": {"-DB"}},
	}
	targetPlan, err := ForTarget(cfg, target, config.Variant{}, ".build", "debug", toolchain.Platform{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, source := range targetPlan.Sources {
		got[source.Source] = WithSourceFlags(targetPlan.Flags, source).RawCompiler
	}
	for source, want := range map[string][]string{
		"a.c":   {"-march=x86-64-v2", "-Wall"},
		"b.cpp": {"-march=x86-64-v2", "-Wall", "-Wextra", "-DB"},
		"c.S":   {"-march=x86-64-v2", "-Wa,--noexecstack"},
		"d.mm":  {"-march=x86-64-v2", "-Wall", "-Wextra"},
	} {
		if !slices.Equal(got[source], want) {
			t.Errorf("%s flags = %q, want %q", source, got[source], want)
		}
	}
}

func TestForTarget_FlagsExpandBuildDir(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{}}
	target := config.Target{
		Name: "plugin", Type: "shared_library", Sources: []string{"plugin.c"},
		Flags: config.Flags{Linker: []string{"-Wl,--version-script={buildDir}/gen/exports.txt"}},
	}
	targetPlan, err := ForTarget(cfg, target, config.Variant{}, ".build", "release", toolchain.Platform{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-Wl,--version-script=" + filepath.Join(".build", "release") + "/gen/exports.txt"}
	if !slices.Equal(targetPlan.Flags.RawLinker, want) {
		t.Errorf("linker flags = %q, want %q", targetPlan.Flags.RawLinker, want)
	}
}

func TestTargetConfig_PositionIndependentExecutable(t *testing.T) {
	pic := true
	for targetType, want := range map[string][2]bool{
		"executable":     {false, true},
		"static_library": {true, false},
		"shared_library": {true, false},
	} {
		flags := targetConfig(config.Target{Type: targetType, PIC: &pic}, config.Variant{})
		if got := [2]bool{flags.PIC, flags.PIE}; got != want {
			t.Errorf("%s: PIC, PIE = %v, want %v", targetType, got, want)
		}
	}
}
