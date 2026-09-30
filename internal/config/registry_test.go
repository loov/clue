package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"cuelang.org/go/mod/modregistrytest"
)

func TestLoad_ImportsDescriptionsFromARegistry(t *testing.T) {
	// A module of shared dependency descriptions, published to a registry.
	registry, err := modregistrytest.New(fstest.MapFS{
		"example.com_ports_v0.1.0/cue.mod/module.cue": {Data: []byte(`module: "example.com/ports@v0"
language: version: "v0.14.0"
`)},
		"example.com_ports_v0.1.0/sdk/sdk.cue": {Data: []byte(`package sdk

import "loov.dev/clue"

sdk: clue.#Git & {
	name: "sdk"
	repo: "https://example.com/sdk"
	ref:  "v1"
	targets: sdk: {type: "static_library", sources: ["**/*.cpp"]}
}
`)},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	t.Setenv("CUE_REGISTRY", registry.Host()+"+insecure")
	cache := t.TempDir()
	t.Setenv("CUE_CACHE_DIR", cache)
	// The cache is read-only, as Go's module cache; allow removing it.
	t.Cleanup(func() {
		_ = filepath.WalkDir(cache, func(path string, _ fs.DirEntry, _ error) error {
			return os.Chmod(path, 0o755)
		})
	})

	dir := writeProject(t, map[string]string{
		"cue.mod/module.cue": `module: "example.com/app@v0"
language: version: "v0.14.0"
deps: "example.com/ports@v0": v: "v0.1.0"
`,
		"clue.cue": `import "example.com/ports/sdk"

name: "registry"
dependencies: [sdk.sdk]
targets: app: {type: "executable", sources: ["main.cpp"], depends: [sdk.sdk.lib.sdk]}
`,
	})
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dependency, ok := cfg.Dependencies["sdk"]; !ok || !dependency.Description().Exists() {
		t.Fatalf("dependencies = %v", cfg.Dependencies)
	}
}
