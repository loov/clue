package discovery

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestLoadOrDiscoverFindsTargetsAndDependencies(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"app/main.cpp":             "#include <lib/math.h>\nint main() { return add(1, 2); }\n",
		"lib/math.cpp":             "#include \"lib/math.h\"\nint add(int a, int b) { return a + b; }\n",
		"include/lib/math.h":       "int add(int, int);\n",
		".build/generated.cpp":     "int generated();\n",
		"vendor/ignored/source.cc": "int vendored();\n",
	}
	for name, contents := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := LoadOrDiscoverForTarget(dir, toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets = %#v", cfg.Targets)
	}
	if cfg.Targets["app"].Type != "executable" || cfg.Targets["lib"].Type != "static_library" {
		t.Fatalf("target types = app:%q lib:%q", cfg.Targets["app"].Type, cfg.Targets["lib"].Type)
	}
	if !slices.Equal(cfg.Targets["app"].Depends, []string{"lib"}) {
		t.Fatalf("app dependencies = %v", cfg.Targets["app"].Depends)
	}
	if !slices.Contains(cfg.Targets["lib"].Headers, filepath.FromSlash("include/lib/math.h")) {
		t.Fatalf("lib headers = %v", cfg.Targets["lib"].Headers)
	}
	if _, err := config.ResolveEnvVars(cfg); err != nil {
		t.Fatalf("resolve environment on discovered config: %v", err)
	}
}

func TestLoadOrDiscoverPrefersExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "explicit"
targets: chosen: {name: "chosen", type: "static_library", sources: ["chosen.cpp"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.cpp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadOrDiscoverForTarget(dir, toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "explicit" || len(cfg.Targets) != 1 || cfg.Targets["chosen"].Name != "chosen" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadOrDiscoverRejectsMultipleMainFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"main.c", "main.cpp"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := LoadOrDiscoverForTarget(dir, toolchain.HostPlatform())
	if err == nil || !strings.Contains(err.Error(), "multiple main source files") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadOrDiscoverSelectsAnAvailableCompiler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.cpp"), []byte("int main() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := discoverCompiler
	t.Cleanup(func() { discoverCompiler = original })
	var gotNames []string
	var gotRequiresCXX bool
	discoverCompiler = func(names []string, _ toolchain.Platform, requiresCXX bool) (string, error) {
		gotNames, gotRequiresCXX = slices.Clone(names), requiresCXX
		return "gcc", nil
	}
	platform := toolchain.HostPlatform()
	cfg, err := LoadOrDiscoverForTarget(dir, platform)
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"clang", "gcc", "msvc"}
	if platform.OS == "windows" {
		wantNames = []string{"msvc", "clang", "gcc"}
	}
	if cfg.Toolchain.Compiler != "gcc" || !gotRequiresCXX || !slices.Equal(gotNames, wantNames) {
		t.Fatalf("compiler = %q, candidates = %v, requires C++ = %t", cfg.Toolchain.Compiler, gotNames, gotRequiresCXX)
	}
}

func TestLoadOrDiscoverIncludesAssemblySources(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"main.c", "startup.S", "helper.s"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := LoadOrDiscoverForTarget(dir, toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"helper.s", "main.c", "startup.S"}
	name := discoveredTargetName(dir, ".")
	if !slices.Equal(cfg.Targets[name].Sources, want) {
		t.Fatalf("sources = %v, want %v", cfg.Targets[name].Sources, want)
	}
}
