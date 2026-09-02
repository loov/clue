package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidConfig(t *testing.T) {
	// Create temp directory with valid config
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Using pure JSON for stub parser
	validConfig := `{"name": "testproject", "version": "1.0.0", "toolchain": {"compiler": "clang", "std": "c++20"}, "targets": {"main": {"name": "main", "type": "executable", "sources": ["main.cpp"]}}}`
	if err := os.WriteFile(configPath, []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "testproject" {
		t.Errorf("Expected name 'testproject', got '%s'", cfg.Name)
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", cfg.Version)
	}
	if cfg.Toolchain.Compiler != "clang" {
		t.Errorf("Expected compiler 'clang', got '%s'", cfg.Toolchain.Compiler)
	}
	if cfg.Toolchain.Std != "c++20" {
		t.Errorf("Expected std 'c++20', got '%s'", cfg.Toolchain.Std)
	}
	if len(cfg.Targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(cfg.Targets))
	}
	if cfg.Targets["main"].Type != "executable" {
		t.Errorf("Expected type 'executable', got '%s'", cfg.Targets["main"].Type)
	}
}

func TestInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Missing required source field (empty array)
	invalidConfig := `{"name": "testproject", "targets": {"main": {"name": "main", "type": "executable", "sources": []}}}`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)
	if err == nil {
		t.Fatal("Expected validation error for empty sources")
	}

	// Error message should mention sources
	errMsg := err.Error()
	if !strings.Contains(errMsg, "sources") {
		t.Errorf("Error should mention 'sources', got: %s", errMsg)
	}
}

func TestMissingConfig(t *testing.T) {
	dir := t.TempDir() // Empty directory

	loader := NewLoader()
	_, err := loader.Load(dir)
	if err == nil {
		t.Fatal("Expected error for missing config")
	}

	// Should produce a RichError with suggestion
	errMsg := err.Error()
	if !strings.Contains(errMsg, "no CUE configuration") {
		t.Errorf("Error should mention missing config, got: %s", errMsg)
	}
}

func TestTargetExtraction(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	config := `{"name": "multilib", "targets": {"util": {"name": "util", "type": "static_library", "sources": ["util.cpp"], "headers": ["util.h"], "includes": ["include/"], "public": {"includes": ["public/"], "defines": ["UTIL_PUBLIC"]}, "sanitizers": ["address"], "lto": true, "pic": true, "coverage": true}, "app": {"name": "app", "type": "executable", "sources": ["main.cpp"], "depends": ["util"], "flags": {"compiler": ["-Wall", "-Wextra"]}}}}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(cfg.Targets))
	}

	// Check util target
	util := cfg.Targets["util"]
	if util.Type != "static_library" {
		t.Errorf("Expected util type 'static_library', got '%s'", util.Type)
	}
	if len(util.Sources) != 1 || util.Sources[0] != "util.cpp" {
		t.Errorf("Expected util sources ['util.cpp'], got %v", util.Sources)
	}
	if len(util.Headers) != 1 || util.Headers[0] != "util.h" {
		t.Errorf("Expected util headers ['util.h'], got %v", util.Headers)
	}
	if len(util.Includes) != 1 || util.Includes[0] != "include/" {
		t.Errorf("Expected util includes ['include/'], got %v", util.Includes)
	}
	if len(util.Public.Includes) != 1 || util.Public.Includes[0] != "public/" ||
		len(util.Public.Defines) != 1 || util.Public.Defines[0] != "UTIL_PUBLIC" {
		t.Errorf("public usage requirements were not extracted: %+v", util.Public)
	}
	if len(util.Sanitizers) != 1 || util.Sanitizers[0] != "address" ||
		util.LTO == nil || !*util.LTO || util.PIC == nil || !*util.PIC ||
		util.Coverage == nil || !*util.Coverage {
		t.Errorf("advanced target flags were not extracted: %+v", util)
	}

	// Check app target
	app := cfg.Targets["app"]
	if app.Type != "executable" {
		t.Errorf("Expected app type 'executable', got '%s'", app.Type)
	}
	if len(app.Depends) != 1 || app.Depends[0] != "util" {
		t.Errorf("Expected app to depend on util, got %v", app.Depends)
	}
	if len(app.Flags.Compiler) != 2 {
		t.Errorf("Expected 2 compiler flags, got %d", len(app.Flags.Compiler))
	}
}

func TestLoaderUnquotesMapLabels(t *testing.T) {
	dir := t.TempDir()
	config := `name: "labels"
targets: "my-app": {
	name: "my-app"
	type: "executable"
	sources: ["main.cpp"]
	depends: ["some-lib"]
}
variants: "fast-build": {optimization: "fast"}
dependencies: "some-lib": {
	type: "vendored"
	path: "vendor"
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["my-app"].Name != "my-app" || cfg.Variants["fast-build"].Name != "fast-build" {
		t.Errorf("quoted labels were not decoded: targets=%#v variants=%#v", cfg.Targets, cfg.Variants)
	}
	if _, ok := cfg.Dependencies["some-lib"]; !ok {
		t.Errorf("quoted dependency label was not decoded: %#v", cfg.Dependencies)
	}
}

func TestLoaderRejectsMismatchedTargetName(t *testing.T) {
	dir := t.TempDir()
	config := `name: "mismatch"
targets: app: {
	name: "different"
	type: "executable"
	sources: ["main.cpp"]
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader().Load(dir); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected target name mismatch, got %v", err)
	}
}

func TestLoaderNewLoader(t *testing.T) {
	loader := NewLoader()
	if loader == nil {
		t.Fatal("NewLoader should not return nil")
	}
	if loader.ctx == nil {
		t.Error("Loader context should not be nil")
	}
}

func TestSuggestFix(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		msg      string
		expected string
	}{
		{"incomplete value", "ensure all required fields are specified"},
		{"conflicting values", "check that values are compatible with their type constraints"},
		{"cannot use value", "verify the value matches the expected type"},
		{"undefined field 'foo'", "check field name spelling or add it to the schema"},
		{"sources empty", "at least one source file is required"},
		{"random error", ""}, // No suggestion
	}

	for _, tc := range tests {
		result := loader.suggestFix(tc.msg)
		if result != tc.expected {
			t.Errorf("suggestFix(%q) = %q, want %q", tc.msg, result, tc.expected)
		}
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"incomplete value", "INCOMPLETE", true},
		{"", "test", false},
	}

	for _, tc := range tests {
		result := containsIgnoreCase(tc.s, tc.substr)
		if result != tc.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tc.s, tc.substr, result, tc.want)
		}
	}
}

func TestVariantExtraction(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	config := `{"name": "project", "targets": {"app": {"name": "app", "type": "executable", "sources": ["main.cpp"]}}, "variants": {"debug": {"name": "debug", "optimization": "none", "debug_info": true, "defines": ["DEBUG"], "sanitizers": ["address"], "lto": true, "pic": true, "coverage": true}, "release": {"name": "release", "optimization": "fast", "debug_info": false}}}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Variants) != 2 {
		t.Fatalf("Expected 2 variants, got %d", len(cfg.Variants))
	}

	debug := cfg.Variants["debug"]
	if debug.Optimization != "none" {
		t.Errorf("Expected debug optimization 'none', got '%s'", debug.Optimization)
	}
	if !debug.DebugInfo {
		t.Error("Expected debug.debug_info to be true")
	}
	if len(debug.Defines) != 1 || debug.Defines[0] != "DEBUG" {
		t.Errorf("Expected debug defines ['DEBUG'], got %v", debug.Defines)
	}
	if len(debug.Sanitizers) != 1 || debug.Sanitizers[0] != "address" ||
		debug.LTO == nil || !*debug.LTO || debug.PIC == nil || !*debug.PIC ||
		debug.Coverage == nil || !*debug.Coverage {
		t.Errorf("advanced variant flags were not extracted: %+v", debug)
	}

	release := cfg.Variants["release"]
	if release.Optimization != "fast" {
		t.Errorf("Expected release optimization 'fast', got '%s'", release.Optimization)
	}
	if release.DebugInfo {
		t.Error("Expected release.debug_info to be false")
	}
	if !release.DebugInfoSet {
		t.Error("Expected explicit release.debug_info to be tracked")
	}
}

func TestInvalidDirectory(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Minimal valid config
	minimalConfig := `{"name": "minimal", "targets": {"app": {"name": "app", "type": "executable", "sources": ["main.cpp"]}}}`
	if err := os.WriteFile(configPath, []byte(minimalConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "minimal" {
		t.Errorf("Expected name 'minimal', got '%s'", cfg.Name)
	}

	// Optional fields should have zero values
	if cfg.Version != "" {
		t.Errorf("Expected empty version, got '%s'", cfg.Version)
	}
	if len(cfg.Variants) != 0 {
		t.Errorf("Expected no variants, got %d", len(cfg.Variants))
	}
}

func TestLoaderRejectsInvalidType(t *testing.T) {
	dir := t.TempDir()
	config := `package config
name: "test"
targets: {
    app: {
        name: "app"
        type: "bad_type"
        sources: ["main.cpp"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("loader should reject invalid target type")
	}
}

func TestLoaderAcceptsValidConfig(t *testing.T) {
	dir := t.TempDir()
	config := `package config
name: "myproject"
version: "1.0.0"
toolchain: {
    compiler: "clang++"
    std: "c++20"
}
targets: {
    mylib: {
        name: "mylib"
        type: "static_library"
        sources: ["lib.cpp"]
        headers: ["lib.h"]
        includes: ["include/"]
    }
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["main.cpp"]
        depends: ["mylib"]
    }
}
variants: {
    debug: {
        name: "debug"
        optimization: "none"
        debug_info: true
        defines: ["DEBUG"]
    }
    release: {
        name: "release"
        optimization: "fast"
        debug_info: false
        defines: ["NDEBUG"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("loader should accept valid config: %v", err)
	}

	// Verify extracted data
	if cfg.Name != "myproject" {
		t.Errorf("expected name 'myproject', got %q", cfg.Name)
	}
	if cfg.Toolchain.Compiler != "clang++" {
		t.Errorf("expected compiler 'clang++', got %q", cfg.Toolchain.Compiler)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(cfg.Targets))
	}
	if len(cfg.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(cfg.Variants))
	}

	// Verify target details
	myapp := cfg.Targets["myapp"]
	if myapp.Type != "executable" {
		t.Errorf("expected myapp type 'executable', got %q", myapp.Type)
	}
	if len(myapp.Depends) != 1 || myapp.Depends[0] != "mylib" {
		t.Errorf("expected myapp to depend on mylib, got %v", myapp.Depends)
	}
}

func TestLoaderErrorMessages(t *testing.T) {
	dir := t.TempDir()
	config := `package config
name: "test"
targets: {
    app: {
        name: "app"
        type: "invalid"
        sources: ["main.cpp"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	// Error should include file location
	if !containsIgnoreCase(errStr, "clue.cue") {
		t.Logf("Warning: error may not include file path: %v", err)
	}
	// Error should mention the constraint
	if !containsIgnoreCase(errStr, "type") {
		t.Logf("Warning: error may not mention field: %v", err)
	}
}
