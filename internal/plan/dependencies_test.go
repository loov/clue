package plan

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

func TestResolveDependencies(t *testing.T) {
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	prebuilt := filepath.Join("vendor", "sdk", "custom-name.a")
	cfg := &config.Config{Targets: map[string]config.Target{
		"app":  {Name: "app", Sources: []string{"main.c"}, Depends: []string{"core", "sdk", "ssl"}},
		"core": {Name: "core", Type: "static_library", Sources: []string{"core.cpp"}, SysLibs: []string{"dl"}},
	}}
	external := map[string]ExternalDependency{
		"sdk": {Name: "sdk", Type: "prebuilt_static", Output: prebuilt, Include: "vendor/sdk/include"},
		"ssl": {Name: "ssl", Type: "pkg_config", Usage: deps.Usage{LinkerFlags: []string{"-lssl"}}},
	}

	got, err := ResolveDependencies(cfg, cfg.Targets["app"], ".build", "debug", platform, external)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UsesCXX {
		t.Error("C++ dependency did not select the C++ linker")
	}
	if !slices.Equal(got.LinkFiles, []string{prebuilt}) || !slices.Equal(got.Usage.LinkerFlags, []string{"-lssl"}) {
		t.Errorf("link files = %v; flags = %v", got.LinkFiles, got.Usage.LinkerFlags)
	}
	if !slices.Equal(got.Usage.Includes, []string{"vendor/sdk/include"}) || !slices.Equal(got.SystemLibraries, []string{"dl"}) {
		t.Errorf("includes = %v; system libraries = %v", got.Usage.Includes, got.SystemLibraries)
	}
}

func TestRuntimeLibraryFlags(t *testing.T) {
	output := filepath.Join(".build", "debug", "bin", "app")
	libraryDir := filepath.Join(".build", "debug", "deps", "answer", "lib")
	got, err := RuntimeLibraryFlags(output, []string{libraryDir}, toolchain.Platform{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "-Wl,-rpath,$ORIGIN/../deps/answer/lib"; !slices.Equal(got, []string{want}) {
		t.Errorf("runtime flags = %q, want [%q]", got, want)
	}
}
