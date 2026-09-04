package plan

import (
	"path/filepath"
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
	targetPlan := ForTarget(cfg, target, variant, "build", "debug", toolchain.Platform{OS: "windows", Arch: "amd64"})
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
