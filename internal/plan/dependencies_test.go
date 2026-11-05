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
	if !slices.Equal(got.Usage.SystemIncludes, []string{"vendor/sdk/include"}) || !slices.Equal(got.SystemLibraries, []string{"dl"}) {
		t.Errorf("includes = %v; system libraries = %v", got.Usage.SystemIncludes, got.SystemLibraries)
	}
}

func TestResolveDependencies_LinksEveryPrebuiltLibrary(t *testing.T) {
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	cfg := &config.Config{Targets: map[string]config.Target{
		"app": {Name: "app", Sources: []string{"main.c"}, Depends: []string{"sdk"}},
	}}
	external := map[string]ExternalDependency{
		"sdk": {Name: "sdk", Type: "prebuilt_static", Output: "sdk/libcore.a", MoreOutputs: []string{"sdk/libauth.a", "sdk/libs3.a"}},
	}
	got, err := ResolveDependencies(cfg, cfg.Targets["app"], ".build", "debug", platform, external)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sdk/libcore.a", "sdk/libauth.a", "sdk/libs3.a"}
	if !slices.Equal(got.LinkFiles, want) {
		t.Errorf("link files = %v, want %v", got.LinkFiles, want)
	}
	var artifacts []string
	for _, artifact := range got.Artifacts {
		artifacts = append(artifacts, artifact.Path)
	}
	// Every library is a link input, so replacing one relinks.
	if !slices.Equal(artifacts, want) {
		t.Errorf("artifacts = %v, want %v", artifacts, want)
	}
}

func TestResolveDependencies_CustomTargetLinksOnlyItsOutputs(t *testing.T) {
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	cfg := &config.Config{Targets: map[string]config.Target{
		"app":   {Name: "app", Sources: []string{"main.c"}, Depends: []string{"renamed", "order"}},
		"lib":   {Name: "lib", Type: "static_library", Sources: []string{"lib.c"}, SysLibs: []string{"dl"}},
		"order": {Name: "order", Type: "custom", Depends: []string{"lib"}, Outputs: []string{"gen/order.h"}},
		"renamed": {
			Name: "renamed", Type: "custom", Link: true, Depends: []string{"lib"},
			Command: []string{"objcopy", "{output:lib}", "{buildDir}/librenamed.a"}, Outputs: []string{"{buildDir}/librenamed.a"},
		},
	}}
	got, err := ResolveDependencies(cfg, cfg.Targets["app"], ".build", "debug", platform, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(".build", "debug", "librenamed.a")}
	if !slices.Equal(got.LinkFiles, want) || len(got.Libraries) != 0 || len(got.SystemLibraries) != 0 {
		t.Errorf("link files = %v, want %v; libraries = %v; system libraries = %v", got.LinkFiles, want, got.Libraries, got.SystemLibraries)
	}
	if len(got.Artifacts) != 1 || got.Artifacts[0].Path != want[0] {
		t.Errorf("artifacts = %v, want %v", got.Artifacts, want)
	}
	if !slices.Equal(got.LinkReads, []string{"gen/order.h"}) {
		t.Errorf("link reads = %v, want [gen/order.h]", got.LinkReads)
	}

	renamed := cfg.Targets["renamed"]
	renamed.LinkWhole = true
	cfg.Targets["renamed"] = renamed
	got, err = ResolveDependencies(cfg, cfg.Targets["app"], ".build", "debug", platform, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LinkFiles) != 0 || !slices.Equal(got.WholeArchives, want) {
		t.Errorf("link files = %v; whole archives = %v, want %v", got.LinkFiles, got.WholeArchives, want)
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
