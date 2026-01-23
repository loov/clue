package build

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDiscoverToolchain_Native(t *testing.T) {
	host := HostPlatform()

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
			expectedAR:   "ar",
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
			tc, err := DiscoverToolchain(tt.toolchain, host)
			if err != nil {
				t.Fatalf("DiscoverToolchain failed: %v", err)
			}

			if tc.CC != tt.expectedCC {
				t.Errorf("CC = %q, want %q", tc.CC, tt.expectedCC)
			}
			if tc.CXX != tt.expectedCXX {
				t.Errorf("CXX = %q, want %q", tc.CXX, tt.expectedCXX)
			}
			if tc.AR != tt.expectedAR {
				t.Errorf("AR = %q, want %q", tc.AR, tt.expectedAR)
			}
			if tc.Name != tt.expectedName {
				t.Errorf("Name = %q, want %q", tc.Name, tt.expectedName)
			}
		})
	}
}

func TestDiscoverToolchain_CCEnvOverride(t *testing.T) {
	host := HostPlatform()

	t.Setenv("CC", "/custom/path/gcc")

	tc, err := DiscoverToolchain("clang", host)
	if err != nil {
		t.Fatalf("DiscoverToolchain failed: %v", err)
	}

	if tc.CC != "/custom/path/gcc" {
		t.Errorf("CC = %q, want %q", tc.CC, "/custom/path/gcc")
	}

	// CXX should not be overridden
	if tc.CXX != "clang++" {
		t.Errorf("CXX = %q, want %q", tc.CXX, "clang++")
	}
}

func TestDiscoverToolchain_CXXEnvOverride(t *testing.T) {
	host := HostPlatform()

	t.Setenv("CXX", "/custom/path/g++")

	tc, err := DiscoverToolchain("clang", host)
	if err != nil {
		t.Fatalf("DiscoverToolchain failed: %v", err)
	}

	if tc.CXX != "/custom/path/g++" {
		t.Errorf("CXX = %q, want %q", tc.CXX, "/custom/path/g++")
	}

	// CC should not be overridden
	if tc.CC != "clang" {
		t.Errorf("CC = %q, want %q", tc.CC, "clang")
	}
}

func TestDiscoverToolchain_CrossLinuxArm64(t *testing.T) {
	target := Platform{OS: "linux", Arch: "arm64"}
	host := HostPlatform()

	// Skip if we're already on arm64 Linux (not a cross-compile)
	if host.OS == target.OS && host.Arch == target.Arch {
		t.Skip("Skipping cross-compilation test on native platform")
	}

	tests := []struct {
		name         string
		toolchain    string
		expectedCC   string
		expectedCXX  string
		expectedAR   string
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
			tc, err := DiscoverToolchain(tt.toolchain, target)
			if err != nil {
				t.Fatalf("DiscoverToolchain failed: %v", err)
			}

			if tc.CC != tt.expectedCC {
				t.Errorf("CC = %q, want %q", tc.CC, tt.expectedCC)
			}
			if tc.CXX != tt.expectedCXX {
				t.Errorf("CXX = %q, want %q", tc.CXX, tt.expectedCXX)
			}
			if tc.AR != tt.expectedAR {
				t.Errorf("AR = %q, want %q", tc.AR, tt.expectedAR)
			}
		})
	}
}

func TestDiscoverToolchain_CrossLinuxAmd64(t *testing.T) {
	target := Platform{OS: "linux", Arch: "amd64"}

	tests := []struct {
		name         string
		toolchain    string
		expectedCC   string
		expectedCXX  string
		expectedAR   string
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
			tc, err := DiscoverToolchain(tt.toolchain, target)
			if err != nil {
				t.Fatalf("DiscoverToolchain failed: %v", err)
			}

			if tc.CC != tt.expectedCC {
				t.Errorf("CC = %q, want %q", tc.CC, tt.expectedCC)
			}
			if tc.CXX != tt.expectedCXX {
				t.Errorf("CXX = %q, want %q", tc.CXX, tt.expectedCXX)
			}
			if tc.AR != tt.expectedAR {
				t.Errorf("AR = %q, want %q", tc.AR, tt.expectedAR)
			}
		})
	}
}

func TestCrossPrefix(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     string
	}{
		{
			name:     "linux arm64",
			platform: Platform{OS: "linux", Arch: "arm64"},
			want:     "aarch64-linux-gnu-",
		},
		{
			name:     "linux amd64",
			platform: Platform{OS: "linux", Arch: "amd64"},
			want:     "x86_64-linux-gnu-",
		},
		{
			name:     "darwin arm64",
			platform: Platform{OS: "darwin", Arch: "arm64"},
			want:     "",
		},
		{
			name:     "darwin amd64",
			platform: Platform{OS: "darwin", Arch: "amd64"},
			want:     "",
		},
		{
			name:     "unsupported platform",
			platform: Platform{OS: "windows", Arch: "amd64"},
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

func TestValidateToolchain_MissingCompiler(t *testing.T) {
	tc := &Toolchain{
		CC:   "nonexistent-gcc",
		CXX:  "nonexistent-g++",
		AR:   "nonexistent-ar",
		Name: "gcc",
	}

	err := ValidateToolchain(tc)
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

func TestValidateToolchain_RealCompiler(t *testing.T) {
	// Try to find a real compiler
	var compilerPath string
	for _, cmd := range []string{"clang", "gcc", "cc"} {
		if path, err := exec.LookPath(cmd); err == nil {
			compilerPath = path
			break
		}
	}

	if compilerPath == "" {
		t.Skip("No C compiler found in PATH, skipping validation test")
	}

	// Try to find corresponding C++ compiler
	var cxxPath string
	if strings.Contains(compilerPath, "clang") {
		cxxPath, _ = exec.LookPath("clang++")
	} else if strings.Contains(compilerPath, "gcc") {
		cxxPath, _ = exec.LookPath("g++")
	} else {
		cxxPath, _ = exec.LookPath("c++")
	}

	if cxxPath == "" {
		t.Skip("No C++ compiler found in PATH, skipping validation test")
	}

	// Find ar
	arPath, err := exec.LookPath("ar")
	if err != nil {
		t.Skip("No ar found in PATH, skipping validation test")
	}

	tc := &Toolchain{
		CC:   compilerPath,
		CXX:  cxxPath,
		AR:   arPath,
		Name: "test",
	}

	err = ValidateToolchain(tc)
	if err != nil {
		t.Errorf("ValidateToolchain failed for real compiler: %v", err)
	}
}

func TestToolchainString(t *testing.T) {
	tests := []struct {
		name       string
		toolchain  *Toolchain
		wantSuffix string
	}{
		{
			name: "native clang",
			toolchain: &Toolchain{
				CC:   "clang",
				CXX:  "clang++",
				AR:   "ar",
				Name: "clang",
			},
			wantSuffix: "(native)",
		},
		{
			name: "native gcc",
			toolchain: &Toolchain{
				CC:   "gcc",
				CXX:  "g++",
				AR:   "ar",
				Name: "gcc",
			},
			wantSuffix: "(native)",
		},
		{
			name: "cross arm64 gcc",
			toolchain: &Toolchain{
				CC:   "aarch64-linux-gnu-gcc",
				CXX:  "aarch64-linux-gnu-g++",
				AR:   "aarch64-linux-gnu-ar",
				Name: "gcc",
			},
			wantSuffix: "(cross)",
		},
		{
			name: "cross amd64 clang",
			toolchain: &Toolchain{
				CC:   "x86_64-linux-gnu-clang",
				CXX:  "x86_64-linux-gnu-clang++",
				AR:   "x86_64-linux-gnu-ar",
				Name: "clang",
			},
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
