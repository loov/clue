package build

import (
	"strings"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	"github.com/loov/clue/internal/toolchain/clang"
	"github.com/loov/clue/internal/toolchain/gcc"
	"github.com/loov/clue/internal/toolchain/msvc"
)

// Toolchain is the interface for C/C++ compiler toolchains.
type Toolchain = toolchain.Toolchain

// Type aliases for backward compatibility.
// These allow existing code to use build.GCCToolchain, build.ClangToolchain, etc.
type (
	GCCToolchain     = gcc.Toolchain
	ClangToolchain   = clang.Toolchain
	MSVCToolchain    = msvc.Toolchain
	MSVCInstallation = msvc.Installation
)

// NewToolchain creates a toolchain implementation based on the name.
// Delegates to toolchain/all package factory.
func NewToolchain(name string, target toolchain.Platform) (Toolchain, error) {
	return all.NewToolchain(name, target)
}

// TryToolchains tries each toolchain name in order and returns the first
// that is available (binaries exist in PATH). Returns error if none available.
var TryToolchains = all.TryToolchains

// ValidateToolchain validates that all toolchain components exist in PATH.
var ValidateToolchain = toolchain.ValidateToolchain

// FindMSVC discovers the MSVC installation on Windows.
var FindMSVC = msvc.FindMSVC

// crossPrefix returns the GNU triplet prefix for cross-compilation.
// Returns empty string if target matches host (native compilation).
// Kept for tests that use it directly (will be removed in Phase 16).
func crossPrefix(target toolchain.Platform) string {
	host := toolchain.HostPlatform()
	if target.OS == host.OS && target.Arch == host.Arch {
		return ""
	}
	return gnuTripletPrefix(target)
}

// gnuTripletPrefix returns the GNU triplet prefix for a given platform.
// This is the raw mapping without host comparison.
// Kept for tests that use it directly (will be removed in Phase 16).
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

// isCrossCompiler checks if a compiler path contains GNU triplet prefix.
// Used by tests that check cross-compilation detection.
func isCrossCompiler(cc string) bool {
	return strings.Contains(cc, "-linux-") || strings.Contains(cc, "-darwin-")
}
