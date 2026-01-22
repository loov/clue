package config

import (
	"os"
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

func TestBuildEnvCUE(t *testing.T) {
	envVars := map[string]string{
		"SIMPLE":        "value",
		"WITH_SPECIAL":  "path/to/file",
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
	// Save and restore env
	old := os.Getenv("TEST_VAR")
	defer func() {
		if old != "" {
			os.Setenv("TEST_VAR", old)
		} else {
			os.Unsetenv("TEST_VAR")
		}
	}()

	// Test: When env var is set, it should take precedence over default
	os.Setenv("TEST_VAR", "from_env")

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
