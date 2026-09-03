// Package all provides factory functions for creating toolchains.
//
// This package aggregates all toolchain implementations (gcc, clang, msvc)
// and provides a unified factory interface for creating toolchains by name.
// It also provides TryToolchains for automatic toolchain discovery.
package all

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
	"github.com/loov/clue/internal/toolchain/gcc"
	"github.com/loov/clue/internal/toolchain/msvc"
)

// NewToolchain creates a toolchain by name for the given target platform.
// Supported names: "gcc", "clang", "msvc"
func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
	return NewConfiguredToolchain(name, target, Config{})
}

// Config provides explicit commands and target information for a toolchain.
type Config struct {
	CC, CXX, AR           string
	TargetTriple, Sysroot string
}

// NewConfiguredToolchain creates a toolchain with optional explicit commands.
func NewConfiguredToolchain(name string, target toolchain.Platform, config Config) (toolchain.Toolchain, error) {
	prefix := crossPrefix(target)
	if target.IsCrossCompile() && prefix == "" && config.TargetTriple == "" && config.CC == "" && config.CXX == "" {
		return nil, fmt.Errorf("cross-compilation from %s to %s requires explicit toolchain.cc/toolchain.cxx or toolchain.targetTriple", toolchain.HostPlatform(), target)
	}

	switch name {
	case "gcc":
		if target.IsCrossCompile() && prefix == "" && config.TargetTriple != "" && config.CC == "" && config.CXX == "" {
			return nil, fmt.Errorf("GCC target %s requires explicit toolchain.cc and toolchain.cxx", target)
		}
		tc := gcc.New(configuredCommand(config.CC, "CC", prefix+"gcc"), configuredCommand(config.CXX, "CXX", prefix+"g++"), configuredArchive(config.AR, prefix+"ar"), target)
		tc.ConfigureTarget(config.TargetTriple, config.Sysroot)
		return tc, nil

	case "clang":
		archiver := prefix + "ar"
		if target.OS == "windows" && prefix == "" {
			archiver = "llvm-ar"
		}
		tc := clang.New(configuredCommand(config.CC, "CC", prefix+"clang"), configuredCommand(config.CXX, "CXX", prefix+"clang++"), configuredArchive(config.AR, archiver), target)
		tc.ConfigureTarget(config.TargetTriple, config.Sysroot)
		return tc, nil

	case "msvc":
		if target.IsCrossCompile() {
			return nil, fmt.Errorf("MSVC cross-compilation from %s to %s is not configured", toolchain.HostPlatform(), target)
		}
		return msvc.FindAndNew(target)

	default:
		return nil, fmt.Errorf("unknown toolchain: %s (supported: gcc, clang, msvc)", name)
	}
}

func configuredCommand(explicit, environment, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return getEnvOr(environment, fallback)
}

func configuredArchive(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return fallback
}

// TryToolchains tries each toolchain name in order and returns the first
// that is available (binaries exist in PATH). Returns error if none available.
func TryToolchains(names []string, target toolchain.Platform) (toolchain.Toolchain, error) {
	return TryToolchainsForLanguages(names, target, false)
}

// TryToolchainsForLanguages also requires a C++ driver when the project uses C++.
func TryToolchainsForLanguages(names []string, target toolchain.Platform, requiresCXX bool) (toolchain.Toolchain, error) {
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
		if requiresCXX {
			if _, err := exec.LookPath(tc.CXX()); err != nil {
				lastErr = fmt.Errorf("c++ compiler not found: %s", tc.CXX())
				continue
			}
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
