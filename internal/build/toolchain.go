package build

import (
	"fmt"
	"os"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// Toolchain is the interface for C/C++ compiler toolchains
type Toolchain = toolchain.Toolchain

// Compile-time interface implementation checks
var (
	_ Toolchain = (*GCCToolchain)(nil)
	_ Toolchain = (*ClangToolchain)(nil)
	_ Toolchain = (*MSVCToolchain)(nil)
)

// NewToolchain creates a toolchain implementation based on the name
func NewToolchain(name string, target toolchain.Platform) (Toolchain, error) {
	// Get cross-compilation prefix if needed
	prefix := crossPrefix(target)

	// Determine compiler paths with environment variable fallback
	var cc, cxx, ar string

	switch name {
	case "gcc":
		cc = getEnvOr("CC", prefix+"gcc")
		cxx = getEnvOr("CXX", prefix+"g++")
		ar = prefix + "ar"
		return &GCCToolchain{
			cc:     cc,
			cxx:    cxx,
			ar:     ar,
			target: target,
		}, nil

	case "clang":
		cc = getEnvOr("CC", prefix+"clang")
		cxx = getEnvOr("CXX", prefix+"clang++")
		ar = prefix + "ar"
		return &ClangToolchain{
			cc:     cc,
			cxx:    cxx,
			ar:     ar,
			target: target,
		}, nil

	case "msvc":
		// MSVC uses vswhere/vcvarsall discovery, not GNU triplet prefixes
		installation, err := FindMSVC()
		if err != nil {
			return nil, err
		}
		return &MSVCToolchain{
			installation: installation,
			target:       target,
		}, nil

	default:
		return nil, fmt.Errorf("unknown toolchain: %s (supported: gcc, clang, msvc)", name)
	}
}

// crossPrefix returns the GNU triplet prefix for cross-compilation
// Returns empty string if target matches host (native compilation)
func crossPrefix(target toolchain.Platform) string {
	// If target matches host, no prefix needed (native compilation)
	host := toolchain.HostPlatform()
	if target.OS == host.OS && target.Arch == host.Arch {
		return ""
	}

	// Cross-compilation: return GNU triplet prefix based on target
	return gnuTripletPrefix(target)
}

// gnuTripletPrefix returns the GNU triplet prefix for a given platform
// This is the raw mapping without host comparison
func gnuTripletPrefix(target toolchain.Platform) string {
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
var ValidateToolchain = toolchain.ValidateToolchain

// getEnvOr returns the value of an environment variable or a fallback value
func getEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// isCrossCompiler checks if a compiler path contains GNU triplet prefix
func isCrossCompiler(cc string) bool {
	// Check if CC contains a GNU triplet prefix (contains hyphens before the compiler name)
	// Examples: aarch64-linux-gnu-gcc, x86_64-linux-gnu-clang
	return strings.Contains(cc, "-linux-") || strings.Contains(cc, "-darwin-")
}
