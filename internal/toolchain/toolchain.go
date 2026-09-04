// Package toolchain provides the interface and shared utilities for C/C++ compiler toolchains.
//
// The Toolchain interface abstracts over different compiler families (GCC, Clang, MSVC)
// and provides a uniform API for compiler discovery, path access, and flag generation.
// Concrete implementations are provided in subpackages: gcc, clang, msvc.
package toolchain

import (
	"fmt"
	"os/exec"
)

// Toolchain is the interface for C/C++ compiler toolchains.
// Implementations: gcc, clang, msvc (see subpackages).
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
	CompilerFlags(config Flags) []string
	LinkerFlags(config Flags, sysLibs []string) []string

	// Compiler identity for cache keys
	Identity() (CompilerIdentity, error)
}

// ValidateToolchain validates the tools every C or C++ build needs. The language
// driver itself is checked when a source or link action uses it, so C-only
// projects do not require a C++ compiler.
func ValidateToolchain(tc Toolchain) error {
	if validator, ok := tc.(interface{ Validate() error }); ok {
		return validator.Validate()
	}
	// Validate C compiler
	if _, err := exec.LookPath(tc.CC()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CC())
	}

	// Validate archiver
	if _, err := exec.LookPath(tc.AR()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.AR())
	}

	return nil
}
