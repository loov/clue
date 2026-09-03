package build

import (
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestPlanTargetResolvesSharedBuildSemantics(t *testing.T) {
	debug := true
	cfg := &config.Config{
		Toolchain: config.Toolchain{CStd: "c17", CXXStd: "c++23"},
		Targets: map[string]config.Target{
			"base": {Name: "base", Type: "interface_library", Public: config.Usage{Includes: []string{"include"}, Defines: []string{"PUBLIC"}}},
		},
	}
	target := config.Target{Name: "app", Type: "executable", Sources: []string{"src/main.c", "src/main.cpp"}, Depends: []string{"base"}}
	variant := config.Variant{Name: "debug", DebugInfo: true, DebugInfoSet: true, LTO: &debug}
	plan := PlanTarget(cfg, target, variant, "build", "debug", toolchain.Platform{OS: "windows", Arch: "amd64"})
	if plan.Output != filepath.Join("build", "debug", "bin", "app.exe") || plan.Flags.Debug != "full" || !plan.Flags.LTO {
		t.Fatalf("plan = %+v", plan)
	}
	if len(plan.Sources) != 2 || plan.Sources[0].Standard != "c17" || plan.Sources[1].Standard != "c++23" {
		t.Fatalf("source plan = %+v", plan.Sources)
	}
	if len(plan.Usage.Includes) != 1 || plan.Usage.Defines[0] != "PUBLIC" {
		t.Fatalf("usage = %+v", plan.Usage)
	}
}
