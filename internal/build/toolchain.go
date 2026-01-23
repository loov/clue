package build

import (
	"os"
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
func crossPrefix(target Platform) string {
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
