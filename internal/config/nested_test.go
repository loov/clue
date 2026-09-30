package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/deps"
)

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const nestedProject = `
name: "nested"
_sdk: {type: "git", repo: "https://example.com/sdk", ref: "v1", targets: sdk: {type: "static_library", sources: ["sdk.cpp"]}}
dependencies: {
	wrapper: {
		type: "git", repo: "https://example.com/wrapper", ref: "v2"
		targets: wrapper: {type: "static_library", sources: ["w.cpp"], depends: ["sdk"]}
		dependencies: sdk: _sdk
	}
	other: {
		type: "git", repo: "https://example.com/other", ref: "v3"
		targets: other: {type: "static_library", sources: ["o.cpp"], depends: ["sdk"]}
		dependencies: sdk: _sdk
	}
}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["wrapper", "other"]}
`

func TestLoad_NestedDependenciesShareOneDeclaration(t *testing.T) {
	cfg, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": nestedProject}))
	if err != nil {
		t.Fatal(err)
	}
	sdk, ok := cfg.Dependencies["sdk"].(*deps.GitDependency)
	if !ok || sdk.Repo != "https://example.com/sdk" || !sdk.Description().Exists() {
		t.Fatalf("dependencies = %v", cfg.Dependencies)
	}
	if got := deps.DeclaredDepends(cfg.Dependencies["wrapper"]); len(got) != 1 || got[0] != "sdk" {
		t.Fatalf("wrapper depends on %q", got)
	}
}

func TestLoad_ConflictingNestedDependencies(t *testing.T) {
	project := `
name: "conflict"
dependencies: {
	wrapper: {
		type: "git", repo: "https://example.com/wrapper", ref: "v2"
		targets: wrapper: {type: "static_library", sources: ["w.cpp"], depends: ["sdk"]}
		dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v1"}
	}
	other: {
		type: "git", repo: "https://example.com/other", ref: "v3"
		targets: other: {type: "static_library", sources: ["o.cpp"], depends: ["sdk"]}
		dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v9"}
	}
}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["wrapper", "other"]}
`
	_, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": project}))
	if err == nil || !strings.Contains(err.Error(), `conflicting declarations of dependency "sdk"`) ||
		!strings.Contains(err.Error(), "v1") || !strings.Contains(err.Error(), "v9") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoad_NestedDependenciesWithoutTargets(t *testing.T) {
	project := `
name: "untargeted"
dependencies: {
	described: {
		type: "vendored", path: "third_party/described"
		dependencies: zlib: {type: "git", repo: "https://example.com/zlib", ref: "v1"}
	}
	inline: {
		type: "vendored", path: "third_party/inline"
		build: {targetType: "static_library", sources: ["i.c"]}
		dependencies: png: {type: "git", repo: "https://example.com/png", ref: "v1"}
	}
}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["described", "inline"]}
`
	cfg, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": project}))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zlib", "png"} {
		if _, ok := cfg.Dependencies[name]; !ok {
			t.Errorf("nested dependency %q missing: %v", name, cfg.Dependencies)
		}
	}
}

func TestExpandDependencies_ReadsFetchedClueFiles(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"clue.cue": `
name: "fetched"
dependencies: lib: {type: "git", repo: "https://example.com/lib", ref: "v1"}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["lib"]}
`,
		// the checkout of lib, which declares its own dependency
		".deps/git/lib-v1/clue.cue": `
dependencies: base: {type: "vendored", path: "third_party/base", build: targetType: "header_only"}
targets: lib: {type: "static_library", sources: ["lib.cpp"], depends: ["base"]}
`,
	})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, ok := cfg.Dependencies["base"].(*deps.VendoredDependency)
	if !ok || base.Path != filepath.Join(".deps", "git", "lib-v1", "third_party", "base") {
		t.Fatalf("dependencies = %v", cfg.Dependencies)
	}
	if added, err := ExpandDependencies(cfg); err != nil || added != 0 {
		t.Fatalf("second expansion added %d, %v", added, err)
	}
}
