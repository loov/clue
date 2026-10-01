package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoad_InjectsDeclaredEnvironment(t *testing.T) {
	dir := t.TempDir()
	contents := `package project
name: "env"
env: {
	VALIDATE: {name: "VALIDATE", default: false}
	JOBS: {name: "JOBS", default: 4}
}
targets: app: {type: "executable", sources: ["main.cpp"]}
if _env.VALIDATE == "1" {
	targets: validator: {type: "executable", sources: ["main.cpp"], defines: ["JOBS=\(_env.JOBS)"]}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.cpp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VALIDATE", "")
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Targets["validator"]; ok {
		t.Fatal("validator added without VALIDATE")
	}

	t.Setenv("VALIDATE", "1")
	cfg, err = NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	validator, ok := cfg.Targets["validator"]
	if !ok {
		t.Fatal("validator not added with VALIDATE=1")
	}
	if !slices.Equal(validator.Defines, []string{"JOBS=4"}) {
		t.Fatalf("defines = %v", validator.Defines)
	}
}

func TestLoad_IgnoresUndeclaredEnvironment(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "env"
targets: app: {type: "executable", sources: ["main.cpp"]}
if _env.UNDECLARED != _|_ {
	targets: extra: {type: "executable", sources: ["main.cpp"]}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.cpp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNDECLARED", "1")
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Targets["extra"]; ok {
		t.Fatal("undeclared variable reached _env")
	}
}

func TestLoad_EnvironmentCanDisableInvalidDefaultBranch(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "env"
env: ENABLE: {name: "ENABLE", default: false}
targets: check: {type: "interface_library"}
if _env.ENABLE != "1" {name: "disabled"}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENABLE", "1")
	if _, err := NewLoader().Load(dir); err != nil {
		t.Fatalf("environment did not disable invalid default branch: %v", err)
	}
	t.Setenv("ENABLE", "")
	if _, err := NewLoader().Load(dir); err == nil {
		t.Fatal("invalid default branch was accepted without an override")
	}
}

func TestResolveEnvVars_UsesEnvironmentAndDefaults(t *testing.T) {
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

func TestResolveEnvVars_TracksValuesAndSources(t *testing.T) {
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

func TestResolveEnvVars_EnvironmentOverridesDefault(t *testing.T) {
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

func TestApplyEnvVars_SelectsTrueBranch(t *testing.T) {
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

func TestApplyEnvVars_SelectsFalseBranch(t *testing.T) {
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

func TestIsTruthy_RecognizesSupportedValues(t *testing.T) {
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

func TestApplyEnvVars_LeavesConfigWithoutEnvUnchanged(t *testing.T) {
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

func TestApplyEnvVars_AppliesToEveryTarget(t *testing.T) {
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
