package build

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Toolchain is the interface for C/C++ compiler toolchains
type Toolchain interface {
	// Compiler paths
	CC() string
	CXX() string
	AR() string

	// Toolchain identification
	Name() string
	IsCrossCompiler() bool
	String() string

	// Flag generation
	CompilerFlags(config Config) []string
	LinkerFlags(config Config, sysLibs []string) []string

	// Compiler identity for cache keys
	Identity() (CompilerIdentity, error)
}

// Compile-time interface implementation checks
var (
	_ Toolchain = (*GCCToolchain)(nil)
	_ Toolchain = (*ClangToolchain)(nil)
	_ Toolchain = (*MSVCToolchain)(nil)
)

// NewToolchain creates a toolchain implementation based on the name
func NewToolchain(name string, target Platform) (Toolchain, error) {
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
func ValidateToolchain(tc Toolchain) error {
	// Validate C compiler
	if _, err := exec.LookPath(tc.CC()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CC())
	}

	// Validate C++ compiler
	if _, err := exec.LookPath(tc.CXX()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CXX())
	}

	// Validate archiver
	if _, err := exec.LookPath(tc.AR()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.AR())
	}

	return nil
}

// getEnvOr returns the value of an environment variable or a fallback value
func getEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Flag mapping variables (shared by toolchain implementations)

// Optimization flag mapping
var optimizationFlags = map[string]string{
	"none":       "-O0",
	"size":       "-Os",
	"fast":       "-O2",
	"aggressive": "-O3",
}

// Warning flag mapping
var warningFlags = map[string][]string{
	"off":      {},
	"default":  {"-Wall"},
	"strict":   {"-Wall", "-Wextra"},
	"pedantic": {"-Wall", "-Wextra", "-Wpedantic"},
}

// Debug flag mapping
var debugFlags = map[string]string{
	"none":    "",
	"minimal": "-g1",
	"full":    "-g",
}

// Flag mapping helper functions

// optimizationFlag returns the optimization flag for a given level
func optimizationFlag(level string) string {
	if flag, ok := optimizationFlags[level]; ok {
		return flag
	}
	return ""
}

// warningFlagsForLevel returns warning flags for a given level
func warningFlagsForLevel(level string) []string {
	if flags, ok := warningFlags[level]; ok {
		return flags
	}
	return []string{}
}

// debugFlag returns the debug flag for a given level
func debugFlag(level string) string {
	if flag, ok := debugFlags[level]; ok {
		return flag
	}
	return ""
}

// isCrossCompiler checks if a compiler path contains GNU triplet prefix
func isCrossCompiler(cc string) bool {
	// Check if CC contains a GNU triplet prefix (contains hyphens before the compiler name)
	// Examples: aarch64-linux-gnu-gcc, x86_64-linux-gnu-clang
	return strings.Contains(cc, "-linux-") || strings.Contains(cc, "-darwin-")
}
