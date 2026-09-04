package build

import (
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

func TestBuildTargetInterfaceLibraryNeedsNoTools(t *testing.T) {
	result, err := (&Builder{}).buildTarget(t.Context(), Options{}, config.Target{Name: "headers", Type: "interface_library"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Output != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDependencyLinkInputs_PrebuiltLibraryUsesExactPath(t *testing.T) {
	library := filepath.Join("vendor", "sdk", "custom-name.a")
	b := &Builder{depResults: map[string]*depBuildResult{
		"sdk": {Name: "sdk", Type: "prebuilt_static", LibPath: library},
	}}
	cfg := &config.Config{
		Targets: map[string]config.Target{"app": {Name: "app", Depends: []string{"sdk"}}},
		Dependencies: map[string]deps.Dependency{
			"sdk": deps.NewVendoredDependency("sdk", "vendor/sdk", &deps.InlineConfig{
				Type: "prebuilt_static", Library: "custom-name.a",
			}),
		},
	}
	usage, err := b.dependencyLinkInputs(Options{Config: cfg}, cfg.Targets["app"])
	if err != nil {
		t.Fatal(err)
	}
	if len(usage.libs) != 0 || len(usage.artifacts) != 1 || len(usage.linkFiles) != 1 || usage.linkFiles[0] != library {
		t.Fatalf("libs=%v artifacts=%v link files=%v", usage.libs, usage.artifacts, usage.linkFiles)
	}
}

func TestDependencyLinkInputs_PkgConfigFlags(t *testing.T) {
	b := &Builder{depResults: map[string]*depBuildResult{
		"ssl": {Name: "ssl", Type: "pkg_config", Usage: deps.Usage{LinkerFlags: []string{"-lssl", "-lcrypto"}}},
	}}
	cfg := &config.Config{
		Targets: map[string]config.Target{"app": {Name: "app", Depends: []string{"ssl"}}},
		Dependencies: map[string]deps.Dependency{
			"ssl": deps.NewPkgConfigDependency("ssl", "openssl", false),
		},
	}
	usage, err := b.dependencyLinkInputs(Options{Config: cfg}, cfg.Targets["app"])
	if err != nil {
		t.Fatal(err)
	}
	if len(usage.flags) != 2 || usage.flags[0] != "-lssl" || usage.flags[1] != "-lcrypto" {
		t.Fatalf("link flags = %v", usage.flags)
	}
}

func TestBuilderAddsExternalSharedLibraryRuntimePath(t *testing.T) {
	builder := &Builder{target: toolchain.Platform{OS: "linux", Arch: "amd64"}}
	cfg := toolchain.Flags{}
	output := filepath.Join(".build", "debug", "bin", "app")
	libraryDir := filepath.Join(".build", "debug", "deps", "answer", "lib")

	if err := builder.addRuntimeLibraryPaths(&cfg, output, []string{libraryDir}); err != nil {
		t.Fatal(err)
	}
	want := "-Wl,-rpath,$ORIGIN/../deps/answer/lib"
	if len(cfg.RawLinker) != 1 || cfg.RawLinker[0] != want {
		t.Errorf("runtime flags = %q, want [%q]", cfg.RawLinker, want)
	}
}
