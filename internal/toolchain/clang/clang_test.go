package clang

import (
	"bytes"
	"os"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestClangToolchain_Name(t *testing.T) {
	tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
	if got := tc.Name(); got != "clang" {
		t.Errorf("Name() = %q, want %q", got, "clang")
	}
}

func TestClangToolchain_Sanitizers(t *testing.T) {
	t.Run("all sanitizers including memory are included", func(t *testing.T) {
		tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
		config := toolchain.Config{
			Sanitizers: []string{"address", "memory", "undefined", "thread"},
		}
		flags := tc.CompilerFlags(config)

		expected := []string{
			"-fsanitize=address",
			"-fsanitize=memory",
			"-fsanitize=undefined",
			"-fsanitize=thread",
		}
		for _, want := range expected {
			if !containsFlag(flags, want) {
				t.Errorf("CompilerFlags() = %v, missing %q", flags, want)
			}
		}
	})

	t.Run("no warning for memory sanitizer", func(t *testing.T) {
		// Capture stderr to verify no warning
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
		config := toolchain.Config{
			Sanitizers: []string{"memory"},
		}
		tc.CompilerFlags(config)

		// Restore stderr
		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		buf.ReadFrom(r)
		stderr := buf.String()

		// Verify NO warning was printed
		if containsSubstring(stderr, "MemorySanitizer") {
			t.Errorf("Clang should not warn about MemorySanitizer, got: %q", stderr)
		}
	})
}

func TestClangToolchain_Coverage(t *testing.T) {
	t.Run("coverage flags in compiler flags", func(t *testing.T) {
		tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
		config := toolchain.Config{Coverage: true}
		flags := tc.CompilerFlags(config)

		if !containsFlag(flags, "-fprofile-instr-generate") {
			t.Errorf("CompilerFlags() = %v, missing -fprofile-instr-generate", flags)
		}
		if !containsFlag(flags, "-fcoverage-mapping") {
			t.Errorf("CompilerFlags() = %v, missing -fcoverage-mapping", flags)
		}
	})

	t.Run("coverage flags in linker flags", func(t *testing.T) {
		tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
		config := toolchain.Config{Coverage: true}
		flags := tc.LinkerFlags(config, nil)

		// Clang needs -fprofile-instr-generate at link time
		if !containsFlag(flags, "-fprofile-instr-generate") {
			t.Errorf("LinkerFlags() = %v, missing -fprofile-instr-generate", flags)
		}
	})
}

func TestClangToolchain_LinkerSanitizers(t *testing.T) {
	tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
	config := toolchain.Config{
		Sanitizers: []string{"address", "memory"},
	}
	flags := tc.LinkerFlags(config, nil)

	// Both address and memory sanitizers should be in linker flags
	if !containsFlag(flags, "-fsanitize=address") {
		t.Errorf("LinkerFlags() = %v, missing -fsanitize=address", flags)
	}
	if !containsFlag(flags, "-fsanitize=memory") {
		t.Errorf("LinkerFlags() = %v, missing -fsanitize=memory", flags)
	}
}

func TestClangToolchain_InheritedBehavior(t *testing.T) {
	tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})

	// Verify accessors work via embedding
	if got := tc.CC(); got != "clang" {
		t.Errorf("CC() = %q, want %q", got, "clang")
	}
	if got := tc.CXX(); got != "clang++" {
		t.Errorf("CXX() = %q, want %q", got, "clang++")
	}
	if got := tc.AR(); got != "llvm-ar" {
		t.Errorf("AR() = %q, want %q", got, "llvm-ar")
	}

	// Verify String() returns expected format
	if got := tc.String(); got != "clang (native)" {
		t.Errorf("String() = %q, want %q", got, "clang (native)")
	}
}

func TestClangToolchain_CrossCompiler(t *testing.T) {
	tc := New("aarch64-linux-gnu-clang", "aarch64-linux-gnu-clang++", "aarch64-linux-gnu-llvm-ar", toolchain.Platform{})

	if !tc.IsCrossCompiler() {
		t.Error("IsCrossCompiler() = false, want true for cross compiler")
	}

	if got := tc.String(); got != "aarch64-linux-gnu-clang (cross)" {
		t.Errorf("String() = %q, want %q", got, "aarch64-linux-gnu-clang (cross)")
	}
}

func TestClangToolchain_BaseFlags(t *testing.T) {
	tc := New("clang", "clang++", "llvm-ar", toolchain.Platform{})
	config := toolchain.Config{
		Optimize: "fast",
		Warnings: "strict",
		Debug:    "full",
		LTO:      true,
		PIC:      true,
	}
	flags := tc.CompilerFlags(config)

	// Verify base flags from gccish are included
	expected := []string{"-O2", "-Wall", "-Wextra", "-g", "-flto", "-fPIC"}
	for _, want := range expected {
		if !containsFlag(flags, want) {
			t.Errorf("CompilerFlags() = %v, missing %q", flags, want)
		}
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
