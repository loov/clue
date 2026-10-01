package plan

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestExpandCustomTarget_SubstitutesVariantPaths(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{
		"lib": {Name: "lib", Type: "static_library"},
		"gen": {Name: "gen", Type: "custom", Command: []string{"gen"}, Outputs: []string{"{buildDir}/gen.h"}},
		"step": {
			Name: "step", Type: "custom", Depends: []string{"lib", "gen"},
			Command: []string{"sh", "-c", "{ cat $1; } > $2", "{variant}", "{output:lib}", "{output:gen}"},
			Outputs: []string{"{buildDir}/out"},
		},
	}}
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	got, err := ExpandCustomTarget(cfg, cfg.Targets["step"], ".build", "release", platform)
	if err != nil {
		t.Fatal(err)
	}
	// {buildDir} and {output:} expand to native paths; the rest is kept as written.
	buildDir := filepath.Join(".build", "release")
	want := []string{"sh", "-c", "{ cat $1; } > $2", "release", filepath.Join(buildDir, "lib", "liblib.a"), buildDir + "/gen.h"}
	if !slices.Equal(got.Command, want) || got.Outputs[0] != buildDir+"/out" {
		t.Fatalf("expanded = %q %q", got.Command, got.Outputs)
	}
	if !CustomTargetPerVariant(cfg.Targets["step"]) || CustomTargetPerVariant(config.Target{Command: []string{"{ x; }"}}) {
		t.Fatal("per-variant detection is wrong")
	}

	missing := cfg.Targets["step"]
	missing.Depends = nil
	if _, err := ExpandCustomTarget(cfg, missing, ".build", "release", platform); err == nil {
		t.Fatal("{output:lib} without depends expanded without error")
	}
}
