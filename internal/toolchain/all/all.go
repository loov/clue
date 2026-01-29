// Package all provides factory functions for creating toolchains.
//
// This package aggregates all toolchain implementations (gcc, clang, msvc)
// and provides a unified factory interface for creating toolchains by name.
// It also provides TryToolchains for automatic toolchain discovery.
package all

import (
	"fmt"
	"os"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
	"github.com/loov/clue/internal/toolchain/gcc"
	"github.com/loov/clue/internal/toolchain/msvc"
)

// NewToolchain creates a toolchain by name for the given target platform.
// Supported names: "gcc", "clang", "msvc"
func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
	prefix := crossPrefix(target)

	switch name {
	case "gcc":
		cc := getEnvOr("CC", prefix+"gcc")
		cxx := getEnvOr("CXX", prefix+"g++")
		ar := prefix + "ar"
		return gcc.New(cc, cxx, ar, target), nil

	case "clang":
		cc := getEnvOr("CC", prefix+"clang")
		cxx := getEnvOr("CXX", prefix+"clang++")
		ar := prefix + "ar"
		return clang.New(cc, cxx, ar, target), nil

	case "msvc":
		return msvc.FindAndNew(target)

	default:
		return nil, fmt.Errorf("unknown toolchain: %s (supported: gcc, clang, msvc)", name)
	}
}

// TryToolchains tries each toolchain name in order and returns the first
// that is available (binaries exist in PATH). Returns error if none available.
func TryToolchains(names []string, target toolchain.Platform) (toolchain.Toolchain, error) {
	var lastErr error
	for _, name := range names {
		tc, err := NewToolchain(name, target)
		if err != nil {
			lastErr = err
			continue
		}

		if err := toolchain.ValidateToolchain(tc); err != nil {
			lastErr = err
			continue
		}

		return tc, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no available toolchain in %v: %w", names, lastErr)
	}
	return nil, fmt.Errorf("no available toolchain in %v", names)
}

// crossPrefix returns the GNU triplet prefix for cross-compilation.
// Returns empty string if target matches host (native compilation).
func crossPrefix(target toolchain.Platform) string {
	host := toolchain.HostPlatform()
	if target.OS == host.OS && target.Arch == host.Arch {
		return ""
	}
	return gnuTripletPrefix(target)
}

// gnuTripletPrefix returns the GNU triplet prefix for a given platform.
// This is the raw mapping without host comparison.
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

// getEnvOr returns the value of an environment variable or a fallback value.
func getEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
