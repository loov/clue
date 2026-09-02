package build

import (
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
)

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
