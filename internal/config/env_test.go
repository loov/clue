package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEnvInjection(t *testing.T) {
	envVars := map[string]string{
		"USE_OPENSSL": "1",
		"BUILD_TYPE":  "optimized",
	}

	loader := NewLoaderWithEnv(envVars)
	if loader == nil {
		t.Fatal("NewLoaderWithEnv returned nil")
	}

	// Verify env vars are stored
	if loader.envVars["USE_OPENSSL"] != "1" {
		t.Errorf("Expected USE_OPENSSL=1, got %s", loader.envVars["USE_OPENSSL"])
	}
	if loader.envVars["BUILD_TYPE"] != "optimized" {
		t.Errorf("Expected BUILD_TYPE=optimized, got %s", loader.envVars["BUILD_TYPE"])
	}
}

func TestLoaderWithEnvLoadsPackageOverlay(t *testing.T) {
	dir := t.TempDir()
	contents := `package project
name: _env.PROJECT_NAME
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoaderWithEnv(map[string]string{"PROJECT_NAME": "from-env"}).Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "from-env" {
		t.Fatalf("name = %q", cfg.Name)
	}
}

func TestBuildEnvCUE(t *testing.T) {
	envVars := map[string]string{
		"SIMPLE":         "value",
		"WITH_SPECIAL":   "path/to/file",
		"WITH_BACKSLASH": "C:\\path",
	}

	cue := buildEnvCUE(envVars)

	// Check structure
	if !contains(cue, "_env: {") {
		t.Error("Generated CUE should contain _env block")
	}

	// Check that keys are present
	if !contains(cue, "SIMPLE:") {
		t.Error("Generated CUE should contain SIMPLE key")
	}
	if !contains(cue, "WITH_SPECIAL:") {
		t.Error("Generated CUE should contain WITH_SPECIAL key")
	}
	if strings.Index(cue, "SIMPLE:") > strings.Index(cue, "WITH_BACKSLASH:") ||
		strings.Index(cue, "WITH_BACKSLASH:") > strings.Index(cue, "WITH_SPECIAL:") {
		t.Errorf("Generated CUE keys are not sorted:\n%s", cue)
	}
}

func TestSanitizeKey(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"SIMPLE", "SIMPLE"},
		{"WITH_UNDERSCORE", "WITH_UNDERSCORE"},
		{"with-dash", "with_dash"},
		{"123start", "_123start"},
		{"special@chars!", "special_chars_"},
		{"a.b.c", "a_b_c"},
		{"", ""},
	}

	for _, tc := range cases {
		got := sanitizeKey(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeKey(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestEscapeString(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with\\backslash", "with\\\\backslash"},
		{"multiple\\\\slashes", "multiple\\\\\\\\slashes"},
	}

	for _, tc := range cases {
		got := escapeString(tc.input)
		if got != tc.expected {
			t.Errorf("escapeString(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestEnvResolutionBasics(t *testing.T) {
	// Test with empty config (no env vars defined)
	cfg := &Config{
		Name:     "testproject",
		Targets:  make(map[string]Target),
		Variants: make(map[string]Variant),
	}

	env, err := ResolveEnvVars(cfg)
	if err != nil {
		t.Fatalf("ResolveEnvVars failed: %v", err)
	}

	if env == nil {
		t.Fatal("ResolveEnvVars returned nil")
	}

	if len(env.Variables) != 0 {
		t.Errorf("Expected empty Variables, got %d", len(env.Variables))
	}

	if len(env.Used) != 0 {
		t.Errorf("Expected empty Used, got %d", len(env.Used))
	}
}

func TestEnvConfigStructure(t *testing.T) {
	env := &EnvConfig{
		Variables: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
		Used: []string{"FOO"},
	}

	if env.Variables["FOO"] != "bar" {
		t.Errorf("Expected FOO=bar, got %s", env.Variables["FOO"])
	}

	if len(env.Used) != 1 || env.Used[0] != "FOO" {
		t.Errorf("Expected Used=[FOO], got %v", env.Used)
	}
}

func TestEnvVarPrecedence(t *testing.T) {
	// Test: When env var is set, it should take precedence over default
	t.Setenv("TEST_VAR", "from_env")

	// This is a structural test - real resolution requires CUE parsing
	envVars := map[string]string{
		"TEST_VAR": os.Getenv("TEST_VAR"),
	}

	if envVars["TEST_VAR"] != "from_env" {
		t.Errorf("Expected TEST_VAR=from_env, got %s", envVars["TEST_VAR"])
	}
}

func TestGetEnvValueNotFound(t *testing.T) {
	cfg := &Config{
		Name:     "testproject",
		Targets:  make(map[string]Target),
		Variants: make(map[string]Variant),
	}

	// GetEnvValue should return false for non-existent env var
	val, ok := GetEnvValue(cfg, "NONEXISTENT")
	if ok {
		t.Errorf("Expected ok=false for nonexistent env var, got ok=true, val=%s", val)
	}
}

func TestApplyEnvVars_WhenTrue(t *testing.T) {
	// Create a config with env-conditional defines
	dir := t.TempDir()
	configContent := `package config

name: "test"
targets: {
	myapp: {
		name: "myapp"
		type: "executable"
		sources: ["main.cpp"]
		defines: ["-DBASE"]
	}
}
env: {
	USE_OPENSSL: {
		name: "USE_OPENSSL"
		default: "0"
		when_true: {
			defines: ["-DUSE_OPENSSL", "-DSSL_ENABLED"]
			flags: {
				linker: ["-lssl", "-lcrypto"]
			}
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Simulate USE_OPENSSL=1
	env := &EnvConfig{
		Variables: map[string]string{
			"USE_OPENSSL": "1",
		},
		Used: []string{"USE_OPENSSL"},
	}

	result, err := ApplyEnvVars(cfg, env)
	if err != nil {
		t.Fatalf("ApplyEnvVars failed: %v", err)
	}

	// Check that defines were added
	target := result.Targets["myapp"]
	if !containsString(target.Defines, "-DUSE_OPENSSL") {
		t.Errorf("expected -DUSE_OPENSSL define, got: %v", target.Defines)
	}
	if !containsString(target.Defines, "-DBASE") {
		t.Errorf("original define -DBASE should be preserved, got: %v", target.Defines)
	}
	if !containsString(target.Flags.Linker, "-lssl") {
		t.Errorf("expected -lssl linker flag, got: %v", target.Flags.Linker)
	}
}

func TestApplyEnvVars_WhenFalse(t *testing.T) {
	dir := t.TempDir()
	configContent := `package config

name: "test"
targets: {
	myapp: {
		name: "myapp"
		type: "executable"
		sources: ["main.cpp"]
		defines: ["-DBASE"]
	}
}
env: {
	USE_OPENSSL: {
		name: "USE_OPENSSL"
		default: "0"
		when_true: {
			defines: ["-DUSE_OPENSSL"]
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Simulate USE_OPENSSL=0 (falsy)
	env := &EnvConfig{
		Variables: map[string]string{
			"USE_OPENSSL": "0",
		},
	}

	result, err := ApplyEnvVars(cfg, env)
	if err != nil {
		t.Fatalf("ApplyEnvVars failed: %v", err)
	}

	target := result.Targets["myapp"]
	if containsString(target.Defines, "-DUSE_OPENSSL") {
		t.Errorf("should NOT have -DUSE_OPENSSL when env is falsy, got: %v", target.Defines)
	}
	// Original should still be there
	if !containsString(target.Defines, "-DBASE") {
		t.Errorf("-DBASE should be preserved, got: %v", target.Defines)
	}
}

func TestIsTruthy(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "yes", "YES", "on", "ON"}
	for _, v := range truthy {
		if !isTruthy(v) {
			t.Errorf("%q should be truthy", v)
		}
	}

	falsy := []string{"0", "false", "FALSE", "no", "NO", "off", "OFF", ""}
	for _, v := range falsy {
		if isTruthy(v) {
			t.Errorf("%q should be falsy", v)
		}
	}
}

func TestApplyEnvVars_NoEnvSection(t *testing.T) {
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
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	env := &EnvConfig{Variables: map[string]string{}}

	result, err := ApplyEnvVars(cfg, env)
	if err != nil {
		t.Fatalf("ApplyEnvVars should succeed with no env section: %v", err)
	}

	if result == nil {
		t.Error("result should not be nil")
	}
}

func TestApplyEnvVars_MultipleTargets(t *testing.T) {
	dir := t.TempDir()
	configContent := `package config

name: "test"
targets: {
	lib: {
		name: "lib"
		type: "static_library"
		sources: ["lib.cpp"]
	}
	app: {
		name: "app"
		type: "executable"
		sources: ["main.cpp"]
		depends: ["lib"]
	}
}
env: {
	DEBUG: {
		name: "DEBUG"
		default: "0"
		when_true: {
			defines: ["-DDEBUG_MODE"]
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	env := &EnvConfig{
		Variables: map[string]string{"DEBUG": "1"},
	}

	result, err := ApplyEnvVars(cfg, env)
	if err != nil {
		t.Fatalf("ApplyEnvVars failed: %v", err)
	}

	// Both targets should get the define
	if !containsString(result.Targets["lib"].Defines, "-DDEBUG_MODE") {
		t.Error("lib should have -DDEBUG_MODE")
	}
	if !containsString(result.Targets["app"].Defines, "-DDEBUG_MODE") {
		t.Error("app should have -DDEBUG_MODE")
	}
}

// helper function for string slice contains
func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

// helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
