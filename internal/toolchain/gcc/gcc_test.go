package gcc

import (
	"bytes"
	"os"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestGCCToolchain_Name(t *testing.T) {
	tc := New("gcc", "g++", "ar", toolchain.Platform{})
	if got := tc.Name(); got != "gcc" {
		t.Errorf("Name() = %q, want %q", got, "gcc")
	}
}

func TestGCCToolchain_Sanitizers(t *testing.T) {
	t.Run("memory sanitizer is skipped with warning", func(t *testing.T) {
		// Capture stderr to verify warning
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Config{
			Sanitizers: []string{"address", "memory", "undefined"},
		}
		flags := tc.CompilerFlags(config)

		// Restore stderr
		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		buf.ReadFrom(r)
		stderr := buf.String()

		// Verify memory sanitizer is NOT in flags
		for _, f := range flags {
			if f == "-fsanitize=memory" {
				t.Error("CompilerFlags() should not include -fsanitize=memory for GCC")
			}
		}

		// Verify warning was printed
		if !containsSubstring(stderr, "MemorySanitizer not available") {
			t.Errorf("expected warning about MemorySanitizer, got: %q", stderr)
		}
	})

	t.Run("address, thread, undefined sanitizers are included", func(t *testing.T) {
		// Suppress stderr for this test
		oldStderr := os.Stderr
		_, w, _ := os.Pipe()
		os.Stderr = w

		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Config{
			Sanitizers: []string{"address", "thread", "undefined"},
		}
		flags := tc.CompilerFlags(config)

		w.Close()
		os.Stderr = oldStderr

		expected := []string{"-fsanitize=address", "-fsanitize=thread", "-fsanitize=undefined"}
		for _, want := range expected {
			if !containsFlag(flags, want) {
				t.Errorf("CompilerFlags() = %v, missing %q", flags, want)
			}
		}
	})
}

func TestGCCToolchain_Coverage(t *testing.T) {
	t.Run("coverage flags in compiler flags", func(t *testing.T) {
		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Config{Coverage: true}
		flags := tc.CompilerFlags(config)

		if !containsFlag(flags, "-fprofile-arcs") {
			t.Errorf("CompilerFlags() = %v, missing -fprofile-arcs", flags)
		}
		if !containsFlag(flags, "-ftest-coverage") {
			t.Errorf("CompilerFlags() = %v, missing -ftest-coverage", flags)
		}
	})

	t.Run("no extra linker flags for coverage", func(t *testing.T) {
		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Config{Coverage: true}
		flags := tc.LinkerFlags(config, nil)

		// GCC links coverage automatically via -lgcov, so no explicit coverage flags
		for _, f := range flags {
			if f == "-fprofile-arcs" || f == "-ftest-coverage" {
				t.Errorf("LinkerFlags() = %v, should not include explicit coverage flags", flags)
			}
		}
	})
}

func TestGCCToolchain_LinkerSanitizers(t *testing.T) {
	// Suppress stderr for this test
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	tc := New("gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Config{
		Sanitizers: []string{"address", "memory"},
	}
	flags := tc.LinkerFlags(config, nil)

	w.Close()
	os.Stderr = oldStderr

	// Address sanitizer should be in linker flags
	if !containsFlag(flags, "-fsanitize=address") {
		t.Errorf("LinkerFlags() = %v, missing -fsanitize=address", flags)
	}

	// Memory sanitizer should NOT be in linker flags
	for _, f := range flags {
		if f == "-fsanitize=memory" {
			t.Error("LinkerFlags() should not include -fsanitize=memory for GCC")
		}
	}
}

func TestGCCToolchain_InheritedBehavior(t *testing.T) {
	tc := New("gcc", "g++", "ar", toolchain.Platform{})

	// Verify accessors work via embedding
	if got := tc.CC(); got != "gcc" {
		t.Errorf("CC() = %q, want %q", got, "gcc")
	}
	if got := tc.CXX(); got != "g++" {
		t.Errorf("CXX() = %q, want %q", got, "g++")
	}
	if got := tc.AR(); got != "ar" {
		t.Errorf("AR() = %q, want %q", got, "ar")
	}

	// Verify String() returns expected format
	if got := tc.String(); got != "gcc (native)" {
		t.Errorf("String() = %q, want %q", got, "gcc (native)")
	}
}

func TestGCCToolchain_CrossCompiler(t *testing.T) {
	tc := New("aarch64-linux-gnu-gcc", "aarch64-linux-gnu-g++", "aarch64-linux-gnu-ar", toolchain.Platform{})

	if !tc.IsCrossCompiler() {
		t.Error("IsCrossCompiler() = false, want true for cross compiler")
	}

	if got := tc.String(); got != "aarch64-linux-gnu-gcc (cross)" {
		t.Errorf("String() = %q, want %q", got, "aarch64-linux-gnu-gcc (cross)")
	}
}

// containsFlag checks if flags contains the given flag.
func containsFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
