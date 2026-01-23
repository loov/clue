package build

import (
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestTargetToBuildConfig_DefaultsFromVariant(t *testing.T) {
	b := NewBuilder("clang", false)
	target := config.Target{Name: "test"}
	variant := config.Variant{Optimization: "fast", DebugInfo: true}

	cfg := b.targetToBuildConfig(target, variant)

	if cfg.Optimize != "fast" {
		t.Errorf("expected Optimize='fast', got '%s'", cfg.Optimize)
	}
	if cfg.Debug != "full" {
		t.Errorf("expected Debug='full' from DebugInfo=true, got '%s'", cfg.Debug)
	}
}

func TestTargetToBuildConfig_TargetOverridesDefaults(t *testing.T) {
	b := NewBuilder("clang", false)
	target := config.Target{
		Name:     "test",
		Optimize: "size",
		Warnings: "strict",
		Debug:    "minimal",
	}
	// Variant has no optimization set
	variant := config.Variant{Optimization: "", DebugInfo: false}

	cfg := b.targetToBuildConfig(target, variant)

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
	b := NewBuilder("clang", false)
	// Target sets debug to minimal
	target := config.Target{
		Name:  "test",
		Debug: "minimal",
	}
	// Variant DebugInfo=true should override to "full"
	variant := config.Variant{DebugInfo: true}

	cfg := b.targetToBuildConfig(target, variant)

	// Variant DebugInfo=true should win over target.Debug
	if cfg.Debug != "full" {
		t.Errorf("expected Debug='full' from variant DebugInfo, got '%s'", cfg.Debug)
	}
}

func TestTargetToBuildConfig_WarningsAsErrors(t *testing.T) {
	b := NewBuilder("clang", false)

	// Test pointer semantics: false should override default true
	falseVal := false
	target := config.Target{
		Name:             "test",
		WarningsAsErrors: &falseVal,
	}
	variant := config.Variant{}

	cfg := b.targetToBuildConfig(target, variant)

	if cfg.WarningsAsErrors != false {
		t.Errorf("expected WarningsAsErrors=false from target, got %v", cfg.WarningsAsErrors)
	}
}

func TestObjectDir_IncludesObjSubdirectory(t *testing.T) {
	b := NewBuilder("clang", false)

	objDir := b.ObjectDir(".build", "debug", "myapp")

	expected := filepath.Join(".build", "debug", "myapp", "obj")
	if objDir != expected {
		t.Errorf("ObjectDir() = %q, want %q", objDir, expected)
	}
}
