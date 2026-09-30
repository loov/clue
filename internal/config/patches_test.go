package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/deps"
)

func patchPaths(dependency deps.Dependency) []string {
	var paths []string
	for _, patch := range deps.Patches(dependency) {
		paths = append(paths, patch.Path)
	}
	return paths
}

func TestLoad_PatchesAreRelativeToTheFileListingThem(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"clue.cue": `
import "clue.local/deps"

name: "patched"
dependencies: [
	deps.lib,
	{name: "described", type: "git", repo: "https://example.com/described", ref: "v1", file: "descriptions/described.cue"},
	{name: "fetched", type: "git", repo: "https://example.com/fetched", ref: "v1"},
]
overrides: {
	lib: extraPatches: ["patches/extra.patch"]
	nested: patches: ["patches/replacement.patch"]
}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["lib"]}
`,
		"deps/lib.cue": `package deps

lib: {name: "lib", type: "git", repo: "https://example.com/lib", ref: "v1", patches: ["patches/lib.patch"], build: targetType: "header_only"}
`,
		"deps/patches/lib.patch":    "lib",
		"patches/extra.patch":       "extra",
		"patches/replacement.patch": "replacement",
		"descriptions/described.cue": `
dependencies: fromfile: {type: "git", repo: "https://example.com/fromfile", ref: "v1", patches: ["p/fromfile.patch"], build: targetType: "header_only"}
targets: described: {type: "static_library", sources: ["d.cpp"]}
`,
		"descriptions/p/fromfile.patch": "fromfile",
		".deps/git/fetched-v1/clue.cue": `
dependencies: nested: {type: "git", repo: "https://example.com/nested", ref: "v1", patches: ["patches/nested.patch"], build: targetType: "header_only"}
targets: fetched: {type: "static_library", sources: ["f.cpp"]}
`,
		".deps/git/fetched-v1/patches/nested.patch": "nested",
	})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]string{
		"lib":      {"deps/patches/lib.patch", "patches/extra.patch"},
		"fromfile": {"descriptions/p/fromfile.patch"},
		"nested":   {"patches/replacement.patch"},
	} {
		got := patchPaths(cfg.Dependencies[name])
		if len(got) != len(want) {
			t.Errorf("%s patches = %q, want %q", name, got, want)
			continue
		}
		for index := range want {
			if got[index] != filepath.Join(dir, want[index]) {
				t.Errorf("%s patches = %q, want %q", name, got, want)
			}
		}
	}
	if lib := cfg.Dependencies["lib"]; lib.CachePath(".") == lib.(*deps.GitDependency).PristinePath(".") {
		t.Error("patched lib uses the directory of its unpatched checkout")
	}
}

func TestLoad_PatchesPartOfTheSource(t *testing.T) {
	project := `
name: "conflict"
dependencies: {
	wrapper: {
		type: "git", repo: "https://example.com/wrapper", ref: "v2"
		targets: wrapper: {type: "static_library", sources: ["w.cpp"], depends: ["sdk"]}
		dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v1", patches: ["a.patch"], build: targetType: "header_only"}
	}
	other: {
		type: "git", repo: "https://example.com/other", ref: "v3"
		targets: other: {type: "static_library", sources: ["o.cpp"], depends: ["sdk"]}
		dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v1", build: targetType: "header_only"}
	}
}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["wrapper", "other"]}
`
	_, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": project, "a.patch": "a"}))
	if err == nil || !strings.Contains(err.Error(), `conflicting declarations of dependency "sdk"`) || !strings.Contains(err.Error(), "with patches") {
		t.Fatalf("expected a conflict over the patches of sdk, got %v", err)
	}

	// Removing the patches in an override resolves it.
	cfg, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": project + `overrides: sdk: patches: []`, "a.patch": "a"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := patchPaths(cfg.Dependencies["sdk"]); len(got) != 0 {
		t.Fatalf("sdk patches = %q", got)
	}

	_, err = NewLoader().Load(writeProject(t, map[string]string{"clue.cue": project}))
	if err == nil || !strings.Contains(err.Error(), `git dependency "sdk": patch:`) || !strings.Contains(err.Error(), "a.patch") {
		t.Fatalf("expected an error naming the missing patch of sdk, got %v", err)
	}
}
