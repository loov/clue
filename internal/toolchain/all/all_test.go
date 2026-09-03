package all

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestNewToolchain_CreatesGCCCompiler(t *testing.T) {
	host := toolchain.HostPlatform()

	tc, err := NewToolchain("gcc", host)
	if err != nil {
		t.Fatalf("NewToolchain(gcc) failed: %v", err)
	}

	if tc.CC() != "gcc" {
		t.Errorf("CC() = %q, want %q", tc.CC(), "gcc")
	}
	if tc.CXX() != "g++" {
		t.Errorf("CXX() = %q, want %q", tc.CXX(), "g++")
	}
	if tc.AR() != "ar" {
		t.Errorf("AR() = %q, want %q", tc.AR(), "ar")
	}
	if tc.Name() != "gcc" {
		t.Errorf("Name() = %q, want %q", tc.Name(), "gcc")
	}
}

func TestNewToolchain_CreatesClangCompiler(t *testing.T) {
	host := toolchain.HostPlatform()

	tc, err := NewToolchain("clang", host)
	if err != nil {
		t.Fatalf("NewToolchain(clang) failed: %v", err)
	}

	if tc.CC() != "clang" {
		t.Errorf("CC() = %q, want %q", tc.CC(), "clang")
	}
	if tc.CXX() != "clang++" {
		t.Errorf("CXX() = %q, want %q", tc.CXX(), "clang++")
	}
	wantAR := "ar"
	if host.OS == "windows" {
		wantAR = "llvm-ar"
	}
	if tc.AR() != wantAR {
		t.Errorf("AR() = %q, want %q", tc.AR(), wantAR)
	}
	if tc.Name() != "clang" {
		t.Errorf("Name() = %q, want %q", tc.Name(), "clang")
	}
}

func TestNewConfiguredToolchain_ClangWindows_UsesLLVMArchiver(t *testing.T) {
	target := toolchain.Platform{OS: "windows", Arch: "amd64"}
	tc, err := NewConfiguredToolchain("clang", target, Config{
		CC: "clang", CXX: "clang++", TargetTriple: "x86_64-pc-windows-msvc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tc.AR() != "llvm-ar" {
		t.Fatalf("AR() = %q, want %q", tc.AR(), "llvm-ar")
	}
}

func TestNewToolchain_RejectsUnknownCompiler(t *testing.T) {
	host := toolchain.HostPlatform()

	_, err := NewToolchain("unknown-compiler", host)
	if err == nil {
		t.Fatal("NewToolchain(unknown) should fail")
	}

	want := "unknown toolchain: unknown-compiler (supported: gcc, clang, msvc)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestNewToolchain_UsesEnvironmentOverrides(t *testing.T) {
	host := toolchain.HostPlatform()

	// Test CC override
	t.Run("CC override", func(t *testing.T) {
		t.Setenv("CC", "/custom/path/gcc")

		tc, err := NewToolchain("clang", host)
		if err != nil {
			t.Fatalf("NewToolchain failed: %v", err)
		}

		if tc.CC() != "/custom/path/gcc" {
			t.Errorf("CC() = %q, want %q", tc.CC(), "/custom/path/gcc")
		}
		// CXX should not be overridden
		if tc.CXX() != "clang++" {
			t.Errorf("CXX() = %q, want %q", tc.CXX(), "clang++")
		}
	})

	// Test CXX override
	t.Run("CXX override", func(t *testing.T) {
		t.Setenv("CXX", "/custom/path/g++")

		tc, err := NewToolchain("gcc", host)
		if err != nil {
			t.Fatalf("NewToolchain failed: %v", err)
		}

		if tc.CXX() != "/custom/path/g++" {
			t.Errorf("CXX() = %q, want %q", tc.CXX(), "/custom/path/g++")
		}
		// CC should not be overridden
		if tc.CC() != "gcc" {
			t.Errorf("CC() = %q, want %q", tc.CC(), "gcc")
		}
	})
}

func TestCrossPrefix(t *testing.T) {
	tests := []struct {
		name     string
		platform toolchain.Platform
		want     string
	}{
		{
			name:     "linux arm64",
			platform: toolchain.Platform{OS: "linux", Arch: "arm64"},
			want:     "aarch64-linux-gnu-",
		},
		{
			name:     "linux amd64",
			platform: toolchain.Platform{OS: "linux", Arch: "amd64"},
			want:     "x86_64-linux-gnu-",
		},
		{
			name:     "darwin arm64",
			platform: toolchain.Platform{OS: "darwin", Arch: "arm64"},
			want:     "",
		},
		{
			name:     "darwin amd64",
			platform: toolchain.Platform{OS: "darwin", Arch: "amd64"},
			want:     "",
		},
		{
			name:     "windows amd64",
			platform: toolchain.Platform{OS: "windows", Arch: "amd64"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gnuTripletPrefix(tt.platform)
			if got != tt.want {
				t.Errorf("gnuTripletPrefix(%v) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

func TestNewToolchainRejectsUnconfiguredCrossTarget(t *testing.T) {
	target := toolchain.Platform{OS: "darwin", Arch: "arm64"}
	if target == toolchain.HostPlatform() {
		target.Arch = "amd64"
	}
	if target == toolchain.HostPlatform() {
		target = toolchain.Platform{OS: "windows", Arch: "amd64"}
	}

	if _, err := NewToolchain("clang", target); err == nil {
		t.Fatal("expected unconfigured cross target to fail")
	}
	tc, err := NewConfiguredToolchain("clang", target, Config{TargetTriple: "aarch64-apple-darwin"})
	if err != nil {
		t.Fatal(err)
	}
	if tc.CC() != "clang" {
		t.Fatalf("CC() = %q", tc.CC())
	}
}

func TestTryToolchains_ReturnsFirstAvailableCompiler(t *testing.T) {
	// Skip if no compiler available
	var availableCompiler string
	for _, name := range []string{"clang", "gcc"} {
		if _, err := exec.LookPath(name); err == nil {
			availableCompiler = name
			break
		}
	}
	if availableCompiler == "" {
		t.Skip("No C compiler found in PATH, skipping test")
	}

	host := toolchain.HostPlatform()

	// Try with unavailable toolchain first, then available one
	tc, err := TryToolchains([]string{"nonexistent-compiler", availableCompiler}, host)
	if err != nil {
		t.Fatalf("TryToolchains failed: %v", err)
	}

	if tc.Name() != availableCompiler {
		t.Errorf("Name() = %q, want %q", tc.Name(), availableCompiler)
	}
}

func TestTryToolchains_ReportsUnavailableCompilers(t *testing.T) {
	host := toolchain.HostPlatform()

	_, err := TryToolchains([]string{"nonexistent-compiler-1", "nonexistent-compiler-2"}, host)
	if err == nil {
		t.Fatal("TryToolchains should fail when no compiler available")
	}

	// Should mention the list of tried toolchains
	expected := "no available toolchain in [nonexistent-compiler-1 nonexistent-compiler-2]"
	if len(err.Error()) < len(expected) || err.Error()[:len(expected)] != expected {
		t.Errorf("error should start with %q, got %q", expected, err.Error())
	}
}

func TestTryToolchainsForLanguagesSkipsMissingCXX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable test stubs use Unix file modes")
	}
	dir := t.TempDir()
	for _, name := range []string{"clang", "gcc", "g++", "ar"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("CC", "")
	t.Setenv("CXX", "")
	tc, err := TryToolchainsForLanguages([]string{"clang", "gcc"}, toolchain.HostPlatform(), true)
	if err != nil {
		t.Fatal(err)
	}
	if tc.Name() != "gcc" {
		t.Fatalf("selected %q, want gcc with an available C++ driver", tc.Name())
	}
}
