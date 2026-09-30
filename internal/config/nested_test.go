package config

import (
	"os"
	"path/filepath"
	"slices"
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

func TestLoad_ProjectDeclarationsAndOverridesResolveConflicts(t *testing.T) {
	conflict := `
dependencies: {
	wrapper: {
		type: "git", repo: "https://example.com/wrapper", ref: "v2"
		targets: wrapper: {type: "static_library", sources: ["w.cpp"], depends: ["sdk"]}
		dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v1", targets: sdk: {type: "static_library", sources: ["s.cpp"]}}
	}
	other: {
		type: "git", repo: "https://example.com/other", ref: "v3"
		targets: other: {type: "static_library", sources: ["o.cpp"], depends: ["sdk"]}
		dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v9"}
	}
}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["wrapper", "other"]}
`
	for name, resolution := range map[string]string{
		"project declaration": `dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v10"}`,
		"override":            `overrides: sdk: ref: "v10"`,
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": "name: \"resolved\"\n" + conflict + resolution + "\n"}))
			if err != nil {
				t.Fatal(err)
			}
			sdk := cfg.Dependencies["sdk"].(*deps.GitDependency)
			if sdk.Ref != "v10" || !sdk.Description().Exists() {
				t.Fatalf("sdk = %s, described %v", sdk.Ref, sdk.Description().Exists())
			}
		})
	}
}

func TestLoad_SameSourceUnderTwoNames(t *testing.T) {
	project := `
name: "duplicate"
dependencies: wrapper: {
	type: "git", repo: "https://example.com/wrapper", ref: "v2"
	targets: wrapper: {type: "static_library", sources: ["w.cpp"], depends: ["vst3"]}
	dependencies: vst3: {type: "git", repo: "https://example.com/sdk", ref: "v1", build: targetType: "header_only"}
}
dependencies: sdk: {type: "git", repo: "https://example.com/sdk", ref: "v1", build: targetType: "header_only"}
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["wrapper", "sdk"]}
`
	_, err := NewLoader().Load(writeProject(t, map[string]string{"clue.cue": project}))
	if err == nil || !strings.Contains(err.Error(), "use one name") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoad_ImportsProjectPackagesWithoutSetup(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"clue.cue": `import "clue.local/deps"

name: "imports"
dependencies: sdk: deps.sdk
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["sdk"]}
`,
		// a second project file without a package clause joins clue.cue
		"extra.cue": `version: "1.2.3"
`,
		"deps/sdk.cue": `package deps

sdk: {
	type: "git", repo: "https://example.com/sdk", ref: "v1"
	targets: sdk: {type: "static_library", sources: ["sdk.cpp"]}
}
`,
	})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "1.2.3" || !cfg.Dependencies["sdk"].Description().Exists() {
		t.Fatalf("version %q, dependencies %v", cfg.Version, cfg.Dependencies)
	}

	// Errors keep the line numbers of the file.
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte("name: \"x\"\n\ntargets: app: {type: \"executable\", sources: 5}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader().Load(dir); err == nil || !strings.Contains(err.Error(), "clue.cue:3:") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoad_RecursiveGlobsAndExclude(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"clue.cue":                 `name: "globs", targets: lib: {type: "static_library", sources: ["src/**/*.cpp"], exclude: ["**/win32/*"]}`,
		"src/a.cpp":                "",
		"src/sub/b.cpp":            "",
		"src/sub/win32/c.cpp":      "",
		"vendor/sdk/x/y.cpp":       "",
		"vendor/sdk/x/linux/z.cpp": "",
	})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join("src", "a.cpp"), filepath.Join("src", "sub", "b.cpp")}
	if got := cfg.Targets["lib"].Sources; !slices.Equal(got, want) {
		t.Fatalf("sources = %q, want %q", got, want)
	}

	sdk := deps.NewVendoredDependency("sdk", filepath.Join(dir, "vendor", "sdk"), &deps.InlineConfig{
		Type: "static_library", Sources: []string{"**/*.cpp"}, Exclude: []string{"**/linux/*"},
	})
	build, err := deps.ResolveBuildConfig(sdk, filepath.Join(dir, "vendor", "sdk"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(build.Sources, []string{filepath.Join("x", "y.cpp")}) {
		t.Fatalf("dependency sources = %q", build.Sources)
	}
}

func TestLoad_ImportableSchema(t *testing.T) {
	sdk := `package deps

import "loov.dev/clue"

sdk: clue.#Git & {
	repo: "https://example.com/sdk"
	ref:  "v1"
	targets: sdk: {type: "static_library", sources: ["a.cpp"], warnings: WARNINGS}
}
`
	project := `import "clue.local/deps"

name: "schema"
dependencies: sdk: deps.sdk
targets: app: {type: "executable", sources: ["main.cpp"], depends: ["sdk"]}
`
	dir := writeProject(t, map[string]string{"clue.cue": project, "deps/sdk.cue": strings.Replace(sdk, "WARNINGS", `"off"`, 1)})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dependency := cfg.Dependencies["sdk"]; dependency.Type() != "git" {
		t.Fatalf("sdk = %#v", dependency)
	}

	// A mistake is reported in the file that makes it.
	dir = writeProject(t, map[string]string{"clue.cue": project, "deps/sdk.cue": strings.Replace(sdk, "WARNINGS", `"loud"`, 1)})
	if _, err := NewLoader().Load(dir); err == nil || !strings.Contains(err.Error(), "loud") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoad_ListedDependenciesAndLibraryReferences(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"deps/wrapper.cue": `package deps

import "loov.dev/clue"

wrapper: clue.#Git & {
	name:   "clap-wrapper"
	repo:   "https://example.com/wrapper"
	ref:    "v1"
	target: "shared"
	targets: {
		shared: {type: "static_library", sources: ["s.cpp"]}
		vst3: {type: "static_library", sources: ["v.cpp"], depends: ["clap-wrapper"]}
	}
}
`,
		"clue.cue": `import "clue.local/deps"

name: "listed"
dependencies: [deps.wrapper]
targets: app: {type: "executable", sources: ["main.cpp"], depends: [deps.wrapper.lib.vst3, deps.wrapper.lib.shared]}
`,
	})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Targets["app"].Depends; !slices.Equal(got, []string{"clap-wrapper:vst3", "clap-wrapper"}) {
		t.Fatalf("depends = %q", got)
	}
	if _, ok := cfg.Dependencies["clap-wrapper:vst3"]; !ok {
		t.Fatalf("dependencies = %v", cfg.Dependencies)
	}

	// A misspelt library is a CUE error, not a missing dependency later.
	project := `import "clue.local/deps"
name: "typo"
dependencies: [deps.wrapper]
targets: app: {type: "executable", sources: ["main.cpp"], depends: [deps.wrapper.lib.vts3]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader().Load(dir); err == nil || !strings.Contains(err.Error(), "vts3") {
		t.Fatalf("error = %v", err)
	}
}
