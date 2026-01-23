package build

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Toolchain represents a C/C++ compiler toolchain
type Toolchain struct {
	CC   string // C compiler path
	CXX  string // C++ compiler path
	AR   string // Archiver path
	Name string // "clang" or "gcc"
}

// DiscoverToolchain discovers the appropriate toolchain for the given name and target platform
func DiscoverToolchain(name string, target Platform) (*Toolchain, error) {
	tc := &Toolchain{Name: name}

	// Get cross-compilation prefix if needed
	prefix := crossPrefix(target)

	// Check CC environment variable first, fall back to toolchain name
	if cc := os.Getenv("CC"); cc != "" {
		tc.CC = cc
	} else {
		if name == "gcc" {
			tc.CC = prefix + "gcc"
		} else {
			tc.CC = prefix + "clang"
		}
	}

	// Check CXX environment variable first, fall back to toolchain++ variant
	if cxx := os.Getenv("CXX"); cxx != "" {
		tc.CXX = cxx
	} else {
		if name == "gcc" {
			tc.CXX = prefix + "g++"
		} else {
			tc.CXX = prefix + "clang++"
		}
	}

	// AR archiver
	tc.AR = prefix + "ar"

	return tc, nil
}

// crossPrefix returns the GNU triplet prefix for cross-compilation
// Returns empty string if target matches host (native compilation)
func crossPrefix(target Platform) string {
	// If target matches host, no prefix needed (native compilation)
	host := HostPlatform()
	if target.OS == host.OS && target.Arch == host.Arch {
		return ""
	}

	// Cross-compilation: return GNU triplet prefix based on target
	return gnuTripletPrefix(target)
}

// gnuTripletPrefix returns the GNU triplet prefix for a given platform
// This is the raw mapping without host comparison
func gnuTripletPrefix(target Platform) string {
	switch target.String() {
	case "linux-arm64":
		return "aarch64-linux-gnu-"
	case "linux-amd64":
		return "x86_64-linux-gnu-"
	case "darwin-amd64", "darwin-arm64":
		// macOS cross-compilation from Linux deferred to v2
		return ""
	default:
		return ""
	}
}

// ValidateToolchain validates that all toolchain components exist in PATH
func ValidateToolchain(tc *Toolchain) error {
	// Validate C compiler
	if _, err := exec.LookPath(tc.CC); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CC)
	}

	// Validate C++ compiler
	if _, err := exec.LookPath(tc.CXX); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CXX)
	}

	// Validate archiver
	if _, err := exec.LookPath(tc.AR); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.AR)
	}

	return nil
}

// IsCrossCompiler returns true if this toolchain is configured for cross-compilation
func (tc *Toolchain) IsCrossCompiler() bool {
	// Check if CC contains a GNU triplet prefix (contains hyphens before the compiler name)
	// Examples: aarch64-linux-gnu-gcc, x86_64-linux-gnu-clang
	return strings.Contains(tc.CC, "-linux-") || strings.Contains(tc.CC, "-darwin-")
}

// String returns a descriptive string for build output
func (tc *Toolchain) String() string {
	if tc.IsCrossCompiler() {
		// Extract the prefix from CC (everything before the final component)
		// e.g., "aarch64-linux-gnu-gcc" -> "aarch64-linux-gnu-gcc (cross)"
		return fmt.Sprintf("%s (cross)", tc.CC)
	}
	return fmt.Sprintf("%s (native)", tc.Name)
}
