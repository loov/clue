package config

import (
	"strings"
	"testing"
)

func TestSchemaEmbedded(t *testing.T) {
	if Schema == "" {
		t.Fatal("Schema should not be empty")
	}

	// Verify key type definitions are present
	expectedTypes := []string{
		"#Target:",
		"#Variant:",
		"#EnvVar:",
		"#Config:",
	}

	for _, typeDef := range expectedTypes {
		if !strings.Contains(Schema, typeDef) {
			t.Errorf("Schema should contain %q", typeDef)
		}
	}
}

func TestSchemaConstraints(t *testing.T) {
	// Verify key constraints are present
	constraints := []string{
		`"executable"`,            // Target type enum
		`"static_library"`,        // Target type enum
		`"shared_library"`,        // Target type enum
		`"O0"`,                    // Optimization level
		`"O2"`,                    // Optimization level
		`=~"^[a-zA-Z]`,           // Name regex constraint
		`[_, ...]`,               // At least one source
	}

	for _, constraint := range constraints {
		if !strings.Contains(Schema, constraint) {
			t.Errorf("Schema should contain constraint %q", constraint)
		}
	}
}
