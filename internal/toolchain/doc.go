// Package toolchain provides the interface and shared utilities for C/C++ compiler toolchains.
//
// The toolchain package abstracts over different compiler families (GCC, Clang, MSVC) with a
// uniform Toolchain interface for compiler discovery, path access, and flag generation. Concrete
// implementations are provided in subpackages.
//
// Key types:
//   - Toolchain: Interface for compiler toolchains (implemented by gcc, clang, msvc subpackages)
//   - Flags: Semantic build flags for compiler and linker flag generation
//   - Platform: Target platform specification (OS, architecture, C++ standard)
//   - CompilerIdentity: Unique identity for a compiler binary (path, mtime, size) for cache keys
//
// Subpackages:
//   - gcc: GCC toolchain implementation
//   - clang: Clang toolchain implementation
//   - msvc: MSVC toolchain implementation
//   - gccish: Shared utilities for GCC-like compilers (GCC, Clang)
//   - all: Auto-detection across all toolchain types
//
// The package also provides response file utilities for platforms with command-line length
// limits (e.g., Windows), automatically using @file.rsp syntax when command lines exceed
// thresholds.
//
// Example:
//
//	tc, err := all.NewToolchain("gcc", toolchain.HostPlatform())
//	if err != nil {
//	    return err
//	}
//	flags := tc.CompilerFlags(toolchain.Flags{Optimize: "fast"})
//	identity, _ := tc.Identity() // for cache keys
package toolchain
