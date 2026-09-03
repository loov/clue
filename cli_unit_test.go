package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestSelectConfiguredTestsByNameAndLabel(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{
		"unit": {Name: "unit", Test: &config.Test{Labels: []string{"fast"}}},
		"slow": {Name: "slow", Test: &config.Test{Labels: []string{"integration"}}},
		"app":  {Name: "app"},
	}}
	got, err := selectConfiguredTests(cfg, []string{"fast", "slow"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"slow", "unit"}) {
		t.Fatalf("selected tests = %v", got)
	}
}

func TestLoadConfigUsesSelectedTarget(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "target-aware"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["main.cpp"]
	defines: [if _target.os == "windows" {"WINDOWS_BUILD"}]
}`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, platform, err := loadConfig(dir, "", "windows-amd64", 0)
	if err != nil {
		t.Fatal(err)
	}
	if platform.String() != "windows-amd64" || !slices.Contains(cfg.Targets["app"].Defines, "WINDOWS_BUILD") {
		t.Fatalf("platform=%s defines=%v", platform, cfg.Targets["app"].Defines)
	}
}
