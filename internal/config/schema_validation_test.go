package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSchemaValidation_RejectsInvalidTargetType(t *testing.T) {
	// Create temp directory with invalid config
	dir := t.TempDir()
	configContent := `package config

name: "test"
targets: {
    myapp: {
        name: "myapp"
        type: "invalid_type"
        sources: ["main.cpp"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("expected error for invalid target type, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "type") {
		t.Errorf("error should mention type constraint: %v", err)
	}
	if !strings.Contains(errStr, "invalid_type") {
		t.Errorf("error should mention the invalid value: %v", err)
	}
}

func TestSchemaValidation_RejectsInvalidTargetName(t *testing.T) {
	dir := t.TempDir()
	// Target name starting with number violates regex
	configContent := `package config

name: "test"
targets: {
    "123invalid": {
        name: "123invalid"
        type: "executable"
        sources: ["main.cpp"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("expected error for invalid target name pattern, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "123invalid") {
		t.Errorf("error should mention the invalid name: %v", err)
	}
}

func TestSchemaValidation_RejectsInvalidOptimization(t *testing.T) {
	dir := t.TempDir()
	configContent := `package config

name: "test"
targets: {
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["main.cpp"]
    }
}
variants: {
    fast: {
        name: "fast"
        optimization: "O9"
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("expected error for invalid optimization value, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "O9") || !strings.Contains(errStr, "optimization") {
		t.Errorf("error should mention invalid optimization: %v", err)
	}
}

func TestSchemaValidation_RejectsEmptySources(t *testing.T) {
	dir := t.TempDir()
	configContent := `package config

name: "test"
targets: {
    myapp: {
        name: "myapp"
        type: "executable"
        sources: []
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("expected error for empty sources, got nil")
	}
}

func TestSchemaValidation_AcceptsValidConfiguration(t *testing.T) {
	dir := t.TempDir()
	configContent := `package config

name: "valid-project"
targets: {
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["main.cpp"]
    }
    mylib: {
        name: "mylib"
        type: "static_library"
        sources: ["lib.cpp"]
    }
}
variants: {
    debug: {
        name: "debug"
        optimization: "none"
        debug_info: true
    }
    release: {
        name: "release"
        optimization: "aggressive"
        debug_info: false
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("valid config should load successfully: %v", err)
	}

	if cfg.Name != "valid-project" {
		t.Errorf("expected name 'valid-project', got %q", cfg.Name)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(cfg.Targets))
	}
}

func TestSchemaValidation_AcceptsEveryTargetType(t *testing.T) {
	validTypes := []string{"executable", "static_library", "shared_library"}

	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			dir := t.TempDir()
			configContent := `package config

name: "test"
targets: {
    myapp: {
        name: "myapp"
        type: "` + typ + `"
        sources: ["main.cpp"]
    }
}
`
			if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
				t.Fatal(err)
			}

			loader := NewLoader()
			_, err := loader.Load(dir)
			if err != nil {
				t.Errorf("target type %q should be valid: %v", typ, err)
			}
		})
	}

	dir := t.TempDir()
	configContent := `package config
name: "headers"
targets: headers: {
	name: "headers"
	type: "interface_library"
	public: {
		includes: ["include"]
		systemIncludes: ["vendor/include"]
		compilerFlags: ["-pthread"]
		linkerFlags: ["-Wl,--as-needed"]
		sysLibs: ["dl"]
		cxxStd: "c++20"
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatalf("interface library without sources should be valid: %v", err)
	}
	usage := cfg.Targets["headers"].Public
	if usage.CXXStd != "c++20" || !slices.Equal(usage.SystemIncludes, []string{"vendor/include"}) ||
		!slices.Equal(usage.CompilerFlags, []string{"-pthread"}) || !slices.Equal(usage.LinkerFlags, []string{"-Wl,--as-needed"}) ||
		!slices.Equal(usage.SysLibs, []string{"dl"}) {
		t.Fatalf("interface usage = %+v", usage)
	}
}

func TestSchemaValidation_AcceptsEveryOptimizationLevel(t *testing.T) {
	validOpts := []string{"none", "size", "fast", "aggressive"}

	for _, opt := range validOpts {
		t.Run(opt, func(t *testing.T) {
			dir := t.TempDir()
			configContent := `package config

name: "test"
targets: {
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["main.cpp"]
    }
}
variants: {
    custom: {
        name: "custom"
        optimization: "` + opt + `"
    }
}
`
			if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
				t.Fatal(err)
			}

			loader := NewLoader()
			_, err := loader.Load(dir)
			if err != nil {
				t.Errorf("optimization %q should be valid: %v", opt, err)
			}
		})
	}
}
