package build

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
)

func TestBuildTargetInterfaceLibraryNeedsNoTools(t *testing.T) {
	result, err := (&Builder{}).BuildTarget(context.Background(), Options{}, config.Target{Name: "headers", Type: "interface_library"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Output != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDependencyLinkInputs_PrebuiltLibraryUsesExactPath(t *testing.T) {
	library := filepath.Join("vendor", "sdk", "custom-name.a")
	b := &Builder{depResults: map[string]*DepBuildResult{
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
	b := &Builder{depResults: map[string]*DepBuildResult{
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

func TestTargetToBuildConfig_DefaultsFromVariant(t *testing.T) {
	b, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	target := config.Target{Name: "test"}
	variant := config.Variant{Optimization: "fast", DebugInfo: true}

	cfg := b.targetToConfig(target, variant)

	if cfg.Optimize != "fast" {
		t.Errorf("expected Optimize='fast', got '%s'", cfg.Optimize)
	}
	if cfg.Debug != "full" {
		t.Errorf("expected Debug='full' from DebugInfo=true, got '%s'", cfg.Debug)
	}
}

func TestTargetToBuildConfig_TargetOverridesDefaults(t *testing.T) {
	b, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	target := config.Target{
		Name:     "test",
		Optimize: "size",
		Warnings: "strict",
		Debug:    "minimal",
	}
	// Variant has no optimization set
	variant := config.Variant{Optimization: "", DebugInfo: false}

	cfg := b.targetToConfig(target, variant)

	if cfg.Optimize != "size" {
		t.Errorf("expected Optimize='size' from target, got '%s'", cfg.Optimize)
	}
	if cfg.Warnings != "strict" {
		t.Errorf("expected Warnings='strict' from target, got '%s'", cfg.Warnings)
	}
	if cfg.Debug != "minimal" {
		t.Errorf("expected Debug='minimal' from target, got '%s'", cfg.Debug)
	}
}

func TestTargetToBuildConfig_VariantOverridesTarget(t *testing.T) {
	b, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	// Target sets debug to minimal
	target := config.Target{
		Name:  "test",
		Debug: "minimal",
	}
	// Variant DebugInfo=true should override to "full"
	variant := config.Variant{DebugInfo: true}

	cfg := b.targetToConfig(target, variant)

	// Variant DebugInfo=true should win over target.Debug
	if cfg.Debug != "full" {
		t.Errorf("expected Debug='full' from variant DebugInfo, got '%s'", cfg.Debug)
	}
}

func TestTargetToBuildConfig_AdvancedVariantFlags(t *testing.T) {
	b, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	enabled, disabled := true, false
	target := config.Target{
		Sanitizers: []string{"address"}, LTO: &enabled, PIC: &disabled, Coverage: &disabled,
		Debug: "full",
	}
	variant := config.Variant{
		Sanitizers: []string{"undefined"}, LTO: &disabled, PIC: &enabled, Coverage: &enabled,
		DebugInfoSet: true, DebugInfo: false,
	}

	cfg := b.targetToConfig(target, variant)
	if len(cfg.Sanitizers) != 1 || cfg.Sanitizers[0] != "undefined" || cfg.LTO || !cfg.PIC || !cfg.Coverage {
		t.Errorf("advanced variant flags not applied: %+v", cfg)
	}
	if cfg.Debug != "none" {
		t.Errorf("explicit debug_info: false did not disable target debug: %q", cfg.Debug)
	}
}

func TestTargetToBuildConfig_WarningsAsErrors(t *testing.T) {
	b, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	// Test pointer semantics: false should override default true
	falseVal := false
	target := config.Target{
		Name:             "test",
		WarningsAsErrors: &falseVal,
	}
	variant := config.Variant{}

	cfg := b.targetToConfig(target, variant)

	if cfg.WarningsAsErrors != false {
		t.Errorf("expected WarningsAsErrors=false from target, got %v", cfg.WarningsAsErrors)
	}
}

func TestBuilderAddsExternalSharedLibraryRuntimePath(t *testing.T) {
	builder := &Builder{target: Platform{OS: "linux", Arch: "amd64"}}
	cfg := Config{}
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

func TestObjectDir_IncludesObjSubdirectory(t *testing.T) {
	b, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	objDir := b.ObjectDir(".build", "debug", "myapp")

	expected := filepath.Join(".build", "debug", "myapp", "obj")
	if objDir != expected {
		t.Errorf("ObjectDir() = %q, want %q", objDir, expected)
	}
}
