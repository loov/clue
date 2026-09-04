package gcc

import (
	"io"
	"os"
	"slices"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	run()
	closeErr := w.Close()
	os.Stderr = oldStderr
	output, readErr := io.ReadAll(r)
	rCloseErr := r.Close()
	for _, err := range []error{closeErr, readErr, rCloseErr} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return string(output)
}

func TestGCCToolchain_NameReportsGCC(t *testing.T) {
	tc := New("gcc", "g++", "ar", toolchain.Platform{})
	if got := tc.Name(); got != "gcc" {
		t.Errorf("Name() = %q, want %q", got, "gcc")
	}
}

func TestGCCToolchain_CompilerFlagsMapSanitizers(t *testing.T) {
	t.Run("memory sanitizer is skipped with warning", func(t *testing.T) {
		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Flags{
			Sanitizers: []string{"address", "memory", "undefined"},
		}
		var flags []string
		stderr := captureStderr(t, func() { flags = tc.CompilerFlags(config) })

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
		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Flags{
			Sanitizers: []string{"address", "thread", "undefined"},
		}
		var flags []string
		captureStderr(t, func() { flags = tc.CompilerFlags(config) })

		expected := []string{"-fsanitize=address", "-fsanitize=thread", "-fsanitize=undefined"}
		for _, want := range expected {
			if !containsFlag(flags, want) {
				t.Errorf("CompilerFlags() = %v, missing %q", flags, want)
			}
		}
	})
}

func TestGCCToolchain_CompilerFlagsEnableCoverage(t *testing.T) {
	t.Run("coverage flags in compiler flags", func(t *testing.T) {
		tc := New("gcc", "g++", "ar", toolchain.Platform{})
		config := toolchain.Flags{Coverage: true}
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
		config := toolchain.Flags{Coverage: true}
		flags := tc.LinkerFlags(config, nil)

		// GCC links coverage automatically via -lgcov, so no explicit coverage flags
		for _, f := range flags {
			if f == "-fprofile-arcs" || f == "-ftest-coverage" {
				t.Errorf("LinkerFlags() = %v, should not include explicit coverage flags", flags)
			}
		}
	})
}

func TestGCCToolchain_LinkerFlagsIncludeSanitizerRuntime(t *testing.T) {
	tc := New("gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{
		Sanitizers: []string{"address", "memory"},
	}
	var flags []string
	captureStderr(t, func() { flags = tc.LinkerFlags(config, nil) })

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

func TestGCCToolchain_InheritsGCCStyleFlags(t *testing.T) {
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

func TestGCCToolchain_DetectsTargetPrefix(t *testing.T) {
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
	return slices.Contains(flags, flag)
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
