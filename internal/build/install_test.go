package build

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestInstallStagesArtifactsAndPublicHeaders(t *testing.T) {
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "build")
	header := filepath.Join(dir, "include", "clue", "library.h")
	artifact := filepath.Join(buildDir, "release", "lib", "liblibrary.a")
	for path, contents := range map[string]string{header: "header", artifact: "archive"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{BuildDir: buildDir, Targets: map[string]config.Target{
		"library": {Name: "library", Type: "static_library", Headers: []string{header}, Public: config.Usage{Includes: []string{filepath.Join(dir, "include")}}},
	}}
	stage := filepath.Join(dir, "stage")
	result, err := Install(InstallOptions{Config: cfg, Variant: "release", Platform: toolchain.Platform{OS: "linux", Arch: "amd64"}, Prefix: "/usr", DestDir: stage})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(stage, "usr", "lib", "liblibrary.a"), filepath.Join(stage, "usr", "include", "clue", "library.h")}
	if !slices.Equal(result.Files, want) {
		t.Fatalf("installed files = %v, want %v", result.Files, want)
	}
	for _, path := range want {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("installed file %q: %v", path, err)
		}
	}
}

func TestInstallTargets_DoesNotReorderCallerSelection(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{
		"alpha": {Name: "alpha", Type: "static_library"},
		"zeta":  {Name: "zeta", Type: "static_library"},
	}}
	requested := []string{"zeta", "alpha"}
	selected, err := InstallTargets(cfg, requested)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected, []string{"alpha", "zeta"}) {
		t.Fatalf("selected = %v", selected)
	}
	if !slices.Equal(requested, []string{"zeta", "alpha"}) {
		t.Fatalf("requested mutated to %v", requested)
	}
}
