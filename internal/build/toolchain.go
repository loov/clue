package build

import (
	"sort"
	"strings"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	"github.com/loov/clue/internal/toolchain/clang"
	"github.com/loov/clue/internal/toolchain/gcc"
	"github.com/loov/clue/internal/toolchain/msvc"
)

type environmentToolchain interface {
	Environment() map[string]string
}

func toolchainEnvironment(tc Toolchain) []string {
	provider, ok := tc.(environmentToolchain)
	if !ok || provider.Environment() == nil {
		return nil
	}
	environment := provider.Environment()
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+environment[key])
	}
	return result
}

// Toolchain is the interface for C/C++ compiler toolchains.
type Toolchain = toolchain.Toolchain

// Type aliases for internal build package use.
// External callers should import directly from toolchain package.
type (
	Platform = toolchain.Platform
	Config   = toolchain.Config
)

// Function aliases for internal build package use.
// External callers should import directly from toolchain package.
var (
	HostPlatform         = toolchain.HostPlatform
	ParseTarget          = toolchain.ParseTarget
	IsSupportedTarget    = toolchain.IsSupportedTarget
	MaybeUseResponseFile = toolchain.MaybeUseResponseFile
)

// Constant aliases for internal build package use.
const (
	ResponseFileThreshold = toolchain.ResponseFileThreshold
)

// Response file helper functions for internal build package use.
var (
	EstimateCommandLength = toolchain.EstimateCommandLength
	WriteResponseFile     = toolchain.WriteResponseFile
	QuoteResponseFileArg  = toolchain.QuoteResponseFileArg
)

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
