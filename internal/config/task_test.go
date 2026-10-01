package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTask_DependsOnOutputsAndCannotBeDependedOn(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "tasks"
targets: {
	tool: {type: "executable", sources: ["main.cpp"]}
	check: {type: "task", command: ["{output:tool}"], depends: ["lib"]}
	lib: {type: "static_library", sources: ["main.cpp"]}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.cpp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if depends := cfg.Targets["check"].Depends; !slices.Equal(depends, []string{"lib", "tool"}) {
		t.Fatalf("depends = %v", depends)
	}

	lib := cfg.Targets["lib"]
	lib.Depends = []string{"check"}
	cfg.Targets["lib"] = lib
	if _, err := ComputeBuildOrder(cfg); err == nil || !strings.Contains(err.Error(), `depends on task "check"`) {
		t.Fatalf("ComputeBuildOrder error = %v", err)
	}
}
