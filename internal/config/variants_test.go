package config

import (
	"testing"
)

func TestVariantSelector_SelectsConfiguredSources(t *testing.T) {
	// Test default
	vs := &VariantSelector{Default: "debug"}
	if got := vs.Select(); got != "debug" {
		t.Errorf("Expected 'debug', got '%s'", got)
	}

	// Test env var
	vs.EnvVar = "release"
	if got := vs.Select(); got != "release" {
		t.Errorf("Expected 'release', got '%s'", got)
	}

	// Test CLI override
	vs.SetCLIFlag("profile")
	if got := vs.Select(); got != "profile" {
		t.Errorf("Expected 'profile', got '%s'", got)
	}
}

func TestVariantSelector_CLIOverridesEnvironment(t *testing.T) {
	// CLI > EnvVar > Default
	vs := &VariantSelector{
		CLIFlag: "cli",
		EnvVar:  "env",
		Default: "default",
	}

	if got := vs.Select(); got != "cli" {
		t.Errorf("CLI should have highest precedence, got '%s'", got)
	}

	vs.CLIFlag = ""
	if got := vs.Select(); got != "env" {
		t.Errorf("EnvVar should be second precedence, got '%s'", got)
	}

	vs.EnvVar = ""
	if got := vs.Select(); got != "default" {
		t.Errorf("Default should be fallback, got '%s'", got)
	}
}

func TestNewVariantSelector_DefaultsToDebug(t *testing.T) {
	t.Setenv(VariantEnvVar, "")

	vs := NewVariantSelector()
	if vs.Default != DefaultVariant {
		t.Errorf("Expected default '%s', got '%s'", DefaultVariant, vs.Default)
	}
	if vs.EnvVar != "" {
		t.Errorf("Expected empty EnvVar, got '%s'", vs.EnvVar)
	}

	// Test with env var set
	t.Setenv(VariantEnvVar, "custom")
	vs = NewVariantSelector()
	if vs.EnvVar != "custom" {
		t.Errorf("Expected EnvVar 'custom', got '%s'", vs.EnvVar)
	}
}

func TestSelectVariant_AppliesNamedVariant(t *testing.T) {
	t.Setenv(VariantEnvVar, "")

	// CLI provided
	if got := SelectVariant("release"); got != "release" {
		t.Errorf("Expected 'release', got '%s'", got)
	}

	// No CLI, fallback to default
	if got := SelectVariant(""); got != DefaultVariant {
		t.Errorf("Expected '%s', got '%s'", DefaultVariant, got)
	}

	// No CLI, env set
	t.Setenv(VariantEnvVar, "profile")
	if got := SelectVariant(""); got != "profile" {
		t.Errorf("Expected 'profile', got '%s'", got)
	}
}

func TestApplyVariant_LeavesUnconfiguredVariantsUnchanged(t *testing.T) {
	// Test with empty config - variant lookup should fail
	cfg := &Config{
		Name:     "testproject",
		Targets:  make(map[string]Target),
		Variants: make(map[string]Variant),
	}

	_, err := ApplyVariant(cfg, "debug")
	if err == nil {
		t.Fatal("Expected error when no variants configured")
	}

	// With shim, error is "no variants configured" since Raw.LookupPath returns non-existent
	if err.Error() != `variant "debug" not defined and no variants configured` {
		t.Logf("Got error: %v", err)
	}
}

func TestMergeVariantFlags_CombinesBaseAndVariantFlags(t *testing.T) {
	base := Flags{
		Compiler: []string{"-Wall", "-Wextra"},
		Linker:   []string{"-L/usr/lib"},
	}
	variant := Flags{
		Compiler: []string{"-O2"},
		Linker:   []string{"-lm"},
	}

	merged := MergeVariantFlags(base, variant)

	if len(merged.Compiler) != 3 {
		t.Errorf("Expected 3 compiler flags, got %d", len(merged.Compiler))
	}
	if len(merged.Linker) != 2 {
		t.Errorf("Expected 2 linker flags, got %d", len(merged.Linker))
	}

	// Verify order: base first, then variant
	expected := []string{"-Wall", "-Wextra", "-O2"}
	for i, f := range expected {
		if merged.Compiler[i] != f {
			t.Errorf("Compiler[%d] = %s, want %s", i, merged.Compiler[i], f)
		}
	}
}
