package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDepFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("parses simple .d file", func(t *testing.T) {
		depFile := filepath.Join(tmpDir, "simple.d")
		content := `main.o: src/main.cpp include/config.h include/utils.h
`
		if err := os.WriteFile(depFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		deps, err := ParseDepFile(depFile)
		if err != nil {
			t.Fatalf("ParseDepFile failed: %v", err)
		}

		if deps.Target != "main.o" {
			t.Errorf("target: got %q, want %q", deps.Target, "main.o")
		}

		expectedSources := []string{"src/main.cpp", "include/config.h", "include/utils.h"}
		if len(deps.Sources) != len(expectedSources) {
			t.Fatalf("sources length: got %d, want %d", len(deps.Sources), len(expectedSources))
		}

		for i, expected := range expectedSources {
			if deps.Sources[i] != expected {
				t.Errorf("source[%d]: got %q, want %q", i, deps.Sources[i], expected)
			}
		}
	})

	t.Run("handles continuation lines", func(t *testing.T) {
		depFile := filepath.Join(tmpDir, "continuation.d")
		content := `main.o: src/main.cpp \
  include/config.h \
  include/utils.h
`
		if err := os.WriteFile(depFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		deps, err := ParseDepFile(depFile)
		if err != nil {
			t.Fatalf("ParseDepFile failed: %v", err)
		}

		if deps.Target != "main.o" {
			t.Errorf("target: got %q, want %q", deps.Target, "main.o")
		}

		expectedSources := []string{"src/main.cpp", "include/config.h", "include/utils.h"}
		if len(deps.Sources) != len(expectedSources) {
			t.Fatalf("sources length: got %d, want %d", len(deps.Sources), len(expectedSources))
		}

		for i, expected := range expectedSources {
			if deps.Sources[i] != expected {
				t.Errorf("source[%d]: got %q, want %q", i, deps.Sources[i], expected)
			}
		}
	})

	t.Run("ignores phony targets from -MP", func(t *testing.T) {
		depFile := filepath.Join(tmpDir, "phony.d")
		content := `main.o: src/main.cpp \
  include/config.h \
  include/utils.h

include/config.h:

include/utils.h:
`
		if err := os.WriteFile(depFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		deps, err := ParseDepFile(depFile)
		if err != nil {
			t.Fatalf("ParseDepFile failed: %v", err)
		}

		if deps.Target != "main.o" {
			t.Errorf("target: got %q, want %q", deps.Target, "main.o")
		}

		// Should only have the actual dependencies, not the phony targets
		expectedSources := []string{"src/main.cpp", "include/config.h", "include/utils.h"}
		if len(deps.Sources) != len(expectedSources) {
			t.Fatalf("sources length: got %d, want %d", len(deps.Sources), len(expectedSources))
		}

		for i, expected := range expectedSources {
			if deps.Sources[i] != expected {
				t.Errorf("source[%d]: got %q, want %q", i, deps.Sources[i], expected)
			}
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		_, err := ParseDepFile(filepath.Join(tmpDir, "nonexistent.d"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("returns error for malformed content", func(t *testing.T) {
		depFile := filepath.Join(tmpDir, "malformed.d")
		content := `this is not a valid dependency file format
it has no colon
`
		if err := os.WriteFile(depFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := ParseDepFile(depFile)
		if err == nil {
			t.Error("expected error for malformed content")
		}
	})

	t.Run("handles multiple dependencies with spaces", func(t *testing.T) {
		depFile := filepath.Join(tmpDir, "spaces.d")
		content := `output.o:   src/file.cpp    include/header.h     lib/other.h
`
		if err := os.WriteFile(depFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		deps, err := ParseDepFile(depFile)
		if err != nil {
			t.Fatalf("ParseDepFile failed: %v", err)
		}

		expectedSources := []string{"src/file.cpp", "include/header.h", "lib/other.h"}
		if len(deps.Sources) != len(expectedSources) {
			t.Fatalf("sources length: got %d, want %d", len(deps.Sources), len(expectedSources))
		}

		for i, expected := range expectedSources {
			if deps.Sources[i] != expected {
				t.Errorf("source[%d]: got %q, want %q", i, deps.Sources[i], expected)
			}
		}
	})

	t.Run("handles empty continuation lines", func(t *testing.T) {
		depFile := filepath.Join(tmpDir, "empty_continuation.d")
		content := `main.o: src/main.cpp \
  \
  include/config.h
`
		if err := os.WriteFile(depFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		deps, err := ParseDepFile(depFile)
		if err != nil {
			t.Fatalf("ParseDepFile failed: %v", err)
		}

		expectedSources := []string{"src/main.cpp", "include/config.h"}
		if len(deps.Sources) != len(expectedSources) {
			t.Fatalf("sources length: got %d, want %d", len(deps.Sources), len(expectedSources))
		}
	})
}
