package build

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	"github.com/loov/clue/internal/toolchain/clang"
	"github.com/loov/clue/internal/toolchain/gcc"
)

func TestNewToolchain_CreatesNativeCompiler(t *testing.T) {
	host := toolchain.HostPlatform()
	clangAR := "ar"
	if host.OS == "windows" {
		clangAR = "llvm-ar"
	}

	tests := []struct {
		name         string
		toolchain    string
		expectedCC   string
		expectedCXX  string
		expectedAR   string
		expectedName string
	}{
		{
			name:         "clang native",
			toolchain:    "clang",
			expectedCC:   "clang",
			expectedCXX:  "clang++",
			expectedAR:   clangAR,
			expectedName: "clang",
		},
		{
			name:         "gcc native",
			toolchain:    "gcc",
			expectedCC:   "gcc",
			expectedCXX:  "g++",
			expectedAR:   "ar",
			expectedName: "gcc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := newToolchain(tt.toolchain, host)
			if err != nil {
				t.Fatalf("NewToolchain failed: %v", err)
			}

			if tc.CC() != tt.expectedCC {
				t.Errorf("CC() = %q, want %q", tc.CC(), tt.expectedCC)
			}
			if tc.CXX() != tt.expectedCXX {
				t.Errorf("CXX() = %q, want %q", tc.CXX(), tt.expectedCXX)
			}
			if tc.AR() != tt.expectedAR {
				t.Errorf("AR() = %q, want %q", tc.AR(), tt.expectedAR)
			}
			if tc.Name() != tt.expectedName {
				t.Errorf("Name() = %q, want %q", tc.Name(), tt.expectedName)
			}
		})
	}
}

func gnuTripletPrefix(target toolchain.Platform) string {
	switch target.String() {
	case "linux-arm64":
		return "aarch64-linux-gnu-"
	case "linux-amd64":
		return "x86_64-linux-gnu-"
	default:
		return ""
	}
}

func TestNewToolchain_UsesCCEnvironmentOverride(t *testing.T) {
	host := toolchain.HostPlatform()

	t.Setenv("CC", "/custom/path/gcc")

	tc, err := newToolchain("clang", host)
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
}

func TestNewToolchain_UsesCXXEnvironmentOverride(t *testing.T) {
	host := toolchain.HostPlatform()

	t.Setenv("CXX", "/custom/path/g++")

	tc, err := newToolchain("clang", host)
	if err != nil {
		t.Fatalf("NewToolchain failed: %v", err)
	}

	if tc.CXX() != "/custom/path/g++" {
		t.Errorf("CXX() = %q, want %q", tc.CXX(), "/custom/path/g++")
	}

	// CC should not be overridden
	if tc.CC() != "clang" {
		t.Errorf("CC() = %q, want %q", tc.CC(), "clang")
	}
}

func TestNewToolchain_CreatesLinuxARM64CrossCompiler(t *testing.T) {
	target := toolchain.Platform{OS: "linux", Arch: "arm64"}
	host := toolchain.HostPlatform()

	// Skip if we're already on arm64 Linux (not a cross-compile)
	if host.OS == target.OS && host.Arch == target.Arch {
		t.Skip("Skipping cross-compilation test on native platform")
	}

	tests := []struct {
		name        string
		toolchain   string
		expectedCC  string
		expectedCXX string
		expectedAR  string
	}{
		{
			name:        "gcc cross arm64",
			toolchain:   "gcc",
			expectedCC:  "aarch64-linux-gnu-gcc",
			expectedCXX: "aarch64-linux-gnu-g++",
			expectedAR:  "aarch64-linux-gnu-ar",
		},
		{
			name:        "clang cross arm64",
			toolchain:   "clang",
			expectedCC:  "aarch64-linux-gnu-clang",
			expectedCXX: "aarch64-linux-gnu-clang++",
			expectedAR:  "aarch64-linux-gnu-ar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := newToolchain(tt.toolchain, target)
			if err != nil {
				t.Fatalf("NewToolchain failed: %v", err)
			}

			if tc.CC() != tt.expectedCC {
				t.Errorf("CC() = %q, want %q", tc.CC(), tt.expectedCC)
			}
			if tc.CXX() != tt.expectedCXX {
				t.Errorf("CXX() = %q, want %q", tc.CXX(), tt.expectedCXX)
			}
			if tc.AR() != tt.expectedAR {
				t.Errorf("AR() = %q, want %q", tc.AR(), tt.expectedAR)
			}
		})
	}
}

func TestNewToolchain_CreatesLinuxAMD64CrossCompiler(t *testing.T) {
	target := toolchain.Platform{OS: "linux", Arch: "amd64"}
	if toolchain.HostPlatform() == target {
		t.Skip("target is the native platform")
	}

	tests := []struct {
		name        string
		toolchain   string
		expectedCC  string
		expectedCXX string
		expectedAR  string
	}{
		{
			name:        "gcc cross amd64",
			toolchain:   "gcc",
			expectedCC:  "x86_64-linux-gnu-gcc",
			expectedCXX: "x86_64-linux-gnu-g++",
			expectedAR:  "x86_64-linux-gnu-ar",
		},
		{
			name:        "clang cross amd64",
			toolchain:   "clang",
			expectedCC:  "x86_64-linux-gnu-clang",
			expectedCXX: "x86_64-linux-gnu-clang++",
			expectedAR:  "x86_64-linux-gnu-ar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := newToolchain(tt.toolchain, target)
			if err != nil {
				t.Fatalf("NewToolchain failed: %v", err)
			}

			if tc.CC() != tt.expectedCC {
				t.Errorf("CC() = %q, want %q", tc.CC(), tt.expectedCC)
			}
			if tc.CXX() != tt.expectedCXX {
				t.Errorf("CXX() = %q, want %q", tc.CXX(), tt.expectedCXX)
			}
			if tc.AR() != tt.expectedAR {
				t.Errorf("AR() = %q, want %q", tc.AR(), tt.expectedAR)
			}
		})
	}
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
			name:     "unsupported platform",
			platform: toolchain.Platform{OS: "windows", Arch: "amd64"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test gnuTripletPrefix directly (raw mapping without host comparison)
			got := gnuTripletPrefix(tt.platform)
			if got != tt.want {
				t.Errorf("gnuTripletPrefix(%v) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

func TestValidateToolchain_RejectsMissingCompiler(t *testing.T) {
	// Create a test toolchain with nonexistent paths using factory
	tc := gcc.New(
		"nonexistent-gcc",
		"nonexistent-g++",
		"nonexistent-ar",
		toolchain.HostPlatform(),
	)

	err := toolchain.ValidateToolchain(tc)
	if err == nil {
		t.Fatal("ValidateToolchain should fail for nonexistent compiler")
	}

	// Error should mention the compiler name
	if !strings.Contains(err.Error(), "nonexistent-gcc") {
		t.Errorf("error should mention compiler: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say 'not found': %v", err)
	}
}

func TestValidateToolchain_AcceptsAvailableCompiler(t *testing.T) {
	// Try to find a real compiler
	var toolchainName string
	for _, name := range []string{"clang", "gcc"} {
		if _, err := exec.LookPath(name); err == nil {
			toolchainName = name
			break
		}
	}

	if toolchainName == "" {
		t.Skip("No C compiler found in PATH, skipping validation test")
	}

	tc, err := newToolchain(toolchainName, toolchain.HostPlatform())
	if err != nil {
		t.Fatalf("NewToolchain failed: %v", err)
	}

	err = toolchain.ValidateToolchain(tc)
	if err != nil {
		t.Errorf("ValidateToolchain failed for real compiler: %v", err)
	}
}

func TestToolchainString_DescribesCompilerAndTarget(t *testing.T) {
	host := toolchain.HostPlatform()

	tests := []struct {
		name       string
		toolchain  toolchain.Toolchain
		wantSuffix string
	}{
		{
			name:       "native clang",
			toolchain:  clang.New("clang", "clang++", "ar", host),
			wantSuffix: "(native)",
		},
		{
			name:       "native gcc",
			toolchain:  gcc.New("gcc", "g++", "ar", host),
			wantSuffix: "(native)",
		},
		{
			name: "cross arm64 gcc",
			toolchain: gcc.New(
				"aarch64-linux-gnu-gcc",
				"aarch64-linux-gnu-g++",
				"aarch64-linux-gnu-ar",
				toolchain.Platform{OS: "linux", Arch: "arm64"},
			),
			wantSuffix: "(cross)",
		},
		{
			name: "cross amd64 clang",
			toolchain: clang.New(
				"x86_64-linux-gnu-clang",
				"x86_64-linux-gnu-clang++",
				"x86_64-linux-gnu-ar",
				toolchain.Platform{OS: "linux", Arch: "amd64"},
			),
			wantSuffix: "(cross)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.toolchain.String()
			if !strings.Contains(got, tt.wantSuffix) {
				t.Errorf("String() = %q, want to contain %q", got, tt.wantSuffix)
			}
		})
	}
}

// NewToolchain creates a toolchain implementation based on the name.
// Delegates to toolchain/all package factory.
func newToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
	return all.NewToolchain(name, target)
}
