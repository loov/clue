package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeBuildOrder_ConnectsConfiguredDependencies(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Use JSON format (compatible with CUE shim)
	config := `{
	"name": "testproject",
	"targets": {
		"util": {
			"name": "util",
			"type": "static_library",
			"sources": ["util.cpp"]
		},
		"core": {
			"name": "core",
			"type": "static_library",
			"sources": ["core.cpp"],
			"depends": ["util"]
		},
		"app": {
			"name": "app",
			"type": "executable",
			"sources": ["main.cpp"],
			"depends": ["core", "util"]
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	order, err := ComputeBuildOrder(cfg)
	if err != nil {
		t.Fatalf("ComputeBuildOrder failed: %v", err)
	}

	// Verify dependencies come before dependents
	indexOf := func(slice []string, item string) int {
		for i, s := range slice {
			if s == item {
				return i
			}
		}
		return -1
	}

	utilIdx := indexOf(order, "util")
	coreIdx := indexOf(order, "core")
	appIdx := indexOf(order, "app")

	if utilIdx == -1 || coreIdx == -1 || appIdx == -1 {
		t.Fatalf("Missing targets in order: %v", order)
	}

	if utilIdx > coreIdx {
		t.Errorf("util (%d) should come before core (%d)", utilIdx, coreIdx)
	}
	if coreIdx > appIdx {
		t.Errorf("core (%d) should come before app (%d)", coreIdx, appIdx)
	}
	if utilIdx > appIdx {
		t.Errorf("util (%d) should come before app (%d)", utilIdx, appIdx)
	}
}

func TestComputeBuildOrder_RejectsCycle(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Use JSON format (compatible with CUE shim)
	config := `{
	"name": "testproject",
	"targets": {
		"a": {
			"name": "a",
			"type": "static_library",
			"sources": ["a.cpp"],
			"depends": ["b"]
		},
		"b": {
			"name": "b",
			"type": "static_library",
			"sources": ["b.cpp"],
			"depends": ["c"]
		},
		"c": {
			"name": "c",
			"type": "static_library",
			"sources": ["c.cpp"],
			"depends": ["a"]
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = ComputeBuildOrder(cfg)
	if err == nil {
		t.Fatal("Expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cyclic") {
		t.Errorf("Expected cyclic error, got: %v", err)
	}
}

func TestComputeBuildOrder_RejectsUnknownDependency(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Use JSON format (compatible with CUE shim)
	config := `{
	"name": "testproject",
	"targets": {
		"app": {
			"name": "app",
			"type": "executable",
			"sources": ["main.cpp"],
			"depends": ["nonexistent"]
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = ComputeBuildOrder(cfg)
	if err == nil {
		t.Fatal("Expected unknown dependency error")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("Expected error mentioning 'nonexistent', got: %v", err)
	}
}

func TestComputeBuildOrder_OrdersDependenciesFirst(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Use JSON format (compatible with CUE shim)
	config := `{
	"name": "simple",
	"targets": {
		"lib": {
			"name": "lib",
			"type": "static_library",
			"sources": ["lib.cpp"]
		},
		"main": {
			"name": "main",
			"type": "executable",
			"sources": ["main.cpp"],
			"depends": ["lib"]
		}
	}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	order, err := ComputeBuildOrder(cfg)
	if err != nil {
		t.Fatalf("GetBuildOrder failed: %v", err)
	}

	// lib must come before main
	if len(order) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(order))
	}
	if order[0] != "lib" || order[1] != "main" {
		t.Errorf("Expected [lib, main], got %v", order)
	}
}
