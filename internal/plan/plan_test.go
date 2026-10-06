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
