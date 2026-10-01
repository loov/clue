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

func TestHostGeneratorDoesNotExportItsDependencies(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{
		"app":    {Name: "app", Type: "executable", Sources: []string{"app.c"}, Depends: []string{"header"}},
		"header": {Name: "header", Type: "custom", Depends: []string{"gen"}},
		"gen":    {Name: "gen", Type: "executable", Host: true, Depends: []string{"helper"}},
		"helper": {Name: "helper", Type: "static_library", Sources: []string{"helper.cpp"}, Public: config.Usage{Defines: []string{"HOST_ONLY"}}},
	}}
	app := cfg.Targets["app"]
	usage := config.CompileUsage(cfg, app)
	if len(usage.Defines) != 0 || config.TargetUsesCXX(cfg, app) {
		t.Fatalf("host requirements leaked into app: %+v", usage)
	}
	dependencies, err := ResolveDependencies(cfg, app, ".build/cross", "debug", toolchain.Platform{OS: "linux", Arch: "arm64"}, nil)
	if err != nil || len(dependencies.Artifacts) != 0 {
		t.Fatalf("host link dependencies leaked into app: %+v, %v", dependencies, err)
	}
	modules, err := DependencyModuleOutputs(cfg, app, map[string]map[string]string{"helper": {"private": "host.pcm"}})
	if err != nil || len(modules) != 0 {
		t.Fatalf("host modules leaked into app: %v, %v", modules, err)
	}
	if !config.TargetUsesCXX(cfg, cfg.Targets["gen"]) {
		t.Fatal("host generator lost its own C++ dependency")
	}
}
